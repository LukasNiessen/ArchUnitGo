package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
)

func TestGatherSatisfactionViolationsReportsTheMeasurementsThePredicateSaysNoAbout(t *testing.T) {
	// `should satisfy`: one violation per number the user's own function rejects, and nothing about the rest.
	violations := assertion.GatherSatisfactionViolations(
		fixtureMeasurements(), fixtureClasses(),
		func(measurement calculation.Measurement, _ extraction.ClassInfo) bool { return measurement.Value <= 10 },
		"be at most 10 methods wide", kernel.Should)

	want := []string{"internal/api.Handler", "internal/api.Wide"}
	if reported := subjectsOf(violations); !slices.Equal(reported, want) {
		t.Errorf("reported %v, want %v", reported, want)
	}
}

func TestGatherSatisfactionViolationsHandsThePredicateTheClassTheNumberWasReadOff(t *testing.T) {
	// The second argument is the point of the escape hatch: a limit that depends on the subject — an interface
	// is allowed to be wide — is not a threshold, and this is the one predicate that can express it.
	violations := assertion.GatherSatisfactionViolations(
		fixtureMeasurements(), fixtureClasses(),
		func(measurement calculation.Measurement, class extraction.ClassInfo) bool {
			return measurement.Value <= 10 || class.Interface
		},
		"be at most 10 methods wide unless it is an interface", kernel.Should)

	want := []string{"internal/api.Handler"}
	if reported := subjectsOf(violations); !slices.Equal(reported, want) {
		t.Errorf("reported %v, want %v — internal/api.Wide is the interface the predicate exempts", reported, want)
	}
}

func TestGatherSatisfactionViolationsLooksTheClassUpByTheSubjectTheMeasurementNames(t *testing.T) {
	// The measurements of a metric about a class are not the classes one per one — a scope can select more
	// classes than a metric reports on — so the pairing is by identifier and never by position.
	seen := map[string]string{}
	assertion.GatherSatisfactionViolations(
		[]calculation.Measurement{
			{Metric: "method count", Subject: "internal/api.Wide", Value: 12},
			{Metric: "method count", Subject: "internal/api.Handler", Value: 40},
		},
		fixtureClasses(),
		func(measurement calculation.Measurement, class extraction.ClassInfo) bool {
			seen[measurement.Subject] = class.Name
			return true
		},
		"be narrow", kernel.Should)

	want := map[string]string{"internal/api.Wide": "Wide", "internal/api.Handler": "Handler"}
	for subject, name := range want {
		if seen[subject] != name {
			t.Errorf("the predicate was handed %q for %s, want %q", seen[subject], subject, name)
		}
	}
}

func TestGatherSatisfactionViolationsHandsThePredicateNoClassForANumberThatIsNotAboutOne(t *testing.T) {
	// `count, lines of code` is a number about a file and `distance, abstractness` one about a package, and
	// neither has a class for the second argument to be: it is the zero value, which is what Satisfaction says.
	var seen extraction.ClassInfo
	violations := assertion.GatherSatisfactionViolations(
		[]calculation.Measurement{{Metric: "lines of code", Subject: "internal/api/handler.go", Value: 700}},
		fixtureClasses(),
		func(_ calculation.Measurement, class extraction.ClassInfo) bool {
			seen = class
			return false
		},
		"be at most 400 lines long", kernel.Should)

	if seen.Name != "" || seen.Identifier != "" || seen.Path != "" || seen.MethodCount != 0 {
		t.Errorf("the predicate was handed %+v for a file's number, want the zero ClassInfo", seen)
	}
	if reported := subjectsOf(violations); !slices.Equal(reported, []string{"internal/api/handler.go"}) {
		t.Errorf("reported %v, want the file the number was read off", reported)
	}
}

func TestGatherSatisfactionViolationsCarriesTheNumberAndTheRequirementItWasJudgedBy(t *testing.T) {
	// A violation that measured again could disagree with the judgement that produced it, so the number is read
	// once and used twice — for the question and for the report — and the words travel along untouched.
	violations := assertion.GatherSatisfactionViolations(
		fixtureMeasurements(), fixtureClasses(),
		func(measurement calculation.Measurement, class extraction.ClassInfo) bool {
			return measurement.Value <= 10 || class.Interface
		},
		"be at most 10 methods wide", kernel.Should)

	if len(violations) != 1 {
		t.Fatalf("reported %v, want the one class the predicate rejects", subjectsOf(violations))
	}
	reported, ok := violations[0].(assertion.SatisfactionViolation)
	if !ok {
		t.Fatalf("reported a %T, want a SatisfactionViolation", violations[0])
	}
	if reported.Value != 40 {
		t.Errorf("reported the number as %g, want the 40 the metric read", reported.Value)
	}
	if reported.Metric != "method count" {
		t.Errorf("reported the metric as %q, want the words the rule named it in", reported.Metric)
	}
	if reported.Requirement != "be at most 10 methods wide" {
		t.Errorf("reported the requirement as %q, want the user's own words", reported.Requirement)
	}
	if reported.Mood != kernel.Should {
		t.Errorf("reported the mood as %s, want the mood the rule was written in", reported.Mood)
	}
}

func TestGatherSatisfactionViolationsInTheNegatedMoodReportsTheMeasurementsThePredicateSaysYesAbout(t *testing.T) {
	// The negated rule is the same walk with one comparison inverted, so there is no second code path to keep
	// in step — even though only `should satisfy` is offered by the fluent API. The mood travels into the
	// violation too, because archtest phrases the requirement and the finding off it.
	measurements := fixtureMeasurements()

	violations := assertion.GatherSatisfactionViolations(
		measurements, fixtureClasses(),
		func(measurement calculation.Measurement, _ extraction.ClassInfo) bool { return measurement.Value <= 10 },
		"be at most 10 methods wide", kernel.ShouldNot)

	want := []string{"internal/api.ID"}
	if reported := subjectsOf(violations); !slices.Equal(reported, want) {
		t.Fatalf("reported %v, want %v — the one measurement the predicate accepts", reported, want)
	}
	reported, ok := violations[0].(assertion.SatisfactionViolation)
	if !ok {
		t.Fatalf("reported a %T, want a SatisfactionViolation", violations[0])
	}
	if !reported.Mood.Negated() {
		t.Errorf("reported the mood as %s, want the mood the rule was written in", reported.Mood)
	}
}

func TestGatherSatisfactionViolationsOfASatisfiedRuleReportsNothing(t *testing.T) {
	// No offending measurement is no violations, which is the pass.
	violations := assertion.GatherSatisfactionViolations(
		fixtureMeasurements(), fixtureClasses(),
		func(calculation.Measurement, extraction.ClassInfo) bool { return true },
		"be anything at all", kernel.Should)

	if len(violations) != 0 {
		t.Errorf("reported %v, want nothing about a rule every number satisfies", subjectsOf(violations))
	}
}

func TestGatherSatisfactionViolationsKeepsTheOrderTheMeasurementsArrivedIn(t *testing.T) {
	// The order of a report is the order the metric read its subjects in, so the same rule prints the same
	// list twice.
	violations := assertion.GatherSatisfactionViolations(
		fixtureMeasurements(), fixtureClasses(),
		func(calculation.Measurement, extraction.ClassInfo) bool { return false },
		"be narrow", kernel.Should)

	want := []string{"internal/api.Handler", "internal/api.Wide", "internal/api.ID"}
	if reported := subjectsOf(violations); !slices.Equal(reported, want) {
		t.Errorf("reported %v, want %v", reported, want)
	}
}

func TestGatherSatisfactionViolationsOfNoMeasurementsReportsNothing(t *testing.T) {
	// A rule that measured nothing at all is the empty-test guard's answer rather than this one's: every
	// measurement of an empty list satisfies every predicate, in either mood, so a stale glob would otherwise
	// be green forever.
	for _, mood := range []kernel.Mood{kernel.Should, kernel.ShouldNot} {
		violations := assertion.GatherSatisfactionViolations(nil, fixtureClasses(),
			func(calculation.Measurement, extraction.ClassInfo) bool { return true },
			"be narrow", mood)

		if len(violations) != 0 {
			t.Errorf("%s reported %v about no measurements, want nothing", mood, violations)
		}
	}
}

func TestGatherSatisfactionViolationsWithNoPredicateSatisfiesNothing(t *testing.T) {
	// Calling a nil function would take the host test process down, so a missing predicate answers no — the
	// shape a nil files/assertion.FilePredicate has. It cannot arrive from the fluent API, which returns a
	// missing function as the user's error before the project is read.
	negated := assertion.GatherSatisfactionViolations(
		fixtureMeasurements(), fixtureClasses(), nil, "be narrow", kernel.ShouldNot)
	positive := assertion.GatherSatisfactionViolations(
		fixtureMeasurements(), fixtureClasses(), nil, "be narrow", kernel.Should)

	if len(negated) != 0 {
		t.Errorf("`should not` reported %v with no predicate, want nothing", subjectsOf(negated))
	}
	if len(positive) != len(fixtureMeasurements()) {
		t.Errorf("`should` reported %v with no predicate, want every measurement", subjectsOf(positive))
	}
}

// fixtureMeasurements is one metric's numbers as calculation read them, hand-built so that the judgement can
// be tested with no project on disk: one class over a limit of ten, one interface over it and one under.
func fixtureMeasurements() []calculation.Measurement {
	return []calculation.Measurement{
		{Metric: "method count", Subject: "internal/api.Handler", Value: 40},
		{Metric: "method count", Subject: "internal/api.Wide", Value: 12},
		{Metric: "method count", Subject: "internal/api.ID", Value: 0},
	}
}

// fixtureClasses is the population those numbers were read off, in the order a rule selected them.
func fixtureClasses() []extraction.ClassInfo {
	return []extraction.ClassInfo{
		{Name: "Handler", Identifier: "internal/api.Handler", Path: "internal/api/handler.go", MethodCount: 40},
		{Name: "Wide", Identifier: "internal/api.Wide", Path: "internal/api/wide.go", Interface: true, MethodCount: 12},
		{Name: "ID", Identifier: "internal/api.ID", Path: "internal/api/id.go"},
	}
}

// subjectsOf names the subjects a gather reported, in order, for a failure message.
func subjectsOf(violations []kernel.Violation) []string {
	reported := make([]string, 0, len(violations))
	for _, violation := range violations {
		if satisfaction, ok := violation.(assertion.SatisfactionViolation); ok {
			reported = append(reported, satisfaction.Subject)
		}
	}
	return reported
}
