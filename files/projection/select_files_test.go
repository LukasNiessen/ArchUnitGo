package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

func TestSelectFilesWithNoSelectorIsEveryFileOfTheProject(t *testing.T) {
	// `project files` with nothing chained onto it: every file, including the one that depends on nothing
	// and excluding every import path the project depends on.
	selected := projection.SelectFiles(fixtureGraph())

	want := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"internal/db/query.go",
		"main.go",
	}
	if !slices.Equal(selected, want) {
		t.Errorf("SelectFiles() = %v, want %v", selected, want)
	}
}

func TestSelectFilesLooksAtTheWholeIdentifierTheSelectorAsksFor(t *testing.T) {
	graph := fixtureGraph()

	tests := []struct {
		name     string
		selector matching.Filter
		want     []string
	}{
		{
			name:     "with name",
			selector: filenameMatcher(t, "*.go"),
			want:     []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go", "internal/db/query.go", "main.go"},
		},
		{
			name:     "with name, one file",
			selector: filenameMatcher(t, "conn.go"),
			want:     []string{"internal/db/conn.go"},
		},
		{
			name:     "in folder, the folder itself",
			selector: folderMatcher(t, "internal/api"),
			want:     []string{"internal/api/handler.go", "internal/api/router.go"},
		},
		{
			name:     "in folder, the folder and everything below it",
			selector: folderMatcher(t, "internal/**"),
			want:     []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go", "internal/db/query.go"},
		},
		{
			name:     "in folder, the project root",
			selector: folderMatcher(t, "."),
			want:     []string{"main.go"},
		},
		{
			name:     "in path",
			selector: pathMatcher(t, "internal/*/conn.go"),
			want:     []string{"internal/db/conn.go"},
		},
		{
			name:     "in file",
			selector: exactFileMatcher(t, "internal/api/handler.go"),
			want:     []string{"internal/api/handler.go"},
		},
		{
			name:     "in file, a bare filename is not an identifier",
			selector: exactFileMatcher(t, "handler.go"),
			want:     []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected := projection.SelectFiles(graph, test.selector)

			if !slices.Equal(selected, test.want) {
				t.Errorf("SelectFiles(%s) = %v, want %v", test.selector, selected, test.want)
			}
		})
	}
}

func TestSelectFilesCombinesItsSelectorsWithAnd(t *testing.T) {
	graph := fixtureGraph()
	inInternal := folderMatcher(t, "internal/**")
	named := filenameMatcher(t, "*r.go")

	both := projection.SelectFiles(graph, inInternal, named)
	reversed := projection.SelectFiles(graph, named, inInternal)

	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if !slices.Equal(both, want) {
		t.Errorf("SelectFiles(in folder, with name) = %v, want %v", both, want)
	}
	// Combined with AND, so chaining narrows and the order of the verbs cannot matter.
	if !slices.Equal(reversed, both) {
		t.Errorf("SelectFiles(with name, in folder) = %v, want the same selection %v", reversed, both)
	}
}

func TestSelectFilesSelectsNothingWhenTheSelectorsDisagree(t *testing.T) {
	// Two `in file` verbs, which is the shape of the mistake: no file is two files. Zero matches is an
	// ordinary answer here — reporting it is the empty-test guard's job, at the terminal.
	selected := projection.SelectFiles(
		fixtureGraph(),
		exactFileMatcher(t, "main.go"),
		exactFileMatcher(t, "internal/db/conn.go"),
	)

	if len(selected) != 0 {
		t.Errorf("SelectFiles(two named files) = %v, want nothing selected", selected)
	}
}

func TestSelectFilesNeverSelectsAnImportPathTheProjectDependsOn(t *testing.T) {
	// The graph mentions `golang.org/x/tools/go/packages` and `fmt` as targets, and a pattern can match
	// either. Neither is a file of the project, so neither has a self-edge and neither is selectable.
	graph := fixtureGraph()

	for _, pattern := range []string{"**", "*", "fmt", "golang.org/**"} {
		selected := projection.SelectFiles(graph, pathMatcher(t, pattern))

		if slices.Contains(selected, "fmt") || slices.Contains(selected, "golang.org/x/tools/go/packages") {
			t.Errorf("SelectFiles(in path %q) = %v, want no import path among the files", pattern, selected)
		}
	}
}

func TestSelectFilesWithAZeroFilterSelectsNothing(t *testing.T) {
	// A filter that was never built is a mistake, and matching.Filter answers nothing rather than
	// everything so that the mistake cannot pass for a rule about the whole project.
	selected := projection.SelectFiles(fixtureGraph(), matching.Filter{})

	if len(selected) != 0 {
		t.Errorf("SelectFiles(zero filter) = %v, want nothing selected", selected)
	}
}

func TestSelectFilesIsSortedEvenForAnUnsortedGraph(t *testing.T) {
	// A hand-written graph literal need not be ordered, and a report built from a selection has to be
	// reproducible either way.
	unsorted := extraction.Graph{
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.SelfEdge("internal/api/handler.go"),
	}

	selected := projection.SelectFiles(unsorted)

	want := []string{"internal/api/handler.go", "internal/db/conn.go", "main.go"}
	if !slices.Equal(selected, want) {
		t.Errorf("SelectFiles() = %v, want %v", selected, want)
	}
}

func TestSelectFilesOfAnEmptyGraphSelectsNothing(t *testing.T) {
	if selected := projection.SelectFiles(nil); len(selected) != 0 {
		t.Errorf("SelectFiles(nil) = %v, want nothing selected", selected)
	}
	if selected := projection.SelectFiles(extraction.NewGraph()); len(selected) != 0 {
		t.Errorf("SelectFiles(empty graph) = %v, want nothing selected", selected)
	}
}

// fixtureGraph is a small project in the shape the extractor produces one: a self-edge per file, the
// dependencies between the files, and two dependencies that leave the project. `internal/db/query.go`
// depends on nothing, which is the case a selection that read the dependencies instead of the
// self-edges would lose.
func fixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.SelfEdge("internal/db/query.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("main.go", "internal/api/router.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "fmt", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "golang.org/x/tools/go/packages", true, extraction.ImportKindPlain),
	)
}

func filenameMatcher(t *testing.T, glob string) matching.Filter {
	t.Helper()

	return mustMatcher(t, "filename", glob, matching.NewRegexFactory(nil).FilenameMatcher)
}

func folderMatcher(t *testing.T, glob string) matching.Filter {
	t.Helper()

	return mustMatcher(t, "folder", glob, matching.NewRegexFactory(nil).FolderMatcher)
}

func pathMatcher(t *testing.T, glob string) matching.Filter {
	t.Helper()

	return mustMatcher(t, "path", glob, matching.NewRegexFactory(nil).PathMatcher)
}

func exactFileMatcher(t *testing.T, identifier string) matching.Filter {
	t.Helper()

	return mustMatcher(t, "exact file", identifier, matching.NewRegexFactory(nil).ExactFileMatcher)
}

func mustMatcher(t *testing.T, kind, pattern string, build func(string) (matching.Filter, error)) matching.Filter {
	t.Helper()

	filter, err := build(pattern)
	if err != nil {
		t.Fatalf("%s matcher %q failed to compile: %v", kind, pattern, err)
	}
	return filter
}
