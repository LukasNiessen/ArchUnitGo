package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/layers/assertion"
)

// fixtureDependencies are the projected dependencies of a three-layer project: the api reaches both other
// layers, and the db reaches back into the api, which is what makes every clause of a policy over them have
// something to say.
func fixtureDependencies() []kernelprojection.ProjectedEdge {
	return []kernelprojection.ProjectedEdge{
		kernelprojection.NewProjectedEdge("api", "domain",
			extraction.NewEdge("internal/api/handler.go", "internal/domain/order.go", false, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("api", "db",
			extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("db", "api",
			extraction.NewEdge("internal/db/conn.go", "internal/api/router.go", false, extraction.ImportKindPlain)),
	}
}

func TestGatherDependencyViolationsReportsWhatAnAllowlistDidNotName(t *testing.T) {
	// `where layer "api", may only depend on layers "domain"`: the api's dependency on the domain is allowed
	// and its dependency on the db is not, so one violation, carrying the files that produced it.
	policy := []assertion.Clause{assertion.NewClause("api", []string{"domain"}, kernel.Should)}

	violations := assertion.GatherDependencyViolations(policy, fixtureDependencies())

	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"api -> db"}) {
		t.Fatalf("the policy reported %v, want the one dependency it does not allow", pairs)
	}
	violation := layerViolation(t, violations[0])
	if want := []string{"internal/api/handler.go -> internal/db/conn.go"}; !slices.Equal(brokenBy(violation), want) {
		t.Errorf("the violation was broken by %v, want %v", brokenBy(violation), want)
	}
	if violation.Mood != kernel.Should {
		t.Errorf("the violation was judged in mood %s, want the allowlist's %s", violation.Mood, kernel.Should)
	}
}

func TestGatherDependencyViolationsReportsWhatABlocklistForbade(t *testing.T) {
	// `where layer "db", may not depend on layers "api"`: the same walk over the same projection, one flag
	// apart, and the rest of the policy's layers are left alone — the api's two dependencies are nobody's
	// business under this clause.
	policy := []assertion.Clause{assertion.NewClause("db", []string{"api"}, kernel.ShouldNot)}

	violations := assertion.GatherDependencyViolations(policy, fixtureDependencies())

	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"db -> api"}) {
		t.Fatalf("the policy reported %v, want the one dependency it forbids", pairs)
	}
	if mood := layerViolation(t, violations[0]).Mood; mood != kernel.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want the blocklist's %s", mood, kernel.ShouldNot)
	}
}

func TestGatherDependencyViolationsHoldsEveryClauseOfThePolicyAtOnce(t *testing.T) {
	// A policy is a conjunction: one line per layer saying what that layer may reach, and all of them in
	// force. This is the whole reason the module exists — as file rules it is N² sentences.
	policy := []assertion.Clause{
		assertion.NewClause("api", []string{"domain"}, kernel.Should),
		assertion.NewClause("db", []string{"api"}, kernel.ShouldNot),
	}

	violations := assertion.GatherDependencyViolations(policy, fixtureDependencies())

	want := []string{"api -> db", "db -> api"}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, want) {
		t.Errorf("the policy reported %v, want %v: every clause of a policy applies", pairs, want)
	}
}

func TestGatherDependencyViolationsBlamesABlocklistBeforeAnAllowlist(t *testing.T) {
	// The policy's third semantic rule, and it is about the report rather than the pass: a dependency that
	// breaks both kinds of clause is reported once, against the sentence the reader wrote to forbid that very
	// pair of layers, instead of the allowlist that merely fails to mention it.
	policy := []assertion.Clause{
		assertion.NewClause("api", []string{"domain"}, kernel.Should),
		assertion.NewClause("api", []string{"db"}, kernel.ShouldNot),
	}

	violations := assertion.GatherDependencyViolations(policy, fixtureDependencies())

	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"api -> db"}) {
		t.Fatalf("the policy reported %v, want one violation for the one offending dependency", pairs)
	}
	violation := layerViolation(t, violations[0])
	if violation.Mood != kernel.ShouldNot || !slices.Equal(violation.Named, []string{"db"}) {
		t.Errorf("the violation blames `%s %v`, want the blocklist that forbade the pair",
			violation.Mood, violation.Named)
	}
}

func TestGatherDependencyViolationsReportsEveryDependencyOfASealedLayer(t *testing.T) {
	// `may only depend on layers` with nothing named: the layer may reach nothing outside itself, so both of
	// the api's dependencies are violations.
	policy := []assertion.Clause{assertion.NewClause("api", nil, kernel.Should)}

	violations := assertion.GatherDependencyViolations(policy, fixtureDependencies())

	want := []string{"api -> domain", "api -> db"}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, want) {
		t.Errorf("the sealed layer reported %v, want %v", pairs, want)
	}
}

func TestGatherDependencyViolationsIsSilentWhenThePolicyHolds(t *testing.T) {
	// No offending dependency is no violations, which is the pass — and a clause about a layer that depends on
	// nothing at all requires nothing to exist: `may only depend on layers "api"` for the domain is not broken
	// by the domain depending on nobody.
	policy := []assertion.Clause{
		assertion.NewClause("api", []string{"domain", "db"}, kernel.Should),
		assertion.NewClause("db", []string{"domain"}, kernel.ShouldNot),
		assertion.NewClause("domain", []string{"api"}, kernel.Should),
	}

	if violations := assertion.GatherDependencyViolations(policy, fixtureDependencies()); len(violations) != 0 {
		t.Errorf("the policy reported %v, want nothing: every dependency it has is allowed", offendingPairs(t, violations))
	}
}

func TestGatherDependencyViolationsSaysNothingAboutALayerNoClauseIsAbout(t *testing.T) {
	// A policy may be written about part of a project, one layer at a time: a clause about the db says nothing
	// about what the api depends on, and an undeclared layer is the fluent API's error rather than this
	// function's.
	policy := []assertion.Clause{assertion.NewClause("db", []string{"domain"}, kernel.ShouldNot)}

	if violations := assertion.GatherDependencyViolations(policy, fixtureDependencies()); len(violations) != 0 {
		t.Errorf("the policy reported %v, want nothing: no clause is about the api", offendingPairs(t, violations))
	}
}

func TestGatherDependencyViolationsNeverReportsALayerDependingOnItself(t *testing.T) {
	// Intra-layer dependencies are always allowed. ProjectEdges has already dropped these, so this is the
	// invariant held against a hand-built projection: a violation saying a layer may not depend on itself
	// would be a rule nobody can obey.
	policy := []assertion.Clause{assertion.NewClause("api", nil, kernel.Should)}
	inside := []kernelprojection.ProjectedEdge{
		kernelprojection.NewProjectedEdge("api", "api",
			extraction.NewEdge("internal/api/handler.go", "internal/api/router.go", false, extraction.ImportKindPlain)),
	}

	if violations := assertion.GatherDependencyViolations(policy, inside); len(violations) != 0 {
		t.Errorf("the sealed layer reported %v, want nothing: it may always depend on itself", offendingPairs(t, violations))
	}
}

func TestGatherDependencyViolationsOnAnEmptyProjectionOrAnEmptyPolicyReportsNothing(t *testing.T) {
	// Both are somebody else's failure to report: a projection with no edges is what the empty-test guard
	// answers for, and a policy with no clause never became a rule at all.
	policy := []assertion.Clause{assertion.NewClause("api", nil, kernel.Should)}

	if violations := assertion.GatherDependencyViolations(policy, nil); len(violations) != 0 {
		t.Errorf("an empty projection reported %v, want nothing", offendingPairs(t, violations))
	}
	if violations := assertion.GatherDependencyViolations(nil, fixtureDependencies()); len(violations) != 0 {
		t.Errorf("an empty policy reported %v, want nothing", offendingPairs(t, violations))
	}
}

// offendingPairs are the pairs of layers these violations are about, as `api -> db`, in the order they were
// reported — which is the order the dependencies arrived in.
func offendingPairs(t *testing.T, violations []kernel.Violation) []string {
	t.Helper()

	pairs := make([]string, 0, len(violations))
	for _, violation := range violations {
		reported := layerViolation(t, violation)
		pairs = append(pairs, reported.Layer+" -> "+reported.DependsOn)
	}
	return pairs
}

// layerViolation is this violation as the layers module's own type, failing the test when a policy reported
// anything else.
func layerViolation(t *testing.T, violation kernel.Violation) assertion.DependencyViolation {
	t.Helper()

	reported, ok := violation.(assertion.DependencyViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want a DependencyViolation", violation)
	}
	return reported
}

// brokenBy are the file dependencies this violation was broken by, as `a.go -> b.go`.
func brokenBy(violation assertion.DependencyViolation) []string {
	rendered := make([]string, 0, len(violation.Dependencies))
	for _, edge := range violation.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}
