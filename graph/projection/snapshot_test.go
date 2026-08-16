package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestNewSnapshotPutsTheNodesAndTheDependenciesInOneOrder(t *testing.T) {
	// The same project rendered twice has to be the same diagram, byte for byte, or a checked-in artifact is
	// not reviewable. The projection builds its result from maps, so the order is established here.
	nodes := []projection.Node{
		projection.NewNode("main.go"),
		projection.NewExternalNode("net/http"),
		projection.NewNode("internal/api"),
	}
	edges := []projection.Edge{
		projection.NewEdge("main.go", "internal/api"),
		projection.NewEdge("internal/api", "net/http"),
		projection.NewEdge("internal/api", "main.go"),
	}

	snapshot := projection.NewSnapshot("", nodes, edges)

	wantNodes := []string{"internal/api", "main.go", "net/http"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the nodes are ordered %v, want %v", labels, wantNodes)
	}
	wantEdges := []string{"internal/api -> main.go", "internal/api -> net/http", "main.go -> internal/api"}
	if got := edgeEnds(snapshot.Edges()); !slices.Equal(got, wantEdges) {
		t.Errorf("the dependencies are ordered %v, want %v", got, wantEdges)
	}
}

func TestNewSnapshotCountsWhatItWasGiven(t *testing.T) {
	// The summary is derived rather than passed in, because a report that says `12 nodes` above a diagram of
	// eleven is worse than no report at all.
	nodes := []projection.Node{
		projection.NewNode("internal/api"),
		projection.NewNode("internal/db"),
		projection.NewExternalNode("database/sql"),
	}
	edges := []projection.Edge{
		projection.NewEdge("internal/api", "internal/db", plainDependency("a.go", "b.go"), plainDependency("c.go", "d.go")),
		projection.NewEdge("internal/db", "database/sql", externalDependency("b.go", "database/sql")),
	}

	summary := projection.NewSnapshot("", nodes, edges).Summary()

	want := projection.Summary{Nodes: 3, Edges: 2, Dependencies: 3, ExternalNodes: 1, ExternalEdges: 1}
	if summary != want {
		t.Errorf("the summary is %+v, want %+v", summary, want)
	}
}

func TestNewSnapshotCopiesWhatItWasGiven(t *testing.T) {
	// A snapshot is a value that has already been decided, so a caller reusing the slice it passed in must not
	// be able to reach into a report that was built from it.
	nodes := []projection.Node{projection.NewNode("internal/api"), projection.NewNode("internal/db")}
	edges := []projection.Edge{projection.NewEdge("internal/api", "internal/db")}

	snapshot := projection.NewSnapshot("kept", nodes, edges)
	nodes[0] = projection.NewExternalNode("somebody/elses/code")
	edges[0] = projection.NewEdge("nowhere", "nothing")

	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, []string{"internal/api", "internal/db"}) {
		t.Errorf("editing the caller's slice changed the snapshot's nodes to %v", labels)
	}
	if ends := edgeEnds(snapshot.Edges()); !slices.Equal(ends, []string{"internal/api -> internal/db"}) {
		t.Errorf("editing the caller's slice changed the snapshot's dependencies to %v", ends)
	}
}

func TestSnapshotHandsOutCopiesOfItsNodesAndDependencies(t *testing.T) {
	// The other half of the same contract: reading a snapshot must not be able to change it, and every
	// renderer is a reader.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("internal/api")}, []projection.Edge{projection.NewEdge("internal/api", "internal/db")})

	snapshot.Nodes()[0] = projection.NewExternalNode("edited")
	snapshot.Edges()[0] = projection.NewEdge("edited", "edited")

	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, []string{"internal/api"}) {
		t.Errorf("the nodes are %v after a reader edited what it was handed, want the ones the snapshot was built with", labels)
	}
	if ends := edgeEnds(snapshot.Edges()); !slices.Equal(ends, []string{"internal/api -> internal/db"}) {
		t.Errorf("the dependencies are %v after a reader edited what it was handed", ends)
	}
}

func TestSnapshotIsEmptyWhenNoNodeIsInIt(t *testing.T) {
	// The failure the terminal's guard turns into an error: a query whose patterns name nothing renders a
	// blank diagram, and a blank diagram looks exactly like a project that is clean.
	if !(projection.Snapshot{}).Empty() {
		t.Error("the zero snapshot is not empty, want the empty graph")
	}
	if !projection.NewSnapshot("nothing", nil, nil).Empty() {
		t.Error("a snapshot built with no node is not empty")
	}
}

func TestSnapshotIsNotEmptyWhenItHasNodesAndNoDependency(t *testing.T) {
	// A set of files that depend on nothing is a real answer, and often a good one.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("internal/util/orphan.go")}, nil)

	if snapshot.Empty() {
		t.Error("a snapshot with a node and no dependency is empty, want it a report of that node")
	}
}

func TestSnapshotStringRendersTheTitleTheCountsTheNodesAndTheDependencies(t *testing.T) {
	// What a reader of a failing test sees. The nodes are listed as well as the arrows, because a node in no
	// dependency is in none of the lines below it.
	nodes := []projection.Node{
		projection.NewNode("internal/api"),
		projection.NewNode("internal/db"),
		projection.NewNode("internal/util"),
	}
	edges := []projection.Edge{
		projection.NewEdge("internal/api", "internal/db", plainDependency("a.go", "b.go"), plainDependency("c.go", "b.go")),
	}

	got := projection.NewSnapshot("the modules of this project", nodes, edges).String()

	want := "the modules of this project [3 nodes, 1 edge, 2 dependencies]\n" +
		"nodes: internal/api, internal/db, internal/util\n" +
		"internal/api -> internal/db [2 dependencies] [plain]"
	if got != want {
		t.Errorf("the snapshot renders as\n%s\nwant\n%s", got, want)
	}
}

func TestSnapshotStringNamesAnUntitledReportAfterWhatItIs(t *testing.T) {
	// A renderer picks its own headline for an untitled report; this one is for logs and test failures, and a
	// failure with no headline at all reads as a bug.
	got := projection.NewSnapshot("", []projection.Node{projection.NewNode("main.go")}, nil).String()

	want := "dependency graph [1 node, 0 edges, 0 dependencies]\nnodes: main.go"
	if got != want {
		t.Errorf("the untitled snapshot renders as\n%s\nwant\n%s", got, want)
	}
}

// edgeEnds are the pairs of labels a snapshot's dependencies run between, in order, for a test about ordering
// or copying that does not care what each edge stands for.
func edgeEnds(edges []projection.Edge) []string {
	ends := make([]string, 0, len(edges))
	for _, edge := range edges {
		ends = append(ends, edge.SourceLabel()+" -> "+edge.TargetLabel())
	}
	return ends
}

// plainDependency is one of the project's own dependencies, as the extractor emits it.
func plainDependency(source, target string) extraction.Edge {
	return extraction.NewEdge(source, target, false, extraction.ImportKindPlain)
}

// externalDependency is one that leaves the project.
func externalDependency(source, target string) extraction.Edge {
	return extraction.NewEdge(source, target, true, extraction.ImportKindPlain)
}
