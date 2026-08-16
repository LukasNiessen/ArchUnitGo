package projection_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/slices/projection"
)

func TestSelectSliceFilesResolvesTheSlicesOfTheProject(t *testing.T) {
	// What a report needs, and the only way to get it: the slice names were never declared, so the slices of a
	// project are the ones the mapper found in it.
	membership := projection.SelectSliceFiles(fixtureGraph(), mustSliceByPattern(t, "internal/(**)/**"))

	want := map[string][]string{
		"api": {"internal/api/handler.go", "internal/api/router.go"},
		"db":  {"internal/db/conn.go", "internal/db/query.go"},
	}
	if !maps.EqualFunc(membership, want, slices.Equal) {
		t.Errorf(`the slices of "internal/(**)/**" are %v, want %v`, membership, want)
	}
}

func TestSelectSliceFilesLeavesOutTheFilesInNoSlice(t *testing.T) {
	// main.go is under no folder of its own, so the pattern names nothing in it. A file in no slice is in no
	// entry of the result rather than in one keyed by "": a slice called "" is not a slice.
	membership := projection.SelectSliceFiles(fixtureGraph(), mustSliceByPattern(t, "internal/(**)/**"))

	for name, files := range membership {
		if slices.Contains(files, "main.go") {
			t.Errorf("main.go is in slice %q (%v), want it in no slice", name, files)
		}
	}
	if _, nameless := membership[""]; nameless {
		t.Errorf(`the result has an entry keyed "" holding %v, want no such key`, membership[""])
	}
}

func TestSelectSliceFilesResolvesAFileThatDependsOnNothing(t *testing.T) {
	// A file is a node of the graph with a self-edge, so that is what this reads: a file that depends on
	// nothing is still in its slice, which is what makes the membership of a slice complete rather than "the
	// files some dependency of the project happens to touch".
	graph := extraction.NewGraph(extraction.SelfEdge("internal/api/lonely.go"))

	membership := projection.SelectSliceFiles(graph, mustSliceByPattern(t, "internal/(**)/**"))

	want := []string{"internal/api/lonely.go"}
	if !slices.Equal(membership["api"], want) {
		t.Errorf(`slice "api" resolved to %v, want %v`, membership["api"], want)
	}
}

func TestSelectSliceFilesFindsNoSliceWhereThereIsNothingToFind(t *testing.T) {
	// The three loud directions, and all of them are an empty answer rather than an error: whether an empty
	// slicing is a failure is the empty-test guard's question, and only a rule that judges something asks it.
	//
	// This is where a slicing differs from a layer policy: a layer is declared, so an empty layer is a key with
	// no files, while a slicing pattern that matches nothing has no keys at all — nobody said what the slices
	// would have been called.
	tests := []struct {
		name  string
		graph extraction.Graph
		glob  string
	}{
		{"a pattern that matches nothing", fixtureGraph(), "vendor/(**)/**"},
		{"a project with no file in it", extraction.NewGraph(), "internal/(**)/**"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			membership := projection.SelectSliceFiles(test.graph, mustSliceByPattern(t, test.glob))

			if len(membership) != 0 {
				t.Errorf("%q resolved to %v, want no slice at all", test.glob, membership)
			}
		})
	}

	if membership := projection.SelectSliceFiles(fixtureGraph(), nil); len(membership) != 0 {
		t.Errorf("a nil mapper resolved to %v, want no slice at all", membership)
	}
}

func TestSelectSliceFilesSortsEachSlicesFiles(t *testing.T) {
	// Reproducible for a hand-written graph literal too, and not only for one extraction.NewGraph ordered: a
	// report lists a slice's files, and a list that moved between two runs would be read as a change.
	graph := extraction.Graph{
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/api/handler.go"),
	}

	files := projection.SelectSliceFiles(graph, mustSliceByPattern(t, "internal/(**)/**"))["api"]

	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if !slices.Equal(files, want) {
		t.Errorf(`the files of slice "api" are %v, want %v`, files, want)
	}
}

func TestSelectSliceFilesResolvesTheSlicesOfAnyMapper(t *testing.T) {
	// It takes the mapper rather than a pattern, so every slicing this package offers resolves the same way —
	// which is what lets a report about a filename slicing be built at all.
	membership := projection.SelectSliceFiles(suffixFixtureGraph(), projection.SliceByFileSuffix())

	want := []string{"internal/api/order_handler.go", "internal/shop/user_handler.go"}
	if !slices.Equal(membership["handler"], want) {
		t.Errorf(`slice "handler" resolved to %v, want %v`, membership["handler"], want)
	}
}
