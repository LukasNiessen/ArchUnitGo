package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
)

// GatherCycleViolations judges `project files, ..., should, have no cycles`: one CycleViolation per
// circular chain of dependencies between the files the rule selected, in the order the enumeration
// reported them — shortest cycle first, because that is the smallest thing to fix.
//
// No cycles is no violations, which is the pass. There is no boolean beside the list, and a rule that
// selected nothing at all is the empty-test guard's answer rather than this one's: a projection with no
// edges has no cycles, so a stale glob would otherwise be green forever.
//
// The circuits come from cycles.ProjectCircuits over the projected edges of
// projection.PerSelectedFileEdge, which is the vocabulary this violation speaks — files, labeled by
// their identifiers. Passing the strongly connected components of cycles.ProjectCycles instead would
// name the region of the graph the cycle lives in rather than the cycle, and it is the cycle a reader
// has to break.
//
// # Why this one takes no mood
//
// Every other `gather <thing> violations` function in the library takes an assertion.Mood, so that a
// rule and its negation are one walk over one structure. `have no cycles` is the predicate that has no
// negation to thread: `should not have no cycles` demands that the files be cyclic, and a rule like that
// fails exactly when there is no cycle — with nothing to report but the absence of one, which is not
// data a violation can carry. So the fluent API offers this predicate on the positive mood alone,
// FilesShouldNotBuilder has no HaveNoCycles for the type system to refuse, and the flag would be a
// parameter no caller could vary.
func GatherCycleViolations(circuits []cycles.Circuit) []kernel.Violation {
	violations := make([]kernel.Violation, 0, len(circuits))
	for _, circuit := range circuits {
		violations = append(violations, NewCycleViolation(circuit))
	}
	return violations
}
