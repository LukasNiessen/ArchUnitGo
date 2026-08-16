package assertion_test

import (
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
)

// ZoneViolation is one of the violations every consumer of a rule programs against, so the interface is
// asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.ZoneViolation{}

func TestZoneViolationIsOfTheMetricsZoneKind(t *testing.T) {
	// The kind names the vocabulary as well as the failure, because the zones are a question any library with
	// a component view can ask, and the testing layer picks a phrasing by this key.
	violation := assertion.NewZoneViolation("internal/db", "zone of pain", 0, 0, kernel.ShouldNot)

	if violation.Kind() != assertion.KindMetricsZone {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindMetricsZone)
	}
	if assertion.KindMetricsZone != "metrics-zone" {
		t.Errorf("KindMetricsZone = %q, want the name the family spells it with", assertion.KindMetricsZone)
	}
}

func TestZoneViolationCarriesTheComponentAndTheCoordinatesThatPutItThere(t *testing.T) {
	// The two numbers are the whole diagnosis: a reader told a package is in the zone of pain still has to
	// know whether the way out is an interface or fewer dependents.
	violation := assertion.NewZoneViolation("internal/db", "zone of pain", 0.1, 0.2, kernel.ShouldNot)

	if violation.Component != "internal/db" {
		t.Errorf("Component = %q, want the folder the rule does not hold for", violation.Component)
	}
	if violation.Zone != "zone of pain" {
		t.Errorf("Zone = %q, want the words the rule was written in", violation.Zone)
	}
	if violation.Abstractness != 0.1 || violation.Instability != 0.2 {
		t.Errorf("the coordinates are (A %g, I %g), want (A 0.1, I 0.2)", violation.Abstractness, violation.Instability)
	}
	if !violation.Mood.Negated() {
		t.Error("Mood is the positive one, want the mood the rule was written in")
	}
}

func TestZoneViolationPrintsTheRequirementAsTheRuleStatedIt(t *testing.T) {
	tests := []struct {
		name      string
		violation assertion.ZoneViolation
		want      string
	}{
		{
			name:      "the zone of pain",
			violation: assertion.NewZoneViolation("internal/db", "zone of pain", 0, 0, kernel.ShouldNot),
			want:      "internal/db: should not, be in zone of pain (abstractness 0, instability 0)",
		},
		{
			name:      "the zone of uselessness",
			violation: assertion.NewZoneViolation("internal/port", "zone of uselessness", 1, 1, kernel.ShouldNot),
			want:      "internal/port: should not, be in zone of uselessness (abstractness 1, instability 1)",
		},
		{
			// The requirement is rendered as the rule stated it rather than as its negation, which is what
			// keeps Mood.Holds the one place in the library that inverts anything.
			name:      "the positive mood",
			violation: assertion.NewZoneViolation(".", "zone of pain", 0.25, 0.5, kernel.Should),
			want:      ".: should, be in zone of pain (abstractness 0.25, instability 0.5)",
		},
		{
			// As many digits as it takes to say exactly which float64 the number is, so a ratio is never
			// quietly rounded in a log line somebody is trying to explain.
			name:      "a ratio that does not divide out",
			violation: assertion.NewZoneViolation("internal/api", "zone of pain", 1.0/3.0, 0, kernel.ShouldNot),
			want:      "internal/api: should not, be in zone of pain (abstractness 0.3333333333333333, instability 0)",
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
