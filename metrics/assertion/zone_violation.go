// Package assertion is the metrics module's half of the ASSERT stage: it judges the numbers a project's
// files, classes and packages come to and reports one violation per subject that disagrees with a rule about
// them.
//
// ZoneViolation and SatisfactionViolation are its violation types, and GatherZoneViolations and
// GatherSatisfactionViolations the two functions that make one, which is the shape every assertion package in
// the library has: one type per rule family, one `gather <thing> violations` per predicate, and no other way
// for a violation of that family to exist. Both halves are data. A violation says which component was where
// in the abstractness/instability plane, or which number a subject came to and what was asked of it, and not
// a word about it, because message construction belongs to the testing layer, where one place controls
// phrasing, numbering and color.
//
// The mood travels here as it does everywhere, even though the metrics family spells it inside the predicate
// — `should not be in zone of pain` and `should satisfy` are one verb each rather than a mood stage and a
// predicate, for the reason AGENTS.md gives about the layer clauses. What the fluent API fuses, this package
// still takes apart: both gather functions ask the positive question of every subject and hand the answer to
// assertion.Mood.Holds, so there is no negative code path to keep in step with the positive one even though
// only one of the two moods is offered.
//
// The package is pure, like every assertion package in the library: no filesystem, no clock, no globals, and
// nothing in it knows Go. It takes the subjects metrics/projection selected, the arithmetic
// metrics/calculation defines and the class facts metrics/extraction wrote down, and hands back a
// []assertion.Violation — so a rule's judgement can be tested against a hand-built subject before any project
// is extracted at all.
package assertion

import (
	"strconv"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// KindMetricsZone is the kind of ZoneViolation: a package sitting in one of the two corners of the
// abstractness/instability plane.
//
// It names the vocabulary as well as the failure, the way `file-cycle` does, because the zones are a question
// that can be asked of any vocabulary a library grows a component view for. The kind is what the testing layer
// picks a phrasing by, so two families sharing one name would be two shapes of data under one key.
const KindMetricsZone kernel.ViolationKind = "metrics-zone"

// ZoneViolation says that one of the packages a rule selected is in the zone of pain or in the zone of
// uselessness — concrete and depended upon, or abstract and depended upon by nothing.
//
// It is what `metrics, ..., distance, should not be in zone of pain` reports, one per offending component, and
// it carries the two numbers that put it there rather than a sentence about them. That is the whole diagnosis:
// a reader who is told a package is in the zone of pain still has to know whether the way out is an interface
// or fewer dependents, and abstractness and instability are which.
type ZoneViolation struct {
	// Component is the label of the package the rule does not hold for, as metrics/projection spells it —
	// `internal/db`, and `.` for the project root.
	Component string
	// Zone is what the zone is called, in the words the rule was written in — `zone of pain`,
	// `zone of uselessness`.
	//
	// It is the name and not the region, for the reason AdherenceViolation carries its requirement as a
	// string: the testing layer phrases this violation and may not import the module's arithmetic, so what
	// crosses into a report is the word the user typed. Which region that word means is
	// calculation.Zone's business, one layer down, and it has already been answered by the time a violation
	// exists.
	Zone string
	// Abstractness is A of the component, the share of its declared types that are interfaces: 0 for a
	// package of nothing but concrete types, 1 for a package of nothing but interfaces.
	Abstractness float64
	// Instability is I of the component, the share of its couplings that are its own outgoing ones: 0 for a
	// package nothing but its own dependents can break, 1 for one nothing depends on.
	Instability float64
	// Mood is which way round the requirement was written. It is `should not` for both predicates this family
	// offers, and it is carried rather than assumed so that a report reads the requirement off the violation
	// exactly as it does for every other family.
	Mood kernel.Mood
}

// NewZoneViolation records that this component is in this zone, at this point of the plane, under a rule
// written in this mood.
//
// It is the only way a violation of this family is made, and every field of it is immutable: two strings, two
// numbers and a flag. The Zone value it was judged against is deliberately not among them — a violation is a
// value a report reads, and the region has already done its work.
func NewZoneViolation(component, zone string, abstractness, instability float64, mood kernel.Mood) ZoneViolation {
	return ZoneViolation{
		Component:    component,
		Zone:         zone,
		Abstractness: abstractness,
		Instability:  instability,
		Mood:         mood,
	}
}

// Kind is KindMetricsZone.
func (ZoneViolation) Kind() kernel.ViolationKind {
	return KindMetricsZone
}

// String renders the violation as the offending package, the requirement it broke in the words the rule was
// written in, and the point of the plane it is at — `internal/db: should not, be in zone of pain (abstractness
// 0, instability 0)` — for a log line or a test failure.
//
// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. The numbers are printed with as
// many digits as it takes to say exactly which float64 they are, for the reason calculation.Measurement's own
// rendering does. The user-facing message is still the testing layer's to build, from these same fields.
func (v ZoneViolation) String() string {
	return v.Component + ": " + v.Mood.String() + ", be in " + v.Zone +
		" (abstractness " + format(v.Abstractness) + ", instability " + format(v.Instability) + ")"
}

// format renders one of the two coordinates the shortest way that still says exactly which float64 it is, so
// that a whole number reads as `0` rather than as `0.000000` and a ratio is never quietly rounded into a
// different number in a log line somebody is trying to explain.
func format(coordinate float64) string {
	return strconv.FormatFloat(coordinate, 'g', -1, 64)
}
