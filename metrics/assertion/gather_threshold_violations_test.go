package assertion_test

import (
	"math"
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

func TestGatherThresholdViolationsReportsTheNumbersOnTheWrongSideOfTheFigure(t *testing.T) {
	// The five comparisons over the same numbers, each reporting exactly the subjects its own side of the figure
	// leaves out. One function and one violation type serve all five, so this is where the five are told apart.
	tests := []struct {
		name      string
		threshold calculation.Threshold
		want      []string
	}{
		{
			name:      "should be below",
			threshold: calculation.Below(12),
			want:      []string{"internal/api.Handler", "internal/api.Wide"},
		},
		{
			name:      "should be above",
			threshold: calculation.Above(12),
			want:      []string{"internal/api.Wide", "internal/api.ID"},
		},
		{
			name:      "should be",
			threshold: calculation.Exactly(12),
			want:      []string{"internal/api.Handler", "internal/api.ID"},
		},
		{
			name:      "should be below or equal",
			threshold: calculation.BelowOrEqual(12),
			want:      []string{"internal/api.Handler"},
		},
		{
			name:      "should be above or equal",
			threshold: calculation.AboveOrEqual(12),
			want:      []string{"internal/api.ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := assertion.GatherThresholdViolations(fixtureMeasurements(), test.threshold, kernel.Should)

			if reported := thresholdSubjectsOf(violations); !slices.Equal(reported, test.want) {
				t.Errorf("`be %s` reported %v, want %v", test.threshold, reported, test.want)
			}
		})
	}
}

func TestGatherThresholdViolationsCarriesTheNumberAndTheComparisonItWasJudgedBy(t *testing.T) {
	// A violation that measured again could disagree with the judgement that produced it, so the number is read
	// once and used twice — for the question and for the report — and the comparison is taken apart into the two
	// halves a report reads.
	violations := assertion.GatherThresholdViolations(
		fixtureMeasurements(), calculation.BelowOrEqual(12), kernel.Should)

	if len(violations) != 1 {
		t.Fatalf("reported %v, want the one class over the limit", thresholdSubjectsOf(violations))
	}
	reported, ok := violations[0].(assertion.ThresholdViolation)
	if !ok {
		t.Fatalf("reported a %T, want a ThresholdViolation", violations[0])
	}
	if reported.Value != 40 {
		t.Errorf("reported the number as %g, want the 40 the metric read", reported.Value)
	}
	if reported.Metric != "method count" {
		t.Errorf("reported the metric as %q, want the words the rule named it in", reported.Metric)
	}
	if reported.Comparison != "below or equal" {
		t.Errorf("reported the comparison as %q, want the words the rule was written with", reported.Comparison)
	}
	if reported.Limit != 12 {
		t.Errorf("reported the limit as %g, want the figure the rule compared against", reported.Limit)
	}
	if reported.Mood != kernel.Should {
		t.Errorf("reported the mood as %s, want the mood the rule was written in", reported.Mood)
	}
}

func TestGatherThresholdViolationsCarriesNoComparisonWordForTheEqualityItself(t *testing.T) {
	// `should be` compares by the equality alone, so there is no word between `be` and the figure — and the
	// violation carries the empty string rather than a word the user never typed.
	violations := assertion.GatherThresholdViolations(
		fixtureMeasurements(), calculation.Exactly(40), kernel.Should)

	for _, violation := range violations {
		reported, ok := violation.(assertion.ThresholdViolation)
		if !ok {
			t.Fatalf("reported a %T, want a ThresholdViolation", violation)
		}
		if reported.Comparison != "" {
			t.Errorf("reported the comparison as %q, want no word at all", reported.Comparison)
		}
		if reported.Limit != 40 {
			t.Errorf("reported the limit as %g, want the figure the number had to be", reported.Limit)
		}
	}
}

func TestGatherThresholdViolationsInTheNegatedMoodReportsTheComplement(t *testing.T) {
	// The negated rule is the same walk with one comparison inverted, so there is no second code path to keep in
	// step — even though the fluent API offers `should` alone. Between the two moods every measurement is
	// reported exactly once, which is what says the mood is a flag rather than a second rule.
	measurements := fixtureMeasurements()
	threshold := calculation.BelowOrEqual(12)

	failing := thresholdSubjectsOf(assertion.GatherThresholdViolations(measurements, threshold, kernel.Should))
	satisfying := thresholdSubjectsOf(assertion.GatherThresholdViolations(measurements, threshold, kernel.ShouldNot))

	if want := []string{"internal/api.Handler"}; !slices.Equal(failing, want) {
		t.Errorf("`should be below or equal 12` reported %v, want %v", failing, want)
	}
	if want := []string{"internal/api.Wide", "internal/api.ID"}; !slices.Equal(satisfying, want) {
		t.Errorf("`should not be below or equal 12` reported %v, want %v", satisfying, want)
	}
	both := slices.Concat(failing, satisfying)
	slices.Sort(both)
	want := []string{"internal/api.Handler", "internal/api.ID", "internal/api.Wide"}
	if !slices.Equal(both, want) {
		t.Errorf("the two moods reported %v between them, want every measurement exactly once, %v", both, want)
	}
	negated, ok := assertion.GatherThresholdViolations(measurements, threshold, kernel.ShouldNot)[0].(assertion.ThresholdViolation)
	if !ok {
		t.Fatal("the negated mood reported something other than a ThresholdViolation")
	}
	if !negated.Mood.Negated() {
		t.Errorf("reported the mood as %s, want the mood the rule was written in", negated.Mood)
	}
}

func TestGatherThresholdViolationsOfASatisfiedRuleReportsNothing(t *testing.T) {
	// No offending measurement is no violations, which is the pass.
	violations := assertion.GatherThresholdViolations(
		fixtureMeasurements(), calculation.BelowOrEqual(400), kernel.Should)

	if len(violations) != 0 {
		t.Errorf("reported %v, want nothing about a rule every number keeps", thresholdSubjectsOf(violations))
	}
}

func TestGatherThresholdViolationsKeepsTheOrderTheMeasurementsArrivedIn(t *testing.T) {
	// The order of a report is the order the metric read its subjects in, so the same rule prints the same list
	// twice.
	violations := assertion.GatherThresholdViolations(
		fixtureMeasurements(), calculation.Above(400), kernel.Should)

	want := []string{"internal/api.Handler", "internal/api.Wide", "internal/api.ID"}
	if reported := thresholdSubjectsOf(violations); !slices.Equal(reported, want) {
		t.Errorf("reported %v, want %v", reported, want)
	}
}

func TestGatherThresholdViolationsOfNoMeasurementsReportsNothing(t *testing.T) {
	// A rule that measured nothing at all is the empty-test guard's answer rather than this one's: every
	// measurement of an empty list is on the right side of every figure, in either mood, so a stale glob would
	// otherwise be green forever.
	for _, mood := range []kernel.Mood{kernel.Should, kernel.ShouldNot} {
		violations := assertion.GatherThresholdViolations(nil, calculation.Below(400), mood)

		if len(violations) != 0 {
			t.Errorf("%s reported %v about no measurements, want nothing", mood, violations)
		}
	}
}

func TestGatherThresholdViolationsReportsANumberOnNoSideOfItsFigure(t *testing.T) {
	// A ratio of nothing over nothing is a real measurement, and it satisfies no comparison at all — so it is
	// reported under `should` rather than quietly passing a threshold it was never in.
	violations := assertion.GatherThresholdViolations(
		[]calculation.Measurement{{Metric: "abstractness", Subject: "internal/api", Value: math.NaN()}},
		calculation.Below(0.5), kernel.Should)

	if reported := thresholdSubjectsOf(violations); !slices.Equal(reported, []string{"internal/api"}) {
		t.Errorf("reported %v, want the package whose number is on no side of the figure", reported)
	}
}

func TestGatherThresholdViolationsWithNoComparisonHoldsForNothing(t *testing.T) {
	// The zero Threshold admits no side of its figure, so a rule written with one reports every measurement under
	// `should` and none under `should not` — the shape a zero Zone gives the zone rules. It cannot arrive from the
	// fluent API, where each of the five verbs names its comparison.
	zero := calculation.Threshold{}

	positive := assertion.GatherThresholdViolations(fixtureMeasurements(), zero, kernel.Should)
	negated := assertion.GatherThresholdViolations(fixtureMeasurements(), zero, kernel.ShouldNot)

	if len(positive) != len(fixtureMeasurements()) {
		t.Errorf("`should` reported %v with no comparison, want every measurement", thresholdSubjectsOf(positive))
	}
	if len(negated) != 0 {
		t.Errorf("`should not` reported %v with no comparison, want nothing", thresholdSubjectsOf(negated))
	}
}

// thresholdSubjectsOf names the subjects a threshold gather reported, in order, for a failure message.
func thresholdSubjectsOf(violations []kernel.Violation) []string {
	reported := make([]string, 0, len(violations))
	for _, violation := range violations {
		if threshold, ok := violation.(assertion.ThresholdViolation); ok {
			reported = append(reported, threshold.Subject)
		}
	}
	return reported
}
