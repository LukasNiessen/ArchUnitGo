package fluentapi_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	metricsassertion "github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

// `should satisfy` is a rule, so it is the fluentapi.Checkable every consumer of a rule programs against,
// asserted at compile time rather than in a test that could be deleted.
var _ kernel.Checkable = fluentapi.MetricsSatisfactionCondition{}

func TestShouldSatisfyReportsTheNumbersThePredicateSaysNoAbout(t *testing.T) {
	// The whole rule through the public chain: the fixture's Handler is the only file over four lines, and the
	// predicate is the user's own comparison rather than one of the five figures the family names.
	rule := fluentapi.Metrics(measuredProject(t)).
		Count().
		LinesOfCode().
		ShouldSatisfy(func(measurement calculation.Measurement, _ metricsextraction.ClassInfo) bool {
			return measurement.Value <= 4
		}, "be at most 4 lines long")

	violations := check(t, rule, nil)

	if got := satisfactionOffendersOf(violations); !slices.Equal(got, []string{"internal/api/handler.go"}) {
		t.Errorf("%s reported %v, want [internal/api/handler.go]", rule, got)
	}
	reported, ok := violations[0].(metricsassertion.SatisfactionViolation)
	if !ok {
		t.Fatalf("reported a %T, want a metrics SatisfactionViolation", violations[0])
	}
	if reported.Metric != "lines of code" || reported.Value != 9 {
		t.Errorf("reported %s, want the 9 lines of code the fixture was written with", reported)
	}
	if reported.Requirement != "be at most 4 lines long" {
		t.Errorf("reported the requirement as %q, want the user's own words", reported.Requirement)
	}
	if reported.Mood.Negated() {
		t.Errorf("reported the mood as %s, want the `should` the verb spells", reported.Mood)
	}
}

func TestShouldSatisfyIsHandedTheClassEveryClassMetricWasReadOff(t *testing.T) {
	// The second argument is what makes this the escape hatch rather than a sixth threshold: a limit that
	// depends on the subject — an interface may be as wide as it likes — is not a figure to compare against.
	rule := fluentapi.Metrics(measuredProject(t)).
		Count().
		MethodCount().
		ShouldSatisfy(func(measurement calculation.Measurement, class metricsextraction.ClassInfo) bool {
			return measurement.Value < 1 || class.Interface
		}, "declare no method unless it is an interface")

	violations := check(t, rule, nil)

	// Handler has two methods and is a struct; Router has one and is the fixture's interface; Connection has
	// none. A rule handed no class could not tell the first two apart.
	if got := satisfactionOffendersOf(violations); !slices.Equal(got, []string{"internal/api.Handler"}) {
		t.Errorf("%s reported %v, want [internal/api.Handler]", rule, got)
	}
}

func TestShouldSatisfyJudgesTheNumberOfACustomMetric(t *testing.T) {
	// The two halves of the issue in one chain, which is what the escape hatch is for: a number the library
	// never named, judged by a comparison it never named either.
	rule := fluentapi.Metrics(measuredProject(t)).
		CustomMetric("public surface", "how many methods and fields a type exposes",
			func(class metricsextraction.ClassInfo) float64 {
				return float64(class.MethodCount + class.FieldCount)
			}).
		ShouldSatisfy(func(measurement calculation.Measurement, _ metricsextraction.ClassInfo) bool {
			return measurement.Value <= 2
		}, "expose at most 2 methods and fields")

	violations := check(t, rule, nil)

	if got := satisfactionOffendersOf(violations); !slices.Equal(got, []string{"internal/api.Handler"}) {
		t.Errorf("%s reported %v, want [internal/api.Handler]", rule, got)
	}
	reported, ok := violations[0].(metricsassertion.SatisfactionViolation)
	if !ok {
		t.Fatalf("reported a %T, want a metrics SatisfactionViolation", violations[0])
	}
	if reported.Metric != "public surface" || reported.Value != 4 {
		t.Errorf("reported %s, want the user's own metric at 4", reported)
	}
}

func TestShouldSatisfyIsASatisfiedRulesPass(t *testing.T) {
	// No offending number is no violations, which is what a passing rule looks like from here.
	rule := fluentapi.Metrics(measuredProject(t)).
		Count().
		LinesOfCode().
		ShouldSatisfy(func(measurement calculation.Measurement, _ metricsextraction.ClassInfo) bool {
			return measurement.Value <= 400
		}, "be at most 400 lines long")

	if violations := check(t, rule, nil); len(violations) != 0 {
		t.Errorf("%s reported %v, want nothing", rule, violations)
	}
}

func TestShouldSatisfyIsJudgedOverTheSubjectsItsScopeSelected(t *testing.T) {
	// The scope decides which numbers the predicate is asked about, exactly as it does for Measure.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("internal/db").
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return false },
			"be a file that does not exist")

	violations := check(t, rule, nil)

	if got := satisfactionOffendersOf(violations); !slices.Equal(got, []string{"internal/db/conn.go"}) {
		t.Errorf("%s reported %v, want the one file the scope selected", rule, got)
	}
}

func TestShouldSatisfyThatMeasuredNothingIsAnEmptyTestViolation(t *testing.T) {
	// The highest-value defensive decision in the library, wired in here too: no number measured means no
	// number the predicate says no about, so a stale glob would otherwise pass forever.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("nowhere/**").
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true },
			"be at most 400 lines long")

	violations := check(t, rule, nil)

	if len(violations) != 1 {
		t.Fatalf("%s reported %v, want the one empty-test violation", rule, violations)
	}
	empty, ok := violations[0].(assertion.EmptyTestViolation)
	if !ok {
		t.Fatalf("reported a %T, want an EmptyTestViolation", violations[0])
	}
	if empty.Subject != "measurements" {
		t.Errorf("the empty selection was reported about %q, want the population the rule judged", empty.Subject)
	}
	if len(empty.Selectors) != 1 {
		t.Errorf("the empty selection reported %v, want the scope verb that selected nothing", empty.Selectors)
	}
}

func TestShouldSatisfyOverAScopeWithNoClassIsAnEmptyTestViolation(t *testing.T) {
	// The case the `measurements` vocabulary is chosen for: the root package is selected, it declares no class,
	// and a rule about a class metric therefore has nothing to judge even though the glob matched a file.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder(".").
		Count().
		MethodCount().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true },
			"declare at most 10 methods")

	violations := check(t, rule, nil)

	if len(violations) != 1 || violations[0].Kind() != assertion.KindEmptyTest {
		t.Fatalf("%s reported %v, want the one empty-test violation", rule, violations)
	}
}

func TestShouldSatisfyDoesNotCallThePredicateUntilTheRuleIsChecked(t *testing.T) {
	// Building a rule does no work, and the user's own function is part of what is not done: it is asked its
	// questions by Check and by nothing else, over a scope that does measure something.
	asked := 0
	rule := fluentapi.Metrics(measuredProject(t)).
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool {
			asked++
			return true
		}, "be at most 400 lines long")

	if asked != 0 {
		t.Errorf("the predicate was asked %d times while the rule was being built, want none", asked)
	}

	if violations := check(t, rule, nil); len(violations) != 0 {
		t.Fatalf("%s reported %v, want nothing: no file of the fixture is 400 lines long", rule, violations)
	}
	if asked == 0 {
		t.Error("Check asked the predicate nothing, want one question per number the rule measured")
	}
}

func TestShouldSatisfyDoesNotCallThePredicateOfARuleItsScopeRejected(t *testing.T) {
	// The order the pipeline runs in, seen from the predicate: a scope that could not be resolved has named no
	// number, so the user's function is asked nothing at all and the rejection is what comes back.
	asked := 0
	rule := fluentapi.Metrics(nil).
		InFolder("[unclosed").
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool {
			asked++
			return false
		}, "be short")

	if _, err := rule.Check(nil); err == nil {
		t.Fatalf("%s checked without an error, want the pattern its scope could not compile", rule)
	}
	if asked != 0 {
		t.Errorf("the predicate was asked %d times about a rule whose scope was rejected, want none", asked)
	}
}

func TestShouldSatisfyOfAnEmptySelectionCanBeAllowed(t *testing.T) {
	// The opt-out is the same one every other rule takes, from the same options bag.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("nowhere/**").
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true },
			"be at most 400 lines long")

	violations := check(t, rule, &kernel.CheckOptions{AllowEmptyTests: true})

	if len(violations) != 0 {
		t.Errorf("%s reported %v with AllowEmptyTests, want nothing", rule, violations)
	}
}

func TestTheCheckOptionsReachShouldSatisfy(t *testing.T) {
	// The same options every other rule takes, and they change which numbers the predicate is asked about: the
	// fixture's test file is a subject only when the options say so.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("internal/api").
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return false },
			"be a file that does not exist")

	byDefault := satisfactionOffendersOf(check(t, rule, nil))
	withTests := satisfactionOffendersOf(check(t, rule, &kernel.CheckOptions{IncludeTestFiles: true}))

	if !slices.Equal(byDefault, []string{"internal/api/handler.go", "internal/api/router.go"}) {
		t.Errorf("%s reported %v by default, want the two files the project ships", rule, byDefault)
	}
	if !slices.Contains(withTests, "internal/api/handler_test.go") {
		t.Errorf("%s reported %v with IncludeTestFiles, want the test file among them", rule, withTests)
	}
}

func TestShouldSatisfyWithoutTheHalvesOfAComparisonIsAUserError(t *testing.T) {
	// A rule with no function says nothing about the numbers it measured, and a rule with no words says it in a
	// way nobody could read. Neither is a rule failure, so both are the user's own error naming the verb.
	tests := []struct {
		name string
		rule fluentapi.MetricsSatisfactionCondition
		want error
	}{
		{
			name: "no predicate",
			rule: fluentapi.Metrics(nil).Count().LinesOfCode().ShouldSatisfy(nil, "be at most 400 lines long"),
			want: fluentapi.ErrNoPredicate,
		},
		{
			name: "no requirement",
			rule: fluentapi.Metrics(nil).Count().LinesOfCode().
				ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true }, "  "),
			want: fluentapi.ErrNoRequirement,
		},
		{
			// Neither half given: the function is the first argument and the one a rule cannot be run without at
			// all, so it is the rejection the user is told about rather than the missing words behind it.
			name: "neither the function nor the words",
			rule: fluentapi.Metrics(nil).Count().LinesOfCode().ShouldSatisfy(nil, "  "),
			want: fluentapi.ErrNoPredicate,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations, err := test.rule.Check(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Check error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != "should satisfy" {
				t.Errorf("UserError.Operation = %q, want the verb `should satisfy`", user.Operation)
			}
			if !errors.Is(err, test.want) {
				t.Errorf("Check error = %v, want it to wrap %v", err, test.want)
			}
			if len(violations) != 0 {
				t.Errorf("Check returned %v beside the error, want nothing", violations)
			}
			if !strings.Contains(test.rule.String(), "rejected") {
				t.Errorf("%s renders without the rejection, want it visible in a test failure", test.rule)
			}
		})
	}
}

func TestShouldSatisfyRejectsAPatternItsScopeCouldNotCompile(t *testing.T) {
	rule := fluentapi.Metrics(nil).
		InFolder("[unclosed").
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true },
			"be at most 400 lines long")

	violations, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the scope verb `in folder`", user.Operation)
	}
	if len(violations) != 0 {
		t.Errorf("Check returned %v beside the error, want nothing", violations)
	}
}

func TestShouldSatisfyKeepsTheScopesOwnRejection(t *testing.T) {
	// The first thing the user got wrong is the one they are told about, so a pattern the scope could not compile
	// is not overwritten by the predicate chained onto it, missing function and missing words included.
	rule := fluentapi.Metrics(nil).InFolder("[unclosed").Count().LinesOfCode().ShouldSatisfy(nil, "")

	_, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the scope verb `in folder`", user.Operation)
	}
}

func TestShouldSatisfyRejectsALocatorThatIsNotAProject(t *testing.T) {
	rule := fluentapi.Metrics(&extraction.ProjectLocator{Directory: t.TempDir()}).
		Count().
		LinesOfCode().
		ShouldSatisfy(func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true },
			"be at most 400 lines long")

	_, err := rule.Check(nil)

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("Check error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
}

func TestShouldSatisfyRendersTheSentenceTheUserTyped(t *testing.T) {
	// The words stand in for the comparison, because a closure has no readable form — and this sentence is the
	// heading archtest prints above every violation the rule reported.
	always := func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true }

	tests := []struct {
		rule fluentapi.MetricsSatisfactionCondition
		want string
	}{
		{
			rule: fluentapi.Metrics(nil).ForClassesMatching("*Service").Count().MethodCount().
				ShouldSatisfy(always, "be at most 10 methods wide"),
			want: `metrics, classname matches "*Service", count, method count, ` +
				`should satisfy "be at most 10 methods wide"`,
		},
		{
			rule: fluentapi.Metrics(nil).
				CustomMetric("public surface", "how many methods and fields a type exposes", zeroMeasure).
				ShouldSatisfy(always, "expose at most 20 methods and fields"),
			want: `metrics, custom metric, public surface ("how many methods and fields a type exposes"), ` +
				`should satisfy "expose at most 20 methods and fields"`,
		},
		{
			// The zero MetricBuilder names no metric, and a part that is not there is left out rather than
			// rendered as an empty word.
			rule: fluentapi.MetricBuilder{}.ShouldSatisfy(always, "be anything at all"),
			want: `metrics, should satisfy "be anything at all"`,
		},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if got := test.rule.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestShouldSatisfyDoesNotWriteIntoTheSentenceOfTheRuleItWasAskedOf(t *testing.T) {
	// The append trap one stage further on: both predicates grow the same rule's stages, so a stage list shared
	// between them would leave one rule rendering as the other.
	rule := fluentapi.Metrics(nil).Count().LinesOfCode()
	always := func(calculation.Measurement, metricsextraction.ClassInfo) bool { return true }

	short := rule.ShouldSatisfy(always, "be short")
	generated := rule.ShouldSatisfy(always, "be generated")

	if got := short.String(); got != `metrics, count, lines of code, should satisfy "be short"` {
		t.Errorf("the first rule renders as %q, want its own requirement", got)
	}
	if got := generated.String(); got != `metrics, count, lines of code, should satisfy "be generated"` {
		t.Errorf("the second rule renders as %q, want its own requirement", got)
	}
	if got := rule.String(); got != "metrics, count, lines of code" {
		t.Errorf("the rule they were asked of renders as %q, want the stages it was built from", got)
	}
}

func TestShouldSatisfyCanBeStoredAndCheckedTwice(t *testing.T) {
	// A rule is a value: nothing is read when it is built, checking it does not change it, and the user's
	// function is asked the same questions the second time round.
	rule := fluentapi.Metrics(measuredProject(t)).
		Count().
		LinesOfCode().
		ShouldSatisfy(func(measurement calculation.Measurement, _ metricsextraction.ClassInfo) bool {
			return measurement.Value <= 4
		}, "be at most 4 lines long")

	first := satisfactionOffendersOf(check(t, rule, nil))
	second := satisfactionOffendersOf(check(t, rule, nil))

	if !slices.Equal(first, second) {
		t.Errorf("the rule reported %v and then %v, want the same answer twice", first, second)
	}
}

func TestShouldSatisfyAsksThePredicateOncePerMeasurement(t *testing.T) {
	// What Satisfaction promises the user: one question per number, so a predicate counting its own calls is
	// counting the subjects the rule measured.
	asked := []string{}
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("internal/api").
		Count().
		LinesOfCode().
		ShouldSatisfy(func(measurement calculation.Measurement, _ metricsextraction.ClassInfo) bool {
			asked = append(asked, measurement.Subject)
			return true
		}, "be at most 400 lines long")

	check(t, rule, nil)

	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if !slices.Equal(asked, want) {
		t.Errorf("the predicate was asked about %v, want %v — one question per measurement, in order", asked, want)
	}
}

// satisfactionOffendersOf names the subjects a `should satisfy` rule reported, in order, for a failure message.
func satisfactionOffendersOf(violations []assertion.Violation) []string {
	reported := make([]string, 0, len(violations))
	for _, violation := range violations {
		if satisfaction, ok := violation.(metricsassertion.SatisfactionViolation); ok {
			reported = append(reported, satisfaction.Subject)
		}
	}
	return reported
}
