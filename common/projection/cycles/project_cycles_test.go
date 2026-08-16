package cycles

import (
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// projectedEdges builds a hand-written projection out of `source -> target` pairs, the way a rule's
// MapFunction would have produced it: labels only, since a cycle is a question about labels.
func projectedEdges(pairs ...string) []projection.ProjectedEdge {
	edges := make([]projection.ProjectedEdge, 0, len(pairs))
	for _, pair := range pairs {
		source, target, _ := strings.Cut(pair, " -> ")
		edges = append(edges, projection.NewProjectedEdge(source, target))
	}
	slices.SortFunc(edges, func(left, right projection.ProjectedEdge) int {
		if bySource := strings.Compare(left.SourceLabel(), right.SourceLabel()); bySource != 0 {
			return bySource
		}
		return strings.Compare(left.TargetLabel(), right.TargetLabel())
	})
	return edges
}

func labelsOf(edges []projection.ProjectedEdge) []string {
	labels := make([]string, 0, len(edges))
	for _, edge := range edges {
		labels = append(labels, edge.SourceLabel()+" -> "+edge.TargetLabel())
	}
	return labels
}

func cycleLabels(cycles [][]projection.ProjectedEdge) [][]string {
	rendered := make([][]string, 0, len(cycles))
	for _, cycle := range cycles {
		rendered = append(rendered, labelsOf(cycle))
	}
	return rendered
}

// sliceByFolder is the same folder projection the projection package's own tests use: a MapFunction
// relabelling a file identifier to the folder holding it, dropping what leaves the project.
func sliceByFolder() projection.MapFunction {
	folder := func(identifier string) string {
		if index := strings.LastIndex(identifier, "/"); index >= 0 {
			return identifier[:index]
		}
		return "."
	}
	return func(edge extraction.Edge) (projection.MappedEdge, bool) {
		if edge.External {
			return projection.MappedEdge{}, false
		}
		return projection.MappedEdge{SourceLabel: folder(edge.Source), TargetLabel: folder(edge.Target)}, true
	}
}

func TestProjectCyclesFindsNothingInAnAcyclicProjection(t *testing.T) {
	edges := projectedEdges("a -> b", "b -> c", "a -> c", "d -> c")

	if cycles := ProjectCycles(edges); len(cycles) != 0 {
		t.Errorf("ProjectCycles = %v, want no cycle", cycleLabels(cycles))
	}
}

func TestProjectCyclesOfNothingIsNothing(t *testing.T) {
	if cycles := ProjectCycles(nil); len(cycles) != 0 {
		t.Errorf("ProjectCycles(nil) = %v, want no cycle", cycleLabels(cycles))
	}
}

func TestProjectCyclesFindsATwoNodeCycle(t *testing.T) {
	edges := projectedEdges("a -> b", "b -> a")

	cycles := ProjectCycles(edges)

	want := [][]string{{"a -> b", "b -> a"}}
	if got := cycleLabels(cycles); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ProjectCycles = %v, want %v", got, want)
	}
}

func TestProjectCyclesReportsEveryEdgeInsideTheComponent(t *testing.T) {
	// a -> b -> c -> a with a shortcut from a to c: four edges, one component, and every one of the four
	// is on some cycle — which is why the component rather than one path through it is what is reported.
	edges := projectedEdges("a -> b", "b -> c", "c -> a", "a -> c")

	cycles := ProjectCycles(edges)

	want := [][]string{{"a -> b", "a -> c", "b -> c", "c -> a"}}
	if got := cycleLabels(cycles); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ProjectCycles = %v, want %v", got, want)
	}
}

func TestProjectCyclesLeavesOutTheEdgesThatOnlyTouchTheComponent(t *testing.T) {
	// x depends on the cycle and y is depended on by it: neither closes it, so neither edge is part of it.
	edges := projectedEdges("x -> a", "a -> b", "b -> a", "b -> y")

	cycles := ProjectCycles(edges)

	want := [][]string{{"a -> b", "b -> a"}}
	if got := cycleLabels(cycles); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ProjectCycles = %v, want %v", got, want)
	}
}

func TestProjectCyclesIgnoresASelfEdge(t *testing.T) {
	// A node depending on itself is not a dependency anywhere else in the PROJECT stage, so it is not a
	// cycle here either — and a self-edge on a label that really is in a cycle is not part of it.
	edges := projectedEdges("a -> a", "b -> c", "c -> b", "b -> b")

	cycles := ProjectCycles(edges)

	want := [][]string{{"b -> c", "c -> b"}}
	if got := cycleLabels(cycles); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ProjectCycles = %v, want %v", got, want)
	}
}

func TestProjectCyclesSeparatesTwoIndependentCyclesAndOrdersThem(t *testing.T) {
	// The search finds the two components in the reverse of the wanted order: visiting the labels in
	// sorted order reaches y through a, so {y, z} closes before {a, b}. What is under test is therefore
	// the output's ordering, and not the order the components happen to be discovered in.
	edges := projectedEdges("a -> b", "b -> a", "a -> y", "y -> z", "z -> y")

	cycles := ProjectCycles(edges)

	want := [][]string{{"a -> b", "b -> a"}, {"y -> z", "z -> y"}}
	if got := cycleLabels(cycles); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ProjectCycles = %v, want %v", got, want)
	}
}

func TestProjectCyclesNamesEveryLabelOfTheComponentAsASource(t *testing.T) {
	// The property an assertion reads a cycle's labels off: every label of a component has an outgoing
	// edge that stays inside it, so the source labels of a cycle's edges are the component itself.
	edges := projectedEdges("a -> b", "b -> c", "c -> a", "x -> a", "b -> y")

	cycles := ProjectCycles(edges)
	if len(cycles) != 1 {
		t.Fatalf("ProjectCycles = %v, want one cycle", cycleLabels(cycles))
	}

	sources := make([]string, 0, len(cycles[0]))
	for _, edge := range cycles[0] {
		if !slices.Contains(sources, edge.SourceLabel()) {
			sources = append(sources, edge.SourceLabel())
		}
	}
	slices.Sort(sources)
	if want := []string{"a", "b", "c"}; !slices.Equal(sources, want) {
		t.Errorf("the sources of %v = %v, want %v", labelsOf(cycles[0]), sources, want)
	}
}

func TestProjectCyclesIsAFunctionOfTheProjectionAlone(t *testing.T) {
	edges := projectedEdges("a -> b", "b -> c", "c -> a", "d -> e", "e -> d", "f -> a")
	first := cycleLabels(ProjectCycles(edges))

	for range 8 {
		again := cycleLabels(ProjectCycles(edges))
		if !slices.EqualFunc(first, again, slices.Equal) {
			t.Fatalf("ProjectCycles = %v, want the same answer every run: %v", again, first)
		}
	}
}

func TestProjectCyclesNamesTheFilesOfACyclicFolderPair(t *testing.T) {
	// The whole pipeline on a hand-built graph: two folders importing each other, projected into folders,
	// with the raw edges still naming the files a report has to point at.
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/db/repo.go"),
		extraction.SelfEdge("internal/util/noop.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/db/repo.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/repo.go", "internal/api/handler.go", false, extraction.ImportKindAliased),
		extraction.NewEdge("internal/db/repo.go", "database/sql", true, extraction.ImportKindPlain),
	)

	cycles := ProjectCycles(projection.ProjectEdges(graph, sliceByFolder()))

	want := [][]string{{"internal/api -> internal/db", "internal/db -> internal/api"}}
	if got := cycleLabels(cycles); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("ProjectCycles = %v, want %v", got, want)
	}
	files := make([]extraction.Edge, 0, 2)
	for _, edge := range cycles[0] {
		files = append(files, edge.CumulatedEdges()...)
	}
	wantFiles := []extraction.Edge{
		extraction.NewEdge("internal/api/handler.go", "internal/db/repo.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/repo.go", "internal/api/handler.go", false, extraction.ImportKindAliased),
	}
	if !slices.Equal(files, wantFiles) {
		t.Errorf("the cycle cumulates %v, want %v", files, wantFiles)
	}
}

func TestProjectCyclesFindsTheFileLevelCycleToo(t *testing.T) {
	// One implementation, every vocabulary: the same two files, projected as files rather than folders.
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/db/repo.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/db/repo.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/repo.go", "internal/api/handler.go", false, extraction.ImportKindAliased),
	)

	cycles := ProjectCycles(projection.ProjectEdges(graph, projection.PerInternalEdge()))

	want := [][]string{{
		"internal/api/handler.go -> internal/db/repo.go",
		"internal/db/repo.go -> internal/api/handler.go",
	}}
	if got := cycleLabels(cycles); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ProjectCycles = %v, want %v", got, want)
	}
}

func TestProjectCyclesFindsNoCycleAmongTheFoldersOfThisRepository(t *testing.T) {
	// The level above the hand-written fixtures, and the library dogfooding the rule it exists to offer:
	// this repository, extracted the way a check will do it, has no cyclic folder dependency.
	root, err := extraction.LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}
	graph, err := extraction.CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph(%q, nil) failed: %v", root, err)
	}

	projected := projection.ProjectEdges(graph, sliceByFolder())
	if len(projected) == 0 {
		t.Fatalf("the folder projection of %q is empty, so a green result would mean nothing", root)
	}

	if cycles := ProjectCycles(projected); len(cycles) != 0 {
		t.Errorf("the folders of this repository are cyclic: %v", cycleLabels(cycles))
	}
}
