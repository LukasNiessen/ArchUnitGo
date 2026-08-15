package assertion_test

import (
	"math"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
)

// ThresholdViolation is one of the violations every consumer of a rule programs against, so the interface is
// asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.ThresholdViolation{}

func TestThresholdViolationIsOfTheMetricsThresholdKind(t *testing.T) {
	// The kind names the vocabulary as well as the failure, as KindMetricsZone does, because the testing layer
	// picks a phrasing by this key and two families under one name would be two shapes of data.
	violation := assertion.NewThresholdViolation(
		"internal/api/handler.go", "lines of code", 900, "below", 400, kernel.Should)

	if violation.Kind() != assertion.KindMetricsThreshold {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindMetricsThreshold)
	}
	if assertion.KindMetricsThreshold != "metrics-threshold" {
		t.Errorf("KindMetricsThreshold = %q, want the name the family spells it with", assertion.KindMetricsThreshold)
	}
}

func TestTheFiveComparisonsAreOneKindOfViolation(t *testing.T) {
	// One kind and not five: what differs between `should be below` and `should be above or equal` is the words
	// and the side, both of them fields, so a reader who has learned to read one of the five has learned all of
	// them and the testing layer phrases them once.
	for _, comparison := range []string{"below", "above", "", "below or equal", "above or equal"} {
		violation := assertion.NewThresholdViolation("main.go", "lines of code", 3, comparison, 400, kernel.Should)

		if violation.Kind() != assertion.KindMetricsThreshold {
			t.Errorf("a `be %s` violation is of kind %q, want the one kind the family reports",
				comparison, violation.Kind())
		}
	}
}

func TestThresholdViolationCarriesBothHalvesOfTheComparison(t *testing.T) {
	// The number and the figure together are the finding: a reader told only that a file is too long has to go
	// and measure it again to know by how much, and one told only the limit does not know what was found.
	violation := assertion.NewThresholdViolation(
		"internal/api/handler.go", "lines of code", 900, "below or equal", 400, kernel.Should)

	if violation.Subject != "internal/api/handler.go" {
		t.Errorf("Subject = %q, want the subject the number was read off", violation.Subject)
	}
	if violation.Metric != "lines of code" {
		t.Errorf("Metric = %q, want the words the rule named the metric in", violation.Metric)
	}
	if violation.Value != 900 {
		t.Errorf("Value = %g, want the number that was found", violation.Value)
	}
	if violation.Comparison != "below or equal" {
		t.Errorf("Comparison = %q, want the words the rule was written with", violation.Comparison)
	}
	if violation.Limit != 400 {
		t.Errorf("Limit = %g, want the figure the number was compared against", violation.Limit)
	}
	if violation.Mood.Negated() {
		t.Error("Mood is the negated one, want the mood the rule was written in")
	}
}

func TestThresholdViolationPrintsTheRequirementAsTheRuleStatedIt(t *testing.T) {
	tests := []struct {
		name      string
		violation assertion.ThresholdViolation
		want      string
	}{
		{
			name: "a count over its limit",
			violation: assertion.NewThresholdViolation(
				"internal/api/handler.go", "lines of code", 900, "below", 400, kernel.Should),
			want: "internal/api/handler.go: should, be below 400 (lines of code = 900)",
		},
		{
			name: "a count under its floor",
			violation: assertion.NewThresholdViolation(
				"internal/db/conn.go", "statements", 0, "above or equal", 1, kernel.Should),
			want: "internal/db/conn.go: should, be above or equal 1 (statements = 0)",
		},
		{
			// The comparison that is the equality itself has no word of its own, so the figure follows `be`
			// directly rather than after a gap where a word would have been.
			name: "the number that had to be a figure exactly",
			violation: assertion.NewThresholdViolation(
				"internal/api", "abstractness", 0.5, "", 1, kernel.Should),
			want: "internal/api: should, be 1 (abstractness = 0.5)",
		},
		{
			// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
			// Mood.Holds the one place in the library that inverts anything. Only the assertion package can pin
			// this, because no fluent verb passes the negated mood.
			name: "the negated mood",
			violation: assertion.NewThresholdViolation(
				"main.go", "lines of code", 3, "below", 400, kernel.ShouldNot),
			want: "main.go: should not, be below 400 (lines of code = 3)",
		},
		{
			// As many digits as it takes to say exactly which float64 each number is, so neither the figure nor
			// the finding is quietly rounded in a log line somebody is trying to explain.
			name: "a ratio that does not divide out",
			violation: assertion.NewThresholdViolation(
				"internal/api", "instability", 1.0/3.0, "above or equal", 0.1, kernel.Should),
			want: "internal/api: should, be above or equal 0.1 (instability = 0.3333333333333333)",
		},
		{
			name: "a number that is on no side of its figure",
			violation: assertion.NewThresholdViolation(
				"internal/api", "abstractness", math.NaN(), "below", 0.5, kernel.Should),
			want: "internal/api: should, be below 0.5 (abstractness = NaN)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.violation.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}
