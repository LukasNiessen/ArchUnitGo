package fluentapi_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// errCheckFailed stands in for the technical failure a terminal returns instead of violations — a project
// that will not load, a pattern that will not compile.
var errCheckFailed = errors.New("no go.mod at or above the locator")

func TestLoggedCheckWritesTheThreeRecordsEveryRuleWritesIdentically(t *testing.T) {
	log := &bytes.Buffer{}
	options := &fluentapi.CheckOptions{Logging: &logging.Options{Writer: log, Level: logging.LevelDebug}}
	reported := []assertion.Violation{
		assertion.NewEmptyTestViolation("files", matching.FolderMatcher(mustGlob(t, "internal/apis/**"))),
		assertion.NewEmptyTestViolation("files to depend on"),
	}

	violations, err := options.LoggedCheck(rule("project files, should not, depend on files"),
		func(logger *logging.Logger) ([]assertion.Violation, error) {
			// What is left to a terminal is the part only it knows: the progress of its own pipeline.
			logger.LogProgress("selected files", 12)
			return reported, nil
		})
	if err != nil {
		t.Fatalf("LoggedCheck failed: %v", err)
	}
	if len(violations) != len(reported) {
		t.Fatalf("LoggedCheck returned %v, want the check's own violations %v", violations, reported)
	}
	// The rule, its violations and its outcome, written in one place so that no family gets to log a little
	// differently — and the terminal's own progress record in between, where it wrote it.
	want := strings.Join([]string{
		`info  start check: project files, should not, depend on files`,
		`debug progress: selected files: 12`,
		`warn  violation: empty-test: files: path without filename matches "internal/apis/**" -> nothing`,
		`warn  violation: empty-test: files to depend on -> nothing`,
		`info  end check: project files, should not, depend on files: 2 violations`,
		``,
	}, "\n")
	if log.String() != want {
		t.Errorf("the log holds\n%s\nwant\n%s", log.String(), want)
	}
}

func TestLoggedCheckChangesNothingAboutACheckThatIsNotLogged(t *testing.T) {
	// The default, and the promise the whole design rests on: with no bag, the door is a call to the check
	// and nothing else. The closure still runs, its logger is still usable, and the answer is the same.
	reported := []assertion.Violation{assertion.NewEmptyTestViolation("files")}

	tests := []struct {
		name    string
		options *fluentapi.CheckOptions
	}{
		{name: "nil options", options: nil},
		{name: "no logging bag", options: &fluentapi.CheckOptions{}},
		{name: "a logging bag with no destination", options: &fluentapi.CheckOptions{Logging: &logging.Options{}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ran := false

			violations, err := test.options.LoggedCheck(rule("a rule"),
				func(logger *logging.Logger) ([]assertion.Violation, error) {
					ran = true
					// The logger is never nil, so the closure has no branch to write.
					if logger == nil {
						t.Error("the check was handed a nil logger; it should be handed one that writes nothing")
					}
					logger.LogProgress("selected files", 3)
					return reported, nil
				})

			if !ran {
				t.Error("the check was not run")
			}
			if err != nil {
				t.Fatalf("LoggedCheck failed: %v", err)
			}
			if len(violations) != 1 {
				t.Errorf("LoggedCheck returned %v, want the check's own violations", violations)
			}
		})
	}
}

func TestLoggedCheckLogsAFailingCheckAndStillReturnsTheFailure(t *testing.T) {
	log := &bytes.Buffer{}
	options := &fluentapi.CheckOptions{Logging: &logging.Options{Writer: log}}

	violations, err := options.LoggedCheck(rule("a rule"),
		func(*logging.Logger) ([]assertion.Violation, error) {
			return nil, errCheckFailed
		})

	// A failure this library reports is the error a caller gets, and a log line is never how it is reported.
	if !errors.Is(err, errCheckFailed) {
		t.Fatalf("LoggedCheck error = %v, want the check's own failure", err)
	}
	if violations != nil {
		t.Errorf("LoggedCheck returned %v beside an error, want nothing: the two are mutually exclusive", violations)
	}
	want := "info  start check: a rule\nerror end check: a rule: failed: " + errCheckFailed.Error() + "\n"
	if log.String() != want {
		t.Errorf("the log holds %q, want %q", log.String(), want)
	}
}

func TestLoggedCheckDoesNotRunACheckWhoseLogCouldNotBeOpened(t *testing.T) {
	// A check that cannot open the log it was asked for fails rather than running quietly. The path is a file
	// where a folder has to be.
	occupied := filepath.Join(t.TempDir(), "occupied")
	writeFixtureFile(t, filepath.Dir(occupied), filepath.Base(occupied), "not a folder\n")
	options := &fluentapi.CheckOptions{
		Logging: &logging.Options{File: filepath.Join(occupied, "archunit.log")},
	}
	ran := false

	violations, err := options.LoggedCheck(rule("a rule"),
		func(*logging.Logger) ([]assertion.Violation, error) {
			ran = true
			return nil, nil
		})

	if ran {
		t.Error("the check ran although the log it was asked for could not be opened")
	}
	if violations != nil {
		t.Errorf("LoggedCheck returned %v beside an error, want nothing", violations)
	}
	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("LoggedCheck error = %v, want a *archerror.TechnicalError", err)
	}
}

func TestLoggedCheckReportsALogItCouldNotWrite(t *testing.T) {
	// The disk that filled up half way through a suite: the check itself succeeded, so the truncated artifact
	// nobody was told about is the only thing left to report.
	options := &fluentapi.CheckOptions{Logging: &logging.Options{Writer: failingWriter{}}}

	violations, err := options.LoggedCheck(rule("a rule"),
		func(*logging.Logger) ([]assertion.Violation, error) {
			return []assertion.Violation{assertion.NewEmptyTestViolation("files")}, nil
		})

	if !errors.Is(err, errWriterFull) {
		t.Fatalf("LoggedCheck error = %v, want the writer's own failure", err)
	}
	if violations != nil {
		t.Errorf("LoggedCheck returned %v beside an error, want nothing", violations)
	}
}

func TestLoggedCheckPrefersTheChecksOwnFailureToTheLogs(t *testing.T) {
	// Both went wrong at once. A rule that could not be run is the more useful of the two things to say, so
	// the log's failure loses.
	options := &fluentapi.CheckOptions{Logging: &logging.Options{Writer: failingWriter{}}}

	_, err := options.LoggedCheck(rule("a rule"),
		func(*logging.Logger) ([]assertion.Violation, error) {
			return nil, errCheckFailed
		})

	if !errors.Is(err, errCheckFailed) {
		t.Fatalf("LoggedCheck error = %v, want the check's own failure", err)
	}
	if errors.Is(err, errWriterFull) {
		t.Errorf("LoggedCheck error = %v, want the log's failure to lose to the check's", err)
	}
}

func TestLoggedCheckNamesARuleThatCannotRenderItself(t *testing.T) {
	// A terminal always passes itself, so this is for a caller of the door with no sentence to give — and the
	// records still have to be a pair a reader can match up.
	tests := []struct {
		name string
		rule rule
		want string
	}{
		{name: "a rule that renders itself", rule: "project files, should have no cycles", want: "project files, should have no cycles"},
		{name: "one that renders as nothing", rule: "", want: "a rule"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &bytes.Buffer{}
			options := &fluentapi.CheckOptions{Logging: &logging.Options{Writer: log}}

			if _, err := options.LoggedCheck(test.rule, noViolations); err != nil {
				t.Fatalf("LoggedCheck failed: %v", err)
			}

			want := "info  start check: " + test.want + "\ninfo  end check: " + test.want + ": no violations\n"
			if log.String() != want {
				t.Errorf("the log holds %q, want %q", log.String(), want)
			}
		})
	}

	// And no rule at all, which cannot be spelled as a typed nil in the table above.
	log := &bytes.Buffer{}
	options := &fluentapi.CheckOptions{Logging: &logging.Options{Writer: log}}
	if _, err := options.LoggedCheck(nil, noViolations); err != nil {
		t.Fatalf("LoggedCheck failed: %v", err)
	}
	if want := "info  start check: a rule\ninfo  end check: a rule: no violations\n"; log.String() != want {
		t.Errorf("the log holds %q, want %q", log.String(), want)
	}
}

// TestALogFileHoldsEveryRuleOfASuiteInTheOrderTheyRan is the level above the unit tests: the door as a suite
// goes through it, over the log file a CI job archives. A log opened once per check and appended to is what
// makes a suite's rules one readable artifact, and it is what makes a parallel suite behave.
func TestALogFileHoldsEveryRuleOfASuiteInTheOrderTheyRan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build", "archunit.log")
	options := &fluentapi.CheckOptions{Logging: &logging.Options{File: path}}

	for _, sentence := range []string{"the first rule", "the second rule"} {
		if _, err := options.LoggedCheck(rule(sentence), noViolations); err != nil {
			t.Fatalf("LoggedCheck failed: %v", err)
		}
	}

	want := strings.Join([]string{
		"info  start check: the first rule",
		"info  end check: the first rule: no violations",
		"info  start check: the second rule",
		"info  end check: the second rule: no violations",
		"",
	}, "\n")
	if got := readLogFile(t, path); got != want {
		t.Errorf("the archived log holds\n%s\nwant\n%s", got, want)
	}
}

// noViolations is the passing check, for the tests that are about the records around one rather than about
// what it reported.
func noViolations(*logging.Logger) ([]assertion.Violation, error) {
	return nil, nil
}

// rule is a rule that renders itself as whatever it was built from, standing in for a terminal: the door asks
// a rule for its own sentence and nothing else.
type rule string

func (r rule) String() string {
	return string(r)
}

// failingWriter is the disk that filled up: every write fails, and the door has to report it from the Close
// it does on the caller's behalf.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriterFull
}

// errWriterFull is what that writer reports.
var errWriterFull = errors.New("no space left on device")

// readLogFile is the log a build would archive, read back as the text it is.
func readLogFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the log file %q failed: %v", path, err)
	}
	return string(content)
}
