package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// SelectExternalModules returns the import paths the project depends on that any selector accepts, each
// once and sorted.
//
// It is the object half of `project files, ..., should not, depend on external modules, matching
// "github.com/deprecated/**"`, and it is SelectFiles' counterpart on the other side of the project's
// boundary: SelectFiles answers which of the project's own files a rule is about, and this answers which of
// somebody else's modules it is about.
//
// What counts as external is extraction.Edge.External and nothing else, which the extractor settled once
// because it is the only layer that knows which code is this project's own. That means the standard library
// is among these — `fmt` and `net/http` are modules this project depends on and does not own — together with
// every third-party module, every vendored copy and every module nested inside the project. A rule that
// means third-party alone says so with a pattern: `*.*/**` matches an import path whose first segment holds
// a dot, which is every module path with a domain in it and no package of the standard library.
//
// The target is the import path exactly as the file wrote it, so it names a package and not the module it
// came from — `golang.org/x/tools/go/packages`, never `golang.org/x/tools`. A rule about a whole module is
// written with a trailing `/**`, which matches the path itself as well as everything under it.
//
// The selectors are combined with OR, which is the one place in this library that they are: an external
// dependency policy is a list of alternatives — `matching "github.com/lib/pq"` or `matching
// "github.com/deprecated/**"` — and a module cannot be two modules at once, so ANDing two of them would
// name the empty set and make chaining a second one meaningless. No selector at all is every external
// module the project depends on, which is `depend on external modules` with nothing chained onto it.
//
// Selecting nothing is an ordinary answer here, and it is a more ordinary one than an empty SelectFiles:
// this population *is* a set of dependencies, so "no module matches" and "no file depends on such a module"
// are one statement, and for a rule forbidding the dependency that statement is the pass. Which is why the
// terminal does not put this population through the empty-test guard.
func SelectExternalModules(graph extraction.Graph, selectors ...matching.Filter) []string {
	selected := make([]string, 0, len(graph))
	for _, edge := range graph {
		// An external edge is always a dependency: extraction.NewGraph canonicalises an edge from a node to
		// itself into a plain self-edge, so a node of the project can never be reached through this flag.
		if !edge.External {
			continue
		}
		if matchesAny(edge.Target, selectors) {
			selected = append(selected, edge.Target)
		}
	}
	// Sorted here rather than inherited from the graph, so that the result is reproducible even for a
	// hand-written graph literal that never went through extraction.NewGraph — and one entry per module
	// however many of the project's files import it, because this is a population and not a list of edges.
	slices.Sort(selected)
	return slices.Compact(selected)
}

// matchesAny reports whether at least one selector accepts this import path. It is the OR the object verbs
// of `depend on external modules` are combined with — stated here and nowhere else, as the AND of the scope
// verbs is stated in matchesEvery — and an import path no selector has anything to say about is accepted,
// which is what makes a rule with no object verb one about every external module.
func matchesAny(identifier string, selectors []matching.Filter) bool {
	if len(selectors) == 0 {
		return true
	}
	for _, selector := range selectors {
		if selector.Matches(identifier) {
			return true
		}
	}
	return false
}
