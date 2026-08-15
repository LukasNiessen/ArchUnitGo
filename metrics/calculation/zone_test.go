package calculation_test

import (
	"math"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

func TestTheZoneOfPainIsTheConcreteAndDependedUponCorner(t *testing.T) {
	zone := calculation.ZoneOfPain()

	tests := []struct {
		name         string
		abstractness float64
		instability  float64
		want         bool
	}{
		{name: "the corner itself", abstractness: 0, instability: 0, want: true},
		{name: "nearly at the corner", abstractness: 0.1, instability: 0.1, want: true},
		{name: "as far along one axis as the zone reaches", abstractness: 0, instability: 0.3, want: true},
		{name: "just past the reach of one axis", abstractness: 0, instability: 0.31, want: false},
		// Euclidean and not per-axis: both coordinates are inside 0.3 on their own, and the point is still
		// 0.3√2 ≈ 0.424 from the corner.
		{name: "inside on both axes but outside in the plane", abstractness: 0.3, instability: 0.3, want: false},
		{name: "on the main sequence", abstractness: 0.5, instability: 0.5, want: false},
		{name: "in the other corner", abstractness: 1, instability: 1, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := zone.Contains(test.abstractness, test.instability); got != test.want {
				t.Errorf("%s contains (A %g, I %g) = %t, want %t", zone, test.abstractness, test.instability, got, test.want)
			}
		})
	}
}

func TestTheZoneOfUselessnessIsTheAbstractAndUnusedCorner(t *testing.T) {
	zone := calculation.ZoneOfUselessness()

	tests := []struct {
		name         string
		abstractness float64
		instability  float64
		want         bool
	}{
		{name: "the corner itself", abstractness: 1, instability: 1, want: true},
		{name: "nearly at the corner", abstractness: 0.9, instability: 0.9, want: true},
		// Along one axis rather than at the corner, and comfortably inside: 1 - 0.7 is 0.30000000000000004 in
		// float64, so the boundary itself is only worth asserting where the corner is 0 and the subtraction is
		// exact, which is the zone of pain's test above.
		{name: "well along one axis", abstractness: 1, instability: 0.75, want: true},
		{name: "past the reach of one axis", abstractness: 1, instability: 0.69, want: false},
		{name: "on the main sequence", abstractness: 0.5, instability: 0.5, want: false},
		{name: "in the other corner", abstractness: 0, instability: 0, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := zone.Contains(test.abstractness, test.instability); got != test.want {
				t.Errorf("%s contains (A %g, I %g) = %t, want %t", zone, test.abstractness, test.instability, got, test.want)
			}
		})
	}
}

func TestNoComponentOnTheMainSequenceIsInEitherZone(t *testing.T) {
	// The closest a point on the line A + I = 1 can come to either corner is 1/√2 ≈ 0.707, so a component that
	// is as abstract as its dependents' need for stability demands is never reported — which is what makes
	// these rules about the corners rather than a second rule about the line.
	for step := 0; step <= 100; step++ {
		abstractness := float64(step) / 100
		instability := 1 - abstractness
		for _, zone := range []calculation.Zone{calculation.ZoneOfPain(), calculation.ZoneOfUselessness()} {
			if zone.Contains(abstractness, instability) {
				t.Errorf("%s contains (A %g, I %g), which is on the main sequence", zone, abstractness, instability)
			}
		}
	}
}

func TestTheTwoZonesNeverOverlap(t *testing.T) {
	// Their corners are √2 apart and each reaches less than half of that, so no component is ever reported by
	// both rules — and a reader told a package is in one of them knows which way out to take.
	pain, uselessness := calculation.ZoneOfPain(), calculation.ZoneOfUselessness()

	for abstractnessStep := 0; abstractnessStep <= 20; abstractnessStep++ {
		for instabilityStep := 0; instabilityStep <= 20; instabilityStep++ {
			abstractness, instability := float64(abstractnessStep)/20, float64(instabilityStep)/20
			if pain.Contains(abstractness, instability) && uselessness.Contains(abstractness, instability) {
				t.Errorf("(A %g, I %g) is in both zones", abstractness, instability)
			}
		}
	}
}

func TestTheTwoZonesAreEquallyStrict(t *testing.T) {
	// Symmetry, by construction: the corner of the one is as far from the main sequence as the corner of the
	// other, so neither rule is harsher than its twin.
	pain, uselessness := calculation.ZoneOfPain(), calculation.ZoneOfUselessness()

	for step := 0; step <= 20; step++ {
		along := float64(step) / 20
		// (A, I) in the zone of pain has its mirror image (1 - A, 1 - I) in the zone of uselessness.
		inPain := pain.Contains(along, along/2)
		inUselessness := uselessness.Contains(1-along, 1-along/2)
		if inPain != inUselessness {
			t.Errorf("(A %g, I %g) is in the zone of pain = %t, but its mirror image is in the zone of "+
				"uselessness = %t", along, along/2, inPain, inUselessness)
		}
	}
}

func TestTheZonesAreNamedAsTheFamilySpellsThem(t *testing.T) {
	// The two names are the words the fluent sentence renders and the words a violation carries, so they are
	// part of the surface.
	if got := calculation.ZoneOfPain().Name(); got != "zone of pain" {
		t.Errorf("the zone of pain is called %q, want %q", got, "zone of pain")
	}
	if got := calculation.ZoneOfUselessness().Name(); got != "zone of uselessness" {
		t.Errorf("the zone of uselessness is called %q, want %q", got, "zone of uselessness")
	}
	if got := calculation.ZoneOfPain().String(); got != "zone of pain" {
		t.Errorf("the zone of pain renders as %q, want %q", got, "zone of pain")
	}
}

func TestTheZeroZoneContainsNothing(t *testing.T) {
	// A region nobody built is not a region a component can be caught in, so a rule written with one reports
	// every component under `should` and none under `should not` — and it cannot arrive from the fluent API,
	// where each verb names its zone.
	zero := calculation.Zone{}

	if zero.Name() != "" {
		t.Errorf("the zero Zone is called %q, want no name at all", zero.Name())
	}
	for _, abstractness := range []float64{0, 0.5, 1} {
		for _, instability := range []float64{0, 0.5, 1} {
			if zero.Contains(abstractness, instability) {
				t.Errorf("the zero Zone contains (A %g, I %g), want a region that catches nothing",
					abstractness, instability)
			}
		}
	}
}

func TestAZoneReachesAsFarAsItsExtentInEveryDirection(t *testing.T) {
	// The reach is a straight line across the plane, so the boundary is a circle rather than a square: a point
	// at the extent along either axis is in, and the diagonal at the same per-axis offset is out.
	zone := calculation.ZoneOfPain()
	extent := 0.3

	if !zone.Contains(extent, 0) || !zone.Contains(0, extent) {
		t.Errorf("%s does not reach %g along an axis, want it to", zone, extent)
	}
	if zone.Contains(math.Nextafter(extent, 1), 0) {
		t.Errorf("%s reaches past %g along an axis, want it not to", zone, extent)
	}
}
