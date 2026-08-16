package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// SelectLayerFiles resolves every declared layer against the project: the identifiers of the files that
// belong to it, sorted, keyed by the layer's name.
//
// Every declared layer is a key of the result, including the ones no file is in — an empty entry is the
// whole point of the function, because a layer whose pattern matches nothing is the stale glob the
// empty-test guard exists for, and a missing key could not be told from a layer nobody declared.
//
// A file is a node of the graph with a self-edge, so that is what this reads: a file that depends on
// nothing is still in its layer, and an import path the project depends on is not a file of it and is
// never in a layer, however well it matches. Which nodes have a self-edge is the extractor's promise —
// extraction.ExtractGraph emits one per file of the project.
//
// Membership is LayerOf's, so a file that two layers' patterns describe is in the one declared first,
// exactly as it is for the projected edges. A layer declared twice under one name is one key holding the
// union of both declarations, because that is what fluentapi merged the declarations into.
//
// Selecting nothing is a perfectly ordinary answer here, and not an error: whether an empty layer is a
// failure is the question assertion.GatherEmptyTestViolations exists to answer, and only a rule that
// judges something gets to ask it.
func SelectLayerFiles(graph extraction.Graph, layers ...Layer) map[string][]string {
	membership := make(map[string][]string, len(layers))
	for _, layer := range layers {
		// Declared before anything is matched, so that a layer no file is in is an empty entry rather
		// than an absent one.
		membership[layer.Name()] = nil
	}

	for _, edge := range graph.SelfEdges() {
		if name, layered := LayerOf(edge.Source, layers...); layered {
			membership[name] = append(membership[name], edge.Source)
		}
	}

	for _, files := range membership {
		// Sorted here rather than inherited from the graph, so that the result is reproducible even for
		// a hand-written graph literal that never went through extraction.NewGraph.
		slices.Sort(files)
	}
	return membership
}
