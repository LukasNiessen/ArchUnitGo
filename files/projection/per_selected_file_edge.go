package projection

import (
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// PerSelectedFileEdge is the MapFunction of a rule about the dependencies *between* the files it
// selected: one mapped edge per dependency whose both ends are among those files, labeled by the file
// identifiers they already carry.
//
// It is the mapper `project files, in folder "internal/**", should, have no cycles` is projected
// through, and it is what SelectFiles is to the files themselves — the module's own half of the PROJECT
// stage, over the same graph. A rule about nodes rather than dependencies wants
// kernel.Identity instead, which is the only mapper that keeps a file's self-edge.
//
// Both ends have to be selected, and that is the whole of what the scope means for a rule about
// dependencies: a dependency that leaves the scope is a dependency the selected files have, not a
// dependency between them. The consequence is worth knowing — a cycle that leaves the scope and comes
// back through a file the rule did not select is not this rule's cycle, and widening the scope is what
// makes it visible.
//
// The selection arrives as the identifiers SelectFiles resolved, not as the matching.Filter values the
// user chained, for two reasons. The AND the scope verbs are combined with is stated in SelectFiles and
// nowhere else; and a filter matches a string rather than a node, so `in path "**"` would keep an edge
// to `fmt` — an import path is not a file of this project and can never be one end of a dependency
// between two of them. Selected files are the project's own by construction, which is why the external
// edges are dropped as well, and a self-edge is dropped like everywhere in the `per <thing> edge`
// family: it names a node instead of carrying a dependency.
//
// An empty selection projects nothing at all, which is the loud direction — a projection with no edges
// is what assertion.GatherEmptyTestViolations reports on, rather than a rule that quietly passes.
func PerSelectedFileEdge(selected []string) kernel.MapFunction {
	members := make(map[string]struct{}, len(selected))
	for _, file := range selected {
		members[file] = struct{}{}
	}

	perInternalEdge := kernel.PerInternalEdge()
	return func(edge extraction.Edge) (kernel.MappedEdge, bool) {
		if _, source := members[edge.Source]; !source {
			return kernel.MappedEdge{}, false
		}
		if _, target := members[edge.Target]; !target {
			return kernel.MappedEdge{}, false
		}
		return perInternalEdge(edge)
	}
}
