package fluentapi_test

import (
	"slices"
	"strings"
	"testing"
)

func TestASecondClauseKeepsTheFirstOne(t *testing.T) {
	// The verb one stage later, which is what makes a whole layer policy one chain: the clause already written
	// is kept, so a policy of N clauses is typed as a list and checked in one pass. Without it an N-layer
	// policy would be N rules and a reader would assemble it in their head.
	root := writeLayeredFixtureProject(t)
	first := fixturePolicy(t, root).WhereLayer("domain").MayNotDependOnLayers("db")
	both := first.WhereLayer("api").MayNotDependOnLayers("db")

	one, err := first.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", first, err)
	}
	two, err := both.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", both, err)
	}

	if pairs := offendingPairs(t, one); !slices.Equal(pairs, []string{"domain -> db"}) {
		t.Errorf("the one-clause policy reported %v, want the dependency its clause forbids", pairs)
	}
	if pairs := offendingPairs(t, two); !slices.Equal(pairs, []string{"api -> db", "domain -> db"}) {
		t.Errorf("the two-clause policy reported %v, want both clauses in force", pairs)
	}
}

func TestTwoClausesMayBeWrittenAboutOneLayer(t *testing.T) {
	// A policy may say more than one thing about a layer, and both are in force: the clauses of a policy are a
	// conjunction, so `where layer` is not a lookup that replaces what was said about that layer before.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).
		WhereLayer("api").MayNotDependOnLayers("db").
		WhereLayer("api").MayNotDependOnLayers("domain")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}

	want := []string{"api -> db", "api -> domain"}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, want) {
		t.Errorf("the policy reported %v, want %v: both clauses are about the same layer", pairs, want)
	}
}

func TestOpeningAClauseLeavesThePolicyItWasOpenedOnAlone(t *testing.T) {
	// The example in the LayersBuilder doc, which is the reason the declaration stage is a value of its own: one
	// set of declared layers, two policies over it. Opening a clause hands back a new builder, so the two are
	// two rules and the layers they share are still just declarations.
	root := writeLayeredFixtureProject(t)
	layers := fixturePolicy(t, root)
	sealed := layers.WhereLayer("domain").MayOnlyDependOnLayers()
	blocked := layers.WhereLayer("domain").MayNotDependOnLayers("api")

	brokenBySealing, err := sealed.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", sealed, err)
	}
	brokenByBlocking, err := blocked.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", blocked, err)
	}

	if pairs := offendingPairs(t, brokenBySealing); !slices.Equal(pairs, []string{"domain -> db"}) {
		t.Errorf("the sealed policy reported %v, want the one dependency the domain has", pairs)
	}
	if len(brokenByBlocking) != 0 {
		t.Errorf("the blocking policy reported %v, want nothing: the domain does not reach the api",
			messages(t, brokenByBlocking))
	}
	if rendered := layers.String(); strings.Contains(rendered, "where layer") {
		t.Errorf("the declarations read as %q, want them unchanged by the clauses opened on them", rendered)
	}
}
