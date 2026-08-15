package projection

import (
	"path"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// PerComponentEdge is the MapFunction of the metrics module's component view: one mapped edge per
// dependency between two of the selected files, labeled by the folders those files are in.
//
// Projected through common/projection.ProjectEdges it is where the coupling half of a component comes
// from — one projected edge per (depending folder, depended-on folder) pair — which is what
// SelectComponents turns into the afferent and efferent coupling every distance metric is a formula over.
// The counting half is read off the files themselves and needs no graph at all.
//
// A component is a folder, and the label is the folder identifier metrics/extraction already spells on
// every file it read — `internal/api`, and `.` for the project root — so a component and the files it is
// made of are named the same way by construction rather than by agreement.
//
// Three of the reading's decisions are this projection rather than the arithmetic's:
//
//   - Both ends have to be among the selected files, exactly as files/projection.PerSelectedFileEdge
//     requires: a dependency that leaves the scope is a dependency the selected files have, not a
//     dependency between the components the rule is about. Widening the scope is what makes the rest
//     visible, which is the same trade metrics/extraction.ExtractFileInfo makes about a class's methods.
//   - A dependency between two files of one folder maps to two equal labels, and ProjectEdges drops
//     those. That is "a component is not coupled to itself": instability is about the dependencies that
//     cross a package boundary, and the imports inside one package are what make it a package.
//   - The dependencies that leave the project are dropped, through kernel.PerInternalEdge, because an
//     import path of the standard library or of another module is not a folder of this project and could
//     never be a component of it. Going through the shared factory is what keeps that decision — and the
//     self-edge drop of the whole `per <thing> edge` family — stated in one place.
//
// An empty selection projects nothing, which is the loud direction: a component population of no
// components is what the empty-test guard reports on, rather than a rule that quietly passes because the
// folder it was written about has been renamed.
func PerComponentEdge(selected []string) kernel.MapFunction {
	files := membership(selected)

	perInternalEdge := kernel.PerInternalEdge()
	return func(edge extraction.Edge) (kernel.MappedEdge, bool) {
		if _, source := files[edge.Source]; !source {
			return kernel.MappedEdge{}, false
		}
		if _, target := files[edge.Target]; !target {
			return kernel.MappedEdge{}, false
		}
		if _, kept := perInternalEdge(edge); !kept {
			return kernel.MappedEdge{}, false
		}
		return kernel.MappedEdge{
			SourceLabel: path.Dir(edge.Source),
			TargetLabel: path.Dir(edge.Target),
		}, true
	}
}

// membership is a population of files as the question a mapper asks about one identifier at a time. It is
// built once, when the mapper is, because a MapFunction is called once per edge of the graph and a linear
// scan of the selection per edge would make a rule quadratic in the size of the project.
func membership(files []string) map[string]struct{} {
	members := make(map[string]struct{}, len(files))
	for _, file := range files {
		members[file] = struct{}{}
	}
	return members
}
