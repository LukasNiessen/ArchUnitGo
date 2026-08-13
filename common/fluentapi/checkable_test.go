package fluentapi_test

import (
	"errors"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// The seam is open: a terminal declared outside the package, in a rule family the kernel has never
// heard of, is a Checkable. Nothing about the interface is sealed, and nothing has to be registered.
var _ fluentapi.Checkable = dependencyRule{}

// dependencyRule stands in for the terminal of `project files, in folder X, should not, depend on
// files, in folder Y` until the files module lands: it selects a subject from a graph, runs the
// empty-test guard over what it selected, and reports one violation per offending edge. It exists to
// exercise the contract, not to be the rule.
type dependencyRule struct {
	graph     extraction.Graph
	subject   matching.Filter
	forbidden matching.Filter
	failure   error
}

func (r dependencyRule) Check(options *fluentapi.CheckOptions) ([]assertion.Violation, error) {
	if r.failure != nil {
		return nil, r.failure
	}

	matched := 0
	for _, node := range r.graph.Nodes() {
		if r.subject.Matches(node) {
			matched++
		}
	}
	// The guard first, and on its own: a rule whose subject is empty has nothing else to say.
	if guard := assertion.GatherEmptyTestViolations(matched, options.EmptyTestOptions("files", r.subject)); len(guard) > 0 {
		return guard, nil
	}

	var violations []assertion.Violation
	for _, edge := range r.graph {
		if edge.IsSelfEdge() {
			continue
		}
		if r.subject.Matches(edge.Source) && r.forbidden.Matches(edge.Target) {
			violations = append(violations, forbiddenDependencyViolation{Edge: edge})
		}
	}
	return violations, nil
}

const kindForbiddenDependency assertion.ViolationKind = "test-forbidden-dependency"

type forbiddenDependencyViolation struct {
	Edge extraction.Edge
}

func (forbiddenDependencyViolation) Kind() assertion.ViolationKind {
	return kindForbiddenDependency
}

func TestCheckableRunsARuleAndReturnsItsViolations(t *testing.T) {
	graph := extraction.NewGraph(
		extraction.NewEdge("cmd/server/main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/store.go", false, extraction.ImportKindPlain),
		extraction.SelfEdge("internal/db/store.go"),
	)
	db := matching.FolderMatcher(mustGlob(t, "internal/db/**"))

	tests := []struct {
		name    string
		subject matching.Filter
		options *fluentapi.CheckOptions
		want    []assertion.ViolationKind
	}{
		{
			name:    "the api reaches the database, and that is the violation",
			subject: matching.FolderMatcher(mustGlob(t, "internal/api/**")),
			options: nil,
			want:    []assertion.ViolationKind{kindForbiddenDependency},
		},
		{
			name:    "a subject that keeps off the database passes, and an empty list is the pass",
			subject: matching.FolderMatcher(mustGlob(t, "cmd/**")),
			options: &fluentapi.CheckOptions{},
			want:    nil,
		},
		{
			name:    "a stale folder name selects nothing, and the guard catches it through the options",
			subject: matching.FolderMatcher(mustGlob(t, "internal/apis/**")),
			options: nil,
			want:    []assertion.ViolationKind{assertion.KindEmptyTest},
		},
		{
			name:    "unless the user really meant an empty selection",
			subject: matching.FolderMatcher(mustGlob(t, "internal/apis/**")),
			options: &fluentapi.CheckOptions{AllowEmptyTests: true},
			want:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule := dependencyRule{graph: graph, subject: test.subject, forbidden: db}

			// The consumer holds a Checkable and knows nothing else about the rule.
			violations := gatherAll(t, test.options, rule)

			if len(violations) != len(test.want) {
				t.Fatalf("Check(%+v) = %v, want %d violations", test.options, violations, len(test.want))
			}
			for i, kind := range test.want {
				if violations[i].Kind() != kind {
					t.Errorf("violation %d is %q, want %q", i, violations[i].Kind(), kind)
				}
			}
		})
	}
}

func TestCheckableCarriesTheOffendingEdgeAsDataNotProse(t *testing.T) {
	rule := dependencyRule{
		graph: extraction.NewGraph(
			extraction.NewEdge("internal/api/handler.go", "internal/db/store.go", false, extraction.ImportKindPlain),
		),
		subject:   matching.FolderMatcher(mustGlob(t, "internal/api/**")),
		forbidden: matching.FolderMatcher(mustGlob(t, "internal/db/**")),
	}

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("Check(nil) failed: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("Check(nil) = %v, want the one forbidden dependency", violations)
	}
	violation, ok := violations[0].(forbiddenDependencyViolation)
	if !ok {
		t.Fatalf("got %T, want the rule family's own violation type", violations[0])
	}
	if violation.Edge.Source != "internal/api/handler.go" || violation.Edge.Target != "internal/db/store.go" {
		t.Errorf("the violation carries %v, want the edge that broke the rule", violation.Edge)
	}
}

func TestCheckableReportsATechnicalFailureAsAnErrorNotAViolation(t *testing.T) {
	// A project that will not load is the library's or the environment's problem, so it travels as an
	// error. A failing rule never does.
	broken := errors.New("loading the project failed")
	rule := dependencyRule{failure: broken}

	violations, err := rule.Check(nil)

	if !errors.Is(err, broken) {
		t.Errorf("Check(nil) error = %v, want the technical failure", err)
	}
	if violations != nil {
		t.Errorf("Check(nil) = %v alongside an error, want no violations: they say nothing", violations)
	}
}

// gatherAll is the shape of every consumer in the library: it takes rules as Checkable, runs each of
// them with the same options, and collects the violations. It cannot tell which module a rule came
// from, and that is the point of the seam.
func gatherAll(t *testing.T, options *fluentapi.CheckOptions, rules ...fluentapi.Checkable) []assertion.Violation {
	t.Helper()

	var violations []assertion.Violation
	for _, rule := range rules {
		found, err := rule.Check(options)
		if err != nil {
			t.Fatalf("Check(%+v) failed: %v", options, err)
		}
		violations = append(violations, found...)
	}
	return violations
}
