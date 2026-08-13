package extraction

import (
	"slices"
	"testing"
)

// fixtureGraph is a small hand-built graph: two files in the api folder, one in db, one external
// dependency, and a self-edge for a file that depends on nothing.
func fixtureGraph() Graph {
	return NewGraph(
		SelfEdge("internal/api/handler.go"),
		SelfEdge("internal/api/router.go"),
		SelfEdge("internal/db/repo.go"),
		SelfEdge("internal/util/noop.go"),
		NewEdge("internal/api/handler.go", "internal/db/repo.go", false, ImportKindPlain),
		NewEdge("internal/api/router.go", "internal/api/handler.go", false, ImportKindPlain),
		NewEdge("internal/db/repo.go", "database/sql", true, ImportKindPlain),
		NewEdge("internal/db/repo.go", "github.com/lib/pq", true, ImportKindBlank),
	)
}

func TestNewGraphMergesParallelEdges(t *testing.T) {
	graph := NewGraph(
		NewEdge("a.go", "b.go", false, ImportKindPlain),
		NewEdge("a.go", "b.go", false, ImportKindAliased),
		NewEdge(`.\a.go`, "./b.go", false, ImportKindBlank),
		NewEdge("a.go", "c.go", false, ImportKindDot),
	)

	if len(graph) != 2 {
		t.Fatalf("graph has %d edges, want 2:\n%v", len(graph), graph)
	}

	merged, found := graph.Find("a.go", "b.go")
	if !found {
		t.Fatalf("a.go -> b.go missing from:\n%v", graph)
	}
	want := []ImportKind{ImportKindPlain, ImportKindAliased, ImportKindBlank}
	if got := merged.ImportKinds.Kinds(); !slices.Equal(got, want) {
		t.Errorf("ImportKinds = %v, want %v", got, want)
	}
}

func TestNewGraphKeepsSourceTargetUnique(t *testing.T) {
	graph := fixtureGraph().Add(
		NewEdge("internal/api/handler.go", "internal/db/repo.go", false, ImportKindDot),
	)

	seen := make(map[edgeKey]int)
	for _, edge := range graph {
		seen[edge.key()]++
	}
	for key, count := range seen {
		if count != 1 {
			t.Errorf("%q -> %q appears %d times, want 1", key.source, key.target, count)
		}
	}
}

func TestNewGraphNormalizesIdentifiers(t *testing.T) {
	graph := NewGraph(Edge{Source: `internal\api\handler.go`, Target: "./internal/db/repo.go"})

	if len(graph) != 1 {
		t.Fatalf("graph has %d edges, want 1", len(graph))
	}
	if graph[0].Source != "internal/api/handler.go" || graph[0].Target != "internal/db/repo.go" {
		t.Errorf("edge = %v, want normalised identifiers", graph[0])
	}
}

func TestNewGraphDropsEdgesWithoutIdentifiers(t *testing.T) {
	graph := NewGraph(
		NewEdge("", "b.go", false, ImportKindPlain),
		NewEdge("a.go", "  ", false, ImportKindPlain),
		NewEdge("a.go", "b.go", false, ImportKindPlain),
	)

	if len(graph) != 1 {
		t.Fatalf("graph = %v, want only the edge with two identifiers", graph)
	}
}

func TestNewGraphOrdersEdgesReproducibly(t *testing.T) {
	edges := []Edge{
		NewEdge("b.go", "a.go", false, ImportKindPlain),
		NewEdge("a.go", "z.go", false, ImportKindPlain),
		NewEdge("a.go", "b.go", false, ImportKindPlain),
		SelfEdge("c.go"),
	}
	want := []string{"a.go -> b.go", "a.go -> z.go", "b.go -> a.go", "c.go -> c.go"}

	// Built repeatedly, and from different input orders, the result has to be identical: map
	// iteration must never leak into a report.
	for run := range 10 {
		shuffled := slices.Clone(edges)
		shuffled = append(shuffled[run%len(shuffled):], shuffled[:run%len(shuffled)]...)

		graph := NewGraph(shuffled...)

		got := make([]string, 0, len(graph))
		for _, edge := range graph {
			got = append(got, edge.Source+" -> "+edge.Target)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("run %d: order = %v, want %v", run, got, want)
		}
	}
}

func TestGraphAddDoesNotMutateTheReceiver(t *testing.T) {
	base := NewGraph(NewEdge("a.go", "b.go", false, ImportKindPlain))

	extended := base.Add(NewEdge("a.go", "c.go", false, ImportKindPlain))

	if len(base) != 1 {
		t.Errorf("base graph has %d edges, want 1: Add mutated it", len(base))
	}
	if len(extended) != 2 {
		t.Errorf("extended graph has %d edges, want 2", len(extended))
	}
	if _, found := base.Find("a.go", "c.go"); found {
		t.Error("the added edge leaked into the base graph")
	}
}

func TestGraphAddMergesIntoExistingEdges(t *testing.T) {
	base := NewGraph(NewEdge("a.go", "b.go", false, ImportKindPlain))

	extended := base.Add(NewEdge("a.go", "b.go", false, ImportKindDot))

	if len(extended) != 1 {
		t.Fatalf("extended graph = %v, want one merged edge", extended)
	}
	if extended[0].ImportKinds.Len() != 2 {
		t.Errorf("ImportKinds = %v, want plain and dot", extended[0].ImportKinds)
	}
}

func TestGraphNodes(t *testing.T) {
	want := []string{
		"database/sql",
		"github.com/lib/pq",
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/repo.go",
		"internal/util/noop.go",
	}

	if got := fixtureGraph().Nodes(); !slices.Equal(got, want) {
		t.Errorf("Nodes() = %v, want %v", got, want)
	}
}

func TestGraphNodesIncludesFilesWithoutDependencies(t *testing.T) {
	// internal/util/noop.go depends on nothing and nothing depends on it. Its self-edge is the
	// only reason it is a node at all.
	graph := fixtureGraph()

	if !slices.Contains(graph.Nodes(), "internal/util/noop.go") {
		t.Errorf("Nodes() = %v, want it to include the file with no dependencies", graph.Nodes())
	}
}

func TestGraphFind(t *testing.T) {
	graph := fixtureGraph()

	edge, found := graph.Find(`internal\db\repo.go`, "./github.com/lib/pq")
	if !found {
		t.Fatalf("Find should normalise its arguments before looking:\n%v", graph)
	}
	if !edge.External || !edge.ImportKinds.Contains(ImportKindBlank) {
		t.Errorf("edge = %v, want an external blank import", edge)
	}

	if _, found := graph.Find("internal/db/repo.go", "internal/api/handler.go"); found {
		t.Error("Find should not invent an edge the graph does not have")
	}
}

func TestGraphString(t *testing.T) {
	graph := NewGraph(
		NewEdge("a.go", "fmt", true, ImportKindPlain),
		SelfEdge("a.go"),
	)

	want := "a.go -> itself\na.go -> fmt (external) [plain]"
	if got := graph.String(); got != want {
		t.Errorf("String() =\n%s\nwant\n%s", got, want)
	}
}
