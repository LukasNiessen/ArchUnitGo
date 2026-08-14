package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

// FilesCyclesCondition is the terminal of a rule about circular dependencies between files —
// `project files, in folder "internal/**", should, have no cycles` — and it is a fluentapi.Checkable,
// which is the one thing every consumer of a rule programs against:
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/**").Should().HaveNoCycles()
//	violations, err := rule.Check(nil)
//
// It is what FilesShouldBuilder.HaveNoCycles returns, it carries the scope and the mood it was asked of
// unchanged, and it is immutable like every stage before it — so a rule can be stored, passed to a
// helper and checked as often as it is useful. Nothing has been read when it is built: the project is
// located, extracted, projected and judged by Check, and by nothing else.
//
// The predicate has no object stage. `have no cycles` is a sentence on its own — the files it is about
// are the ones the scope named — so this terminal is the end of the chain.
type FilesCyclesCondition struct {
	rule filesRule
}

// HaveNoCycles is the predicate that forbids circular dependencies: `have no cycles`.
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/**").Should().HaveNoCycles()
//
// It is about the dependencies *between* the files the scope selected, so a cycle that leaves the scope
// and comes back through a file the rule did not select is not this rule's cycle — widening the scope is
// what makes it visible. projection.PerSelectedFileEdge is that reading, written out.
//
// It exists on the positive mood alone, which is why FilesShouldNotBuilder has no method of this name:
// `should not have no cycles` is a double negative demanding that the files be cyclic, and a rule like
// that fails with nothing to report but the absence of a cycle. The files module's own
// assertion.GatherCycleViolations says the whole of why.
func (b FilesShouldBuilder) HaveNoCycles() FilesCyclesCondition {
	// The field is named rather than converted from the builder: the two types have one field of the same
	// type today by coincidence, and the next predicate — one that takes an object — gives its condition a
	// second field, at which point a conversion would stop compiling and a reader would have to work out
	// what it had meant.
	//nolint:staticcheck // S1016: the shape these two types share is a coincidence, not a kinship.
	return FilesCyclesCondition{rule: b.rule}
}

// Check runs the rule: one violation per circular chain of dependencies between the files it selected,
// and an empty result when there is none, which is the pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, select the scope's files,
// project the dependencies between them, judge the cycles — and the only stage of the chain that reads
// anything. The violations are the files module's own assertion.CycleViolation values, each carrying the
// cycle it found as a readable path, or the one EmptyTestViolation of a scope that selected no file at
// all.
//
// The error is technical or the user's — a pattern a scope verb could not compile, a locator naming no
// Go project, a project that will not load — and never a failing rule. When it is non-nil the
// violations say nothing.
func (c FilesCyclesCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	graph, selected, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}

	if empty := options.GatherEmptyTestViolations(c.rule.selection(len(selected))); len(empty) > 0 {
		// A rule with no subject is reported instead of being judged: no file selected means no
		// dependency between two of them, so every such rule would otherwise pass forever.
		return empty, nil
	}

	edges := kernelprojection.ProjectEdges(graph, projection.PerSelectedFileEdge(selected))
	// The completeness flag is deliberately dropped: it bounds the size of the report and not the
	// answer. A truncated enumeration still holds every cycle it did find, and a rule that reports
	// cycles by the thousand is broken by the first of them.
	circuits, _ := cycles.ProjectCircuits(edges, nil)
	return filesassertion.GatherCycleViolations(circuits), nil
}

// String renders the whole rule as the sentence the user typed, as `project files, path without
// filename matches "internal/**", should, have no cycles`.
func (c FilesCyclesCondition) String() string {
	return c.rule.render("have no cycles")
}
