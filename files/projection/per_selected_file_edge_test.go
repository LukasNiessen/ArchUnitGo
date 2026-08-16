package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

func TestPerSelectedFileEdgeKeepsTheDependenciesBetweenTheSelectedFiles(t *testing.T) {
	graph := fixtureGraph()
	selected := projection.SelectFiles(graph)

	projected := kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected))

	want := []string{
		"internal/api/handler.go -> internal/db/conn.go",
		"main.go -> internal/api/handler.go",
		"main.go -> internal/api/router.go",
	}
	if dependencies := edgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the projection of every file is %v, want %v", dependencies, want)
	}
}

func TestPerSelectedFileEdgeDropsADependencyThatLeavesTheSelection(t *testing.T) {
	// The scope of a rule about dependencies is read as "the dependencies between these files": a
	// dependency on a file the rule did not select is a dependency the selection has, not one inside it.
	graph := fixtureGraph()
	selected := projection.SelectFiles(graph, folderMatcher(t, "internal/api"))

	projected := kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected))

	if len(projected) != 0 {
		t.Errorf("the projection of `in folder internal/api` is %v, want no dependency between its two files", edgeStrings(projected))
	}
	// main.go -> internal/api/handler.go arrives at the selection and internal/api/handler.go ->
	// internal/db/conn.go leaves it, so both ends have to be selected for either to be kept.
	wider := projection.SelectFiles(graph, folderMatcher(t, "internal/**"))
	widened := kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(wider))
	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if dependencies := edgeStrings(widened); !slices.Equal(dependencies, want) {
		t.Errorf("the projection of `in folder internal/**` is %v, want %v", dependencies, want)
	}
}

func TestPerSelectedFileEdgeProjectsNothingForAnEmptySelection(t *testing.T) {
	// A scope that named no file has no dependencies to judge. Projecting nothing is the loud direction:
	// it is what the empty-test guard reports on, where projecting everything would judge a rule against
	// files nobody selected.
	graph := fixtureGraph()

	for _, selected := range [][]string{nil, {}} {
		projected := kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected))

		if len(projected) != 0 {
			t.Errorf("the projection of %v files is %v, want nothing projected", selected, edgeStrings(projected))
		}
	}
}

func TestPerSelectedFileEdgeNeverKeepsADependencyThatLeavesTheProject(t *testing.T) {
	// Only the project's own files are selectable, so an external target cannot be one end of a kept
	// edge — and it stays that way even when a caller passes an import path among the identifiers, which
	// is the mistake a mapper reading the selection alone would honor.
	graph := fixtureGraph()
	selected := append(projection.SelectFiles(graph), "fmt", "golang.org/x/tools/go/packages")

	projected := kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected))

	for _, edge := range projected {
		if edge.TargetLabel() == "fmt" || edge.TargetLabel() == "golang.org/x/tools/go/packages" {
			t.Errorf("the projection holds %s, want the dependencies that leave the project dropped", edge)
		}
	}
}

func TestPerSelectedFileEdgeDropsTheSelfEdgeThatOnlyNamesAFile(t *testing.T) {
	// The rule of the whole `per <thing> edge` family: each of them is about dependencies, so none keeps
	// the edge that exists only to make a file a node. A rule about the files themselves is projected
	// through kernel.Identity instead.
	mapper := projection.PerSelectedFileEdge([]string{"main.go"})

	if _, kept := mapper(extraction.SelfEdge("main.go")); kept {
		t.Error("PerSelectedFileEdge keeps a selected file's self-edge, want it dropped")
	}
	nodes := kernel.ProjectToNodes(fixtureGraph(), projection.PerSelectedFileEdge(projection.SelectFiles(fixtureGraph())))
	for _, node := range nodes {
		if node.Label() == "internal/db/query.go" {
			t.Errorf("the projection has a node for %s, want a file that depends on nothing to have none", node.Label())
		}
	}
}

func TestPerSelectedFileEdgeLabelsAFileByItsIdentifierAndKeepsTheRawEdges(t *testing.T) {
	// The labels are the identifiers the extractor minted, because a rule about files is written against
	// exactly those strings — and the raw edges survive under them, which is what lets a violation name
	// the import that made the dependency.
	graph := fixtureGraph()
	selected := projection.SelectFiles(graph)

	projected := kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected))

	found := false
	for _, edge := range projected {
		if edge.SourceLabel() != "main.go" || edge.TargetLabel() != "internal/api/handler.go" {
			continue
		}
		found = true
		raw := edge.CumulatedEdges()
		if len(raw) != 1 {
			t.Fatalf("%s cumulates %v, want the one raw edge it was projected from", edge, raw)
		}
		if raw[0].Source != "main.go" || raw[0].Target != "internal/api/handler.go" {
			t.Errorf("%s cumulates %v, want the raw edge between the same two files", edge, raw)
		}
		if !raw[0].ImportKinds.Contains(extraction.ImportKindPlain) {
			t.Errorf("%s cumulates %v, want the import kind the extractor read", edge, raw)
		}
	}
	if !found {
		t.Errorf("the projection %v has no edge between the two files that depend on each other", edgeStrings(projected))
	}
}

func TestPerSelectedFileEdgeFindsTheCyclesBetweenTheSelectedFiles(t *testing.T) {
	// The projection a `should have no cycles` rule about files is judged over, end to end on a hand-built
	// graph: the mapper, the projected edges, and the circuits inside them.
	graph := cyclicFixtureGraph()
	selected := projection.SelectFiles(graph)

	circuits, complete := cycles.ProjectCircuits(kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected)), nil)

	if !complete {
		t.Fatalf("the enumeration of %d circuits is truncated, want a fixture small enough to enumerate whole", len(circuits))
	}
	want := []string{
		"internal/api/handler.go -> internal/db/conn.go -> internal/api/handler.go",
		"internal/api/handler.go -> internal/db/conn.go -> internal/db/query.go -> internal/api/handler.go",
	}
	if found := circuitStrings(circuits); !slices.Equal(found, want) {
		t.Errorf("the cycles between the selected files are %v, want %v", found, want)
	}
}

func TestPerSelectedFileEdgeFindsNoCycleWhenTheScopeCutsItOpen(t *testing.T) {
	// The consequence of reading the scope as "the dependencies between these files": a cycle running
	// through a file the rule did not select is not this rule's cycle. Widening the scope is what makes it
	// visible, and it is the same graph either way.
	graph := cyclicFixtureGraph()
	selected := projection.SelectFiles(graph, folderMatcher(t, "internal/api"))

	circuits, _ := cycles.ProjectCircuits(kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected)), nil)

	if len(circuits) != 0 {
		t.Errorf("the cycles inside `in folder internal/api` are %v, want none: the cycle leaves that folder", circuitStrings(circuits))
	}
}

// cyclicFixtureGraph is fixtureGraph with the dependency that closes it: internal/db/conn.go depends
// back on internal/api/handler.go, which puts three files in one cycle and two elementary circuits
// between them.
func cyclicFixtureGraph() extraction.Graph {
	return fixtureGraph().Add(
		extraction.NewEdge("internal/db/conn.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "internal/db/query.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/query.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
	)
}

// edgeStrings names the dependencies of a projection, which is what a failing test needs to read.
func edgeStrings(edges []kernel.ProjectedEdge) []string {
	rendered := make([]string, 0, len(edges))
	for _, edge := range edges {
		rendered = append(rendered, edge.SourceLabel()+" -> "+edge.TargetLabel())
	}
	return rendered
}

// circuitStrings renders the cycles as the readable paths they are, which is also how a violation
// reports one.
func circuitStrings(circuits []cycles.Circuit) []string {
	rendered := make([]string, 0, len(circuits))
	for _, circuit := range circuits {
		rendered = append(rendered, circuit.String())
	}
	return rendered
}
