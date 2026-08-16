package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/layers/projection"
)

func TestPerLayerEdgeProjectsTheDependenciesBetweenTheDeclaredLayers(t *testing.T) {
	// The structure a whole policy is judged over: one edge per pair of layers, labeled by the layers' names
	// rather than by the files, which is why an N-layer policy is one rule instead of N² file rules.
	layers := fixtureLayers(t)

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerLayerEdge(layers...))

	want := []string{"api -> db", "db -> api"}
	if dependencies := edgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the dependencies between %v are %v, want %v", layerNames(layers), dependencies, want)
	}
}

func TestPerLayerEdgeCumulatesTheFileDependenciesEachPairOfLayersStandsFor(t *testing.T) {
	// After relabelling, the files are nowhere else — so a violation about layers can still name the concrete
	// dependencies a reader has to go and unpick, which is the whole reason ProjectEdges keeps them.
	layers := fixtureLayers(t)

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerLayerEdge(layers...))

	for _, dependency := range projected {
		if len(dependency.CumulatedEdges()) == 0 {
			t.Errorf("the dependency %s carries no file dependency, want the ones it was built from", dependency)
		}
	}
	if len(projected) == 0 {
		t.Fatalf("nothing was projected from %v, want the dependencies between the fixture's layers", layerNames(layers))
	}
	found := fileDependencyStrings(projected[0].CumulatedEdges())
	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if !slices.Equal(found, want) {
		t.Errorf("the dependency %s was built from %v, want %v", projected[0], found, want)
	}
}

func TestPerLayerEdgeDropsADependencyInsideOneLayer(t *testing.T) {
	// "Intra-layer dependencies are always allowed", and it falls out of the projection: both ends carry one
	// label, so the mapped edge is a self-edge and ProjectEdges drops it. A policy is about the dependencies
	// *between* layers, so an edge inside one is not a dependency it has.
	layers := fixtureLayers(t)

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerLayerEdge(layers...))

	// internal/api/handler.go -> internal/api/router.go is the edge inside `api`, and internal/db/conn.go ->
	// internal/db/query.go the one inside `db`. Neither may reach the assertion, and the raw edges each
	// projected dependency cumulates are where they would turn up if it did: asking for `api -> api` instead
	// would be asking ProjectEdges whether it drops self-edges, which it does by construction, so a projection
	// that stopped labeling one end by its layer would smuggle both edges past under a mixed pair of labels.
	if len(projected) == 0 {
		t.Fatalf("nothing was projected from %v, want the dependencies between the fixture's layers", layerNames(layers))
	}
	insideOneLayer := []string{
		"internal/api/handler.go -> internal/api/router.go",
		"internal/db/conn.go -> internal/db/query.go",
	}
	for _, dependency := range projected {
		cumulated := fileDependencyStrings(dependency.CumulatedEdges())
		for _, inside := range insideOneLayer {
			if slices.Contains(cumulated, inside) {
				t.Errorf("the dependency %s was built from %s, which is inside one layer: want it dropped",
					dependency, inside)
			}
		}
	}
}

func TestPerLayerEdgeDropsAnEdgeWithAnEndInNoDeclaredLayer(t *testing.T) {
	// "Edges where either end belongs to no declared layer are ignored", which is what makes a policy about
	// part of a project possible: main.go is in no layer, so its dependency on the api is nobody's business.
	api := projection.NewLayer("api", folderMatcher(t, "internal/api/**"))
	db := projection.NewLayer("db", folderMatcher(t, "internal/db/**"))

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerLayerEdge(api, db))

	for _, dependency := range projected {
		for _, edge := range dependency.CumulatedEdges() {
			if edge.Source == "main.go" || edge.Target == "main.go" {
				t.Errorf("the dependency %s was built from the unlayered %s -> %s, want it ignored",
					dependency, edge.Source, edge.Target)
			}
		}
	}
}

func TestPerLayerEdgeDropsTheDependenciesThatLeaveTheProject(t *testing.T) {
	// A layer is a set of this project's own files, so an import of the standard library or of a third-party
	// module can only be in no layer anyway — and the drop goes through the shared PerInternalEdge, so that
	// decision is not made twice.
	orm := projection.NewLayer("orm", pathMatcher(t, "gorm.io/**"))
	db := projection.NewLayer("db", folderMatcher(t, "internal/db/**"))

	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerLayerEdge(db, orm))

	if dependencies := edgeStrings(projected); len(dependencies) != 0 {
		t.Errorf("the projection is %v, want nothing: gorm.io/gorm is not a file of this project", dependencies)
	}
}

func TestPerLayerEdgeProjectsNothingWhenNoLayerIsDeclared(t *testing.T) {
	// The loud direction. A policy whose folders have all been renamed projects no edge at all, which is what
	// the empty-test guard reports on rather than a rule that quietly holds over nothing.
	projected := kernel.ProjectEdges(fixtureGraph(), projection.PerLayerEdge())

	if dependencies := edgeStrings(projected); len(dependencies) != 0 {
		t.Errorf("the projection is %v, want nothing when no layer was declared", dependencies)
	}
}

// fixtureLayers are the two layers the fixture project is judged as: the api folder and the db folder, which
// depend on each other in the fixture and so make both directions of a policy observable.
func fixtureLayers(t *testing.T) []projection.Layer {
	t.Helper()

	return []projection.Layer{
		projection.NewLayer("api", folderMatcher(t, "internal/api/**")),
		projection.NewLayer("db", folderMatcher(t, "internal/db/**")),
	}
}

// edgeStrings renders projected edges as `api -> db`, in the order they were projected, for a message about
// what a projection came to.
func edgeStrings(projected []kernel.ProjectedEdge) []string {
	rendered := make([]string, 0, len(projected))
	for _, edge := range projected {
		rendered = append(rendered, edge.SourceLabel()+" -> "+edge.TargetLabel())
	}
	return rendered
}

// fileDependencyStrings renders extracted edges as `a.go -> b.go`, for a message about which file
// dependencies a projected edge was built from.
func fileDependencyStrings(graph extraction.Graph) []string {
	rendered := make([]string, 0, len(graph))
	for _, edge := range graph {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}
