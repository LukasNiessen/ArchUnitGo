package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// patternModifier is which of this module's four pattern modifiers a chain wrote most recently: `focus on`,
// `reachable from`, `dependents of` or `collapse by pattern`. It is what an `except` qualifies, and it is a
// field of the builder because the four of them append to four different fields of the query — nothing in a
// resolved query says which of them was written last.
//
// The modifiers that take no pattern do not clear it. `titled` and `including external dependencies` are not
// selectors, so an exclusion after one of them still qualifies the pattern it reads as following.
type patternModifier int

const (
	// noPatternModifier is a report no pattern modifier has been written on, where an `except` has nothing to
	// qualify. It is the zero value, which is what `project graph` on its own has to be.
	noPatternModifier patternModifier = iota
	focusedOn
	reachedFrom
	dependedOnBy
	collapsedByPattern
)

// Except takes the nodes these patterns name back out of the pattern modifier it follows: `project graph,
// focus on "app/**" within 1 hop, except "**/generated/**"`.
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		FocusOn("app/**", 1).
//		Except("**/generated/**").
//		Snapshot()
//
// It is the companion every selector in this library has, said here about a report: the generated package, the
// vendored copy and the mock folder are what make a diagram of a system unreadable, and taking them out is one
// call rather than a pattern that enumerates the folders a reader does want. `collapse by pattern "third
// party" "**", except "github.com/our-org/**"` is the same verb doing the other job — a catch-all group with
// the one thing that does not belong in it named out loud.
//
// The patterns are read against the whole identifier, as every pattern of this module is, which is why there
// are no targeted forms of the verb here: `except in path` would be a second spelling of this one. They name
// nodes outside the project too, so an exclusion of a dependency module works exactly as an exclusion of the
// project's own folder does.
//
// It qualifies the pattern modifier the chain wrote most recently — the last `focus on`, `reachable from`,
// `dependents of` or `collapse by pattern` — and it is repeatable: several patterns in one call, or several
// calls, all veto. Two mistakes are reported by the terminal as a UserError, the way a pattern that will not
// compile is: `except` before any pattern modifier, which is an exclusion with nothing to qualify, and
// `except` with no pattern at all.
//
// An exclusion narrows what the modifier it qualifies selects, and not the report as a whole. Excluding a
// folder from a `focus on` does not hide a node the focus keeps as a *neighbor* of something else it selected
// — that node is one hop from code the report is about, which is what the focus asked to be shown. The
// modifier that hides a node from the whole report is a `focus on` of its own, or a pattern that never names
// it.
func (b GraphBuilder) Except(patterns ...string) GraphBuilder {
	excepted := b.modifying()
	qualified := lastSelector(&excepted.query, b.qualified)
	var selectors []matching.Filter
	if qualified != nil {
		selectors = []matching.Filter{*qualified}
	}
	narrowed, err := matching.Excepting(selectors, b.factory, patterns, nil)
	if err != nil {
		return b.rejecting("except", strings.Join(patterns, ", "), err)
	}
	*qualified = narrowed[0]
	return excepted
}

// lastSelector is the selector of the pattern modifier written most recently, to be read and written through:
// the one an `except` qualifies. It is nil when the report has no such modifier, which matching.Excepting
// turns into the exclusion-without-a-selector error.
//
// It is a function of a query rather than a method of the builder because a method that writes into one has to
// take a pointer receiver, and every other method of GraphBuilder takes a value — a type with both is a
// finding, and the value receivers are what makes a builder immutable. The pointer is into the caller's own
// resolved query, which GraphBuilder.modifying has already cloned the slices of.
func lastSelector(query *projection.SnapshotOptions, qualified patternModifier) *matching.Filter {
	switch qualified {
	case focusedOn:
		if len(query.Focus) > 0 {
			return &query.Focus[len(query.Focus)-1].Selector
		}
	case reachedFrom:
		if len(query.ReachableFrom) > 0 {
			return &query.ReachableFrom[len(query.ReachableFrom)-1]
		}
	case dependedOnBy:
		if len(query.DependentsOf) > 0 {
			return &query.DependentsOf[len(query.DependentsOf)-1]
		}
	case collapsedByPattern:
		if len(query.CollapseGroups) > 0 {
			return &query.CollapseGroups[len(query.CollapseGroups)-1].Selector
		}
	case noPatternModifier:
	}
	return nil
}
