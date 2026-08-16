package fluentapi_test

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	metricsassertion "github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

// The five predicates that compare a number against a figure are rules, so they are the fluentapi.Checkable
// every consumer of a rule programs against, asserted at compile time rather than in a test that could be
// deleted.
var _ kernel.Checkable = fluentapi.MetricsThresholdCondition{}

func TestShouldBeBelowReportsTheNumbersOverTheFigure(t *testing.T) {
	// The whole rule through the public chain: the fixture's files are 9, 4, 3 and 3 lines long, so a limit of
	// four reports the two that are not under it.
	rule := fluentapi.Metrics(measuredProject(t)).Count().LinesOfCode().ShouldBeBelow(4)

	violations := check(t, rule, nil)

	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if got := thresholdOffendersOf(violations); !slices.Equal(got, want) {
		t.Errorf("%s reported %v, want %v", rule, got, want)
	}
	reported, ok := violations[0].(metricsassertion.ThresholdViolation)
	if !ok {
		t.Fatalf("reported a %T, want a metrics ThresholdViolation", violations[0])
	}
	if reported.Metric != "lines of code" || reported.Value != 9 {
		t.Errorf("reported %s, want the 9 lines of code the fixture was written with", reported)
	}
	if reported.Comparison != "below" || reported.Limit != 4 {
		t.Errorf("reported %s, want the comparison and the figure the rule was written with", reported)
	}
	if reported.Mood.Negated() {
		t.Errorf("reported the mood as %s, want the `should` the verb spells", reported.Mood)
	}
}

func TestEachThresholdVerbReportsTheNumbersItsOwnComparisonLeavesOut(t *testing.T) {
	// The five verbs over one metric and one figure, which is where they are told apart: the fixture's files
	// measure 9, 4, 3 and 3 lines, in the order the selection produced them, and four is the figure.
	locator := measuredProject(t)

	tests := []struct {
		verb string
		rule fluentapi.MetricsThresholdCondition
		want []string
	}{
		{
			verb: "should be below",
			rule: fluentapi.Metrics(locator).Count().LinesOfCode().ShouldBeBelow(4),
			want: []string{"internal/api/handler.go", "internal/api/router.go"},
		},
		{
			verb: "should be above",
			rule: fluentapi.Metrics(locator).Count().LinesOfCode().ShouldBeAbove(4),
			want: []string{"internal/api/router.go", "internal/db/conn.go", "main.go"},
		},
		{
			verb: "should be",
			rule: fluentapi.Metrics(locator).Count().LinesOfCode().ShouldBe(4),
			want: []string{"internal/api/handler.go", "internal/db/conn.go", "main.go"},
		},
		{
			verb: "should be below or equal",
			rule: fluentapi.Metrics(locator).Count().LinesOfCode().ShouldBeBelowOrEqual(4),
			want: []string{"internal/api/handler.go"},
		},
		{
			verb: "should be above or equal",
			rule: fluentapi.Metrics(locator).Count().LinesOfCode().ShouldBeAboveOrEqual(4),
			want: []string{"internal/db/conn.go", "main.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.verb, func(t *testing.T) {
			if got := thresholdOffendersOf(check(t, test.rule, nil)); !slices.Equal(got, test.want) {
				t.Errorf("%s reported %v, want %v", test.rule, got, test.want)
			}
		})
	}
}

func TestAThresholdARuleKeepsIsThePass(t *testing.T) {
	// No offending number is no violations, which is what a passing rule looks like from here.
	rule := fluentapi.Metrics(measuredProject(t)).Count().LinesOfCode().ShouldBeBelow(400)

	if violations := check(t, rule, nil); len(violations) != 0 {
		t.Errorf("%s reported %v, want nothing", rule, violations)
	}
}

func TestAThresholdIsJudgedOverTheSubjectsItsScopeSelected(t *testing.T) {
	// The scope decides which numbers are compared, exactly as it does for Measure.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("internal/db").
		Count().
		LinesOfCode().
		ShouldBeAbove(400)

	violations := check(t, rule, nil)

	if got := thresholdOffendersOf(violations); !slices.Equal(got, []string{"internal/db/conn.go"}) {
		t.Errorf("%s reported %v, want the one file the scope selected", rule, got)
	}
}

func TestAThresholdJudgesTheNumberOfAMetricAboutAClass(t *testing.T) {
	// Which population a metric reads is the metric's own business, and a threshold is written over it unchanged:
	// the fixture's Handler has two methods, its Router one and its Connection none.
	rule := fluentapi.Metrics(measuredProject(t)).Count().MethodCount().ShouldBeBelowOrEqual(1)

	violations := check(t, rule, nil)

	if got := thresholdOffendersOf(violations); !slices.Equal(got, []string{"internal/api.Handler"}) {
		t.Errorf("%s reported %v, want the one class over the limit", rule, got)
	}
}

func TestAThresholdJudgesTheNumberOfACustomMetric(t *testing.T) {
	// A custom metric is a metric, so the same five verbs follow it: the fixture's Handler exposes two fields and
	// two methods, and nothing else it declares exposes more than one thing.
	rule := fluentapi.Metrics(measuredProject(t)).
		CustomMetric("public surface", "how many methods and fields a type exposes",
			func(class metricsextraction.ClassInfo) float64 {
				return float64(class.MethodCount + class.FieldCount)
			}).
		ShouldBeBelowOrEqual(2)

	violations := check(t, rule, nil)

	if got := thresholdOffendersOf(violations); !slices.Equal(got, []string{"internal/api.Handler"}) {
		t.Errorf("%s reported %v, want [internal/api.Handler]", rule, got)
	}
	reported, ok := violations[0].(metricsassertion.ThresholdViolation)
	if !ok {
		t.Fatalf("reported a %T, want a metrics ThresholdViolation", violations[0])
	}
	if reported.Metric != "public surface" || reported.Value != 4 {
		t.Errorf("reported %s, want the user's own metric at 4", reported)
	}
}

func TestAThresholdThatMeasuredNothingIsAnEmptyTestViolation(t *testing.T) {
	// The highest-value defensive decision in the library, wired into these five verbs too: no number measured
	// means no number over the figure, so a stale glob would otherwise pass forever.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("nowhere/**").
		Count().
		LinesOfCode().
		ShouldBeBelow(400)

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

func TestAThresholdOverAScopeWithNoClassIsAnEmptyTestViolation(t *testing.T) {
	// The case the `measurements` vocabulary is chosen for: the root package is selected, it declares no class,
	// and a rule about a class metric therefore has nothing to compare even though the glob matched a file.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder(".").
		Count().
		MethodCount().
		ShouldBeBelow(10)

	violations := check(t, rule, nil)

	if len(violations) != 1 || violations[0].Kind() != assertion.KindEmptyTest {
		t.Fatalf("%s reported %v, want the one empty-test violation", rule, violations)
	}
}

func TestAThresholdOfAnEmptySelectionCanBeAllowed(t *testing.T) {
	// The opt-out is the same one every other rule takes, from the same options bag.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("nowhere/**").
		Count().
		LinesOfCode().
		ShouldBeBelow(400)

	violations := check(t, rule, &kernel.CheckOptions{AllowEmptyTests: true})

	if len(violations) != 0 {
		t.Errorf("%s reported %v with AllowEmptyTests, want nothing", rule, violations)
	}
}

func TestTheCheckOptionsReachAThreshold(t *testing.T) {
	// The same options every other rule takes, and they change which numbers are compared: the fixture's test
	// file is a subject only when the options say so.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder("internal/api").
		Count().
		LinesOfCode().
		ShouldBeAbove(400)

	byDefault := thresholdOffendersOf(check(t, rule, nil))
	withTests := thresholdOffendersOf(check(t, rule, &kernel.CheckOptions{IncludeTestFiles: true}))

	if !slices.Equal(byDefault, []string{"internal/api/handler.go", "internal/api/router.go"}) {
		t.Errorf("%s reported %v by default, want the two files the project ships", rule, byDefault)
	}
	if !slices.Contains(withTests, "internal/api/handler_test.go") {
		t.Errorf("%s reported %v with IncludeTestFiles, want the test file among them", rule, withTests)
	}
}

func TestAThresholdWithAFigureThatIsNotANumberIsAUserError(t *testing.T) {
	// NaN is the one float64 that is on no side of itself, so a rule written with one would report every number it
	// measured and never say why. That is not a rule the code has broken, so it is the user's own error naming
	// the verb they typed, before the project is read.
	// No locator, because such a rule is rejected before any project is read.
	judged := fluentapi.Metrics(nil).Count().LinesOfCode()

	tests := []struct {
		verb string
		rule fluentapi.MetricsThresholdCondition
	}{
		{verb: "should be below", rule: judged.ShouldBeBelow(math.NaN())},
		{verb: "should be above", rule: judged.ShouldBeAbove(math.NaN())},
		{verb: "should be", rule: judged.ShouldBe(math.NaN())},
		{verb: "should be below or equal", rule: judged.ShouldBeBelowOrEqual(math.NaN())},
		{verb: "should be above or equal", rule: judged.ShouldBeAboveOrEqual(math.NaN())},
	}

	for _, test := range tests {
		t.Run(test.verb, func(t *testing.T) {
			violations, err := test.rule.Check(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Check error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the verb %q", user.Operation, test.verb)
			}
			if !errors.Is(err, fluentapi.ErrLimitNotANumber) {
				t.Errorf("Check error = %v, want it to wrap ErrLimitNotANumber", err)
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

func TestAnInfiniteFigureIsAnOrdinaryThreshold(t *testing.T) {
	// The other two float64 values that are not ordinary numbers are not rejected: `should be below +Inf` is the
	// rule that a count is finite at all, and somebody could mean it.
	below := fluentapi.Metrics(measuredProject(t)).Count().LinesOfCode().ShouldBeBelow(math.Inf(1))
	above := fluentapi.Metrics(measuredProject(t)).Count().LinesOfCode().ShouldBeAbove(math.Inf(1))

	if violations := check(t, below, nil); len(violations) != 0 {
		t.Errorf("%s reported %v, want every finite number under it", below, violations)
	}
	if got := len(thresholdOffendersOf(check(t, above, nil))); got != 4 {
		t.Errorf("%s reported %d numbers, want all four of the fixture's files", above, got)
	}
}

func TestAThresholdRejectsAPatternItsScopeCouldNotCompile(t *testing.T) {
	rule := fluentapi.Metrics(nil).InFolder("[unclosed").Count().LinesOfCode().ShouldBeBelow(400)

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

func TestAThresholdKeepsTheScopesOwnRejection(t *testing.T) {
	// The first thing the user got wrong is the one they are told about, so a pattern the scope could not compile
	// is not overwritten by the figure chained onto it, a figure that is not a number included.
	rule := fluentapi.Metrics(nil).InFolder("[unclosed").Count().LinesOfCode().ShouldBeBelow(math.NaN())

	_, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the scope verb `in folder`", user.Operation)
	}
}

func TestAThresholdRejectsALocatorThatIsNotAProject(t *testing.T) {
	rule := fluentapi.Metrics(&extraction.ProjectLocator{Directory: t.TempDir()}).
		Count().
		LinesOfCode().
		ShouldBeBelow(400)

	_, err := rule.Check(nil)

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("Check error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
}

func TestAThresholdRendersTheSentenceTheUserTyped(t *testing.T) {
	// The figure is part of the sentence, which is the difference between these five and `should satisfy` — and
	// this sentence is the heading archtest prints above every violation the rule reported.
	tests := []struct {
		rule fluentapi.MetricsThresholdCondition
		want string
	}{
		{
			rule: fluentapi.Metrics(nil).ForClassesMatching("*Service").Count().MethodCount().
				ShouldBeBelowOrEqual(10),
			want: `metrics, classname matches "*Service", count, method count, should be below or equal 10`,
		},
		{
			rule: fluentapi.Metrics(nil).InFolder("internal/**").Count().LinesOfCode().ShouldBeBelow(400),
			want: `metrics, path without filename matches "internal/**", count, lines of code, should be below 400`,
		},
		{
			rule: fluentapi.Metrics(nil).Distance().Instability().ShouldBeAboveOrEqual(0.2),
			want: "metrics, distance, instability, should be above or equal 0.2",
		},
		{
			rule: fluentapi.Metrics(nil).Distance().Abstractness().ShouldBeAbove(0),
			want: "metrics, distance, abstractness, should be above 0",
		},
		{
			// The comparison that is the equality itself has no word of its own, so the figure follows `be`
			// directly rather than after a gap where a word would have been.
			rule: fluentapi.Metrics(nil).Distance().Abstractness().ShouldBe(1),
			want: "metrics, distance, abstractness, should be 1",
		},
		{
			rule: fluentapi.Metrics(nil).
				CustomMetric("public surface", "how many methods and fields a type exposes", zeroMeasure).
				ShouldBeBelowOrEqual(20),
			want: `metrics, custom metric, public surface ("how many methods and fields a type exposes"), ` +
				"should be below or equal 20",
		},
		{
			// The zero MetricBuilder names no metric, and a part that is not there is left out rather than
			// rendered as an empty word.
			rule: fluentapi.MetricBuilder{}.ShouldBeAbove(0),
			want: "metrics, should be above 0",
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

func TestAThresholdRendersTheFigureExactly(t *testing.T) {
	// As many digits as it takes to say exactly which float64 the figure is, so a rule is never rendered with a
	// limit different from the one it judges by.
	rule := fluentapi.Metrics(nil).Distance().Abstractness().ShouldBeBelow(1.0 / 3.0)

	if got := rule.String(); got != "metrics, distance, abstractness, should be below 0.3333333333333333" {
		t.Errorf("String() = %q, want the figure unrounded", got)
	}
}

func TestAThresholdDoesNotWriteIntoTheSentenceOfTheRuleItWasAskedOf(t *testing.T) {
	// The append trap one stage further on: two thresholds grow the same rule's stages, so a stage list shared
	// between them would leave one rule rendering as the other.
	rule := fluentapi.Metrics(nil).Count().LinesOfCode()

	short := rule.ShouldBeBelow(400)
	long := rule.ShouldBeAbove(10)

	if got := short.String(); got != "metrics, count, lines of code, should be below 400" {
		t.Errorf("the first rule renders as %q, want its own comparison", got)
	}
	if got := long.String(); got != "metrics, count, lines of code, should be above 10" {
		t.Errorf("the second rule renders as %q, want its own comparison", got)
	}
	if got := rule.String(); got != "metrics, count, lines of code" {
		t.Errorf("the rule they were asked of renders as %q, want the stages it was built from", got)
	}
}

func TestAThresholdCanBeStoredAndCheckedTwice(t *testing.T) {
	// A rule is a value: nothing is read when it is built, and checking it does not change it.
	rule := fluentapi.Metrics(measuredProject(t)).Count().LinesOfCode().ShouldBeBelow(4)

	first := thresholdOffendersOf(check(t, rule, nil))
	second := thresholdOffendersOf(check(t, rule, nil))

	if !slices.Equal(first, second) {
		t.Errorf("the rule reported %v and then %v, want the same answer twice", first, second)
	}
}

func TestTheThresholdPredicatesAreExactlySixWithNoSynonyms(t *testing.T) {
	// The issue's whole point, held to mechanically: six threshold predicates and nothing else on the stage a
	// number is judged at. A synonym is how a fluent API stops sounding like one language, and it is added by
	// someone who did not know the comparison was already spelled — so the check is on the method set rather than
	// on a reviewer's memory.
	synonyms := []string{
		"ShouldEqual", "ShouldNotEqual", "ShouldBeAtMost", "ShouldBeAtLeast",
		"ShouldBeLessThan", "ShouldBeLessThanOrEqual", "ShouldBeGreaterThan", "ShouldBeGreaterThanOrEqual",
		"ShouldBeUnder", "ShouldBeOver", "ShouldBeUnderOrEqual", "ShouldBeOverOrEqual",
		"ShouldBeMax", "ShouldBeMin", "ShouldBeExactly", "ShouldBeBetween", "ShouldNotExceed",
		"MustBeBelow", "IsBelow", "HaveValueBelow",
	}
	want := []string{
		"ShouldBe", "ShouldBeAbove", "ShouldBeAboveOrEqual",
		"ShouldBeBelow", "ShouldBeBelowOrEqual", "ShouldSatisfy",
	}

	judged := methodsOf(fluentapi.MetricBuilder{})
	predicates := make([]string, 0, len(judged))
	for _, method := range judged {
		if strings.HasPrefix(method, "Should") {
			predicates = append(predicates, method)
		}
	}

	slices.Sort(predicates)
	if !slices.Equal(predicates, want) {
		t.Errorf("the stage a number is judged at offers %v, want exactly the six %v", predicates, want)
	}
	// The stages before it offer none of the six, so a chain cannot compare a number it has not named — and none
	// of them grows a synonym either. The two zone verbs on `distance` are rules about a region rather than
	// comparisons against a figure, which is why they are the one `Should` this walk expects elsewhere.
	stages := map[string][]string{
		"the scope stage":  methodsOf(fluentapi.Metrics(nil)),
		"`count`":          methodsOf(fluentapi.Metrics(nil).Count()),
		"`distance`":       methodsOf(fluentapi.Metrics(nil).Distance()),
		"a built rule":     methodsOf(fluentapi.MetricsThresholdCondition{}),
		"the escape hatch": methodsOf(fluentapi.MetricsSatisfactionCondition{}),
	}
	zoneVerbs := []string{"ShouldNotBeInZoneOfPain", "ShouldNotBeInZoneOfUselessness"}
	for stage, methods := range stages {
		for _, method := range methods {
			if slices.Contains(synonyms, method) {
				t.Errorf("%s has %s, want the six threshold predicates and no synonym of one", stage, method)
			}
			if slices.Contains(want, method) {
				t.Errorf("%s has %s, want a threshold predicate only where a metric has been named", stage, method)
			}
			if strings.HasPrefix(method, "Should") && !slices.Contains(zoneVerbs, method) {
				t.Errorf("%s has %s, want no predicate but the two zone rules before a metric is named", stage, method)
			}
		}
	}
}

// thresholdOffendersOf names the subjects a threshold rule reported, in order, for a failure message.
func thresholdOffendersOf(violations []assertion.Violation) []string {
	reported := make([]string, 0, len(violations))
	for _, violation := range violations {
		if threshold, ok := violation.(metricsassertion.ThresholdViolation); ok {
			reported = append(reported, threshold.Subject)
		}
	}
	return reported
}

// methodsOf names the methods a fluent stage offers, which is the vocabulary a user can type at it.
func methodsOf(stage any) []string {
	stageType := reflect.TypeOf(stage)
	names := make([]string, 0, stageType.NumMethod())
	for index := range stageType.NumMethod() {
		names = append(names, stageType.Method(index).Name)
	}
	return names
}
