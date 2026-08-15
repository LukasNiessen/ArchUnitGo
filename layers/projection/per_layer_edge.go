package projection

import (
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// PerLayerEdge is the MapFunction of a layer policy: one mapped edge per dependency between two files
// that are both in a declared layer, labeled by the names of their layers.
//
// Projected through common/projection.ProjectEdges it is the structure a whole policy is judged over —
// one projected edge per (depending layer, depended-on layer) pair, cumulating the concrete file
// dependencies that produced it, which is what lets a violation about layers still name the files a
// reader has to go and open.
//
// Two of the policy's three semantic rules are this projection rather than the assertion's:
//
//   - An edge whose two files are in the same layer maps to two equal labels, and ProjectEdges drops
//     those. That is "intra-layer dependencies are always allowed", and it is worth having here rather
//     than as a comparison in the judgement: a policy is about the dependencies *between* layers, so an
//     edge inside one is not a dependency the projection has.
//   - An edge with an end in no declared layer is dropped outright. That is "edges where either end
//     belongs to no declared layer are ignored", and it is what makes a policy about part of a project
//     possible at all: the files nobody has assigned a layer to are not silently everybody's business.
//
// The dependencies that leave the project are dropped too, through kernel.PerInternalEdge: a layer is a
// set of this project's own files, so an import of the standard library or of a third-party module can
// only be in no layer anyway, and going through the shared factory is what keeps that decision — and the
// self-edge drop of the whole `per <thing> edge` family — stated in one place.
//
// No declared layer at all projects nothing, which is the loud direction: a projection with no edges is
// what the empty-test guard reports on, rather than a policy that quietly passes because the folders it
// was written about have been renamed.
func PerLayerEdge(layers ...Layer) kernel.MapFunction {
	perInternalEdge := kernel.PerInternalEdge()
	return func(edge extraction.Edge) (kernel.MappedEdge, bool) {
		if _, kept := perInternalEdge(edge); !kept {
			return kernel.MappedEdge{}, false
		}
		source, layered := LayerOf(edge.Source, layers...)
		if !layered {
			return kernel.MappedEdge{}, false
		}
		target, layered := LayerOf(edge.Target, layers...)
		if !layered {
			return kernel.MappedEdge{}, false
		}
		return kernel.MappedEdge{SourceLabel: source, TargetLabel: target}, true
	}
}
