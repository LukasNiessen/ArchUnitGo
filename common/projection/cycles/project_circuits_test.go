package cycles

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// circuitStrings renders circuits the way a report would name them, which is what these tests compare:
// `api -> db -> api`, in the order ProjectCircuits returned them.
func circuitStrings(circuits []Circuit) []string {
	rendered := make([]string, 0, len(circuits))
	for _, circuit := range circuits {
		rendered = append(rendered, circuit.String())
	}
	return rendered
}

// allCircuits is ProjectCircuits with the limit taken off, for the fixtures small enough that the whole
// set is the thing under test.
func allCircuits(t *testing.T, edges []projection.ProjectedEdge) []Circuit {
	t.Helper()
	circuits, complete := ProjectCircuits(edges, &CircuitOptions{MaxCircuits: -1})
	if !complete {
		t.Fatalf("ProjectCircuits reported a truncated enumeration for %v, want the whole set", labelsOf(edges))
	}
	return circuits
}

func TestProjectCircuitsFindsNothingInAnAcyclicProjection(t *testing.T) {
	edges := projectedEdges("a -> b", "b -> c", "a -> c", "d -> c")

	if circuits := allCircuits(t, edges); len(circuits) != 0 {
		t.Errorf("ProjectCircuits = %v, want no cycle", circuitStrings(circuits))
	}
}

func TestProjectCircuitsOfNothingIsNothing(t *testing.T) {
	circuits, complete := ProjectCircuits(nil, nil)

	if len(circuits) != 0 {
		t.Errorf("ProjectCircuits(nil, nil) = %v, want no cycle", circuitStrings(circuits))
	}
	if !complete {
		t.Error("ProjectCircuits(nil, nil) reported a truncated enumeration, want the whole set")
	}
}

func TestProjectCircuitsNamesAMutualDependency(t *testing.T) {
	edges := projectedEdges("a -> b", "b -> a")

	circuits := allCircuits(t, edges)

	if want := []string{"a -> b -> a"}; !slices.Equal(circuitStrings(circuits), want) {
		t.Errorf("ProjectCircuits = %v, want %v", circuitStrings(circuits), want)
	}
}

func TestProjectCircuitsNamesEachCycleOfOneComponentSeparately(t *testing.T) {
	// The same projection as project_cycles_test.go's TestProjectCyclesReportsEveryEdgeInsideTheComponent,
	// which reports it as one component of four edges. The two cycles inside it are what a reader has to
	// break, and only this function names them.
	edges := projectedEdges("a -> b", "b -> c", "c -> a", "a -> c")

	circuits := allCircuits(t, edges)

	want := []string{"a -> c -> a", "a -> b -> c -> a"}
	if got := circuitStrings(circuits); !slices.Equal(got, want) {
		t.Errorf("ProjectCircuits = %v, want %v", got, want)
	}
}

func TestProjectCircuitsOrdersTheShortestCycleFirst(t *testing.T) {
	// Every cycle of three labels that all depend on each other: three mutual dependencies and two
	// triangles. Shortest first, because the smallest cycle is the smallest thing to fix, and then by
	// labels — each circuit is rooted at its own least label, so that orders them totally.
	edges := projectedEdges(
		"a -> b", "b -> a",
		"a -> c", "c -> a",
		"b -> c", "c -> b",
	)

	circuits := allCircuits(t, edges)

	want := []string{
		"a -> b -> a",
		"a -> c -> a",
		"b -> c -> b",
		"a -> b -> c -> a",
		"a -> c -> b -> a",
	}
	if got := circuitStrings(circuits); !slices.Equal(got, want) {
		t.Errorf("ProjectCircuits = %v, want %v", got, want)
	}
}

func TestProjectCircuitsSeparatesTwoIndependentCycles(t *testing.T) {
	edges := projectedEdges("a -> b", "b -> a", "a -> y", "y -> z", "z -> y")

	circuits := allCircuits(t, edges)

	want := []string{"a -> b -> a", "y -> z -> y"}
	if got := circuitStrings(circuits); !slices.Equal(got, want) {
		t.Errorf("ProjectCircuits = %v, want %v", got, want)
	}
}

func TestProjectCircuitsIgnoresASelfEdge(t *testing.T) {
	// The same convention as everywhere else in the PROJECT stage: a node depending on itself is not a
	// dependency, so it is not a one-label cycle either — and it is not part of a cycle its label is in.
	edges := projectedEdges("a -> a", "b -> c", "c -> b", "b -> b")

	circuits := allCircuits(t, edges)

	if want := []string{"b -> c -> b"}; !slices.Equal(circuitStrings(circuits), want) {
		t.Errorf("ProjectCircuits = %v, want %v", circuitStrings(circuits), want)
	}
}

func TestProjectCircuitsLeavesOutTheEdgesThatOnlyTouchTheCycle(t *testing.T) {
	edges := projectedEdges("x -> a", "a -> b", "b -> a", "b -> y")

	circuits := allCircuits(t, edges)

	if len(circuits) != 1 {
		t.Fatalf("ProjectCircuits = %v, want one cycle", circuitStrings(circuits))
	}
	want := []string{"a -> b", "b -> a"}
	if got := labelsOf(circuits[0].Edges()); !slices.Equal(got, want) {
		t.Errorf("the edges of %v = %v, want %v", circuits[0], got, want)
	}
}

func TestProjectCircuitsCarriesTheEdgesInCircuitOrder(t *testing.T) {
	// The property a report is built on: the edges are the cycle's own chain, each one starting where the
	// previous one ended, and the last one closing back onto the first one's source.
	edges := projectedEdges("a -> b", "b -> c", "c -> a")

	circuits := allCircuits(t, edges)
	if len(circuits) != 1 {
		t.Fatalf("ProjectCircuits = %v, want one cycle", circuitStrings(circuits))
	}

	chain := circuits[0].Edges()
	if got, want := labelsOf(chain), []string{"a -> b", "b -> c", "c -> a"}; !slices.Equal(got, want) {
		t.Fatalf("the edges of %v = %v, want %v", circuits[0], got, want)
	}
	for index, edge := range chain {
		next := chain[(index+1)%len(chain)]
		if edge.TargetLabel() != next.SourceLabel() {
			t.Errorf("%v is followed by %v, which does not continue it", edge, next)
		}
	}
	if got, want := circuits[0].Labels(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("the labels of %v = %v, want %v", circuits[0], got, want)
	}
	if got, want := circuits[0].Length(), 3; got != want {
		t.Errorf("the length of %v = %d, want %d", circuits[0], got, want)
	}
}

func TestCircuitHandsOutCopiesOfWhatItHolds(t *testing.T) {
	// A circuit that has been reported must not change afterwards, which is why both accessors clone —
	// the same reason projection.ProjectedEdge hides its cumulated edges behind one.
	circuits := allCircuits(t, projectedEdges("a -> b", "b -> a"))
	if len(circuits) != 1 {
		t.Fatalf("ProjectCircuits = %v, want one cycle", circuitStrings(circuits))
	}

	edges := circuits[0].Edges()
	edges[0] = projection.NewProjectedEdge("tampered", "tampered")
	labels := circuits[0].Labels()
	labels[0] = "tampered"

	if got, want := circuits[0].String(), "a -> b -> a"; got != want {
		t.Errorf("the circuit is %q after its accessors were written to, want %q", got, want)
	}
}

func TestCircuitZeroValueRendersAsNothing(t *testing.T) {
	var circuit Circuit

	if got := circuit.String(); got != "" {
		t.Errorf("the zero Circuit renders as %q, want the empty string", got)
	}
	if got := circuit.Length(); got != 0 {
		t.Errorf("the zero Circuit has length %d, want 0", got)
	}
	if got := circuit.Labels(); len(got) != 0 {
		t.Errorf("the zero Circuit has labels %v, want none", got)
	}
}

func TestProjectCircuitsStopsAtItsLimitAndSaysSo(t *testing.T) {
	edges := projectedEdges(
		"a -> b", "b -> a",
		"a -> c", "c -> a",
		"b -> c", "c -> b",
	)

	circuits, complete := ProjectCircuits(edges, &CircuitOptions{MaxCircuits: 2})

	if len(circuits) != 2 {
		t.Errorf("ProjectCircuits with a limit of 2 = %v, want two cycles", circuitStrings(circuits))
	}
	if complete {
		t.Error("ProjectCircuits with a limit of 2 reported the whole set, want a truncated one")
	}
}

func TestProjectCircuitsSpendsOneLimitAcrossEveryComponent(t *testing.T) {
	// Two independent cycles and room for one: the limit is the size of the report, not a per-component
	// allowance, and the second component is what the truncation is about.
	edges := projectedEdges("a -> b", "b -> a", "y -> z", "z -> y")

	circuits, complete := ProjectCircuits(edges, &CircuitOptions{MaxCircuits: 1})

	if want := []string{"a -> b -> a"}; !slices.Equal(circuitStrings(circuits), want) {
		t.Errorf("ProjectCircuits with a limit of 1 = %v, want %v", circuitStrings(circuits), want)
	}
	if complete {
		t.Error("ProjectCircuits with a limit of 1 reported the whole set, want a truncated one")
	}

	both, complete := ProjectCircuits(edges, &CircuitOptions{MaxCircuits: 2})
	if want := []string{"a -> b -> a", "y -> z -> y"}; !slices.Equal(circuitStrings(both), want) {
		t.Errorf("ProjectCircuits with a limit of 2 = %v, want %v", circuitStrings(both), want)
	}
	if !complete {
		t.Error("ProjectCircuits with a limit of 2 reported a truncated set, want the whole one")
	}
}

// completeProjection is the projection holding every edge between every pair of labels: the worst case for
// an enumeration, and the one whose circuit count is a closed form — sum over k of C(n,k)*(k-1)!.
func completeProjection(labels ...string) []projection.ProjectedEdge {
	pairs := make([]string, 0, len(labels)*(len(labels)-1))
	for _, source := range labels {
		for _, target := range labels {
			if source != target {
				pairs = append(pairs, source+" -> "+target)
			}
		}
	}
	return projectedEdges(pairs...)
}

func TestProjectCircuitsIsBoundedByDefault(t *testing.T) {
	// A default bag is the safe answer rather than the exhaustive one, because counting circuits is
	// exponential in the worst case. Seven mutually dependent labels hold 2365 elementary circuits, which
	// is more than the default limit — so this is the default itself being honored, on a projection the
	// bound can be seen to bite on, rather than a limit passed in by hand.
	edges := completeProjection("a", "b", "c", "d", "e", "f", "g")

	if circuits := allCircuits(t, edges); len(circuits) != 2365 {
		t.Fatalf("ProjectCircuits found %d circuits, want 2365", len(circuits))
	}

	circuits, complete := ProjectCircuits(edges, nil)
	if len(circuits) != 1000 || complete {
		t.Errorf("ProjectCircuits with a default bag = %d circuits, complete %t, want 1000 and false",
			len(circuits), complete)
	}
}

func TestCircuitOptionsWithDefaults(t *testing.T) {
	for name, testCase := range map[string]struct {
		options *CircuitOptions
		want    CircuitOptions
	}{
		// The default is written out rather than named, because comparing it against the constant
		// WithDefaults assigns would agree with any value that constant is given.
		"nil means the defaults":      {options: nil, want: CircuitOptions{MaxCircuits: 1000}},
		"the zero bag means them too": {options: &CircuitOptions{}, want: CircuitOptions{MaxCircuits: 1000}},
		"a limit is kept":             {options: &CircuitOptions{MaxCircuits: 7}, want: CircuitOptions{MaxCircuits: 7}},
		"unbounded is kept":           {options: &CircuitOptions{MaxCircuits: -1}, want: CircuitOptions{MaxCircuits: -1}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := testCase.options.WithDefaults(); *got != testCase.want {
				t.Errorf("WithDefaults() = %+v, want %+v", *got, testCase.want)
			}
		})
	}
}

func TestCircuitOptionsWithDefaultsLeavesTheCallersBagAlone(t *testing.T) {
	options := &CircuitOptions{}

	_ = options.WithDefaults()

	if options.MaxCircuits != 0 {
		t.Errorf("WithDefaults wrote %d back into the caller's bag, want it untouched", options.MaxCircuits)
	}
}

func TestProjectCircuitsIsAFunctionOfTheProjectionAlone(t *testing.T) {
	edges := projectedEdges("a -> b", "b -> c", "c -> a", "a -> c", "d -> e", "e -> d", "f -> a")
	first := circuitStrings(allCircuits(t, edges))

	for range 8 {
		again := circuitStrings(allCircuits(t, edges))
		if !slices.Equal(first, again) {
			t.Fatalf("ProjectCircuits = %v, want the same answer every run: %v", again, first)
		}
	}
}

func TestProjectCircuitsAndProjectCyclesAgreeOnWhichLabelsAreCyclic(t *testing.T) {
	// The two doors are one fact at two resolutions, so the labels they name have to be the same set:
	// every label of a cyclic component is on some circuit, and every circuit lies inside one component.
	edges := projectedEdges(
		"a -> b", "b -> c", "c -> a", "a -> c",
		"y -> z", "z -> y",
		"x -> a", "b -> q",
	)

	fromCircuits := make([]string, 0, len(edges))
	for _, circuit := range allCircuits(t, edges) {
		for _, label := range circuit.Labels() {
			if !slices.Contains(fromCircuits, label) {
				fromCircuits = append(fromCircuits, label)
			}
		}
	}
	fromComponents := make([]string, 0, len(edges))
	for _, cycle := range ProjectCycles(edges) {
		for _, edge := range cycle {
			if !slices.Contains(fromComponents, edge.SourceLabel()) {
				fromComponents = append(fromComponents, edge.SourceLabel())
			}
		}
	}
	slices.Sort(fromCircuits)
	slices.Sort(fromComponents)

	if !slices.Equal(fromCircuits, fromComponents) {
		t.Errorf("the circuits are on %v and the components on %v, want the same labels",
			fromCircuits, fromComponents)
	}
}

func TestProjectCircuitsNamesTheFilesOfACyclicFolderPair(t *testing.T) {
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

	circuits := allCircuits(t, projection.ProjectEdges(graph, sliceByFolder()))

	want := []string{"internal/api -> internal/db -> internal/api"}
	if got := circuitStrings(circuits); !slices.Equal(got, want) {
		t.Fatalf("ProjectCircuits = %v, want %v", got, want)
	}
	files := make([]extraction.Edge, 0, 2)
	for _, edge := range circuits[0].Edges() {
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

func TestProjectCircuitsFindsTheFileLevelCycleToo(t *testing.T) {
	// One implementation, every vocabulary: the same two files, projected as files rather than folders.
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/db/repo.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/db/repo.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/repo.go", "internal/api/handler.go", false, extraction.ImportKindAliased),
	)

	circuits := allCircuits(t, projection.ProjectEdges(graph, projection.PerInternalEdge()))

	want := []string{"internal/api/handler.go -> internal/db/repo.go -> internal/api/handler.go"}
	if got := circuitStrings(circuits); !slices.Equal(got, want) {
		t.Errorf("ProjectCircuits = %v, want %v", got, want)
	}
}

func TestProjectCircuitsFindsNoCycleAmongTheFoldersOfThisRepository(t *testing.T) {
	// The level above the hand-written fixtures, and the library dogfooding the rule this exists to
	// serve: this repository, extracted the way a check will do it, has no cyclic folder dependency —
	// and now at the resolution that would name one.
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

	circuits, complete := ProjectCircuits(projected, nil)
	if len(circuits) != 0 {
		t.Errorf("the folders of this repository are cyclic: %v", circuitStrings(circuits))
	}
	if !complete {
		t.Error("ProjectCircuits truncated an enumeration that found nothing")
	}
}

func TestProjectCircuitsFindsEveryFileLevelCycleOfThisRepository(t *testing.T) {
	// The same extraction at the file resolution, which is the vocabulary a `should have no cycles` rule
	// on files will use. Two files of one package importing each other is legal Go and would show up
	// here, so this asserts what is true today rather than that the enumeration found nothing.
	root, err := extraction.LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}
	graph, err := extraction.CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph(%q, nil) failed: %v", root, err)
	}

	projected := projection.ProjectEdges(graph, projection.PerInternalEdge())
	if len(projected) == 0 {
		t.Fatalf("the file projection of %q is empty, so a green result would mean nothing", root)
	}

	circuits, complete := ProjectCircuits(projected, nil)
	if !complete {
		t.Errorf("this repository has more than %d file-level cycles, which cannot be right",
			DefaultMaxCircuits)
	}
	if len(circuits) != 0 {
		t.Errorf("the files of this repository are cyclic: %v", circuitStrings(circuits))
	}
}
