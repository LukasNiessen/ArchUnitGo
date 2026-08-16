package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

// GatherZoneViolations judges the two rules about where a project's packages sit in the abstractness/
// instability plane — `not in zone of pain`, `not in zone of uselessness` — in either mood: one ZoneViolation
// per selected component the rule does not hold for, in the order the components arrived, which is the sorted
// order projection.SelectComponents produced them in.
//
// No offending component is no violations, which is the pass. A rule that selected nothing at all is the
// empty-test guard's answer rather than this one's: no component means no component in a zone, so a stale glob
// would otherwise be green forever whichever mood it was written in.
//
// One function serves both zones and both moods, and it is the same walk either way. Which region is being
// asked about is the calculation.Zone passed in — the zone of pain and the zone of uselessness differ here by
// nothing but that value — and the mood is assertion.Mood.Holds over the one question the zone answers, so
// there is no negative code path to keep in step with the positive one:
//
//	should      violates when the component is not in the zone
//	should not  violates when it is
//
// Only `should not` is offered by the fluent API, because `should be in zone of pain` is a rule demanding that
// a package be badly designed. The mood is still threaded through rather than assumed, for the reason the
// library threads it everywhere: the day a family wants the other polarity, the assertion is already written,
// and until then this function has no branch that only one caller ever takes.
//
// The zero calculation.Zone contains nothing, so a rule written with one reports every selected component
// under `should` and none under `should not` — which is the same shape a zero matching.Filter gives the naming
// rules, and it cannot arrive from the fluent API, where each of the two verbs names its zone.
func GatherZoneViolations(components []projection.Component, zone calculation.Zone, mood kernel.Mood) []kernel.Violation {
	violations := make([]kernel.Violation, 0, len(components))
	for _, component := range components {
		// Both coordinates are read once and used twice — for the question and for the report — because a
		// violation that recalculated them could disagree with the judgement that produced it.
		abstractness := calculation.AbstractnessOf(component)
		instability := calculation.InstabilityOf(component)
		if mood.Holds(zone.Contains(abstractness, instability)) {
			continue
		}
		violations = append(violations, NewZoneViolation(component.Label, zone.Name(), abstractness, instability, mood))
	}
	return violations
}
