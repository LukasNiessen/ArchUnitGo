package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// SelectSliceFiles resolves the slices of a project through the mapper that names them: the identifiers
// of the files that belong to each slice, sorted, keyed by the slice's name.
//
// It takes the mapper rather than a pattern, and that is the difference from layers.SelectLayerFiles. A
// layer is declared, so the layers of a policy are known before any file is read and an empty one is a
// key with no values. A slice is not declared anywhere — the names are cut out of the identifiers — so
// the slices of a project are exactly the ones some file is in, and a slicing pattern that matches
// nothing resolves to no keys at all. Both of those are what the empty-test guard reports on; the
// difference is only where the emptiness shows up.
//
// A file is a node of the graph with a self-edge, so that is what this reads, through the mapper: a file
// that depends on nothing is still in its slice, and an import path the project depends on is not a file
// of it and is never in a slice, however well it matches. Which nodes have a self-edge is the extractor's
// promise — extraction.ExtractGraph emits one per file of the project — and that a slicing mapper keeps
// them is this package's, stated where the mappers are.
//
// A nil mapper resolves nothing, the way common/projection.ProjectEdges projects nothing through one.
// Resolving nothing is a perfectly ordinary answer and never an error: whether an empty slicing is a
// failure is the question the empty-test guard exists to answer, and only a rule that judges something
// gets to ask it.
func SelectSliceFiles(graph extraction.Graph, mapper kernel.MapFunction) map[string][]string {
	membership := make(map[string][]string)
	if mapper == nil {
		return membership
	}

	for _, edge := range graph.SelfEdges() {
		mapped, sliced := mapper(edge)
		if !sliced || mapped.SourceLabel == "" {
			continue
		}
		membership[mapped.SourceLabel] = append(membership[mapped.SourceLabel], edge.Source)
	}

	for _, files := range membership {
		// Sorted here rather than inherited from the graph, so that the result is reproducible even for
		// a hand-written graph literal that never went through extraction.NewGraph.
		slices.Sort(files)
	}
	return membership
}
