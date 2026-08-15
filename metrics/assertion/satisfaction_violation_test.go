package assertion_test

import (
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
)

// SatisfactionViolation is one of the violations every consumer of a rule programs against, so the interface
// is asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.SatisfactionViolation{}

func TestSatisfactionViolationIsOfTheMetricsSatisfactionKind(t *testing.T) {
	// The kind names the vocabulary as well as the failure, as KindMetricsZone does, because the testing layer
	// picks a phrasing by this key and two families under one name would be two shapes of data.
	violation := assertion.NewSatisfactionViolation(
		"internal/api.Handler", "method count", 40, "be at most 10 methods wide", kernel.Should)

	if violation.Kind() != assertion.KindMetricsSatisfaction {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindMetricsSatisfaction)
	}
	if assertion.KindMetricsSatisfaction != "metrics-satisfaction" {
		t.Errorf("KindMetricsSatisfaction = %q, want the name the family spells it with",
			assertion.KindMetricsSatisfaction)
	}
}

func TestSatisfactionViolationCarriesTheNumberAndTheWordsItWasAskedIn(t *testing.T) {
	// The number is the whole diagnosis: a reader told their own predicate said no still has to know what the
	// metric actually came to, and the requirement is the only readable form the predicate has.
	violation := assertion.NewSatisfactionViolation(
		"internal/api.Handler", "public surface", 26, "expose at most 20 methods and fields", kernel.Should)

	if violation.Subject != "internal/api.Handler" {
		t.Errorf("Subject = %q, want the subject the number was read off", violation.Subject)
	}
	if violation.Metric != "public surface" {
		t.Errorf("Metric = %q, want the words the rule named the metric in", violation.Metric)
	}
	if violation.Value != 26 {
		t.Errorf("Value = %g, want the number that was found", violation.Value)
	}
	if violation.Requirement != "expose at most 20 methods and fields" {
		t.Errorf("Requirement = %q, want the user's own words", violation.Requirement)
	}
	if violation.Mood.Negated() {
		t.Error("Mood is the negated one, want the mood the rule was written in")
	}
}

func TestSatisfactionViolationPrintsTheRequirementAsTheRuleStatedIt(t *testing.T) {
	tests := []struct {
		name      string
		violation assertion.SatisfactionViolation
		want      string
	}{
		{
			name: "a count that is too high",
			violation: assertion.NewSatisfactionViolation(
				"internal/api.Handler", "method count", 40, "be at most 10 methods wide", kernel.Should),
			want: `internal/api.Handler: should, satisfy "be at most 10 methods wide" (method count = 40)`,
		},
		{
			// The requirement is rendered as the rule stated it rather than as its negation, which is what
			// keeps Mood.Holds the one place in the library that inverts anything. Only the assertion package
			// can pin this, because no fluent verb passes the negated mood.
			name: "the negated mood",
			violation: assertion.NewSatisfactionViolation(
				"main.go", "lines of code", 3, "be a stub", kernel.ShouldNot),
			want: `main.go: should not, satisfy "be a stub" (lines of code = 3)`,
		},
		{
			// As many digits as it takes to say exactly which float64 the number is, so a ratio is never
			// quietly rounded in a log line somebody is trying to explain.
			name: "a ratio that does not divide out",
			violation: assertion.NewSatisfactionViolation(
				"internal/api", "abstractness", 1.0/3.0, "be mostly concrete", kernel.Should),
			want: `internal/api: should, satisfy "be mostly concrete" (abstractness = 0.3333333333333333)`,
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
