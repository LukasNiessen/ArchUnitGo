package logging_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// errWriterFull is what a writer that has run out of room reports, standing in for the disk that filled up
// half way through a suite — the one failure the log's own error path exists for.
var errWriterFull = errors.New("no space left on device")

func TestALoggerWithNoDestinationWritesNothing(t *testing.T) {
	// Off by default, which is the whole first sentence of what logging is here: a nil bag, the zero bag and
	// a nil logger all take the five records and write none of them, so a terminal calls them
	// unconditionally.
	tests := []struct {
		name    string
		options *logging.Options
	}{
		{name: "a nil bag", options: nil},
		{name: "the zero bag", options: &logging.Options{}},
		{name: "a bag with a level and no destination", options: &logging.Options{Level: logging.LevelDebug}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := mustLogger(t, test.options)

			writeEveryRecord(logger)

			if err := logger.Close(); err != nil {
				t.Errorf("closing a log with no destination failed: %v", err)
			}
		})
	}

	// And the nil logger, which is what a caller holding one from a failed open would have.
	var absent *logging.Logger
	writeEveryRecord(absent)
	if err := absent.Close(); err != nil {
		t.Errorf("closing a nil logger failed: %v", err)
	}
}

func TestTheFiveRecordsAreTheWholeVocabularyOfALog(t *testing.T) {
	log := &bytes.Buffer{}
	logger := mustLogger(t, &logging.Options{Writer: log, Level: logging.LevelDebug})

	logger.StartCheck(`project files, path without filename matches "internal/api/**", should not, depend on files`)
	logger.LogProgress("selected files", 12)
	logger.LogMetric(measurement("internal/api/handler.go: lines of code = 120"))
	logger.LogViolation(assertion.NewEmptyTestViolation("files", matching.FolderMatcher(mustGlob(t, "internal/apis/**"))))
	logger.EndCheck(`project files, path without filename matches "internal/api/**", should not, depend on files`, 1, nil)
	closeLog(t, logger)

	// The five shapes of line, in the order a check writes them, spelled out in full: the level in its
	// column, the vocabulary word, and what the record has to say. This is the contract a reader of a log
	// learns once, so it is asserted byte for byte.
	want := strings.Join([]string{
		`info  start check: project files, path without filename matches "internal/api/**", should not, depend on files`,
		`debug progress: selected files: 12`,
		`info  metric: internal/api/handler.go: lines of code = 120`,
		`warn  violation: empty-test: files: path without filename matches "internal/apis/**" -> nothing`,
		`info  end check: project files, path without filename matches "internal/api/**", should not, depend on files: 1 violation`,
		``,
	}, "\n")
	if log.String() != want {
		t.Errorf("the log holds\n%s\nwant\n%s", log.String(), want)
	}
}

func TestTheLevelSaysWhichRecordsAreWritten(t *testing.T) {
	tests := []struct {
		name  string
		level logging.Level
		want  []string
	}{
		{
			name:  "debug is the step-by-step and everything above it",
			level: logging.LevelDebug,
			want:  []string{"progress", "start check", "metric", "violation", "end check"},
		},
		{
			name:  "info is the default, and leaves out only the progress",
			level: logging.LevelInfo,
			want:  []string{"start check", "metric", "violation", "end check"},
		},
		{
			name:  "warn is the log of a suite that should be reporting nothing at all",
			level: logging.LevelWarn,
			want:  []string{"violation"},
		},
		{
			name:  "error is the checks that could not be run",
			level: logging.LevelError,
			want:  nil,
		},
		{
			name:  "a threshold above error silences the log, which is a legitimate way to say so",
			level: logging.LevelError + 1,
			want:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &bytes.Buffer{}
			logger := mustLogger(t, &logging.Options{Writer: log, Level: test.level})

			writeEveryRecord(logger)
			closeLog(t, logger)

			if got := recordWords(log.String()); !equalWords(got, test.want) {
				t.Errorf("a log at %v holds the records %v, want %v", test.level, got, test.want)
			}
		})
	}
}

func TestEndCheckSaysWhatCameOfTheRule(t *testing.T) {
	tests := []struct {
		name       string
		violations int
		err        error
		want       string
	}{
		{name: "the pass is a word rather than a bare zero", violations: 0, want: "info  end check: a rule: no violations\n"},
		{name: "one violation is singular", violations: 1, want: "info  end check: a rule: 1 violation\n"},
		{name: "and more than one is not", violations: 3, want: "info  end check: a rule: 3 violations\n"},
		{
			name: "a check that could not be run is the one record at error",
			err:  errors.New("no go.mod at or above the locator"),
			want: "error end check: a rule: failed: no go.mod at or above the locator\n",
		},
		{
			name:       "and a failure is a failure however many violations were gathered before it",
			violations: 2,
			err:        errors.New("no go.mod at or above the locator"),
			want:       "error end check: a rule: failed: no go.mod at or above the locator\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &bytes.Buffer{}
			logger := mustLogger(t, &logging.Options{Writer: log})

			logger.EndCheck("a rule", test.violations, test.err)
			closeLog(t, logger)

			if log.String() != test.want {
				t.Errorf("the log holds %q, want %q", log.String(), test.want)
			}
		})
	}
}

func TestAViolationIsLoggedAsItsKindAndWhateverItSaysAboutItself(t *testing.T) {
	tests := []struct {
		name      string
		violation assertion.Violation
		want      string
	}{
		{
			name:      "a violation that renders itself, which is every one in the library",
			violation: assertion.NewEmptyTestViolation("files"),
			want:      "warn  violation: empty-test: files -> nothing\n",
		},
		{
			name:      "one that does not is logged as its kind rather than as a gap",
			violation: bareViolation{kind: "file-dependency"},
			want:      "warn  violation: file-dependency\n",
		},
		{
			name:      "and one with no kind either names the gap",
			violation: bareViolation{},
			want:      "warn  violation: unknown\n",
		},
		{
			name:      "a nil violation is not a line",
			violation: nil,
			want:      "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &bytes.Buffer{}
			logger := mustLogger(t, &logging.Options{Writer: log})

			logger.LogViolation(test.violation)
			closeLog(t, logger)

			if log.String() != test.want {
				t.Errorf("the log holds %q, want %q", log.String(), test.want)
			}
		})
	}
}

func TestAMetricIsRenderedOnlyWhenItWouldBeWritten(t *testing.T) {
	// A metrics rule measures one number per file, class or package of its scope, so a log turned down past
	// info must not pay for formatting thousands of them. The same holds for a violation.
	counted := &countingStringer{rendered: "internal/api: abstractness = 0.5"}
	log := &bytes.Buffer{}
	logger := mustLogger(t, &logging.Options{Writer: log, Level: logging.LevelWarn})

	logger.LogMetric(counted)
	logger.LogMetric(nil)
	closeLog(t, logger)

	if counted.calls != 0 {
		t.Errorf("the measurement was rendered %d times at warn, want not at all", counted.calls)
	}
	if log.String() != "" {
		t.Errorf("the log holds %q at warn, want no metric record", log.String())
	}

	// And at info it is rendered exactly once, because one record is one line.
	shown := &countingStringer{rendered: "internal/api: abstractness = 0.5"}
	written := &bytes.Buffer{}
	enabled := mustLogger(t, &logging.Options{Writer: written})
	enabled.LogMetric(shown)
	closeLog(t, enabled)

	if shown.calls != 1 {
		t.Errorf("the measurement was rendered %d times, want once", shown.calls)
	}
	if written.String() != "info  metric: internal/api: abstractness = 0.5\n" {
		t.Errorf("the log holds %q, want the measurement's own rendering", written.String())
	}
}

func TestALogFileIsCreatedWithItsFolders(t *testing.T) {
	// The CI half of the issue: a build wants the log as an artifact, and the folder it collects artifacts
	// from is one nobody has created yet when the first check runs.
	folder := filepath.Join(t.TempDir(), "build", "reports")
	path := filepath.Join(folder, "archunit.log")
	logger := mustLogger(t, &logging.Options{File: path})

	logger.StartCheck("a rule")
	closeLog(t, logger)

	if got := readLogFile(t, path); got != "info  start check: a rule\n" {
		t.Errorf("the log file holds %q, want the record the check wrote", got)
	}
	info, err := os.Stat(folder)
	if err != nil {
		t.Fatalf("the folder of the log file was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a folder", folder)
	}
}

func TestALogFileIsOpenedUnderTheStampedName(t *testing.T) {
	// The name a build has to look for is Options.Path, and it is a method so that a test archiving the log
	// does not have to reimplement the insertion to find out what it was called.
	folder := t.TempDir()
	options := &logging.Options{
		File:      filepath.Join(folder, "archunit.log"),
		Timestamp: time.Date(2026, time.August, 15, 9, 30, 0, 0, time.UTC),
	}
	logger := mustLogger(t, options)

	logger.StartCheck("a rule")
	closeLog(t, logger)

	stamped := filepath.Join(folder, "archunit-2026-08-15T09-30-00.log")
	if options.Path() != stamped {
		t.Fatalf("Path() = %q, want %q", options.Path(), stamped)
	}
	if got := readLogFile(t, stamped); got != "info  start check: a rule\n" {
		t.Errorf("the stamped log file holds %q, want the record the check wrote", got)
	}
	if _, err := os.Stat(options.File); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the unstamped filename %q exists as well; the stamp has to name the file, not sit beside it", options.File)
	}
}

func TestALogFileIsAppendedToUnlessOverwriteSaysOtherwise(t *testing.T) {
	tests := []struct {
		name      string
		overwrite bool
		want      string
	}{
		{
			name:      "appending is the default, so a suite's rules all reach one log",
			overwrite: false,
			want:      "info  start check: the first rule\ninfo  start check: the second rule\n",
		},
		{
			name:      "overwrite truncates once per check, so a suite that sets it keeps its last rule alone",
			overwrite: true,
			want:      "info  start check: the second rule\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "archunit.log")
			options := &logging.Options{File: path, Overwrite: test.overwrite}

			// Two checks of one suite, each opening the log the options name, which is what a log file is:
			// opened once per check and closed by it.
			first := mustLogger(t, options)
			first.StartCheck("the first rule")
			closeLog(t, first)
			second := mustLogger(t, options)
			second.StartCheck("the second rule")
			closeLog(t, second)

			if got := readLogFile(t, path); got != test.want {
				t.Errorf("the log file holds %q, want %q", got, test.want)
			}
		})
	}
}

func TestBothDestinationsGetTheSameRecords(t *testing.T) {
	// A run watched by a person in a terminal *and* archived by the build that ran it, which is the reason
	// the two knobs are not exclusive.
	console := &bytes.Buffer{}
	path := filepath.Join(t.TempDir(), "archunit.log")
	logger := mustLogger(t, &logging.Options{Writer: console, File: path})

	logger.StartCheck("a rule")
	logger.EndCheck("a rule", 0, nil)
	closeLog(t, logger)

	want := "info  start check: a rule\ninfo  end check: a rule: no violations\n"
	if console.String() != want {
		t.Errorf("the injected writer holds %q, want %q", console.String(), want)
	}
	if got := readLogFile(t, path); got != want {
		t.Errorf("the log file holds %q, want %q", got, want)
	}
}

func TestALogFileThatCannotBeOpenedIsATechnicalError(t *testing.T) {
	// A check that cannot open the log it was asked for fails rather than running quietly: a log asked for
	// and not delivered makes the run look like it was watched.
	occupied := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(occupied, []byte("not a folder\n"), 0o600); err != nil {
		t.Fatalf("writing the fixture file failed: %v", err)
	}

	logger, err := logging.NewLogger(&logging.Options{File: filepath.Join(occupied, "archunit.log")})

	if logger != nil {
		t.Errorf("NewLogger returned %v beside an error, want no logger", logger)
	}
	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("NewLogger error = %v, want a *archerror.TechnicalError", err)
	}
	if !strings.Contains(err.Error(), "archunit.log") {
		t.Errorf("NewLogger error = %v, want it to name the log file", err)
	}
}

func TestARecordThatCouldNotBeWrittenIsReportedFromClose(t *testing.T) {
	// A log line has nowhere to return an error to, so the first failure is kept until Close can report it.
	// The alternative is a truncated artifact that looks exactly like a run with nothing more to say.
	logger := mustLogger(t, &logging.Options{Writer: failingWriter{}, Level: logging.LevelDebug})

	writeEveryRecord(logger)
	err := logger.Close()

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("Close() = %v, want a *archerror.TechnicalError", err)
	}
	if !errors.Is(err, errWriterFull) {
		t.Errorf("Close() = %v, want it to wrap the writer's own failure", err)
	}
	// Reported once: a closed logger has nothing left to say, and a caller looping over a suite must not see
	// the first rule's disk failure again on the second.
	if again := logger.Close(); again != nil {
		t.Errorf("the second Close() = %v, want nothing left to report", again)
	}
}

func TestCloseIsSafeTwiceAndSilencesTheLog(t *testing.T) {
	log := &bytes.Buffer{}
	logger := mustLogger(t, &logging.Options{Writer: log})

	logger.StartCheck("a rule")
	closeLog(t, logger)
	if err := logger.Close(); err != nil {
		t.Errorf("the second Close() = %v, want nothing left to release", err)
	}

	// A closed logger writes nothing rather than writing into a closed file, which is what makes Close safe
	// to defer beside a caller that logs on the way out.
	logger.EndCheck("a rule", 0, nil)
	if log.String() != "info  start check: a rule\n" {
		t.Errorf("the log holds %q, want only what was written before it was closed", log.String())
	}
}

// TestALogOfAWholeCheckThroughTheKernelsOwnDoor is the level above the unit tests: the records a real check
// writes, in the order LoggedCheck writes them, over a file destination a build would archive. It is the
// shape of a log a reader actually opens — start, progress, the numbers, the violations, the outcome.
func TestALogOfAWholeCheckThroughTheKernelsOwnDoor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build", "archunit.log")
	console := &bytes.Buffer{}
	logger := mustLogger(t, &logging.Options{Writer: console, File: path, Level: logging.LevelDebug})

	// What a terminal does with the logger it is handed, and what the kernel does around it.
	sentence := `project files, path without filename matches "internal/api/**", should, have no cycles`
	logger.StartCheck(sentence)
	logger.LogProgress("selected files", 4)
	logger.LogProgress("dependencies between the selected files", 6)
	logger.LogProgress("cycles", 1)
	violation := assertion.NewEmptyTestViolation("files", matching.FolderMatcher(mustGlob(t, "internal/api/**")))
	logger.LogViolation(violation)
	logger.EndCheck(sentence, 1, nil)
	closeLog(t, logger)

	want := strings.Join([]string{
		`info  start check: ` + sentence,
		`debug progress: selected files: 4`,
		`debug progress: dependencies between the selected files: 6`,
		`debug progress: cycles: 1`,
		`warn  violation: empty-test: files: path without filename matches "internal/api/**" -> nothing`,
		`info  end check: ` + sentence + `: 1 violation`,
		``,
	}, "\n")
	if got := readLogFile(t, path); got != want {
		t.Errorf("the archived log holds\n%s\nwant\n%s", got, want)
	}
	if console.String() != want {
		t.Errorf("the console log holds\n%s\nwant\n%s", console.String(), want)
	}
}

// writeEveryRecord writes one of each of the five records, so that a test about the level or the destination
// says which of them reached the log rather than restating the vocabulary.
func writeEveryRecord(logger *logging.Logger) {
	logger.LogProgress("selected files", 3)
	logger.StartCheck("a rule")
	logger.LogMetric(measurement("internal/api: abstractness = 0.5"))
	logger.LogViolation(assertion.NewEmptyTestViolation("files"))
	logger.EndCheck("a rule", 1, nil)
}

// recordWords are the vocabulary words a log's lines were written under, in the order they were written: what
// a test about the level asserts on, without restating what each record says.
func recordWords(log string) []string {
	var words []string
	for _, line := range strings.Split(strings.TrimSuffix(log, "\n"), "\n") {
		if line == "" {
			continue
		}
		_, record, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		word, _, _ := strings.Cut(strings.TrimSpace(record), ":")
		words = append(words, word)
	}
	return words
}

// equalWords compares two lists of vocabulary words as sets, because the order the five records are written
// in is what TestTheFiveRecordsAreTheWholeVocabularyOfALog asserts and not what a level test is about.
func equalWords(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	remaining := make(map[string]int, len(want))
	for _, word := range want {
		remaining[word]++
	}
	for _, word := range got {
		if remaining[word] == 0 {
			return false
		}
		remaining[word]--
	}
	return true
}

// mustLogger opens the log these options describe, failing the test when it cannot: every test here is about
// what a working log holds, except the one that is about the open failing.
func mustLogger(t *testing.T, options *logging.Options) *logging.Logger {
	t.Helper()

	logger, err := logging.NewLogger(options)
	if err != nil {
		t.Fatalf("NewLogger(%+v) failed: %v", options, err)
	}
	return logger
}

// closeLog closes the log and fails the test when anything was left unreported, which is what a caller of
// LoggedCheck does with the error Close returns.
func closeLog(t *testing.T, logger *logging.Logger) {
	t.Helper()

	if err := logger.Close(); err != nil {
		t.Fatalf("closing the log failed: %v", err)
	}
}

// readLogFile is the log a build would archive, read back as the text it is.
func readLogFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the log file %q failed: %v", path, err)
	}
	return string(content)
}

func mustGlob(t *testing.T, glob string) matching.Pattern {
	t.Helper()

	pattern, err := matching.NewGlobPattern(glob, nil)
	if err != nil {
		t.Fatalf("NewGlobPattern(%q, nil) failed: %v", glob, err)
	}
	return pattern
}

// measurement stands in for calculation.Measurement, which this package must not import: a metric record is
// whatever renders itself, and that is the whole of what the logger asks of it.
type measurement string

func (m measurement) String() string {
	return string(m)
}

// countingStringer counts how often it was rendered, which is how the tests hold the logger to rendering a
// measurement only when the level would let the line be written.
type countingStringer struct {
	rendered string
	calls    int
}

func (c *countingStringer) String() string {
	c.calls++
	return c.rendered
}

// bareViolation is a violation that does not render itself, which no type in the library is today: it is here
// because a log line has to name the gap rather than be an empty line if one ever is.
type bareViolation struct {
	kind assertion.ViolationKind
}

func (v bareViolation) Kind() assertion.ViolationKind {
	return v.kind
}

// failingWriter is the disk that filled up: every write fails, and the log has to remember the first failure
// until Close can report it.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriterFull
}
