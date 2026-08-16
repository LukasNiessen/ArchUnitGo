package archtest_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
)

// The handles a framework hands a test satisfy the helper's interface, both the stdlib's own and one written
// by hand, which is the whole of what "framework-agnostic" means here. Checked at compile time, because an
// interface that had grown a method *testing.T does not have would fail nowhere else.
var (
	_ archtest.TestingT = (*testing.T)(nil)
	_ archtest.TestingT = (*testing.B)(nil)
	_ archtest.TestingT = testing.TB(nil)
	_ archtest.TestingT = (*recorder)(nil)
	_ archtest.TestingT = (*bareRecorder)(nil)
)

func TestARuleThatHoldsIsReportedNowhereAtAll(t *testing.T) {
	// A passing assertion says nothing: a suite that printed its successes would bury the one failure someone
	// has to read. The pass is the empty violation list, as it is everywhere else in the library.
	framework := &recorder{}
	rule := &stubRule{sentence: "project files, should, have no cycles"}

	archtest.AssertPasses(framework, rule, nil)

	if failures := framework.failures(t); len(failures) != 0 {
		t.Errorf("a rule that holds reported %v, want nothing", failures)
	}
	if rule.checks != 1 {
		t.Errorf("the rule was checked %d times, want exactly once", rule.checks)
	}
}

func TestARuleThatDoesNotHoldIsOneFailureCarryingTheWholeReport(t *testing.T) {
	// One Error call, not one per violation: a framework counts failures, and a rule that names forty files is
	// one thing that is wrong. The rule's own sentence heads the report, and under it is what ResultFactory
	// shaped — so this layer has one idea of how a failure reads, whoever prints it.
	framework := &recorder{}
	rule := &stubRule{
		sentence:   `project files, in folder "common/matching", should, filename matches "regex_factory.go"`,
		violations: namingViolations(t, "common/matching/filter.go", "common/matching/match_target.go"),
	}

	archtest.AssertPasses(framework, rule, nil)

	want := `project files, in folder "common/matching", should, filename matches "regex_factory.go"` + "\n" +
		"2 violations:\n" +
		`  1. common/matching/filter.go: should, filename matches "*_service.go"; it does not` + "\n" +
		`  2. common/matching/match_target.go: should, filename matches "*_service.go"; it does not`
	failures := framework.failures(t)
	if len(failures) != 1 {
		t.Fatalf("a rule that does not hold reported %d failures, want the one report:\n%v", len(failures), failures)
	}
	if failures[0] != want {
		t.Errorf("the failure reads\n%s\nwant\n%s", failures[0], want)
	}
}

func TestTheFailureIsAttributedToTheTestThatAssertedIt(t *testing.T) {
	// Helper() is what puts the file and line of the user's own AssertPasses call on the failure instead of a
	// line inside this file, and it is called whether or not the rule holds — a helper that only marked itself
	// on the way to a failure would be one whose passing calls a framework still counts as frames of its own.
	for _, violations := range [][]kernel.Violation{nil, namingViolations(t, "a.go")} {
		framework := &recorder{}

		archtest.AssertPasses(framework, &stubRule{violations: violations}, nil)

		if framework.helpers != 1 {
			t.Errorf("with %d violations the helper marked itself %d times, want exactly once", len(violations), framework.helpers)
		}
	}
}

func TestAFrameworkWithoutHelperStillGetsTheReport(t *testing.T) {
	// The framework-agnostic promise held to the word: Helper() is a convenience the stdlib has and a smaller
	// framework may not, so it is asked for rather than required. Error is the whole interface, and a handle
	// with nothing but Error reports exactly what the stdlib's does.
	framework := &bareRecorder{}
	rule := &stubRule{sentence: "project files, should, have no cycles", violations: namingViolations(t, "a.go")}

	archtest.AssertPasses(framework, rule, nil)

	stdlibShaped := &recorder{}
	archtest.AssertPasses(stdlibShaped, &stubRule{
		sentence:   "project files, should, have no cycles",
		violations: namingViolations(t, "a.go"),
	}, nil)

	failures := framework.failures(t)
	if len(failures) != 1 {
		t.Fatalf("a handle with nothing but Error reported %d failures, want the one report:\n%v", len(failures), failures)
	}
	if want := stdlibShaped.failures(t); failures[0] != want[0] {
		t.Errorf("it reads\n%s\nwant what a handle with Helper() gets\n%s", failures[0], want[0])
	}
}

func TestATechnicalFailureOfTheCheckIsReportedAsOneRatherThanAsAViolation(t *testing.T) {
	// A project that will not load and a rule that does not hold are different problems for whoever reads the
	// output, and the words say which. The violations are not reported at all: Check's contract is that they
	// say nothing when the error is non-nil, so naming a file here would be inventing an architecture mistake.
	framework := &recorder{}
	rule := &stubRule{
		sentence:   "project files, should, have no cycles",
		violations: namingViolations(t, "invented.go"),
		err:        errors.New("no go.mod at or above /tmp/nowhere"),
	}

	archtest.AssertPasses(framework, rule, nil)

	failures := framework.failures(t)
	if len(failures) != 1 {
		t.Fatalf("a check that failed reported %d failures, want the one:\n%v", len(failures), failures)
	}
	want := "the rule could not be checked: no go.mod at or above /tmp/nowhere"
	if failures[0] != want {
		t.Errorf("the failure reads\n%s\nwant\n%s", failures[0], want)
	}
	if strings.Contains(failures[0], "invented.go") {
		t.Errorf("the failure reads\n%s\nwant nothing said about the violations the failed check returned", failures[0])
	}
}

func TestANilRuleIsReportedRatherThanCrashingTheHostTest(t *testing.T) {
	// A mistake in the test rather than in the code under it — a rule field never assigned, an entry in a list
	// of a suite's rules left empty. It is reported as a failure and not passed over, because an assertion that
	// quietly asserted nothing is what this library treats as the worst outcome there is, and it is reported
	// rather than a panic, because taking somebody's test process down is the second worst.
	framework := &recorder{}

	archtest.AssertPasses(framework, nil, nil)

	failures := framework.failures(t)
	if len(failures) != 1 {
		t.Fatalf("a nil rule reported %d failures, want the one:\n%v", len(failures), failures)
	}
	if want := "there is no rule to check: AssertPasses was given a nil rule"; failures[0] != want {
		t.Errorf("the failure reads %q, want %q", failures[0], want)
	}
}

func TestTheCheckOptionsReachTheRule(t *testing.T) {
	// Half of what the options bag is for: the knobs that say how the rule is run are the rule's, not the
	// report's, and they arrive at Check as the bag it already takes. A nil bag arrives as the defaults rather
	// than as nothing, so a Checkable never has to answer the nil question twice.
	framework := &recorder{}
	rule := &stubRule{}

	archtest.AssertPasses(framework, rule, &archtest.AssertOptions{
		Check: fluentapi.CheckOptions{AllowEmptyTests: true, IncludeTestFiles: true},
	})

	if rule.options == nil {
		t.Fatal("the rule was checked with a nil options bag, want the resolved one")
	}
	if !rule.options.AllowEmptyTests || !rule.options.IncludeTestFiles {
		t.Errorf("the rule was checked with %+v, want the caller's own knobs", rule.options)
	}

	defaulted := &stubRule{}
	archtest.AssertPasses(framework, defaulted, nil)
	if defaulted.options == nil {
		t.Fatal("with a nil bag the rule was checked with nil, want the resolved defaults")
	}
	if defaulted.options.AllowEmptyTests {
		t.Errorf("with a nil bag the rule was checked with %+v, want the strict defaults", defaulted.options)
	}
}

func TestTheMessageOptionsShapeTheFailureItReports(t *testing.T) {
	// The other half: how many violations the report lists and what it is painted in are the report's knobs,
	// and they reach the factory that owns them. Color is decoration only, here as everywhere — the painted
	// failure strips to the plain one, heading and all.
	plain := &recorder{}
	colored := &recorder{}
	violations := namingViolations(t, "a.go", "b.go", "c.go")

	archtest.AssertPasses(plain, &stubRule{sentence: "a rule", violations: violations},
		&archtest.AssertOptions{Message: archtest.MessageOptions{MaxViolations: 1}})
	archtest.AssertPasses(colored, &stubRule{sentence: "a rule", violations: violations},
		&archtest.AssertOptions{Message: archtest.MessageOptions{MaxViolations: 1, Palette: archtest.DefaultPalette()}})

	plainFailure := plain.failures(t)[0]
	coloredFailure := colored.failures(t)[0]
	if !strings.Contains(plainFailure, "... and 2 violations not listed, because MaxViolations is 1") {
		t.Errorf("the limited failure reads\n%s\nwant the two it left out named", plainFailure)
	}
	if strings.Contains(plainFailure, "\x1b") {
		t.Errorf("the default failure carries an escape sequence, want plain text:\n%q", plainFailure)
	}
	if plainText(coloredFailure) != plainFailure {
		t.Errorf("the painted failure strips to\n%s\nwant\n%s", plainText(coloredFailure), plainFailure)
	}
	if !strings.HasPrefix(coloredFailure, "\x1b[33ma rule\x1b[0m\n") {
		t.Errorf("the painted failure reads\n%s\nwant the rule's sentence painted in the requirement color", coloredFailure)
	}
}

func TestARuleThatCannotDescribeItselfIsReportedWithoutAHeading(t *testing.T) {
	// A Checkable of a caller's own may be nothing but a Check method, and a rule that renders as nothing is
	// the same case. Either way the report is the violations: a heading saying the rule has no name would cost
	// a line and tell a reader nothing.
	for name, rule := range map[string]fluentapi.Checkable{
		"a rule with no String":     muteRule{violations: namingViolations(t, "a.go")},
		"a rule that renders empty": &stubRule{violations: namingViolations(t, "a.go")},
	} {
		framework := &recorder{}

		archtest.AssertPasses(framework, rule, nil)

		want := "1 violation:\n" + `  1. a.go: should, filename matches "*_service.go"; it does not`
		if failure := framework.failures(t)[0]; failure != want {
			t.Errorf("%s reports\n%s\nwant\n%s", name, failure, want)
		}
	}
}

// stubRule is a rule as this layer sees one: a Checkable that answers with what the test put in it, and
// records how it was asked. A real terminal would extract the project first, which is exactly what a test of
// the report layer has no business doing.
type stubRule struct {
	sentence   string
	violations []kernel.Violation
	err        error
	options    *fluentapi.CheckOptions
	checks     int
}

func (r *stubRule) Check(options *fluentapi.CheckOptions) ([]kernel.Violation, error) {
	r.checks++
	r.options = options
	return r.violations, r.err
}

func (r *stubRule) String() string {
	return r.sentence
}

// muteRule is the same rule without a String: the shape a Checkable of a user's own is allowed to have, since
// Check is the whole of the interface.
type muteRule struct {
	violations []kernel.Violation
}

func (r muteRule) Check(_ *fluentapi.CheckOptions) ([]kernel.Violation, error) {
	return r.violations, nil
}

// recorder is a test framework's handle as the assert helper sees one: it records the failures instead of
// failing, so that a test can read what a user's test would have been shown. It has Helper() because the
// stdlib's handle does.
type recorder struct {
	calls   [][]any
	helpers int
}

func (r *recorder) Error(args ...any) {
	r.calls = append(r.calls, args)
}

func (r *recorder) Helper() {
	r.helpers++
}

// failures are the messages the helper reported, one per Error call, and the promise that each call carried
// exactly one string: a framework prints what it is handed, and a helper that passed it two arguments would
// report a rule as `a rule 1 violation:` in one framework and something else in the next.
func (r *recorder) failures(t *testing.T) []string {
	t.Helper()

	messages := make([]string, 0, len(r.calls))
	for _, args := range r.calls {
		if len(args) != 1 {
			t.Fatalf("Error was called with %d arguments, %v, want the one message", len(args), args)
		}
		message, ok := args[0].(string)
		if !ok {
			t.Fatalf("Error was called with a %T, want the message as a string", args[0])
		}
		messages = append(messages, message)
	}
	return messages
}

// bareRecorder is a handle with nothing but Error: the smaller framework the helper must not lock out, and
// the reason Helper() is asked for rather than required.
type bareRecorder struct {
	calls [][]any
}

func (r *bareRecorder) Error(args ...any) {
	r.calls = append(r.calls, args)
}

func (r *bareRecorder) failures(t *testing.T) []string {
	t.Helper()

	return (&recorder{calls: r.calls}).failures(t)
}
