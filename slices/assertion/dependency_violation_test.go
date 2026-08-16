package assertion_test

import (
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/slices/assertion"
)

func TestADependencyViolationCarriesTheTwoSlicesTheMoodAndTheFilesThatBrokeIt(t *testing.T) {
	// A violation is data and not a sentence: the pair of slices, the mood the rule was written in, and the
	// concrete dependencies a reader has to go and unpick. The words are the testing layer's.
	edge := extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain)

	violation := assertion.NewDependencyViolation("api", "db", kernel.ShouldNot, edge)

	if violation.Slice != "api" || violation.DependsOn != "db" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", violation.Slice, violation.DependsOn, "api", "db")
	}
	if violation.Mood != kernel.ShouldNot {
		t.Errorf("the rule is in mood %s, want %s: a forbidden dependency and a required one fail in opposite ways",
			violation.Mood, kernel.ShouldNot)
	}
	if len(violation.Dependencies) != 1 {
		t.Errorf("the violation carries %v, want the one dependency it was broken by", violation.Dependencies)
	}
}

func TestADependencyViolationIsOfTheSliceDependencyKind(t *testing.T) {
	// The key a report picks a phrasing by, and it names the vocabulary as well as the failure: every
	// vocabulary the library grows has a rule about what may depend on what.
	violation := assertion.NewDependencyViolation("api", "db", kernel.ShouldNot)

	if kind := violation.Kind(); kind != assertion.KindSliceDependency {
		t.Errorf("the violation is of kind %q, want %q", kind, assertion.KindSliceDependency)
	}
	if assertion.KindSliceDependency != "slice-dependency" {
		t.Errorf("KindSliceDependency = %q, want the name every ArchUnit port spells it with", assertion.KindSliceDependency)
	}
}

func TestADependencyViolationDoesNotChangeWhenTheProjectionItWasFoundInDoes(t *testing.T) {
	// A violation that has been reported must not change when the projection it was found in is walked on, so
	// the edges are copied on the way in.
	edges := []extraction.Edge{
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
	}

	violation := assertion.NewDependencyViolation("api", "db", kernel.ShouldNot, edges...)
	edges[0] = extraction.NewEdge("internal/api/handler.go", "main.go", false, extraction.ImportKindPlain)

	if target := violation.Dependencies[0].Target; target != "internal/db/conn.go" {
		t.Errorf("the violation was broken by a dependency on %q, want %q: the edges were shared rather than copied",
			target, "internal/db/conn.go")
	}
}

func TestADependencyViolationRendersThePairOfSlicesTheRuleAndWhatWasFound(t *testing.T) {
	// The log line, rendered as the user stated the rule rather than as its negation — which is what keeps
	// Mood.Holds the one place in the library that inverts anything. A required dependency that is missing has
	// nothing to show, and says so by naming the slices alone.
	forbidden := assertion.NewDependencyViolation("api", "db", kernel.ShouldNot,
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/router.go", "internal/db/query.go", false, extraction.ImportKindPlain))
	missing := assertion.NewDependencyViolation("db", "domain", kernel.Should)

	sentences := map[string]string{
		forbidden.String(): "api -> db: should not contain dependency (internal/api/handler.go -> " +
			"internal/db/conn.go, internal/api/router.go -> internal/db/query.go)",
		missing.String(): "db -> domain: should contain dependency",
	}

	for rendered, want := range sentences {
		if rendered != want {
			t.Errorf("the violation reads %q, want %q", rendered, want)
		}
	}
}
