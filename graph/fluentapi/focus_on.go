package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// FocusOn narrows the report to the files this pattern names together with their neighborhood, depth hops
// out in both directions:
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		FocusOn("internal/api/**", 1).
//		Snapshot()
//
// It is how a diagram of four hundred files becomes a diagram about one part of a system: depth 0 is those
// files alone, 1 adds everything one dependency away on either side, and so on. Both directions, because
// `what is around this code` is not the same question as `what does it depend on` — following the arrows one
// way only is ReachableFrom and DependentsOf, and those follow them all the way.
//
// The pattern is matched against the whole identifier, so `internal/api/**` is that folder and everything
// below it, and `**/handler.go` is every file with that name. It matches identifiers, never the labels a
// collapse draws, so focusing and collapsing in the same chain means the neighbors of the *files* named,
// drawn as folders — which is the only reading in which the two modifiers stay order-independent.
//
// A negative depth means 0 rather than the whole graph: a mistyped depth should narrow a report, never blow
// it up.
//
// The modifier is chainable and order-independent. Two focuses narrow the report to the nodes both of them
// name, as every modifier of this library narrows rather than widens, so `focus on "internal/**" within 0
// hops` and `focus on "**/*_gen.go" within 0 hops` together are the generated files under internal — and two
// focuses on unrelated folders are a report of nothing, which the terminal reports rather than renders.
func (b GraphBuilder) FocusOn(pattern string, depth int) GraphBuilder {
	selector, err := b.factory.PathMatcher(pattern)
	if err != nil {
		return b.rejecting("focus on", pattern, err)
	}
	focused := b.modifying()
	focused.qualified = focusedOn
	focused.query.Focus = append(focused.query.Focus, projection.Focus{Selector: selector, Depth: depth})
	return focused
}

// ReachableFrom narrows the report to the files this pattern names and everything they depend on,
// transitively:
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		ReachableFrom("cmd/**").
//		IncludingExternalDependencies().
//		Snapshot()
//
// It is the `what does this pull in` report — what a binary actually reaches, how far a package's imports
// spread — and it follows the arrows forwards with no bound, so a chain of ten dependencies is all ten. The
// named files are in the result, because a report about what code depends on that does not show the code is
// missing its subject.
//
// The pattern is matched against the whole identifier. A cycle is no problem: a node is reached once.
//
// The modifier is chainable and order-independent, and two of them narrow the report to what both sets reach —
// the shared dependencies of two entry points, which is a useful report on its own.
func (b GraphBuilder) ReachableFrom(pattern string) GraphBuilder {
	selector, err := b.factory.PathMatcher(pattern)
	if err != nil {
		return b.rejecting("reachable from", pattern, err)
	}
	reaching := b.modifying()
	reaching.qualified = reachedFrom
	reaching.query.ReachableFrom = append(reaching.query.ReachableFrom, selector)
	return reaching
}

// DependentsOf narrows the report to the files this pattern names and everything that depends on them,
// transitively:
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		DependentsOf("internal/db/**").
//		Snapshot()
//
// It is ReachableFrom with the arrows followed backwards, and it is the impact-analysis report: who would
// notice if this code changed. A folder nothing depends on is a report of itself alone, which is the answer
// worth knowing before deleting it.
//
// The pattern is matched against the whole identifier, the named files are in the result, and the traversal is
// unbounded.
//
// The modifier is chainable and order-independent. Combined with ReachableFrom in one chain it is the
// `everything between these two` report: the nodes that are both downstream of one pattern and upstream of
// another.
func (b GraphBuilder) DependentsOf(pattern string) GraphBuilder {
	selector, err := b.factory.PathMatcher(pattern)
	if err != nil {
		return b.rejecting("dependents of", pattern, err)
	}
	depending := b.modifying()
	depending.qualified = dependedOnBy
	depending.query.DependentsOf = append(depending.query.DependentsOf, selector)
	return depending
}
