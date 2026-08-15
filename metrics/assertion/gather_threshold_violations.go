package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// GatherThresholdViolations judges the five threshold predicates that compare a measurement against a figure —
// `should be below`, `should be above`, `should be`, `should be below or equal`, `should be above or equal` — in
// either mood: one ThresholdViolation per measurement that is not what the comparison required, in the order
// the measurements arrived, which is the order the metric read its subjects in.
//
// No offending measurement is no violations, which is the pass. A rule that measured nothing at all is the
// empty-test guard's answer rather than this one's: every measurement of an empty list is on the right side of
// every figure, in either mood, so a stale glob would otherwise be green forever.
//
// One function serves all five comparisons and both moods, and it is the same walk either way. Which comparison
// is being asked is the calculation.Threshold passed in — the five differ here by nothing but that value, which
// is what says none of them is a synonym of another — and the mood is assertion.Mood.Holds over the one question
// the threshold answers, so there is no negative code path to keep in step with the positive one:
//
//	should      violates when the number is not on the side of the figure the comparison admits
//	should not  violates when it is
//
// Only `should` is offered by the fluent API, because each of the six threshold predicates AGENTS.md fixes
// spells its own mood and five of them are these. The mood is still threaded through rather than assumed, for
// the reason the library threads it everywhere: the day a family wants the other polarity, the assertion is
// already written, and until then this function has no branch that only one caller ever takes.
//
// The zero calculation.Threshold holds for no number, so a rule written with one reports every measurement
// under `should` and none under `should not` — which is the same shape the zero Zone gives the two zone rules,
// and it cannot arrive from the fluent API, where each of the five verbs names its comparison.
func GatherThresholdViolations(
	measurements []calculation.Measurement,
	threshold calculation.Threshold,
	mood kernel.Mood,
) []kernel.Violation {
	violations := make([]kernel.Violation, 0, len(measurements))
	for _, measurement := range measurements {
		if mood.Holds(threshold.Holds(measurement.Value)) {
			continue
		}
		// The comparison is taken apart into the words and the figure here, once, because a violation is data a
		// report reads and the arithmetic has already done its work by the time one exists.
		violations = append(violations, NewThresholdViolation(
			measurement.Subject, measurement.Metric, measurement.Value,
			threshold.Comparison(), threshold.Limit(), mood))
	}
	return violations
}
