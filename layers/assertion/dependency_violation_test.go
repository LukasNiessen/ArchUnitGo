package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/layers/assertion"
)

func TestADependencyViolationCarriesTheTwoLayersTheClauseAndTheFilesThatBrokeIt(t *testing.T) {
	// A violation is data and not a sentence: the pair of layers, the clause in the shape it was judged in,
	// and the concrete dependencies a reader has to go and unpick. The words are the testing layer's.
	clause := assertion.NewClause("db", []string{"api"}, kernel.ShouldNot)
	edge := extraction.NewEdge("internal/db/conn.go", "internal/api/handler.go", false, extraction.ImportKindPlain)

	violation := assertion.NewDependencyViolation(clause, "api", edge)

	if violation.Layer != "db" || violation.DependsOn != "api" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", violation.Layer, violation.DependsOn, "db", "api")
	}
	if want := []string{"api"}; !slices.Equal(violation.Named, want) {
		t.Errorf("the broken clause named %v, want %v", violation.Named, want)
	}
	if violation.Mood != kernel.ShouldNot {
		t.Errorf("the broken clause is in mood %s, want %s: an allowlist and a blocklist are different rules", violation.Mood, kernel.ShouldNot)
	}
	if len(violation.Dependencies) != 1 {
		t.Errorf("the violation carries %v, want the one dependency it was broken by", violation.Dependencies)
	}
}

func TestADependencyViolationIsOfTheLayerDependencyKind(t *testing.T) {
	// The key a report picks a phrasing by, and it names the vocabulary as well as the failure: every
	// vocabulary the library grows has a rule about what may depend on what.
	violation := assertion.NewDependencyViolation(assertion.NewClause("db", []string{"api"}, kernel.ShouldNot), "api")

	if kind := violation.Kind(); kind != assertion.KindLayerDependency {
		t.Errorf("the violation is of kind %q, want %q", kind, assertion.KindLayerDependency)
	}
	if assertion.KindLayerDependency != "layer-dependency" {
		t.Errorf("KindLayerDependency = %q, want the name every ArchUnit port spells it with", assertion.KindLayerDependency)
	}
}

func TestADependencyViolationDoesNotChangeWhenTheClauseOrTheProjectionItWasFoundInDoes(t *testing.T) {
	// A violation that has been reported must not change when the projection it was found in is walked on, so
	// both the named layers and the edges are copied on the way in and on the way out.
	named := []string{"api"}
	edges := []extraction.Edge{
		extraction.NewEdge("internal/db/conn.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
	}

	violation := assertion.NewDependencyViolation(assertion.NewClause("db", named, kernel.ShouldNot), "api", edges...)
	named[0] = "domain"
	edges[0] = extraction.NewEdge("internal/db/conn.go", "main.go", false, extraction.ImportKindPlain)

	if want := []string{"api"}; !slices.Equal(violation.Named, want) {
		t.Errorf("the violation names %v, want %v: the clause's layers were shared rather than copied", violation.Named, want)
	}
	if target := violation.Dependencies[0].Target; target != "internal/api/handler.go" {
		t.Errorf("the violation was broken by a dependency on %q, want %q: the edges were shared rather than copied",
			target, "internal/api/handler.go")
	}
}

func TestADependencyViolationRendersTheClauseTheOfferingLayerAndWhatWasFound(t *testing.T) {
	// The log line, rendered as the policy stated the clause rather than as its negation — which is what keeps
	// Mood.Holds the one place in the library that inverts anything.
	blocked := assertion.NewDependencyViolation(
		assertion.NewClause("db", []string{"api"}, kernel.ShouldNot), "api",
		extraction.NewEdge("internal/db/conn.go", "internal/api/handler.go", false, extraction.ImportKindPlain))
	unnamed := assertion.NewDependencyViolation(
		assertion.NewClause("api", []string{"domain"}, kernel.Should), "db",
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain))
	sealed := assertion.NewDependencyViolation(assertion.NewClause("domain", nil, kernel.Should), "db")

	sentences := map[string]string{
		blocked.String(): `db: may not depend on layers "api" -> api (internal/db/conn.go -> internal/api/handler.go)`,
		unnamed.String(): `api: may only depend on layers "domain" -> db (internal/api/handler.go -> internal/db/conn.go)`,
		sealed.String():  `domain: may only depend on no layers -> db`,
	}

	for rendered, want := range sentences {
		if rendered != want {
			t.Errorf("the violation reads %q, want %q", rendered, want)
		}
	}
}
