package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
	"github.com/LukasNiessen/ArchUnitGo/layers/projection"
)

// LayersPolicyCondition is a layer policy with at least one clause in it: the terminal of this module, and a
// fluentapi.Checkable, which is the one thing every consumer of a rule programs against.
//
//	rule := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("db").DefinedByFolder("internal/db/**").
//		WhereLayer("db").MayNotDependOnLayers("api")
//	violations, err := rule.Check(nil)
//
// It is what both predicates return, and it is the one stage of this grammar that is both a builder and a
// terminal: the clauses of a policy are a list, so each of them has to hand back something the next clause —
// or another layer — can be chained onto, and a policy is a rule the moment it has one clause. Two types, one
// to chain on and one to check, would be the same verbs written twice.
//
// A whole policy is one rule and it is checked in one pass, which is the difference between this module and
// the N² file rules it stands in for: the project is extracted once, its files are assigned to layers once,
// and every clause is judged against that one projection. Every clause is in force, and a dependency that
// breaks several is reported once, against the first blocklist clause that forbids it or else the first
// allowlist clause that does not name it.
//
// It is immutable like every stage before it, and nothing has been read when it is built: the project is
// located, extracted, projected and judged by Check, and by nothing else.
type LayersPolicyCondition struct {
	// policy is the declarations and the clauses, the whole rule. The condition adds nothing of its own —
	// it is the policy at the point where it became checkable.
	policy LayersBuilder
}

// Check runs the policy: one violation per pair of layers where the depending one reaches the depended-on one
// and no clause allows it, and an empty result when every dependency between the project's layers agrees with
// the policy, which is the pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, assign each file to a layer,
// project the dependencies between the layers, judge them against the clauses — and the only stage of the
// chain that reads anything.
//
// The violations are the layers module's own assertion.DependencyViolation values, each carrying the two
// layers, the clause that was broken and the concrete file dependencies that broke it, or the
// EmptyTestViolations of a policy one of whose layers no file is in.
//
// The error is technical or the user's — a pattern a `defined by` verb could not compile, a clause naming a
// layer the policy never declared, a blocklist naming nothing, a locator naming no Go project, a project that
// will not load — and never a failing rule. When it is non-nil the violations say nothing.
func (c LayersPolicyCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	graph, membership, err := c.policy.resolve(options)
	if err != nil {
		return nil, err
	}

	if empty := options.GatherEmptyTestViolations(c.policy.populations(membership)...); len(empty) > 0 {
		// A layer no file is in is reported instead of being judged. Every clause about it would be vacuous —
		// it is at neither end of any projected dependency — so a policy whose folders have been renamed
		// would otherwise be green forever, which is the one failure this library refuses to pass silently.
		return empty, nil
	}

	dependencies := kernelprojection.ProjectEdges(graph, projection.PerLayerEdge(c.policy.declaredLayers()...))
	return layersassertion.GatherDependencyViolations(c.policy.clauses, dependencies), nil
}

// String renders the whole policy as the sentence the user typed, as `project layers, layer "api" defined by
// path without filename matches "internal/api/**", layer "db" defined by path without filename matches
// "internal/db/**", where layer "db", may not depend on layers "api"`.
//
// Each layer renders as its patterns' descriptions, because a reader needs to see which part of an identifier
// each was matched against, and each clause renders as the words it was written in.
func (c LayersPolicyCondition) String() string {
	return c.policy.String()
}
