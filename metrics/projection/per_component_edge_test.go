package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

func TestPerComponentEdgeProjectsTheDependenciesBetweenTheFoldersOfTheSelectedFiles(t *testing.T) {
	// The structure the coupling half of every distance metric is read off: one edge per pair of folders,
	// labeled the way metrics/extraction spells a file's directory, so a component and its files are named
	// the same way by construction.
	selected := []string{"main.go", "internal/api/handler.go", "internal/db/conn.go"}

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerComponentEdge(selected))

	want := []string{". -> internal/api", "internal/api -> internal/db"}
	if dependencies := componentEdgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the dependencies between the components are %v, want %v", dependencies, want)
	}
}

func TestPerComponentEdgeDropsAnEdgeWithAnEndOutsideTheSelection(t *testing.T) {
	// The scope means the same thing here as it does for a rule about cycles: a dependency leaving the
	// selection is a dependency the selected files have, not a dependency between the components the rule is
	// about. main.go is not selected, so the root is not a component and depends on nothing.
	selected := []string{"internal/api/handler.go", "internal/db/conn.go"}

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerComponentEdge(selected))

	want := []string{"internal/api -> internal/db"}
	if dependencies := componentEdgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the dependencies between the components are %v, want %v", dependencies, want)
	}
}

func TestPerComponentEdgeDropsADependencyInsideOneFolder(t *testing.T) {
	// "A component is not coupled to itself": both ends carry one label, so the mapped edge is a self-edge
	// and ProjectEdges drops it. The imports inside one package are what make it a package, and instability
	// is about the dependencies that cross a package boundary.
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/api/router.go", false, extraction.ImportKindPlain),
	)
	selected := []string{"internal/api/handler.go", "internal/api/router.go"}

	projected := kernel.ProjectEdges(graph, projection.PerComponentEdge(selected))

	if dependencies := componentEdgeStrings(projected); len(dependencies) != 0 {
		t.Errorf("the projection is %v, want nothing: a folder is not coupled to itself", dependencies)
	}
}

func TestPerComponentEdgeDropsTheDependenciesThatLeaveTheProject(t *testing.T) {
	// An import path of the standard library or of another module is not a folder of this project and could
	// never be a component of it. The drop goes through the shared PerInternalEdge, so that decision is not
	// made twice — and `fmt` would otherwise be labeled `.`, which is the project root.
	selected := []string{"internal/api/handler.go", "fmt", "golang.org/x/tools/go/packages"}

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerComponentEdge(selected))

	if dependencies := componentEdgeStrings(projected); len(dependencies) != 0 {
		t.Errorf("the projection is %v, want nothing: an external import path is not a component", dependencies)
	}
}

func TestPerComponentEdgeCumulatesTheFileDependenciesEachPairOfFoldersStandsFor(t *testing.T) {
	// A component's coupling is one number, but the files behind it are what a reader has to go and unpick,
	// and after relabelling they are nowhere else.
	selected := []string{"internal/api/handler.go", "internal/db/conn.go"}

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerComponentEdge(selected))

	if len(projected) != 1 {
		t.Fatalf("the projection is %v, want the one dependency between the two folders", componentEdgeStrings(projected))
	}
	found := componentFileDependencyStrings(projected[0].CumulatedEdges())
	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if !slices.Equal(found, want) {
		t.Errorf("the dependency %s was built from %v, want %v", projected[0], found, want)
	}
}

func TestPerComponentEdgeProjectsNothingForAnEmptySelection(t *testing.T) {
	// The loud direction. A rule whose folder has been renamed projects no edge at all, so its component
	// population is empty and the empty-test guard reports it rather than the rule quietly holding.
	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerComponentEdge(nil))

	if dependencies := componentEdgeStrings(projected); len(dependencies) != 0 {
		t.Errorf("the projection is %v, want nothing when no file was selected", dependencies)
	}
}

// componentEdgeStrings renders projected edges as `internal/api -> internal/db`, in the order they were
// projected, for a message about what a projection came to.
func componentEdgeStrings(projected []kernel.ProjectedEdge) []string {
	rendered := make([]string, 0, len(projected))
	for _, edge := range projected {
		rendered = append(rendered, edge.SourceLabel()+" -> "+edge.TargetLabel())
	}
	return rendered
}

// componentFileDependencyStrings renders extracted edges as `a.go -> b.go`, for a message about which file
// dependencies a projected edge was built from.
func componentFileDependencyStrings(graph extraction.Graph) []string {
	rendered := make([]string, 0, len(graph))
	for _, edge := range graph {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}
