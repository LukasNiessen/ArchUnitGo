package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// GatherDependencyViolations judges `contain dependency(from, to)` over a slicing: at most one
// DependencyViolation, reported when the projection has that dependency and the mood forbade it, or does
// not have it and the mood required it.
//
// One question, one answer. The predicate is whether the projected dependencies hold an edge from the slice
// called from to the one called to, and assertion.Mood.Holds is the whole of the difference between the two
// moods — which is why a forbidden dependency and a required one need no second walk and no second type.
// The violation of the negated mood carries the concrete file dependencies the pair of slices stood for;
// the violation of the positive mood carries none, because what it reports is that there were none.
//
// The dependencies arrive as the projected edges of projection.SliceByCapture and its siblings — one per
// pair of slices, cumulating the file dependencies that produced it — so two facts are already true of the
// input: a dependency inside one slice is not in it, and neither is one either of whose ends is in no
// slice. A rule about a slice depending on itself is therefore one nobody can obey, and it is rejected
// where it is written, in fluentapi, rather than reported as a violation here.
//
// A slicing that found no slice at all is the empty-test guard's answer rather than this one's: an empty
// projection breaks nothing a negated rule can be broken by, so a renamed folder would otherwise turn a
// whole rule green forever. That guard runs before this function, in the rule's terminal.
func GatherDependencyViolations(
	from, to string,
	dependencies []kernelprojection.ProjectedEdge,
	mood kernel.Mood,
) []kernel.Violation {
	violations := make([]kernel.Violation, 0, 1)

	found, contains := findDependency(from, to, dependencies)
	if mood.Holds(contains) {
		return violations
	}
	return append(violations, NewDependencyViolation(from, to, mood, found.CumulatedEdges()...))
}

// findDependency is the projected dependency between these two slices, and whether the projection has one
// at all. It is the rule's whole predicate, written once so that both moods ask exactly the same question.
//
// A linear scan is the right shape here: the projection holds one edge per pair of slices that depend on
// each other, a rule asks about one pair, and building an index for a single lookup would cost more than
// the walk it replaced.
func findDependency(
	from, to string,
	dependencies []kernelprojection.ProjectedEdge,
) (kernelprojection.ProjectedEdge, bool) {
	for _, dependency := range dependencies {
		if dependency.SourceLabel() == from && dependency.TargetLabel() == to {
			return dependency, true
		}
	}
	return kernelprojection.ProjectedEdge{}, false
}
