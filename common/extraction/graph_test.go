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

func TestNewGraphReducesAnEdgeFromANodeToItselfToASelfEdge(t *testing.T) {
	// The literal bypasses NewEdge on purpose: normalisation, not the caller, is what turns these two
	// identifiers into one node, which is the input class NewGraph normalises raw literals for.
	graph := NewGraph(Edge{
		Source:      "internal/api/handler.go",
		Target:      `internal\api\handler.go`,
		External:    true,
		ImportKinds: NewImportKindSet(ImportKindPlain),
	})

	if len(graph) != 1 {
		t.Fatalf("graph = %v, want one edge", graph)
	}
	// One shape of self-edge in a graph, not two: projections drop self-edges without reading either
	// field, so a self-edge claiming a plain import of an external module has nowhere honest to be read.
	if want := SelfEdge("internal/api/handler.go"); graph[0] != want {
		t.Errorf("graph[0] = %#v, want %#v", graph[0], want)
	}
}

func TestNewGraphMergesAFilesImportOfItselfIntoItsSelfEdge(t *testing.T) {
	graph := NewGraph(
		SelfEdge("a.go"),
		NewEdge("a.go", "a.go", false, ImportKindBlank),
		NewEdge("a.go", "b.go", false, ImportKindPlain),
	)

	self, found := graph.Find("a.go", "a.go")
	if !found {
		t.Fatalf("graph = %v, want the self-edge of a.go", graph)
	}
	if !self.ImportKinds.Empty() {
		t.Errorf("self-edge = %+v, want no import kinds: no import produces a self-edge", self)
	}
	if len(graph) != 2 {
		t.Errorf("graph = %v, want the self-edge and the one dependency", graph)
	}
}

func TestNewGraphMergesAnUnnormalizedImportOfItselfIntoItsSelfEdge(t *testing.T) {
	// The shape that corrupts a graph rather than merely sitting in it: reduce before normalising and
	// "./a.go" is not yet "a.go", so the edge survives with its import kind, then collides with the
	// self-edge on (a.go, a.go) and merges a blank import into it.
	graph := NewGraph(
		SelfEdge("a.go"),
		Edge{Source: "a.go", Target: "./a.go", ImportKinds: NewImportKindSet(ImportKindBlank)},
	)

	// %#v, not %+v: Edge has a String method, and every shape of self-edge prints the same through it.
	if want := (Graph{SelfEdge("a.go")}); !slices.Equal(graph, want) {
		t.Errorf("graph = %#v, want exactly %#v", graph, want)
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

func TestGraphSelfEdges(t *testing.T) {
	// One per file of the fixture, and nothing for the two external targets: an import path is not a
	// node of the project, so no self-edge carries it.
	want := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/repo.go",
		"internal/util/noop.go",
	}

	selfEdges := fixtureGraph().SelfEdges()

	got := make([]string, 0, len(selfEdges))
	for _, edge := range selfEdges {
		if !edge.IsSelfEdge() {
			t.Errorf("SelfEdges() returned %v, which is not a self-edge", edge)
		}
		got = append(got, edge.Source)
	}
	if !slices.Equal(got, want) {
		t.Errorf("SelfEdges() = %v, want the self-edge of each of %v", got, want)
	}
}

func TestGraphDependenciesDropsSelfEdges(t *testing.T) {
	dependencies := fixtureGraph().Dependencies()

	for _, edge := range dependencies {
		if edge.IsSelfEdge() {
			t.Errorf("Dependencies() returned the self-edge %v", edge)
		}
	}
	// The file that depends on nothing is gone from the dependencies, which is exactly why the node
	// projection reads the self-edges instead.
	if _, found := dependencies.Find("internal/util/noop.go", "internal/util/noop.go"); found {
		t.Errorf("Dependencies() = %v, want nothing for the file that depends on nothing", dependencies)
	}
	if _, found := dependencies.Find("internal/api/handler.go", "internal/db/repo.go"); !found {
		t.Errorf("Dependencies() = %v, want the real dependencies kept", dependencies)
	}
}

func TestGraphSelfEdgesAndDependenciesPartitionTheGraph(t *testing.T) {
	graph := fixtureGraph()

	nodes := graph.SelfEdges()
	dependencies := graph.Dependencies()

	if len(nodes)+len(dependencies) != len(graph) {
		t.Errorf("%d self-edges + %d dependencies != %d edges", len(nodes), len(dependencies), len(graph))
	}
	// Order is inherited rather than re-established, so both halves are still reproducible.
	for name, half := range map[string]Graph{"SelfEdges": nodes, "Dependencies": dependencies} {
		if !slices.IsSortedFunc(half, compareEdges) {
			t.Errorf("%s() = %v, want the graph's order kept", name, half)
		}
	}
	if rejoined := nodes.Add(dependencies...); !slices.Equal(rejoined, graph) {
		t.Errorf("the two halves rejoined =\n%s\n\nwant\n%s", rejoined, graph)
	}
}

func TestGraphSelfEdgesAndDependenciesDoNotMutateTheReceiver(t *testing.T) {
	graph := fixtureGraph()
	before := slices.Clone(graph)

	_ = graph.SelfEdges()
	_ = graph.Dependencies()

	if !slices.Equal(graph, before) {
		t.Errorf("graph =\n%s\n\nwant it unchanged:\n%s", graph, before)
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
