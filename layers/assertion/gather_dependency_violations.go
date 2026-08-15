package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// GatherDependencyViolations judges a whole layer policy: one DependencyViolation per projected dependency
// no clause of the policy allows, in the order the dependencies arrived, which is the order
// projection.ProjectEdges sorted them into — by depending layer, then by depended-on layer.
//
// No offending dependency is no violations, which is the pass. A policy whose layers matched no file at all
// is the empty-test guard's answer rather than this one's: a projection with no edges in it breaks nothing,
// so a renamed folder would otherwise turn a whole policy green forever.
//
// The dependencies arrive as the projected edges of projection.PerLayerEdge — one per pair of layers,
// cumulating the file dependencies that produced it — so two of the policy's three semantic rules are
// already true of the input: a dependency inside one layer is not in it, and neither is one either of whose
// ends is in no declared layer. The self-edge check below is the same rule held against a hand-built
// projection, because a violation saying that a layer may not depend on itself would be a rule nobody can
// obey.
//
// The third rule is this function's own: blocklist clauses are asked before allowlist ones. A dependency
// reports at most one violation however many clauses it breaks, and the clause it is reported against is
// the first blocklist clause that forbids it, or else the first allowlist clause that fails to name it —
// so a policy that both forbids `db` and permits only `domain` blames the sentence a reader wrote to forbid
// it rather than the one that happens to omit it. Every clause of the policy is in force, and a clause
// about another layer says nothing about this dependency.
func GatherDependencyViolations(policy []Clause, dependencies []kernelprojection.ProjectedEdge) []kernel.Violation {
	violations := make([]kernel.Violation, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency.IsSelfEdge() {
			// Intra-layer dependencies are always allowed. ProjectEdges has already dropped these, so this
			// is the invariant held against a projection built by hand rather than a case of the judgement.
			continue
		}
		if broken, found := brokenClause(policy, dependency); found {
			violations = append(violations, NewDependencyViolation(broken, dependency.TargetLabel(), dependency.CumulatedEdges()...))
		}
	}
	return violations
}

// brokenClause is the clause a report should blame this dependency on, and whether the policy is broken by
// it at all: the first blocklist clause that forbids it, or else the first allowlist clause that does not
// name it.
//
// Blocklists first is the policy's stated evaluation order, and it is what makes a report stable when the
// two kinds of clause overlap: `may not depend on layers "db"` is the sentence a user wrote about this very
// dependency, and blaming an allowlist that merely fails to mention `db` would send them to the wrong line
// of their test.
func brokenClause(policy []Clause, dependency kernelprojection.ProjectedEdge) (Clause, bool) {
	if blocked, found := firstBroken(policy, dependency, kernel.ShouldNot); found {
		return blocked, true
	}
	return firstBroken(policy, dependency, kernel.Should)
}

// firstBroken is the first clause of this mood, in the order the user typed the policy, that is about the
// depending layer and does not allow this dependency. Both moods come through it, because a clause's own
// Allows is the whole of what they differ by.
func firstBroken(policy []Clause, dependency kernelprojection.ProjectedEdge, mood kernel.Mood) (Clause, bool) {
	for _, clause := range policy {
		if clause.Mood() != mood || clause.Layer() != dependency.SourceLabel() {
			continue
		}
		if !clause.Allows(dependency.TargetLabel()) {
			return clause, true
		}
	}
	return Clause{}, false
}
