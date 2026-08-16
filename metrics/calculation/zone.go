package calculation

import "math"

// zoneExtent is how far from its corner a zone reaches, measured in the abstractness/instability plane.
//
// It is the one number in this family a paper does not fix. Martin names the two corners and says a component
// should stay away from them; how close is too close is a threshold, and this library states it once here
// rather than asking every user to invent one — the same way `have no cycles` does not ask how long a cycle
// has to be to count.
//
// 0.3 marks off a quarter-circle at each corner — about a fourteenth of the plane apiece — and it leaves the
// balanced middle alone. The closest a point on the main sequence can come to either corner is 1/√2 ≈ 0.707, so
// a component on the line is never in a zone, and everything inside one is at least 1/√2 - 0.3 ≈ 0.407 away from
// the line: a normalized distance of about 0.58, well past where a threshold rule would already have complained.
// That is what makes `not in zone of pain` a rule about the corners rather than a second rule about the line.
const zoneExtent = 0.3

// Zone is one of the two corners of the abstractness/instability plane a component should stay out of: what
// it is called, where it is, and how far from it counts as being in it.
//
// The plane is spanned by A and I. The main sequence is the line A + I = 1, the two ends of it are the two
// good extremes — abstract and stable, concrete and unstable — and the two corners the line misses are the
// two ways a package can be wrong at once. Those corners are what this type is:
//
//   - the zone of pain, at A = 0 and I = 0: concrete and depended upon. Nothing about it can change without
//     changing everything that depends on it, and nothing about it is a promise a substitute could keep.
//   - the zone of uselessness, at A = 1 and I = 1: abstract and depended upon by nothing. It is a set of
//     promises nobody asked for, which is dead code with an interface in front of it.
//
// A Zone is data plus one question — Contains — and that question is arithmetic rather than a judgement:
// whether being in a zone is a violation is the mood's business, in metrics/assertion, and the words a report
// says about it are the testing layer's.
//
// A Zone is immutable: get one from ZoneOfPain or ZoneOfUselessness and read it through its methods. The zero
// Zone is nameless and reaches nowhere, so it contains nothing at all — a region nobody built is not a region
// a component can be caught in.
type Zone struct {
	// name is what the zone is called, spelled as the family spells it — `zone of pain`.
	name string
	// abstractness and instability are the corner the zone is measured from: the A and the I a component
	// would have if it were as wrong as this zone can be.
	abstractness float64
	instability  float64
	// extent is how far from that corner the zone reaches, as a straight-line distance in the plane. A zone
	// of no extent is not a region, which is what makes the zero Zone empty rather than a point.
	extent float64
}

// ZoneOfPain is the corner where a component is concrete and everything depends on it: A = 0, I = 0.
//
// It is the expensive corner. Every change to such a package is a change its dependents can see, because
// there is no abstraction between them and it, and it has dependents precisely because it is useful — so the
// cost of the rigidity is paid by the code most likely to be worth changing. The way out is either an
// interface its dependents can be written against, or fewer dependents.
func ZoneOfPain() Zone {
	return Zone{name: "zone of pain", extent: zoneExtent}
}

// ZoneOfUselessness is the corner where a component is abstract and nothing depends on it: A = 1, I = 1.
//
// It is the wasteful corner. Abstraction is a cost paid so that dependents can be written against a promise
// instead of a mechanism, and a package with no dependents has nobody to pay it for — so what is there is
// either dead, or an interface waiting for an implementor that never arrived. The way out is a caller, or a
// deletion.
func ZoneOfUselessness() Zone {
	return Zone{name: "zone of uselessness", abstractness: 1, instability: 1, extent: zoneExtent}
}

// Name is what the zone is called, spelled as the family spells it — `zone of pain`, `zone of uselessness`.
// It is the word the fluent sentence renders and the word a violation carries, and the zero Zone has none.
func (z Zone) Name() string {
	return z.name
}

// Contains reports whether a component at this abstractness and this instability is in the zone: whether it
// is no further than the zone's extent from the zone's corner, in a straight line across the plane.
//
// The distance is Euclidean, which is the same reading DistanceFromMainSequence takes of the same plane —
// a zone is the region around a point exactly as the main sequence is the line between two of them — and it
// is what makes the two zones symmetrical: the corner of the one is at the same distance from the line as the
// corner of the other, so neither zone is stricter than its twin.
//
// The zero Zone contains nothing, whatever it is asked about, and the two zones the library ships never
// overlap: their corners are √2 apart and each reaches less than half of that.
func (z Zone) Contains(abstractness, instability float64) bool {
	if z.extent <= 0 {
		return false
	}
	return math.Hypot(abstractness-z.abstractness, instability-z.instability) <= z.extent
}

// String renders the zone as its name, for logs and test failures.
func (z Zone) String() string {
	return z.name
}
