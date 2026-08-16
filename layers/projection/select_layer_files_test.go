package projection_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/layers/projection"
)

func TestSelectLayerFilesResolvesEachDeclaredLayerAgainstTheProject(t *testing.T) {
	// What a policy is talking about: the files of each layer, sorted, keyed by the name the clauses use.
	graph := fixtureGraph()
	layers := []projection.Layer{
		projection.NewLayer("api", folderMatcher(t, "internal/api/**")),
		projection.NewLayer("db", folderMatcher(t, "internal/db/**")),
	}

	membership := projection.SelectLayerFiles(graph, layers...)

	want := map[string][]string{
		"api": {"internal/api/handler.go", "internal/api/router.go"},
		"db":  {"internal/db/conn.go", "internal/db/query.go"},
	}
	if !maps.EqualFunc(membership, want, slices.Equal) {
		t.Errorf("the files of %v are %v, want %v", layerNames(layers), membership, want)
	}
}

func TestSelectLayerFilesLeavesAFileTheLayersDoNotDescribeOutOfEveryLayer(t *testing.T) {
	// A project is rarely layered end to end: main.go is in neither folder, and a policy about the two says
	// nothing about it rather than putting it somewhere.
	graph := fixtureGraph()
	api := projection.NewLayer("api", folderMatcher(t, "internal/api/**"))

	membership := projection.SelectLayerFiles(graph, api)

	if slices.Contains(membership["api"], "main.go") {
		t.Errorf(`main.go is in layer "api" (%v), want it in no layer at all`, membership["api"])
	}
	if len(membership) != 1 {
		t.Errorf("the result has %d layers in it (%v), want the one that was declared", len(membership), membership)
	}
}

func TestSelectLayerFilesKeepsALayerNoFileIsInAsAnEmptyEntry(t *testing.T) {
	// The stale glob the empty-test guard exists for. An absent key could not be told from a layer nobody
	// declared, so the layer is a key of the result whether or not anything matched it.
	graph := fixtureGraph()
	layers := []projection.Layer{
		projection.NewLayer("api", folderMatcher(t, "internal/api/**")),
		projection.NewLayer("renamed", folderMatcher(t, "internal/transport/**")),
	}

	membership := projection.SelectLayerFiles(graph, layers...)

	files, declared := membership["renamed"]
	if !declared {
		t.Errorf(`the empty layer "renamed" is missing from %v, want it present and empty`, membership)
	}
	if len(files) != 0 {
		t.Errorf(`the files of layer "renamed" are %v, want none: nothing is in that folder`, files)
	}
}

func TestSelectLayerFilesReadsTheProjectsFilesAndNotItsImports(t *testing.T) {
	// A file is a node with a self-edge, which is the extractor's promise, so an import path the project
	// depends on is never in a layer however well it matches. A layer is a set of *this* project's files.
	graph := fixtureGraph()
	external := projection.NewLayer("orm", pathMatcher(t, "gorm.io/**"))

	membership := projection.SelectLayerFiles(graph, external)

	if files := membership["orm"]; len(files) != 0 {
		t.Errorf(`the files of layer "orm" are %v, want none: gorm.io/gorm is somebody else's module`, files)
	}
}

func TestSelectLayerFilesPutsAFileTwoLayersDescribeInTheOneDeclaredFirst(t *testing.T) {
	// Membership is LayerOf's, so the overlap is resolved the same way here as it is for the projected edges:
	// a file in two layers' patterns is in the layer declared first, and in that one only.
	graph := fixtureGraph()
	layers := []projection.Layer{
		projection.NewLayer("api", folderMatcher(t, "internal/api/**")),
		projection.NewLayer("internals", folderMatcher(t, "internal/**")),
	}

	membership := projection.SelectLayerFiles(graph, layers...)

	if !slices.Contains(membership["api"], "internal/api/handler.go") {
		t.Errorf(`internal/api/handler.go is not in layer "api" (%v), want it there`, membership["api"])
	}
	if slices.Contains(membership["internals"], "internal/api/handler.go") {
		t.Errorf(`internal/api/handler.go is also in layer "internals" (%v), want a file in one layer only`, membership["internals"])
	}
}

func TestSelectLayerFilesOnAnEmptyGraphOrWithNoLayerSelectsNothing(t *testing.T) {
	// Both loud directions. A project with no file in it and a policy with no layer in it are the two ways a
	// policy ends up judging nothing, and both are an empty answer here rather than a guess.
	api := projection.NewLayer("api", folderMatcher(t, "internal/api/**"))

	if membership := projection.SelectLayerFiles(extraction.NewGraph(), api); len(membership["api"]) != 0 {
		t.Errorf(`layer "api" has files %v in an empty project, want none`, membership["api"])
	}
	if membership := projection.SelectLayerFiles(fixtureGraph()); len(membership) != 0 {
		t.Errorf("a policy with no layer declared selected %v, want nothing", membership)
	}
}

func TestSelectLayerFilesSortsEachLayersFiles(t *testing.T) {
	// Reproducible for a hand-written graph literal too, and not only for one extraction.NewGraph ordered:
	// a report lists a layer's files, and a list that moved between two runs would be read as a change.
	graph := extraction.Graph{
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/api/handler.go"),
	}
	api := projection.NewLayer("api", folderMatcher(t, "internal/api/**"))

	files := projection.SelectLayerFiles(graph, api)["api"]

	if !slices.Equal(files, sortedStrings(files)) {
		t.Errorf(`the files of layer "api" are %v, want them sorted`, files)
	}
}
