package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/slices/projection"
)

func TestSliceByFileSuffixGroupsTheFilesByWhatTheyAreAcrossTheFolders(t *testing.T) {
	// The reason this projection exists rather than being a pattern one could pass to SliceByPattern: the
	// slices it finds cut across the folders, so a rule over it asks whether the handlers of a project depend
	// on its stores wherever the two of them sit.
	projected := kernel.ProjectEdges(suffixFixtureGraph(), projection.SliceByFileSuffix())

	want := []string{"handler -> store", "test -> store"}
	if dependencies := edgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the dependencies between the filename slices are %v, want %v", dependencies, want)
	}
}

func TestSliceByFileSuffixCumulatesTheFilesOfEverySlice(t *testing.T) {
	// Two handlers in two folders are one slice, which is what makes the projection worth having: the
	// dependency `handler -> store` stands for both of them.
	projected := kernel.ProjectEdges(suffixFixtureGraph(), projection.SliceByFileSuffix())

	if len(projected) == 0 {
		t.Fatal("nothing was projected, want the dependencies between the filename slices")
	}
	found := fileDependencyStrings(projected[0].CumulatedEdges())
	want := []string{
		"internal/api/order_handler.go -> internal/db/memory_store.go",
		"internal/shop/user_handler.go -> internal/db/store.go",
	}
	if !slices.Equal(found, want) {
		t.Errorf("the dependency %s was built from %v, want %v", projected[0], found, want)
	}
}

func TestSliceByFileSuffixNamesOneFile(t *testing.T) {
	mapper := projection.SliceByFileSuffix()

	tests := []struct {
		name       string
		identifier string
		want       string
	}{
		{"the last word of the filename", "internal/api/order_handler.go", "handler"},
		{"a filename of one word is that word", "internal/api/handler.go", "handler"},
		{"the extension is not part of the name", "doc.go", "doc"},
		{"a test file is in the slice test", "internal/db/conn_test.go", "test"},
		{"a test file of a multi-word name is still in the slice test", "internal/db/memory_store_test.go", "test"},
		{"the last word, not the first", "internal/http/http_json_client.go", "client"},
		{"the folders are not part of the name", "internal/order_service/service.go", "service"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapped, sliced := mapper(extraction.SelfEdge(test.identifier))

			if !sliced {
				t.Fatalf("%s is in no slice, want %q", test.identifier, test.want)
			}
			if mapped.SourceLabel != test.want {
				t.Errorf("%s is in slice %q, want %q", test.identifier, mapped.SourceLabel, test.want)
			}
		})
	}
}

func TestSliceByFileSuffixDropsAFileWithNoWordToTake(t *testing.T) {
	// The same loud direction as every other mapper here: a file it cannot name is in no slice, rather than in
	// a slice called "".
	mapper := projection.SliceByFileSuffix()

	for _, identifier := range []string{"internal/api/.go", "internal/api/handler_.go"} {
		if mapped, sliced := mapper(extraction.SelfEdge(identifier)); sliced {
			t.Errorf("%s was put in slice %q, want it in no slice", identifier, mapped.SourceLabel)
		}
	}
}

func TestSliceByFileSuffixDropsWhatLeavesTheProject(t *testing.T) {
	// An import path has a last word too — `gorm.io/gorm` would be `gorm` — and it is still not a file of this
	// project, so it is in no slice.
	mapper := projection.SliceByFileSuffix()

	edge := extraction.NewEdge("internal/db/memory_store.go", "gorm.io/gorm", true, extraction.ImportKindPlain)
	if mapped, sliced := mapper(edge); sliced {
		t.Errorf("%s was kept as %v, want it dropped", edge, mapped)
	}
}

// suffixFixtureGraph is a project whose filenames say what its files are, in the Go convention this projection
// reads: two handlers in two folders, two stores in one, and a test file, so that both the grouping across
// folders and the slice a test file lands in are observable.
func suffixFixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("internal/api/order_handler.go"),
		extraction.SelfEdge("internal/shop/user_handler.go"),
		extraction.SelfEdge("internal/db/memory_store.go"),
		extraction.SelfEdge("internal/db/store.go"),
		extraction.SelfEdge("internal/db/store_test.go"),
		extraction.NewEdge("internal/api/order_handler.go", "internal/db/memory_store.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/shop/user_handler.go", "internal/db/store.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/shop/user_handler.go", "internal/api/order_handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/store_test.go", "internal/db/memory_store.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/memory_store.go", "gorm.io/gorm", true, extraction.ImportKindPlain),
	)
}
