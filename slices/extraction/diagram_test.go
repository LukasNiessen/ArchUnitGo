package extraction_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/slices/extraction"
)

func TestNewDiagramDeclaresTheEndsOfEveryArrow(t *testing.T) {
	// PlantUML's own rule, and the one that makes `[api] --> [db]` a whole diagram: an arrow says its ends
	// exist. Having it here rather than in the parser is what makes a hand-built diagram read like a parsed
	// one.
	diagram := extraction.NewDiagram(nil, extraction.Dependency{From: "api", To: "db"})

	want := []string{"api", "db"}
	if got := diagram.Components(); !slices.Equal(got, want) {
		t.Errorf("the components of an undeclared arrow are %v, want %v", got, want)
	}
	if !diagram.Declares("db") {
		t.Error(`the diagram does not declare "db", want the head of the arrow declared`)
	}
}

func TestNewDiagramKeepsTheOrderTheComponentsWereDeclaredIn(t *testing.T) {
	// A report about a component the project has no slice for names them in this order, so a reader finds them
	// in the file in the order the report listed them. Sorted would be a different, unhelpful order.
	diagram := extraction.NewDiagram(
		[]string{"ui", "api", "domain"},
		extraction.Dependency{From: "api", To: "db"},
	)

	want := []string{"ui", "api", "domain", "db"}
	if got := diagram.Components(); !slices.Equal(got, want) {
		t.Errorf("the components are %v, want %v — declarations first, in order, then the arrows' ends", got, want)
	}
}

func TestNewDiagramReadsAThingSaidTwiceAsOneThing(t *testing.T) {
	// A diagram that names a component twice is a diagram of one component, and one that draws the same arrow
	// twice allows one dependency: a duplicate is a copy-and-paste, not a second statement.
	diagram := extraction.NewDiagram(
		[]string{"api", "db", "api"},
		extraction.Dependency{From: "api", To: "db"},
		extraction.Dependency{From: "api", To: "db"},
	)

	wantComponents := []string{"api", "db"}
	if got := diagram.Components(); !slices.Equal(got, wantComponents) {
		t.Errorf("the components are %v, want %v", got, wantComponents)
	}
	wantDependencies := []extraction.Dependency{{From: "api", To: "db"}}
	if got := diagram.Dependencies(); !slices.Equal(got, wantDependencies) {
		t.Errorf("the dependencies are %v, want %v", got, wantDependencies)
	}
}

func TestNewDiagramDeclaresAComponentThatDependsOnItselfAndDrawsNoArrowForIt(t *testing.T) {
	// projection.ProjectEdges drops self-edges, so a projection can never hold the dependency this arrow
	// states. The diagram carries the shape a projection can be compared against, which is what keeps the
	// assertion from having to know that one of the two sides never has such an edge.
	diagram := extraction.NewDiagram(nil, extraction.Dependency{From: "api", To: "api"})

	if !diagram.Declares("api") {
		t.Error(`the diagram does not declare "api", want the end of the arrow declared`)
	}
	if got := diagram.Dependencies(); len(got) != 0 {
		t.Errorf("the dependencies are %v, want none: a slice may always depend on itself", got)
	}
	if diagram.Draws("api", "api") {
		t.Error(`the diagram draws "api" -> "api", want no such dependency`)
	}
}

func TestNewDiagramCopiesWhatItWasBuiltFrom(t *testing.T) {
	// A diagram that has been read must not change when the caller reuses the slice it was built from, for the
	// reason assertion.NewEmptyTestViolation copies its selectors.
	components := []string{"api", "db"}
	dependencies := []extraction.Dependency{{From: "api", To: "db"}}

	diagram := extraction.NewDiagram(components, dependencies...)
	components[0] = "rewritten"
	dependencies[0] = extraction.Dependency{From: "db", To: "api"}

	if !diagram.Declares("api") || diagram.Declares("rewritten") {
		t.Errorf("the components are %v, want the ones the diagram was built with", diagram.Components())
	}
	if !diagram.Draws("api", "db") || diagram.Draws("db", "api") {
		t.Errorf("the dependencies are %v, want the ones the diagram was built with", diagram.Dependencies())
	}
}

func TestTheListsADiagramHandsOutAreCopies(t *testing.T) {
	// A caller that sorts the components in order to report them must not be sorting the diagram itself.
	diagram := extraction.NewDiagram([]string{"ui", "api"}, extraction.Dependency{From: "ui", To: "api"})

	slices.Sort(diagram.Components())
	diagram.Dependencies()[0] = extraction.Dependency{From: "api", To: "ui"}

	if got := diagram.Components(); !slices.Equal(got, []string{"ui", "api"}) {
		t.Errorf("the components are %v after a caller sorted what it was handed, want [ui api]", got)
	}
	if !diagram.Draws("ui", "api") {
		t.Errorf("the dependencies are %v after a caller overwrote what it was handed, want ui -> api", diagram.Dependencies())
	}
}

func TestADiagramDrawsAnArrowInOneDirectionOnly(t *testing.T) {
	// The direction is the whole meaning of an arrow: `[api] --> [db]` says nothing about whether the db may
	// reach the api, exactly as `contain dependency("api", "db")` says nothing about the converse.
	diagram := extraction.NewDiagram(nil, extraction.Dependency{From: "api", To: "db"})

	if !diagram.Draws("api", "db") {
		t.Error(`the diagram does not draw "api" -> "db", want the arrow it was built with`)
	}
	if diagram.Draws("db", "api") {
		t.Error(`the diagram draws "db" -> "api", want an arrow to point one way only`)
	}
}

func TestADiagramNobodyDrewIsEmpty(t *testing.T) {
	// The zero value, and what makes a text with nothing in it ErrEmptyDiagram rather than a rule that holds
	// forever.
	var nobodys extraction.Diagram

	if !nobodys.Empty() {
		t.Errorf("the zero diagram holds %v and %v, want it empty", nobodys.Components(), nobodys.Dependencies())
	}
	if extraction.NewDiagram([]string{"api"}).Empty() {
		t.Error("a diagram declaring one component is empty, want it not empty")
	}
}

func TestADiagramRendersAsItsTwoSizes(t *testing.T) {
	// How a rule that was given a diagram as a string says what it was given. The whole document would bury
	// the rule it is part of, which is why this is a summary and not a drawing.
	tests := []struct {
		name    string
		diagram extraction.Diagram
		want    string
	}{
		{"the diagram nobody drew", extraction.Diagram{}, "0 components, 0 dependencies"},
		{"one of each", extraction.NewDiagram(nil, extraction.Dependency{From: "api", To: "db"}), "2 components, 1 dependency"},
		{"one component alone", extraction.NewDiagram([]string{"api"}), "1 component, 0 dependencies"},
		{
			"three and two",
			extraction.NewDiagram(
				[]string{"api", "domain", "db"},
				extraction.Dependency{From: "api", To: "domain"},
				extraction.Dependency{From: "api", To: "db"},
			),
			"3 components, 2 dependencies",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.diagram.String(); got != test.want {
				t.Errorf("the diagram renders as %q, want %q", got, test.want)
			}
		})
	}
}
