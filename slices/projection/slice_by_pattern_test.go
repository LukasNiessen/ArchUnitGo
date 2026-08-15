package projection_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/slices/projection"
)

func TestSliceByPatternProjectsTheDependenciesBetweenTheSlices(t *testing.T) {
	// The structure a whole rule is judged over: one edge per pair of slices, labeled by the names the
	// pattern cut out of the identifiers rather than by the files themselves.
	mapper := mustSliceByPattern(t, "internal/(**)/**")

	projected := kernel.ProjectEdges(fixtureGraph(), mapper)

	want := []string{"api -> db", "db -> api"}
	if dependencies := edgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf(`the dependencies between the slices of "internal/(**)/**" are %v, want %v`, dependencies, want)
	}
}

func TestSliceByPatternCumulatesTheFileDependenciesEachPairOfSlicesStandsFor(t *testing.T) {
	// After relabelling, the files are nowhere else — so a violation about slices can still name the concrete
	// dependencies a reader has to go and unpick.
	mapper := mustSliceByPattern(t, "internal/(**)/**")

	projected := kernel.ProjectEdges(fixtureGraph(), mapper)

	if len(projected) == 0 {
		t.Fatalf(`nothing was projected through "internal/(**)/**", want the dependencies between api and db`)
	}
	found := fileDependencyStrings(projected[0].CumulatedEdges())
	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if !slices.Equal(found, want) {
		t.Errorf("the dependency %s was built from %v, want %v", projected[0], found, want)
	}
}

func TestSliceByPatternDropsADependencyInsideOneSlice(t *testing.T) {
	// "A slice may always depend on itself", and it falls out of the projection: both ends carry one label, so
	// the mapped edge is a self-edge and ProjectEdges drops it. A rule about slices is about the dependencies
	// *between* them, so an edge inside one is not a dependency it has.
	//
	// internal/api/handler.go -> internal/api/router.go is the edge inside `api`, and internal/db/conn.go ->
	// internal/db/query.go the one inside `db`. Neither may reach the assertion, and the raw edges each
	// projected dependency cumulates are where they would turn up if a projection stopped labeling one end by
	// its slice and smuggled them past under a mixed pair of labels.
	mapper := mustSliceByPattern(t, "internal/(**)/**")

	projected := kernel.ProjectEdges(fixtureGraph(), mapper)

	inside := []string{
		"internal/api/handler.go -> internal/api/router.go",
		"internal/db/conn.go -> internal/db/query.go",
	}
	for _, dependency := range projected {
		if dependency.IsSelfEdge() {
			t.Errorf("the projection kept %s, want the dependencies inside one slice dropped", dependency)
		}
		for _, raw := range fileDependencyStrings(dependency.CumulatedEdges()) {
			if slices.Contains(inside, raw) {
				t.Errorf("%s cumulates %q, which is a dependency inside one slice", dependency, raw)
			}
		}
	}
}

func TestASlicingMapperDropsWhatIsInNoSlice(t *testing.T) {
	// A file the pattern does not describe is in no slice, and neither is an import path that leaves the
	// project however well it matches — a slice is a set of this project's own files. An edge with such an end
	// is dropped, which is what makes a rule about part of a project possible.
	mapper := mustSliceByPattern(t, "internal/(**)/**")

	tests := []struct {
		name string
		edge extraction.Edge
	}{
		{
			name: "a source in no slice",
			edge: extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		},
		{
			name: "a target in no slice",
			edge: extraction.NewEdge("internal/api/handler.go", "main.go", false, extraction.ImportKindPlain),
		},
		{
			name: "a dependency that leaves the project",
			edge: extraction.NewEdge("internal/api/handler.go", "fmt", true, extraction.ImportKindPlain),
		},
		{
			name: "a dependency on a module whose path the pattern would name",
			edge: extraction.NewEdge("internal/api/handler.go", "internal/gorm/gorm.go", true, extraction.ImportKindPlain),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if mapped, sliced := mapper(test.edge); sliced {
				t.Errorf("%s was kept as %v, want it dropped", test.edge, mapped)
			}
		})
	}
}

func TestASlicingMapperKeepsASelfEdgeUnderTheNameOfItsSlice(t *testing.T) {
	// Identity's exception rather than the `per <thing> edge` family's rule, and the reason is that nobody
	// declared the slice names: reading which files are in a slice means reading the self-edges through the
	// very mapper that names them, which is what SelectSliceFiles does. It costs the dependency rule nothing,
	// because ProjectEdges drops an edge whose two labels are equal in any case.
	mapper := mustSliceByPattern(t, "internal/(**)/**")

	mapped, sliced := mapper(extraction.SelfEdge("internal/api/handler.go"))

	if !sliced {
		t.Fatal("the self-edge of internal/api/handler.go was dropped, want it kept as the node of its slice")
	}
	if mapped.SourceLabel != "api" || mapped.TargetLabel != "api" {
		t.Errorf("the self-edge came to %v, want both ends labeled %q", mapped, "api")
	}
}

func TestSliceByCaptureWithTheZeroPatternProjectsNothing(t *testing.T) {
	// The loud direction: a pattern that names nothing projects nothing, and an empty projection is what the
	// empty-test guard reports on — rather than a rule that quietly passes because it is about no slice.
	projected := kernel.ProjectEdges(fixtureGraph(), projection.SliceByCapture(matching.Pattern{}))

	if len(projected) != 0 {
		t.Errorf("the zero pattern projected %v, want nothing", edgeStrings(projected))
	}
}

func TestSliceByPatternWantsExactlyOneCapture(t *testing.T) {
	// A pattern that has to name a slice must say what the name is, exactly once: with no group there is no
	// name, and with two there is no saying which of them was meant.
	tests := []struct {
		name string
		glob string
	}{
		{"no capture", "internal/**"},
		{"two captures", "internal/(*)/(*)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper, err := projection.SliceByPattern(test.glob)

			if !errors.Is(err, matching.ErrOneCapture) {
				t.Errorf("SliceByPattern(%q) error = %v, want matching.ErrOneCapture", test.glob, err)
			}
			if mapper != nil {
				t.Errorf("SliceByPattern(%q) returned a mapper beside the error, want none", test.glob)
			}
		})
	}
}

func TestSliceByPatternReportsAnInvalidGlob(t *testing.T) {
	mapper, err := projection.SliceByPattern("internal/(**)/[unclosed")

	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("an unclosed character class error = %v, want matching.ErrInvalidPattern", err)
	}
	if mapper != nil {
		t.Error("a glob that does not compile returned a mapper beside the error, want none")
	}
}

// fixtureGraph is the project every test in this package is about: a main file, an api folder of two files, a
// db folder of two files, and the dependencies between them, together with two imports that leave the project.
//
// It is the same shape the files and layers modules' fixtures have, deliberately: a slicing is another skin
// over the same vocabulary, so a reader comparing them sees the projection and not the fixture. main.go is
// under no folder of its own, which makes it this package's answer to "a file the slicing pattern does not
// describe is in no slice".
func fixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.SelfEdge("internal/db/query.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/api/router.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "internal/db/query.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/query.go", "internal/api/router.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "fmt", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "gorm.io/gorm", true, extraction.ImportKindPlain),
	)
}

func mustSliceByPattern(t *testing.T, glob string) kernel.MapFunction {
	t.Helper()

	mapper, err := projection.SliceByPattern(glob)
	if err != nil {
		t.Fatalf("SliceByPattern(%q): %v", glob, err)
	}
	return mapper
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
