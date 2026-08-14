package projection

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

func nodeLabels(nodes []ProjectedNode) []string {
	labels := make([]string, 0, len(nodes))
	for _, node := range nodes {
		labels = append(labels, node.Label())
	}
	return labels
}

func findProjectedNode(t *testing.T, nodes []ProjectedNode, label string) ProjectedNode {
	t.Helper()
	for _, node := range nodes {
		if node.Label() == label {
			return node
		}
	}
	t.Fatalf("no projected node %q in %v", label, nodeLabels(nodes))
	return ProjectedNode{}
}

func TestProjectToNodesGivesEveryFileANode(t *testing.T) {
	nodes := ProjectToNodes(fixtureGraph(), PerEdge())

	want := []string{
		"database/sql",
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/query.go",
		"internal/db/repo.go",
		"internal/util/noop.go",
	}
	if got := nodeLabels(nodes); !slices.Equal(got, want) {
		t.Errorf("nodes = %v, want %v", got, want)
	}
}

func TestProjectToNodesKeepsAFileThatDependsOnNothing(t *testing.T) {
	// The self-edge is the only thing in the graph about noop.go, and a rule about naming or placement
	// is about every file — so losing it here would silently shrink the population that rule judges.
	nodes := ProjectToNodes(fixtureGraph(), PerEdge())

	noop := findProjectedNode(t, nodes, "internal/util/noop.go")
	if len(noop.Incoming()) != 0 || len(noop.Outgoing()) != 0 {
		t.Errorf("%s has edges, want none: its only edge is its own self-edge", noop)
	}
}

func TestProjectToNodesWiresIncomingAndOutgoing(t *testing.T) {
	nodes := ProjectToNodes(fixtureGraph(), PerEdge())

	handler := findProjectedNode(t, nodes, "internal/api/handler.go")
	if got, want := labelsOf(handler.Outgoing()), []string{
		"internal/api/handler.go -> internal/db/query.go",
		"internal/api/handler.go -> internal/db/repo.go",
	}; !slices.Equal(got, want) {
		t.Errorf("handler.Outgoing() = %v, want %v", got, want)
	}
	if got, want := labelsOf(handler.Incoming()), []string{
		"internal/api/router.go -> internal/api/handler.go",
	}; !slices.Equal(got, want) {
		t.Errorf("handler.Incoming() = %v, want %v", got, want)
	}
}

func TestProjectToNodesNamesASliceWhoseFilesOnlyDependOnEachOther(t *testing.T) {
	// db's two files are a node of the folder projection even though the folder depends on nothing, and
	// the api-to-api dependency is not an edge of the api node.
	nodes := ProjectToNodes(fixtureGraph(), sliceByFolder())

	if got, want := nodeLabels(nodes), []string{"internal/api", "internal/db", "internal/util"}; !slices.Equal(got, want) {
		t.Errorf("nodes = %v, want %v", got, want)
	}
	api := findProjectedNode(t, nodes, "internal/api")
	if got, want := labelsOf(api.Outgoing()), []string{"internal/api -> internal/db"}; !slices.Equal(got, want) {
		t.Errorf("api.Outgoing() = %v, want %v", got, want)
	}
	if len(api.Incoming()) != 0 {
		t.Errorf("api.Incoming() = %v, want nothing depending on it", labelsOf(api.Incoming()))
	}
}

func TestProjectToNodesLeavesEveryEdgeWhoseLabelsAreEqualOutOfBothLists(t *testing.T) {
	for _, mapper := range []struct {
		name   string
		mapper MapFunction
	}{
		{name: "PerEdge", mapper: PerEdge()},
		{name: "sliceByFolder", mapper: sliceByFolder()},
	} {
		for _, node := range ProjectToNodes(fixtureGraph(), mapper.mapper) {
			for _, edge := range slices.Concat(node.Incoming(), node.Outgoing()) {
				if edge.IsSelfEdge() {
					t.Errorf("%s: node %s carries the self-edge %s, want it named the node only", mapper.name, node, edge)
				}
			}
		}
	}
}

func TestProjectToNodesCarriesTheSameEdgesAsProjectEdges(t *testing.T) {
	graph := fixtureGraph()

	projected := ProjectEdges(graph, PerEdge())
	nodes := ProjectToNodes(graph, PerEdge())

	outgoing := make([]ProjectedEdge, 0, len(projected))
	incoming := make([]ProjectedEdge, 0, len(projected))
	for _, node := range nodes {
		outgoing = append(outgoing, node.Outgoing()...)
		incoming = append(incoming, node.Incoming()...)
	}
	// Every dependency appears once on its source and once on its target, and it is the very same
	// projected edge — raw edges included — that a rule about dependencies reads. Read off the sources in
	// label order the edges come out in the projection's own order; off the targets they come out grouped
	// by target instead, which is why that half is compared as a set.
	if got, want := labelsOf(outgoing), labelsOf(projected); !slices.Equal(got, want) {
		t.Errorf("the outgoing edges of every node = %v, want the projected edges %v", got, want)
	}
	byTarget := labelsOf(incoming)
	slices.Sort(byTarget)
	if want := labelsOf(projected); !slices.Equal(byTarget, want) {
		t.Errorf("the incoming edges of every node = %v, want the projected edges %v", byTarget, want)
	}
	for index, edge := range outgoing {
		if !slices.Equal(edge.CumulatedEdges(), projected[index].CumulatedEdges()) {
			t.Errorf("%s cumulates %v, want %v", edge, edge.CumulatedEdges(), projected[index].CumulatedEdges())
		}
	}
}

func TestProjectToNodesLeavesExternalNodesOutWhenTheMapFunctionDropsThem(t *testing.T) {
	nodes := ProjectToNodes(fixtureGraph(), PerInternalEdge())

	for _, node := range nodes {
		if node.Label() == "database/sql" {
			t.Errorf("nodes = %v, want no external node", nodeLabels(nodes))
		}
	}
}

func TestProjectToNodesHandsOutCopiesOfItsEdgeLists(t *testing.T) {
	nodes := ProjectToNodes(fixtureGraph(), PerEdge())
	handler := findProjectedNode(t, nodes, "internal/api/handler.go")

	outgoing := handler.Outgoing()
	outgoing[0] = NewProjectedEdge("nonsense.go", "elsewhere.go")
	incoming := handler.Incoming()
	incoming[0] = NewProjectedEdge("nonsense.go", "elsewhere.go")

	if got := labelsOf(handler.Outgoing()); got[0] != "internal/api/handler.go -> internal/db/query.go" {
		t.Errorf("Outgoing() = %v after a caller wrote through it, want the node unchanged", got)
	}
	if got := labelsOf(handler.Incoming()); got[0] != "internal/api/router.go -> internal/api/handler.go" {
		t.Errorf("Incoming() = %v after a caller wrote through it, want the node unchanged", got)
	}
}

func TestProjectToNodesWithoutAMapFunctionProjectsNothing(t *testing.T) {
	if nodes := ProjectToNodes(fixtureGraph(), nil); len(nodes) != 0 {
		t.Errorf("ProjectToNodes(graph, nil) = %v, want nothing", nodeLabels(nodes))
	}
}

func TestProjectedNodeStringRendersItsLabelAndDegrees(t *testing.T) {
	nodes := ProjectToNodes(fixtureGraph(), PerEdge())
	handler := findProjectedNode(t, nodes, "internal/api/handler.go")

	if got, want := handler.String(), "internal/api/handler.go [1 in, 2 out]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestProjectToNodesGivesThisRepositoryOneNodePerFolder(t *testing.T) {
	// The level above the hand-written fixture, as in project_edges_test.go: a real extraction, projected
	// into folders, where every folder of this library has to be a node — including the one whose files
	// depend on nothing outside it.
	root, err := extraction.LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}
	graph, err := extraction.CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph(%q, nil) failed: %v", root, err)
	}

	nodes := ProjectToNodes(graph, sliceByFolder())

	for _, folder := range []string{"common/extraction", "common/matching", "common/projection", "common/projection/cycles"} {
		node := findProjectedNode(t, nodes, folder)
		if node.Label() != folder {
			t.Errorf("node = %s, want %q", node, folder)
		}
	}
	// common/matching depends on nothing of this library's own, and PerInternalEdge-style dropping is not
	// in play here — sliceByFolder drops the external edges — so it is a node with no outgoing edge at all.
	if matching := findProjectedNode(t, nodes, "common/matching"); len(matching.Outgoing()) != 0 {
		t.Errorf("common/matching depends on %v, want nothing: the kernel's matcher stands alone", labelsOf(matching.Outgoing()))
	}
	if !slices.IsSorted(nodeLabels(nodes)) {
		t.Errorf("nodes = %v, want them ordered by label", nodeLabels(nodes))
	}
}
