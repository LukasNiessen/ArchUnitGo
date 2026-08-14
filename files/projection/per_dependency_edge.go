package projection

import (
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// PerDependencyEdge is the MapFunction of a relational rule about files — `project files, in folder
// "internal/api/**", should not, depend on files, in folder "internal/db/**"` — and it is the projection
// whose two ends are described separately: one mapped edge per dependency that starts at one of the from
// files and ends at one of the to files, labeled by the identifiers they already carry.
//
// The direction is the sentence's own. from is the subject the rule is about and to is the object it
// names, so the dependencies kept are the ones the subject *has*, never the ones pointing at it. Swapping
// the two arguments is the converse rule — `project files, in folder "internal/db/**", should not, depend
// on files, in folder "internal/api/**"` — which is a different sentence and often a true one where this
// is false.
//
// Both populations arrive as the identifiers SelectFiles resolved rather than as the matching.Filter
// values the user chained, for the reasons PerSelectedFileEdge gives: the AND the selectors are combined
// with is stated in SelectFiles and nowhere else, and a filter matches a string rather than a node, so an
// object written as `in path "**"` would otherwise keep an edge to `fmt` — an import path is not a file
// of this project and can never be an end of a dependency between two of them. Selected files are the
// project's own by construction, which is why the external edges are dropped as well, and a self-edge is
// dropped like everywhere in the `per <thing> edge` family: it names a node instead of carrying a
// dependency, and a file cannot break a rule by depending on itself.
//
// An empty population at either end projects nothing at all, which is the loud direction — a projection
// with no edges is what assertion.GatherEmptyTestViolations reports on, rather than a rule that quietly
// passes because the folder it forbids has been renamed.
func PerDependencyEdge(from, to []string) kernel.MapFunction {
	sources := membership(from)
	targets := membership(to)

	perInternalEdge := kernel.PerInternalEdge()
	return func(edge extraction.Edge) (kernel.MappedEdge, bool) {
		if _, source := sources[edge.Source]; !source {
			return kernel.MappedEdge{}, false
		}
		if _, target := targets[edge.Target]; !target {
			return kernel.MappedEdge{}, false
		}
		return perInternalEdge(edge)
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
