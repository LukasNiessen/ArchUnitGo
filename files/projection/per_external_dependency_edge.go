package projection

import (
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// PerExternalDependencyEdge is the MapFunction of a third-party dependency rule about files — `project
// files, in folder "internal/domain/**", should not, depend on external modules, matching "*.*/**"` — and it
// is PerDependencyEdge across the project's boundary: one mapped edge per dependency that starts at one of
// the from files and leaves the project for one of the named modules, labeled by the identifiers they
// already carry.
//
// The direction is not a choice here, unlike in PerDependencyEdge. An external module is not code of this
// project, so nothing in the graph points from it back into the project and the converse rule cannot be
// written at all: from is always the subject the rule is about, and modules is always the set of import
// paths it may or may not reach.
//
// Both populations arrive as identifiers rather than as the matching.Filter values the user chained — the
// files as SelectFiles resolved them, the modules as SelectExternalModules did — for the reason
// PerDependencyEdge gives: each half's own combinator, the scope's AND and the object's OR, is stated in
// the function that resolved it and nowhere else.
//
// Only the edges the extractor marked external are kept, through kernel.PerExternalEdge, so a caller who
// passes one of the project's own files among the modules gets nothing for it: which code is this project's
// own was decided once, in extraction, and a projection that re-decided it would be a second answer to the
// question a `depend on files` rule and a `depend on external modules` rule have to agree about. The
// self-edge is dropped with it, as everywhere in the `per <thing> edge` family.
//
// An empty population at either end projects nothing at all. For the subject that is the loud direction
// PerDependencyEdge describes; for the modules it is the ordinary one, because a project that depends on
// none of the modules a rule names is exactly what a rule forbidding them asks for.
func PerExternalDependencyEdge(from, modules []string) kernel.MapFunction {
	sources := membership(from)
	targets := membership(modules)

	perExternalEdge := kernel.PerExternalEdge()
	return func(edge extraction.Edge) (kernel.MappedEdge, bool) {
		if _, source := sources[edge.Source]; !source {
			return kernel.MappedEdge{}, false
		}
		if _, target := targets[edge.Target]; !target {
			return kernel.MappedEdge{}, false
		}
		return perExternalEdge(edge)
	}
}
