package fluentapi_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
)

func TestAnAllowlistPolicyHoldsWhenEveryLayerReachesOnlyWhatItNamed(t *testing.T) {
	// The layered architecture the module exists for, written as one rule: the api may reach both layers below
	// it, the domain may reach the database, and the database's production code reaches nothing at all.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).
		WhereLayer("api").MayOnlyDependOnLayers("domain", "db").
		WhereLayer("domain").MayOnlyDependOnLayers("db").
		WhereLayer("db").MayOnlyDependOnLayers()

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if len(violations) != 0 {
		t.Errorf("the policy reported %v, want nothing: every dependency the project has is allowed", messages(t, violations))
	}
}

func TestAnAllowlistReportsTheLayerItDidNotName(t *testing.T) {
	// The failing half, over the real project: the domain reaches the database, and an allowlist that names
	// only the api does not allow it. The violation carries the concrete file dependency a reader has to open.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("domain").MayOnlyDependOnLayers("api")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"domain -> db"}) {
		t.Fatalf("the policy reported %v, want the one dependency it does not allow", pairs)
	}
	violation := layerViolation(t, violations[0])
	if want := []string{"internal/domain/order.go -> internal/db/conn.go"}; !slices.Equal(brokenBy(violation), want) {
		t.Errorf("the violation was broken by %v, want %v", brokenBy(violation), want)
	}
	if !slices.Equal(violation.Named, []string{"api"}) {
		t.Errorf("the violation blames a clause naming %v, want the one the user wrote", violation.Named)
	}
}

func TestASealedLayerForbidsEveryDependencyOnAnotherDeclaredLayer(t *testing.T) {
	// `MayOnlyDependOnLayers()` with nothing named: the domain that may not reach the code around it, which is
	// a policy people really write — and the empty allowlist is legal for exactly that reason.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("domain").MayOnlyDependOnLayers()

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"domain -> db"}) {
		t.Errorf("the sealed layer reported %v, want its one dependency on another layer", pairs)
	}
}

func TestASealedLayerMayStillDependOnItself(t *testing.T) {
	// "Intra-layer dependencies are always allowed", through the public chain: the api's two files depend on
	// each other in the fixture, and a sealed api is not broken by that — a rule nobody could obey.
	root := writeNestedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").
		WhereLayer("api").MayOnlyDependOnLayers()

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if len(violations) != 0 {
		t.Errorf("the sealed layer reported %v, want nothing: internal/api/rest/get.go depends on its own layer",
			messages(t, violations))
	}
}

func TestAPolicySaysNothingAboutAnEdgeWithAnEndInNoDeclaredLayer(t *testing.T) {
	// "Edges where either end belongs to no declared layer are ignored", which is what makes a policy about
	// part of a project possible: main.go imports the api and is in no layer, so a sealed api is not broken by
	// being imported, and nothing about main.go is judged.
	root := writeLayeredFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").
		WhereLayer("api").MayOnlyDependOnLayers()

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if len(violations) != 0 {
		t.Errorf("the policy reported %v, want nothing: everything the api reaches is in no declared layer",
			messages(t, violations))
	}
}
