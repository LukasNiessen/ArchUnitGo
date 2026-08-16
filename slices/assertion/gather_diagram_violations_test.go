package assertion_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/slices/assertion"
	slicesextraction "github.com/LukasNiessen/ArchUnitGo/slices/extraction"
)

// fixtureSlices are the slices the fixture projection above is over: what projection.SelectSliceFiles would
// have handed back the keys of.
func fixtureSlices() []string {
	return []string{"api", "db", "domain"}
}

// fixtureDiagram is the drawing the fixture project adheres to: the three slices, and every arrow the
// projection has, including the one back from the db into the api.
func fixtureDiagram() slicesextraction.Diagram {
	return slicesextraction.NewDiagram(fixtureSlices(),
		slicesextraction.Dependency{From: "api", To: "db"},
		slicesextraction.Dependency{From: "api", To: "domain"},
		slicesextraction.Dependency{From: "db", To: "api"},
	)
}

func TestGatherDiagramViolationsReportsNothingWhenTheDrawingAndTheCodeAgree(t *testing.T) {
	// The pass, and it is the whole point of the rule: every dependency the project has is an arrow somebody
	// drew, every slice it has is a component, and every component is a slice.
	violations := assertion.GatherDiagramViolations(
		fixtureDiagram(), fixtureDependencies(), fixtureSlices(), assertion.DiagramOptions{})

	if got := diagramViolations(t, violations); len(got) != 0 {
		t.Errorf("the project disagrees with its own diagram in %v, want it to adhere", got)
	}
}

func TestGatherDiagramViolationsReportsADependencyTheDiagramDoesNotDraw(t *testing.T) {
	// The finding a diagram is drawn for. Both slices are in the drawing and the arrow between them is not, so
	// the violation carries the files that made it: that list is what a reader decides between drawing the arrow
	// and deleting the import by.
	diagram := slicesextraction.NewDiagram(fixtureSlices(),
		slicesextraction.Dependency{From: "api", To: "db"},
		slicesextraction.Dependency{From: "api", To: "domain"},
	)

	violations := assertion.GatherDiagramViolations(
		diagram, fixtureDependencies(), fixtureSlices(), assertion.DiagramOptions{})

	want := []string{"db -> api: undrawn dependency (internal/db/conn.go -> internal/api/router.go)"}
	if got := diagramViolations(t, violations); !slices.Equal(got, want) {
		t.Errorf("the project disagrees with its diagram in %v, want %v", got, want)
	}
}

func TestGatherDiagramViolationsReportsASliceTheDiagramDoesNotDeclareOnceAndNotItsArrows(t *testing.T) {
	// The drawing is missing a component, not every arrow that component is an end of. Reporting both would bury
	// the one thing a reader has to do first, and once the component is drawn the arrows it is genuinely missing
	// are the next run's answer.
	dependencies := append(fixtureDependencies(), kernelprojection.NewProjectedEdge("ui", "db",
		extraction.NewEdge("internal/ui/page.go", "internal/db/conn.go", false, extraction.ImportKindPlain)))

	violations := assertion.GatherDiagramViolations(
		fixtureDiagram(), dependencies, append(fixtureSlices(), "ui"), assertion.DiagramOptions{})

	want := []string{"ui: undeclared slice"}
	if got := diagramViolations(t, violations); !slices.Equal(got, want) {
		t.Errorf("the project disagrees with its diagram in %v, want %v", got, want)
	}
}

func TestGatherDiagramViolationsReportsAComponentTheProjectHasNoSliceFor(t *testing.T) {
	// The other direction, and the one a diagram rots in: a folder that was renamed or deleted after the drawing
	// was made. Nothing in the code points at it, which is exactly why nothing but this finding would report it.
	diagram := slicesextraction.NewDiagram(append(fixtureSlices(), "cache"),
		slicesextraction.Dependency{From: "api", To: "db"},
		slicesextraction.Dependency{From: "api", To: "domain"},
		slicesextraction.Dependency{From: "db", To: "api"},
	)

	violations := assertion.GatherDiagramViolations(
		diagram, fixtureDependencies(), fixtureSlices(), assertion.DiagramOptions{})

	want := []string{"cache: absent component"}
	if got := diagramViolations(t, violations); !slices.Equal(got, want) {
		t.Errorf("the project disagrees with its diagram in %v, want %v", got, want)
	}
}

func TestIgnoringExternalSlicesLeavesOutTheComponentsThisProjectHasNoSliceFor(t *testing.T) {
	// For the drawing that is about more than this one project. Without the modifier such a diagram reports
	// every component of every sibling module; with it, this project is judged by the arrows that are about it.
	diagram := slicesextraction.NewDiagram(append(fixtureSlices(), "billing", "cache"),
		slicesextraction.Dependency{From: "api", To: "db"},
		slicesextraction.Dependency{From: "api", To: "domain"},
		slicesextraction.Dependency{From: "db", To: "api"},
	)

	violations := assertion.GatherDiagramViolations(
		diagram, fixtureDependencies(), fixtureSlices(), assertion.DiagramOptions{IgnoreExternalSlices: true})

	if got := diagramViolations(t, violations); len(got) != 0 {
		t.Errorf("the project disagrees with its diagram in %v, want the absent components left out", got)
	}
}

func TestIgnoringOrphanSlicesLeavesOutOnlyTheSlicesNoDependencyReaches(t *testing.T) {
	// The modifier is for the diagram that draws the architecture rather than the folder tree: a slice that
	// imports nothing and that nothing imports says nothing about what may depend on what. A slice that is an
	// end of an arrow and is missing from the drawing is a hole in it, and this modifier must not hide one —
	// at either end, so ui is only the source of an arrow here and mail is only the target of one.
	dependencies := append(fixtureDependencies(),
		kernelprojection.NewProjectedEdge("ui", "db",
			extraction.NewEdge("internal/ui/page.go", "internal/db/conn.go", false, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("api", "mail",
			extraction.NewEdge("internal/api/handler.go", "internal/mail/send.go", false, extraction.ImportKindPlain)))
	present := append(fixtureSlices(), "ui", "mail", "tools")

	violations := assertion.GatherDiagramViolations(
		fixtureDiagram(), dependencies, present, assertion.DiagramOptions{IgnoreOrphanSlices: true})

	want := []string{"mail: undeclared slice", "ui: undeclared slice"}
	if got := diagramViolations(t, violations); !slices.Equal(got, want) {
		t.Errorf("the project disagrees with its diagram in %v, want %v — tools is an orphan, ui and mail are not",
			got, want)
	}
}

func TestGatherDiagramViolationsReportsTheThreeFindingsInOneOrder(t *testing.T) {
	// The order is half of what the rule promises, and it is the order a reader works in: the arrows that are
	// missing from the drawing, then the components that are, then the components that no longer exist. Within
	// each, the projection's order, then alphabetical, then the diagram's own declaration order.
	diagram := slicesextraction.NewDiagram(
		[]string{"api", "db", "domain", "cache"}, slicesextraction.Dependency{From: "api", To: "db"})
	dependencies := append(fixtureDependencies(), kernelprojection.NewProjectedEdge("ui", "db",
		extraction.NewEdge("internal/ui/page.go", "internal/db/conn.go", false, extraction.ImportKindPlain)))

	violations := assertion.GatherDiagramViolations(
		diagram, dependencies, append(fixtureSlices(), "ui"), assertion.DiagramOptions{})

	want := []string{
		"api -> domain: undrawn dependency (internal/api/handler.go -> internal/domain/order.go)",
		"db -> api: undrawn dependency (internal/db/conn.go -> internal/api/router.go)",
		"ui: undeclared slice",
		"cache: absent component",
	}
	if got := diagramViolations(t, violations); !slices.Equal(got, want) {
		t.Errorf("the project disagrees with its diagram in %v, want %v", got, want)
	}
}

func TestGatherDiagramViolationsReportsTheSameListHoweverTheSlicesArrive(t *testing.T) {
	// The slices are the keys of a map, so their order is nobody's promise: they are sorted here, and a name
	// that arrives twice is one slice. A report that changed between two runs of the same suite would be worse
	// than a wrong one.
	diagram := slicesextraction.NewDiagram([]string{"api"}, slicesextraction.Dependency{From: "api", To: "db"})
	want := []string{"domain: undeclared slice", "ui: undeclared slice"}

	for _, present := range [][]string{
		{"api", "db", "domain", "ui"},
		{"ui", "domain", "db", "api"},
		{"db", "ui", "api", "domain", "ui"},
	} {
		violations := assertion.GatherDiagramViolations(diagram, nil, present, assertion.DiagramOptions{})

		if got := diagramViolations(t, violations); !slices.Equal(got, want) {
			t.Errorf("the slices %v reported %v, want %v", present, got, want)
		}
	}
}

func TestGatherDiagramViolationsOnAProjectWithNoDependencyInIt(t *testing.T) {
	// A diagram is a closed statement, so a project that has the slices and none of the arrows disagrees with it
	// in nothing: an arrow is a permission, not a requirement. What the drawing does require is that the
	// components exist, which is the finding this leaves.
	violations := assertion.GatherDiagramViolations(
		fixtureDiagram(), nil, []string{"api", "db"}, assertion.DiagramOptions{})

	want := []string{"domain: absent component"}
	if got := diagramViolations(t, violations); !slices.Equal(got, want) {
		t.Errorf("a project with no dependency reported %v, want %v", got, want)
	}
}
