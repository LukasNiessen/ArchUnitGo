package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/slices/assertion"
)

func TestAnUndrawnDependencyViolationCarriesTheTwoSlicesAndTheFilesThatMadeIt(t *testing.T) {
	// The finding a diagram is drawn for, and the one that carries files: what a reader has to do is either draw
	// the arrow or delete the import, and after relabelling these files live nowhere else.
	violation := assertion.NewUndrawnDependencyViolation("api", "db",
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain))

	if violation.Finding != assertion.FindingUndrawnDependency {
		t.Errorf("the violation reports %s, want an undrawn dependency", violation.Finding)
	}
	if violation.Slice != "api" || violation.DependsOn != "db" {
		t.Errorf("the violation is about %q -> %q, want api -> db", violation.Slice, violation.DependsOn)
	}
	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if got := drawnThrough(violation); !slices.Equal(got, want) {
		t.Errorf("the violation was found through %v, want %v", got, want)
	}
}

func TestTheTwoFindingsAboutOneNameCarryNothingElse(t *testing.T) {
	// A slice nobody drew and a component nobody wrote are about one name: there is no second slice and no file
	// to point at, and what the violation says is that something is not there at all.
	tests := []struct {
		name      string
		violation assertion.DiagramViolation
		finding   assertion.DiagramFinding
	}{
		{"a slice the diagram does not declare", assertion.NewUndeclaredSliceViolation("ui"), assertion.FindingUndeclaredSlice},
		{"a component the project has no slice for", assertion.NewAbsentComponentViolation("cache"), assertion.FindingAbsentComponent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.violation.Finding != test.finding {
				t.Errorf("the violation reports %s, want %s", test.violation.Finding, test.finding)
			}
			if test.violation.DependsOn != "" {
				t.Errorf("the violation names %q as depended upon, want nothing", test.violation.DependsOn)
			}
			if len(test.violation.Dependencies) != 0 {
				t.Errorf("the violation carries %v, want no file at all", drawnThrough(test.violation))
			}
		})
	}
}

func TestEveryDiagramViolationIsOfTheSliceDiagramKind(t *testing.T) {
	// One kind for the three findings, because a reader checking a project against a drawing wants one list of
	// the ways the two do not match — and the testing layer picks a phrasing by the kind, then words by the
	// finding.
	violations := []assertion.DiagramViolation{
		assertion.NewUndrawnDependencyViolation("api", "db"),
		assertion.NewUndeclaredSliceViolation("ui"),
		assertion.NewAbsentComponentViolation("cache"),
	}

	for _, violation := range violations {
		if violation.Kind() != assertion.KindSliceDiagram {
			t.Errorf("%s is of kind %q, want %q", violation.Finding, violation.Kind(), assertion.KindSliceDiagram)
		}
	}
	if assertion.KindSliceDiagram != "slice-diagram" {
		t.Errorf("KindSliceDiagram = %q, want the name every ArchUnit port spells it with", assertion.KindSliceDiagram)
	}
}

func TestADiagramViolationDoesNotChangeWhenTheProjectionItWasFoundInDoes(t *testing.T) {
	// A violation that has been reported is a fact about a project, so it copies what it was built from — the
	// reason assertion.NewEmptyTestViolation copies its selectors.
	edges := []extraction.Edge{
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
	}

	violation := assertion.NewUndrawnDependencyViolation("api", "db", edges...)
	edges[0] = extraction.NewEdge("rewritten.go", "elsewhere.go", false, extraction.ImportKindPlain)

	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if got := drawnThrough(violation); !slices.Equal(got, want) {
		t.Errorf("the violation was found through %v after its input changed, want %v", got, want)
	}
}

func TestADiagramViolationRendersWhatItIsAboutAndWhatWasFound(t *testing.T) {
	// A log line, not a user's message: the finding's own vocabulary over the names it is about. The sentence a
	// user reads is the testing layer's, built from the same fields.
	tests := []struct {
		name      string
		violation assertion.DiagramViolation
		want      string
	}{
		{
			"an undrawn dependency",
			assertion.NewUndrawnDependencyViolation("api", "db",
				extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain)),
			"api -> db: undrawn dependency (internal/api/handler.go -> internal/db/conn.go)",
		},
		{"an undrawn dependency built by hand", assertion.NewUndrawnDependencyViolation("api", "db"), "api -> db: undrawn dependency"},
		{"an undeclared slice", assertion.NewUndeclaredSliceViolation("ui"), "ui: undeclared slice"},
		{"an absent component", assertion.NewAbsentComponentViolation("cache"), "cache: absent component"},
		{"a finding that is none of the three", assertion.DiagramViolation{Finding: 42, Slice: "api"}, "api: unknown finding"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.violation.String(); got != test.want {
				t.Errorf("the violation renders as %q, want %q", got, test.want)
			}
		})
	}
}

func TestTheThreeFindingsAreNamedInReports(t *testing.T) {
	// The vocabulary a report refers to a finding by, spelled once here so that renaming one is a change to one
	// place.
	tests := []struct {
		finding assertion.DiagramFinding
		want    string
	}{
		{assertion.FindingUndrawnDependency, "undrawn dependency"},
		{assertion.FindingUndeclaredSlice, "undeclared slice"},
		{assertion.FindingAbsentComponent, "absent component"},
		{assertion.DiagramFinding(42), "unknown finding"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if got := test.finding.String(); got != test.want {
				t.Errorf("the finding is named %q, want %q", got, test.want)
			}
		})
	}
}

// drawnThrough are the file dependencies this violation was found through, as `a.go -> b.go`. It is
// brokenBy for the other violation type of this package.
func drawnThrough(violation assertion.DiagramViolation) []string {
	rendered := make([]string, 0, len(violation.Dependencies))
	for _, edge := range violation.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}

// diagramViolations are these violations rendered one per line, in the order they were reported, failing the
// test when a rule reported anything but a DiagramViolation. The order is half of what a diagram check
// promises, so the tests assert on the list rather than on a set.
func diagramViolations(t *testing.T, violations []kernel.Violation) []string {
	t.Helper()

	rendered := make([]string, 0, len(violations))
	for _, violation := range violations {
		reported, ok := violation.(assertion.DiagramViolation)
		if !ok {
			t.Fatalf("the violation is a %T, want a DiagramViolation", violation)
		}
		rendered = append(rendered, reported.String())
	}
	return rendered
}
