package projection

import (
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// fixtureGraph is a small hand-built graph: two files in the api folder, two in db, one file that
// depends on nothing, an api file depending on both db files, and one external dependency.
func fixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/repo.go"),
		extraction.SelfEdge("internal/db/query.go"),
		extraction.SelfEdge("internal/util/noop.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/db/repo.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/query.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/router.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/repo.go", "database/sql", true, extraction.ImportKindPlain),
	)
}

// sliceByFolder is a MapFunction in the shape a module's own `slice by <thing>` factory has: it
// relabels a file identifier to the folder holding it, and drops what leaves the project. It is why a
// projection has to merge — the two api-to-db edges of the fixture come out as one — and why the drop
// of an edge whose labels are equal is by label rather than by raw self-edge.
func sliceByFolder() MapFunction {
	folder := func(identifier string) string {
		if index := strings.LastIndex(identifier, "/"); index >= 0 {
			return identifier[:index]
		}
		return "."
	}
	return func(edge extraction.Edge) (MappedEdge, bool) {
		if edge.External {
			return MappedEdge{}, false
		}
		return MappedEdge{SourceLabel: folder(edge.Source), TargetLabel: folder(edge.Target)}, true
	}
}

func labelsOf(edges []ProjectedEdge) []string {
	labels := make([]string, 0, len(edges))
	for _, edge := range edges {
		labels = append(labels, edge.SourceLabel()+" -> "+edge.TargetLabel())
	}
	return labels
}

func findProjectedEdge(t *testing.T, edges []ProjectedEdge, source, target string) ProjectedEdge {
	t.Helper()
	for _, edge := range edges {
		if edge.SourceLabel() == source && edge.TargetLabel() == target {
			return edge
		}
	}
	t.Fatalf("no projected edge %s -> %s in %v", source, target, labelsOf(edges))
	return ProjectedEdge{}
}

func TestProjectEdgesRelabelsAndMergesParallelProjectedEdges(t *testing.T) {
	projected := ProjectEdges(fixtureGraph(), sliceByFolder())

	// Two raw edges — handler.go to repo.go and handler.go to query.go — are one dependency between two
	// folders, cumulating both.
	merged := findProjectedEdge(t, projected, "internal/api", "internal/db")
	want := extraction.NewGraph(
		extraction.NewEdge("internal/api/handler.go", "internal/db/repo.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/query.go", false, extraction.ImportKindPlain),
	)
	if got := merged.CumulatedEdges(); !slices.Equal(got, want) {
		t.Errorf("CumulatedEdges() = %v, want %v", got, want)
	}
	if got := labelsOf(projected); !slices.Equal(got, []string{"internal/api -> internal/db"}) {
		t.Errorf("projected = %v, want the one folder dependency", got)
	}
}

func TestProjectEdgesDropsAnEdgeWhoseLabelsAreEqual(t *testing.T) {
	// router.go -> handler.go is a real dependency between two files and no dependency at all between
	// folders: after relabelling both ends are `internal/api`.
	projected := ProjectEdges(fixtureGraph(), sliceByFolder())

	for _, edge := range projected {
		if edge.IsSelfEdge() {
			t.Errorf("projected edge %s is a self-edge; ProjectEdges drops those", edge)
		}
	}
}

func TestProjectEdgesDropsTheGraphsSelfEdges(t *testing.T) {
	projected := ProjectEdges(fixtureGraph(), PerEdge())

	want := []string{
		"internal/api/handler.go -> internal/db/query.go",
		"internal/api/handler.go -> internal/db/repo.go",
		"internal/api/router.go -> internal/api/handler.go",
		"internal/db/repo.go -> database/sql",
	}
	if got := labelsOf(projected); !slices.Equal(got, want) {
		t.Errorf("projected = %v, want %v", got, want)
	}
}

func TestProjectEdgesDropsWhatTheMapFunctionDrops(t *testing.T) {
	projected := ProjectEdges(fixtureGraph(), PerInternalEdge())

	for _, edge := range projected {
		if edge.TargetLabel() == "database/sql" {
			t.Errorf("projected = %v, want no external dependency", labelsOf(projected))
		}
		for _, raw := range edge.CumulatedEdges() {
			if raw.External {
				t.Errorf("projected edge %s cumulates the external edge %s", edge, raw)
			}
		}
	}
	if len(projected) != 3 {
		t.Errorf("projected = %v, want the three internal dependencies", labelsOf(projected))
	}
}

func TestProjectEdgesIsOrderedByLabelAndUniquePerLabelPair(t *testing.T) {
	projected := ProjectEdges(fixtureGraph(), PerEdge())

	labels := labelsOf(projected)
	if !slices.IsSorted(labels) {
		t.Errorf("projected = %v, want it ordered by source then target label", labels)
	}
	seen := make(map[string]int, len(labels))
	for _, label := range labels {
		seen[label]++
	}
	for label, count := range seen {
		if count != 1 {
			t.Errorf("%q appears %d times, want 1", label, count)
		}
	}
}

func TestProjectEdgesKeepsEveryRawEdgeOfTheGraphExactlyOnce(t *testing.T) {
	graph := fixtureGraph()
	projected := ProjectEdges(graph, sliceByFolder())

	cumulated := make(map[extraction.Edge]int, len(graph))
	for _, edge := range projected {
		for _, raw := range edge.CumulatedEdges() {
			cumulated[raw]++
		}
	}
	// Every raw edge is under at most one projected edge, and the ones the projection dropped — the
	// self-edges, the folder-internal dependency and the external one — under none.
	for raw, count := range cumulated {
		if count != 1 {
			t.Errorf("raw edge %s is cumulated %d times, want 1", raw, count)
		}
		if _, found := graph.Find(raw.Source, raw.Target); !found {
			t.Errorf("cumulated edge %s is not in the graph it was projected from", raw)
		}
	}
	if len(cumulated) != 2 {
		t.Errorf("projection cumulates %d raw edges, want the two api-to-db imports", len(cumulated))
	}
}

func TestProjectEdgesDropsAnEdgeWithoutALabel(t *testing.T) {
	// A label is missing at either end, because either end is enough to make the edge unreportable:
	// handler.go's edges come out without a source label and router.go's without a target label.
	nameless := func(edge extraction.Edge) (MappedEdge, bool) {
		switch edge.Source {
		case "internal/api/handler.go":
			return MappedEdge{SourceLabel: "", TargetLabel: "internal/db"}, true
		case "internal/api/router.go":
			return MappedEdge{SourceLabel: "internal/api", TargetLabel: ""}, true
		}
		return MappedEdge{SourceLabel: edge.Source, TargetLabel: edge.Target}, true
	}

	projected := ProjectEdges(fixtureGraph(), nameless)

	for _, edge := range projected {
		if edge.SourceLabel() == "" || edge.TargetLabel() == "" {
			t.Errorf("projected %v, want no edge with an empty label", edge)
		}
	}
	// Everything the mapper gave two labels: the three remaining self-edges are dropped as
	// self-edges, so the one dependency left is the external one. Naming it is what stops a dropped
	// pair from reappearing under a label of its own.
	want := []string{"internal/db/repo.go -> database/sql"}
	if got := labelsOf(projected); !slices.Equal(got, want) {
		t.Errorf("projected = %v, want %v", got, want)
	}
}

func TestProjectEdgesWithoutAMapFunctionProjectsNothing(t *testing.T) {
	if projected := ProjectEdges(fixtureGraph(), nil); len(projected) != 0 {
		t.Errorf("ProjectEdges(graph, nil) = %v, want nothing: the empty-test guard reports it", labelsOf(projected))
	}
}

func TestProjectEdgesOfAnEmptyGraphIsEmpty(t *testing.T) {
	if projected := ProjectEdges(nil, PerEdge()); len(projected) != 0 {
		t.Errorf("ProjectEdges(nil, PerEdge()) = %v, want nothing", labelsOf(projected))
	}
}

func TestPerEdgeKeepsEverySelfEdgeAndExternalEdge(t *testing.T) {
	mapper := PerEdge()

	for _, edge := range fixtureGraph() {
		mapped, kept := mapper(edge)
		if !kept {
			t.Errorf("PerEdge() dropped %s, want every edge kept", edge)
			continue
		}
		if mapped.SourceLabel != edge.Source || mapped.TargetLabel != edge.Target {
			t.Errorf("PerEdge()(%s) = %+v, want the identifiers unchanged", edge, mapped)
		}
	}
}

func TestPerInternalEdgeDropsOnlyTheEdgesThatLeaveTheProject(t *testing.T) {
	mapper := PerInternalEdge()

	for _, edge := range fixtureGraph() {
		mapped, kept := mapper(edge)
		if kept == edge.External {
			t.Errorf("PerInternalEdge()(%s) kept = %t, want %t", edge, kept, !edge.External)
		}
		if kept && (mapped.SourceLabel != edge.Source || mapped.TargetLabel != edge.Target) {
			t.Errorf("PerInternalEdge()(%s) = %+v, want the identifiers unchanged", edge, mapped)
		}
	}
}

func TestNewProjectedEdgeCopiesTheRawEdgesItWasGiven(t *testing.T) {
	raw := []extraction.Edge{
		extraction.NewEdge("internal/api/handler.go", "internal/db/repo.go", false, extraction.ImportKindPlain),
	}

	edge := NewProjectedEdge("internal/api", "internal/db", raw...)
	raw[0] = extraction.NewEdge("nonsense.go", "elsewhere.go", false)

	cumulated := edge.CumulatedEdges()
	if len(cumulated) != 1 || cumulated[0].Source != "internal/api/handler.go" {
		t.Errorf("CumulatedEdges() = %v, want the edge the projection was built from", cumulated)
	}

	cumulated[0] = extraction.NewEdge("nonsense.go", "elsewhere.go", false)
	if again := edge.CumulatedEdges(); again[0].Source != "internal/api/handler.go" {
		t.Errorf("CumulatedEdges() = %v after a caller wrote through it, want the projection unchanged", again)
	}
}

func TestProjectedEdgeStringRendersBothLabelsAndTheRawCount(t *testing.T) {
	projected := ProjectEdges(fixtureGraph(), sliceByFolder())
	edge := findProjectedEdge(t, projected, "internal/api", "internal/db")

	if got, want := edge.String(), "internal/api -> internal/db [2 edges]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	self := NewProjectedEdge("internal/api", "internal/api")
	if got, want := self.String(), "internal/api -> itself [0 edges]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestProjectEdgesProjectsThisRepositoryByFolder(t *testing.T) {
	// The level above the hand-written fixture: this repository, extracted the way a check will do it
	// and projected into the vocabulary a slice rule speaks, with nothing hand-built about any step.
	root, err := extraction.LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}
	graph, err := extraction.CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph(%q, nil) failed: %v", root, err)
	}

	projected := ProjectEdges(graph, sliceByFolder())

	// This package depends on the extractor, and the raw edges are still there to name the files that
	// do it — which is the whole reason a projected edge carries them.
	dependency := findProjectedEdge(t, projected, "common/projection", "common/extraction")
	sources := make(map[string]struct{}, len(dependency.CumulatedEdges()))
	for _, raw := range dependency.CumulatedEdges() {
		sources[raw.Source] = struct{}{}
	}
	for _, wanted := range []string{"common/projection/project_edges.go", "common/projection/project_nodes.go"} {
		if _, found := sources[wanted]; !found {
			t.Errorf("%s -> common/extraction cumulates %v, want it to name %q", dependency, sources, wanted)
		}
	}

	// The folder projection of a real project holds no dependency of a folder on itself, because that is
	// what ProjectEdges drops — and every folder pair appears once.
	for _, edge := range projected {
		if edge.IsSelfEdge() {
			t.Errorf("projected edge %s is a self-edge", edge)
		}
	}
	if !slices.IsSorted(labelsOf(projected)) {
		t.Errorf("projected = %v, want it ordered by source then target label", labelsOf(projected))
	}
}
