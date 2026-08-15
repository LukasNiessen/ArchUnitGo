package logging

import (
	"io"
	"path/filepath"
	"strings"
	"time"
)

// filenameStamp is how a timestamped log filename spells the instant it was opened at:
// `2026-08-15T09-30-00`, sortable and legal on every platform this library builds for.
//
// It is RFC 3339 with the colons replaced by hyphens, and the colons are the whole reason it is written
// out here instead of reusing time.RFC3339: a colon is not a legal character in a filename on Windows,
// so a log file named after the standard would be a log file that cannot be created on half the
// platforms the library is built for. The offset is left off for the same reason — `+02:00` carries one
// too — so the location the caller's own time.Time carries decides what the stamp reads as, and a run
// that wants an unambiguous one passes a UTC time.
const filenameStamp = "2006-01-02T15-04-05"

// Options is what a check's log is: where it goes, how much of it is written, and — when a run wants to
// keep it — the file it is archived as. It is the bag CheckOptions.Logging holds, and it is the whole of
// how logging is turned on.
//
// Logging is off by default and there is no way to turn it on globally, which is the one thing worth
// knowing about this type. A library that owned a process-global logger would log for every test in a
// suite the moment one of them asked, and a test that asserts on a log could then not be run beside
// anything else; so the destination is injected per check, a nil *Options logs nothing, and a bag with
// no destination in it logs nothing either.
//
// Every default is a zero value, so a nil bag, the zero bag and an explicitly empty one all describe the
// same silence. Read one through WithDefaults and Path rather than reaching for a field, and every method
// takes a pointer receiver for the reason fluentapi.CheckOptions' do — a nil-safe read has to be one, and
// a type with both kinds of receiver is a finding.
//
// A log record carries no timestamp of its own. The library reads no clock, so two runs of the same
// check over the same code write the same bytes, which is what lets a test assert on a log at all; the
// one instant a log can state is the one the caller passes in Timestamp, and it names the file rather
// than the lines in it.
type Options struct {
	// Writer is where the log is written, and nil — the default — means nowhere.
	//
	// Any io.Writer will do: os.Stderr for a run watched by a person, a log/slog handler's writer for a
	// suite that already has one, a bytes.Buffer for a test that asserts on what a check said. It is
	// injected rather than configured because a library must not own a process-global logger.
	//
	// A writer shared by checks that run in parallel has to be safe for concurrent use — an *os.File is,
	// a bytes.Buffer is not. File below is the destination that needs no such care: it is opened per
	// check and appended to, so two checks writing one log file interleave whole records.
	Writer io.Writer
	// Level is the quietest record worth writing, and the zero value is LevelInfo: the rule, its outcome,
	// its numbers and its violations, without the step-by-step. LevelDebug adds the progress records.
	Level Level
	// File is the path of a log file to write, and the empty string — the default — means no file.
	//
	// It exists for CI: a check's log is worth archiving as a build artifact, and a build that only has
	// the console has nothing to attach. The path is relative to the working directory the test runs in,
	// its folders are created if they are not there yet, and the file is appended to unless Overwrite
	// says otherwise — so a suite's rules all write into one log in the order they ran.
	//
	// Both destinations may be set at once, and a run watched by a person in a terminal *and* archived by
	// the build that ran it is exactly why: the same records go to both.
	File string
	// Timestamp is the instant the log file names itself after, and the zero time — the default — means
	// the filename is left as it was written.
	//
	// A stamp is what makes a log file a per-run artifact: `build/archunit.log` with the time of the run
	// in it is `build/archunit-2026-08-15T09-30-00.log`, one file per build instead of one file every
	// build appends to forever. Path is where the insertion is spelled out.
	//
	// The time is a field rather than a clock the library reads, for the reason
	// rendering.ReportOptions.Timestamp is: a library that stamped itself could not be asserted on, and
	// this repository's own linter forbids the call. A caller who wants the stamp passes time.Now() and
	// owns that decision.
	Timestamp time.Time
	// Overwrite starts the log file fresh instead of appending to what is already in it. False — the
	// default — appends.
	//
	// Appending is the default because a log file is opened once per check: a suite of twenty rules under
	// one filename appends twenty checks' records into the log a reader wants, where truncating would
	// leave the twentieth rule's alone. Overwrite is for the run that owns its log file — one check, or a
	// fixed filename that should hold this run and not the last three — and it truncates once per check,
	// so a suite that sets it keeps only its last rule's records.
	//
	// A timestamped filename is the other way to get a log that holds one run, and the two compose: a
	// stamp gives every run its own file, and appending fills it with every rule of that run.
	Overwrite bool
}

// WithDefaults returns the options a log should actually be written with: a copy of the receiver, or the
// defaults when the receiver is nil. NewLogger starts with this, so that the "nil means defaults"
// contract is honored in one place instead of being re-derived as a nil check per field.
//
// Nothing here is a slice or a map, so the struct copy is the whole of it. The writer is copied as the
// interface value it is, which is the point: the log is written to the caller's own destination and not
// to a copy of it.
func (o *Options) WithDefaults() Options {
	if o == nil {
		return Options{}
	}
	return *o
}

// Path is the log file these options write, with the timestamp inserted before the extension —
// `build/archunit.log` taken at nine thirty is `build/archunit-2026-08-15T09-30-00.log` — and the empty
// string when no file was asked for, which is the default.
//
// The stamp goes before the extension and not after it, so that the file is still opened by whatever
// opens a `.log`, and so that a build's output can be swept with one `archunit-*.log`. A path with no
// extension gets the stamp at the end of it.
//
// It is a method on the options rather than something NewLogger works out privately because it is the
// one part of the file destination a caller has to be able to see: a test that archives a log needs to
// know which name to look for, and it should not have to reimplement the insertion to find out.
func (o *Options) Path() string {
	resolved := o.WithDefaults()
	if resolved.File == "" || resolved.Timestamp.IsZero() {
		return resolved.File
	}
	extension := filepath.Ext(resolved.File)
	return strings.TrimSuffix(resolved.File, extension) + "-" + resolved.Timestamp.Format(filenameStamp) + extension
}
