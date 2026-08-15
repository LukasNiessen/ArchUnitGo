// Package logging is the log a check writes while it runs: off by default, turned on per check through
// fluentapi.CheckOptions.Logging, and never through anything global.
//
// A rule is a value and running it is the only thing that does work, so the log is the only way to see
// what that work was. It answers the question a surprising result always raises — which files did the
// scope actually come to, how many dependencies were projected, what exactly did the rule report — and
// it answers it for a run that has already finished, in a CI job nobody was watching:
//
//	log := &logging.Options{Writer: os.Stderr, Level: logging.LevelDebug, File: "build/archunit.log"}
//	violations, err := rule.Check(&fluentapi.CheckOptions{Logging: log})
//
// Two decisions shape the whole package. The destination is injected per check rather than owned by the
// library, so nothing is logged until a rule is handed a bag that says where to write; and the vocabulary
// is fixed at five records — start check, end check, log progress, log violation, log metric — so that a
// log has the same five shapes of line whatever family of rule wrote it, and a reader who has learned one
// has learned all of them. Level says which of them are written.
//
// Nothing here is a channel for reporting a failure. A technical failure is the error Check returns and a
// broken rule is a violation in the list it returns; both are logged as well, because a log of a run is
// worth having, but the log is never where they are reported.
//
// The package reads no clock. A record carries its level and its words and no timestamp, so two runs of
// the same check over the same code write the same bytes and a test can assert on them; the one instant a
// log can state is Options.Timestamp, and it names the file rather than the lines in it.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

const (
	// logFilePermissions are the mode a log file is created with: readable by anyone, writable by its
	// owner. It is the mode an exported report is written with, for the same reason — a log kept as a
	// build artifact is meant to be read by whatever collects it.
	logFilePermissions = 0o644
	// logFolderPermissions are the mode a folder created for a log file is given, which is the file's mode
	// plus the traversal bit a folder is useless without.
	logFolderPermissions = 0o755
)

const (
	// startCheckRecord and the four below it are the fixed vocabulary, one word per shape of record, and
	// every line a log holds names the one that wrote it. They are spelled here rather than at the five
	// methods so that the whole vocabulary can be read at once — and so that it is visibly a closed list,
	// which is the point of it.
	startCheckRecord = "start check"
	endCheckRecord   = "end check"
	progressRecord   = "progress"
	violationRecord  = "violation"
	metricRecord     = "metric"
)

// recordFormat is the one shape every line in a log has: the level, padded so that the words after it
// line up in a column, then the vocabulary word that wrote the line, then what it has to say.
//
// One format string, in one place, for the same reason the testing layer builds every message in one
// place: a log whose lines are assembled at five call sites is a log whose columns disagree, and column
// alignment is most of what makes a log file readable at all.
const recordFormat = "%-5s %s: %s\n"

// Logger is an open log: somewhere to write, a threshold, and the five records a check can write. It is
// what NewLogger returns and what CheckOptions.Logger hands a terminal.
//
// A nil *Logger is a working logger that writes nothing, and so is one built from options with no
// destination in them. That is what keeps logging out of the shape of the code that logs: a terminal
// calls the five methods unconditionally, and whether anything is written is a decision made once, here.
//
// Every record is one Write of one line. A log file is opened for appending, so two checks running in
// parallel share a file without cutting into each other's lines; an injected Writer shared by parallel
// checks has to be safe for concurrent use itself, which an *os.File is and a bytes.Buffer is not. A
// single Logger belongs to the check it was made for and is not meant to be used from two goroutines at
// once — the library makes one per check, which is what makes a parallel suite behave.
//
// The zero value writes nothing and needs no closing, but a Logger with a file behind it does: Close is
// what flushes and releases it, and it is also where a write that failed is finally reported.
type Logger struct {
	// writer is where records go — the injected writer, the log file, or both — and nil means this logger
	// writes nothing at all, which is the default and the state Close leaves it in.
	writer io.Writer
	// level is the quietest record that is written. It is read from the options once, so a bag a caller
	// changes underneath a running check does not change what that check logs half way through.
	level Level
	// file is the log file this logger opened, and nil when no file was asked for. It is kept so that
	// Close can release it and name it in a failure.
	file *os.File
	// err is the first failure writing a record, kept until Close can report it. A log line has nowhere to
	// return an error to, and the alternative to remembering it is a truncated artifact nobody is told
	// about.
	err error
}

// NewLogger opens the log these options describe: the injected writer, a log file, both, or — for a nil
// bag, and for one with no destination in it — nothing at all.
//
// The returned logger is never nil, so a caller has no branch to write, and it always has to be closed:
//
//	log, err := logging.NewLogger(options)
//	if err != nil {
//		return nil, err
//	}
//	defer log.Close()
//
// The file is created if it is not there, its folders are created with it, and it is opened for appending
// unless Overwrite says to start it fresh — which truncates it once, here, as it is opened. The name it is
// opened under is Options.Path, so a timestamp on the bag lands in the filename.
//
// The error is a TechnicalError naming the log file: a folder that cannot be created, a path that is not
// writable. It is the environment failing rather than the rule being wrong, and a check that cannot open
// the log it was asked for fails instead of running quietly — a log asked for and not delivered is worse
// than no log, because the run looks like it was watched.
func NewLogger(options *Options) (*Logger, error) {
	resolved := options.WithDefaults()
	logger := &Logger{level: resolved.Level}

	destinations := make([]io.Writer, 0, 2)
	if resolved.Writer != nil {
		destinations = append(destinations, resolved.Writer)
	}
	if path := resolved.Path(); path != "" {
		file, err := openLogFile(path, resolved.Overwrite)
		if err != nil {
			return nil, err
		}
		logger.file = file
		destinations = append(destinations, file)
	}
	if len(destinations) > 0 {
		// Through a MultiWriter even for a single destination, so that one writer and two behave
		// identically — including the short write an io.Writer is allowed to report, which MultiWriter
		// turns into the error Close reports.
		logger.writer = io.MultiWriter(destinations...)
	}
	return logger, nil
}

// StartCheck records that a rule is about to be run, at info: `start check: <the rule's own sentence>`.
//
// The sentence is what the rule renders itself as — `project files, path without filename matches
// "internal/api/**", should not, depend on files, ...` — because that is the one string that says which
// rule this is. It is on this record and on the matching end check, and not on the records between them,
// so that a log read by a person is readable and a log written by several checks at once can still be
// read as pairs.
func (l *Logger) StartCheck(rule string) {
	l.record(LevelInfo, startCheckRecord, rule)
}

// EndCheck records that a rule has been run and what came of it: at info, `end check: <the rule>: 2
// violations`, and at error, `end check: <the rule>: failed: <the error>` for a check that could not reach
// the point of judging anything.
//
// The two outcomes are one record because they are one event, and a log with a start check and no end
// check would be a check that never returned. The error is logged and still returned: a failure this
// library reports is the error a caller gets, and a log line is never how it is reported.
//
// The count is the length of the violation list, so `no violations` is the pass. It is spelled out rather
// than left as a bare number because that word is what a reader is looking for.
func (l *Logger) EndCheck(rule string, violations int, err error) {
	if err != nil {
		l.record(LevelError, endCheckRecord, rule+": failed: "+err.Error())
		return
	}
	l.record(LevelInfo, endCheckRecord, rule+": "+countViolations(violations))
}

// LogProgress records one step a check took and how much of the project it came to, at debug: `progress:
// selected files: 12`.
//
// A step and a count, rather than a message the caller assembles, because every step of a check resolves
// something and the useful thing about it is how big that something turned out to be — a scope that
// selected 0 files, or 4000, is the answer to almost every question a log is opened for. It also keeps the
// numbers in a column and out of nine call sites' formatting.
func (l *Logger) LogProgress(step string, count int) {
	l.record(LevelDebug, progressRecord, step+": "+strconv.Itoa(count))
}

// LogViolation records one place the code disagreed with the rule, at warn: `violation: file-dependency:
// internal/api/handler.go: should not, depend on files, ... -> internal/db/pool.go`.
//
// The kind comes first, because it is the one thing every violation carries and the thing a log is
// grepped by. After it is whatever the violation says about itself: every violation type in the library
// renders itself for a log line, and one that does not is logged as its kind alone rather than as a gap.
//
// The user-facing message is still the testing layer's to build, from the violation's own data. A log line
// is not a report — it is the record that this violation was found while the check was running, and it is
// written before the caller has decided what to do with the list.
func (l *Logger) LogViolation(violation assertion.Violation) {
	if !l.enabled(LevelWarn) || violation == nil {
		return
	}
	l.record(LevelWarn, violationRecord, describeViolation(violation))
}

// LogMetric records one number a metrics rule measured, at info: `metric: internal/api/handler.go: lines
// of code = 120`.
//
// It takes the measurement as something that renders itself, and renders it only when the level allows:
// a metrics rule measures one number per file, class or package of its scope, and a log turned down to
// warn should not pay for formatting thousands of them. calculation.Measurement is what the library passes
// here, and its String is what the line holds.
//
// Numbers are info and not debug because they are what a metrics rule is about. A threshold that reports
// nothing is still worth a log of the numbers it held: that is how a limit gets set to something other
// than a guess.
func (l *Logger) LogMetric(measurement fmt.Stringer) {
	if !l.enabled(LevelInfo) || measurement == nil {
		return
	}
	l.record(LevelInfo, metricRecord, measurement.String())
}

// Close releases the log file, if this logger opened one, and reports the first failure the log ran into
// — a record that could not be written, or the file that would not close.
//
// It is safe to call on a nil *Logger, safe to call on a logger with no file behind it, and safe to call
// twice: the second call has nothing left to release and reports nothing. A closed logger writes nothing
// afterwards rather than writing into a closed file.
//
// The error is a TechnicalError, and the one it exists for is the disk that filled up half way through a
// suite: a truncated artifact that nobody was told about looks exactly like a run that had nothing more to
// say. It is the environment failing, never a rule.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	failure := l.err
	l.err = nil
	l.writer = nil
	if l.file != nil {
		if err := l.file.Close(); err != nil && failure == nil {
			failure = archerror.NewTechnicalError("close the log file", l.file.Name(), err)
		}
		l.file = nil
	}
	return failure
}

// enabled reports whether a record at this level would be written at all: a logger with somewhere to write
// and a threshold this level reaches. It is the guard the two records that have to render something first
// ask before rendering it, and it tolerates a nil receiver like every other method here.
func (l *Logger) enabled(level Level) bool {
	return l != nil && l.writer != nil && level >= l.level
}

// record writes one line of the log, and it is the only place in the library that writes one: the level
// padded into its column, the vocabulary word, and what the record has to say.
//
// A failure is remembered rather than returned, for the reason the err field gives. Everything after the
// first failure is still attempted, because a writer that refused one line may well take the next — and
// because dropping the rest would turn a full disk into a log that stops without saying why.
func (l *Logger) record(level Level, word, message string) {
	if !l.enabled(level) {
		return
	}
	line := fmt.Sprintf(recordFormat, level, word, message)
	if _, err := io.WriteString(l.writer, line); err != nil && l.err == nil {
		l.err = archerror.NewTechnicalError("write to the log", word, err)
	}
}

// openLogFile is the file destination: the folders of the path created if they are not there yet, and the
// file opened for appending — truncated first when the caller asked for a fresh one.
//
// Appending rather than seeking is what lets two checks that run in parallel write into one log file:
// every record is one Write to a handle opened for appending, so the operating system puts whole lines
// after each other instead of letting two checks interleave bytes at a shared offset.
func openLogFile(path string, overwrite bool) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), logFolderPermissions); err != nil {
		return nil, archerror.NewTechnicalError("create the folder of the log file", path, err)
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if overwrite {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, logFilePermissions)
	if err != nil {
		return nil, archerror.NewTechnicalError("open the log file", path, err)
	}
	return file, nil
}

// describeViolation is what a violation record holds: the violation's kind, and then whatever the
// violation says about itself.
//
// A violation carries data rather than prose, and the prose a user reads is the testing layer's. What is
// left for a log is the kind — machine-readable, the same word in every port — and the type's own String,
// which every violation in the library has for exactly this. A violation with neither is logged as
// `unknown`, naming the gap instead of writing an empty line.
func describeViolation(violation assertion.Violation) string {
	kind := string(violation.Kind())
	if kind == "" {
		kind = "unknown"
	}
	described, ok := violation.(fmt.Stringer)
	if !ok {
		return kind
	}
	return kind + ": " + described.String()
}

// countViolations is how an end check record states the outcome: `no violations`, `1 violation`, `2
// violations`. The word is what a reader looks for, and a count of zero is the pass rather than a missing
// number.
func countViolations(violations int) string {
	switch violations {
	case 0:
		return "no violations"
	case 1:
		return "1 violation"
	default:
		return strconv.Itoa(violations) + " violations"
	}
}
