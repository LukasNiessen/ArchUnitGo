package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

func TestGatherZoneViolationsReportsTheComponentsInTheZoneOfPain(t *testing.T) {
	// `should not be in zone of pain`: one violation per concrete, depended-upon package, and nothing about
	// the ones that are balanced or in the other corner.
	violations := assertion.GatherZoneViolations(fixtureComponents(), calculation.ZoneOfPain(), kernel.ShouldNot)

	want := []string{"internal/db"}
	if reported := componentsOf(violations); !slices.Equal(reported, want) {
		t.Errorf("reported %v, want %v", reported, want)
	}
}

func TestGatherZoneViolationsReportsTheComponentsInTheZoneOfUselessness(t *testing.T) {
	// The same walk with a different corner, which is the whole difference between the two rules.
	violations := assertion.GatherZoneViolations(fixtureComponents(), calculation.ZoneOfUselessness(), kernel.ShouldNot)

	want := []string{"internal/port"}
	if reported := componentsOf(violations); !slices.Equal(reported, want) {
		t.Errorf("reported %v, want %v", reported, want)
	}
}

func TestGatherZoneViolationsCarriesTheCoordinatesItJudgedBy(t *testing.T) {
	// A violation that recalculated its numbers could disagree with the judgement that produced it, so the
	// two coordinates are read once and used twice — for the question and for the report.
	//
	// The component judged here is the one in the zone of pain whose two coordinates differ — A 0, I 0.25,
	// a quarter of the plane's unit from the corner and so still inside it — because the numbers are the
	// whole diagnosis: a report that swapped them would tell the reader to add an interface where the fix
	// is to shed dependents.
	inTheZone := []projection.Component{{
		Label: "internal/rigid", Classes: 4,
		DependsOn: []string{"internal/db"}, DependedOnBy: []string{"a", "b", "c"},
	}}

	violations := assertion.GatherZoneViolations(inTheZone, calculation.ZoneOfPain(), kernel.ShouldNot)

	if len(violations) != 1 {
		t.Fatalf("reported %v, want the one component in the zone of pain", violations)
	}
	reported, ok := violations[0].(assertion.ZoneViolation)
	if !ok {
		t.Fatalf("reported a %T, want a ZoneViolation", violations[0])
	}
	if reported.Abstractness != 0 || reported.Instability != 0.25 {
		t.Errorf("reported (A %g, I %g), want (A 0, I 0.25), where internal/rigid sits",
			reported.Abstractness, reported.Instability)
	}
	if reported.Zone != "zone of pain" {
		t.Errorf("reported the zone as %q, want the words the rule was written in", reported.Zone)
	}
	if !reported.Mood.Negated() {
		t.Error("reported the positive mood, want the mood the rule was written in")
	}
}

func TestGatherZoneViolationsInThePositiveMoodReportsTheComponentsOutsideTheZone(t *testing.T) {
	// The negated rule is the same walk with one comparison inverted, so there is no second code path to keep
	// in step — even though only `should not` is offered by the fluent API.
	components := fixtureComponents()

	violations := assertion.GatherZoneViolations(components, calculation.ZoneOfPain(), kernel.Should)

	if len(violations) != len(components)-1 {
		t.Fatalf("reported %v, want every component except the one in the zone", componentsOf(violations))
	}
	if slices.Contains(componentsOf(violations), "internal/db") {
		t.Error("reported internal/db, which is in the zone the rule demanded")
	}
	// The mood travels into the violation, not just into the comparison: archtest phrases the requirement
	// (`should` vs `should not`) and the finding (`it is` vs `it is not`) off ZoneViolation.Mood, so a
	// violation carrying the wrong mood would be printed as the opposite rule. Only the assertion package can
	// pin this, because no fluent verb passes the positive mood.
	reported, ok := violations[0].(assertion.ZoneViolation)
	if !ok {
		t.Fatalf("reported a %T, want a ZoneViolation", violations[0])
	}
	if reported.Mood != kernel.Should {
		t.Errorf("reported the mood as %s, want the mood the rule was written in", reported.Mood)
	}
}

func TestGatherZoneViolationsOfComponentsOutsideEveryZoneReportsNothing(t *testing.T) {
	// No offending component is no violations, which is the pass.
	balanced := []projection.Component{{
		Label: "internal/api", Classes: 2, Interfaces: 1,
		DependsOn: []string{"internal/db"}, DependedOnBy: []string{"."},
	}}

	violations := assertion.GatherZoneViolations(balanced, calculation.ZoneOfPain(), kernel.ShouldNot)

	if len(violations) != 0 {
		t.Errorf("reported %v, want nothing about a component on the main sequence", componentsOf(violations))
	}
}

func TestGatherZoneViolationsKeepsTheOrderTheComponentsArrivedIn(t *testing.T) {
	// The order of a report is the order of the selection, which SelectComponents already sorted, so the same
	// rule prints the same list twice.
	inTheZone := []projection.Component{
		{Label: "internal/db", Classes: 1, DependedOnBy: []string{"internal/api"}},
		{Label: "cmd/serve", Classes: 2, DependedOnBy: []string{"."}},
		{Label: "internal/util", Classes: 3},
	}

	violations := assertion.GatherZoneViolations(inTheZone, calculation.ZoneOfPain(), kernel.ShouldNot)

	want := []string{"internal/db", "cmd/serve", "internal/util"}
	if reported := componentsOf(violations); !slices.Equal(reported, want) {
		t.Errorf("reported %v, want %v", reported, want)
	}
}

func TestGatherZoneViolationsOfNoComponentsReportsNothing(t *testing.T) {
	// A rule that selected nothing at all is the empty-test guard's answer rather than this one's: no
	// component means no component in a zone, whichever mood the rule was written in.
	for _, mood := range []kernel.Mood{kernel.Should, kernel.ShouldNot} {
		violations := assertion.GatherZoneViolations(nil, calculation.ZoneOfPain(), mood)

		if len(violations) != 0 {
			t.Errorf("%s reported %v about no components, want nothing", mood, violations)
		}
	}
}

func TestGatherZoneViolationsAgainstTheZeroZoneCatchesNothing(t *testing.T) {
	// A region nobody built is not a region a component can be caught in, so `should not` reports nothing and
	// `should` reports everything — the same shape a zero matching.Filter gives the naming rules. It cannot
	// arrive from the fluent API, where each verb names its zone.
	components := fixtureComponents()

	negated := assertion.GatherZoneViolations(components, calculation.Zone{}, kernel.ShouldNot)
	positive := assertion.GatherZoneViolations(components, calculation.Zone{}, kernel.Should)

	if len(negated) != 0 {
		t.Errorf("`should not` reported %v against the zero Zone, want nothing", componentsOf(negated))
	}
	if len(positive) != len(components) {
		t.Errorf("`should` reported %v against the zero Zone, want every component", componentsOf(positive))
	}
}

// fixtureComponents is a selection holding one component in each zone and two in neither, hand-built so that
// the judgement can be tested with no project on disk:
//
//	internal/db    A 0,   I 0    the zone of pain — concrete and depended upon
//	internal/port  A 1,   I 1    the zone of uselessness — abstract and depended on by nothing
//	internal/api   A 0.5, I 0.5  on the main sequence
//	.              A 0,   I 1    the good concrete corner, at the end of the main sequence
func fixtureComponents() []projection.Component {
	return []projection.Component{
		{Label: ".", Classes: 1, DependsOn: []string{"internal/api"}},
		{
			Label: "internal/api", Classes: 2, Interfaces: 1,
			DependsOn: []string{"internal/db"}, DependedOnBy: []string{"."},
		},
		{Label: "internal/db", Classes: 3, DependedOnBy: []string{"internal/api"}},
		{Label: "internal/port", Classes: 2, Interfaces: 2, DependsOn: []string{"internal/db"}},
	}
}

// componentsOf names the components a gather reported, in order, for a failure message.
func componentsOf(violations []kernel.Violation) []string {
	reported := make([]string, 0, len(violations))
	for _, violation := range violations {
		if zone, ok := violation.(assertion.ZoneViolation); ok {
			reported = append(reported, zone.Component)
		}
	}
	return reported
}
