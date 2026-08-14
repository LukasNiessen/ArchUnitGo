// Package projection is the files module's half of the PROJECT stage: it reshapes an extracted graph
// into the files one rule is about, and drops every file, and every dependency, the rule's scope does
// not name.
//
// Two exported functions, one for each of those halves:
//
//   - SelectFiles answers which files a rule is talking about. It is where `project files, in folder
//     "internal/api/**", with name "*.go"` stops being a sentence and becomes a list of file
//     identifiers: the scope verbs the user chained arrive as compiled matching.Filter values and are
//     combined with AND, which is what makes chaining them narrow the selection.
//   - PerSelectedFileEdge is this module's MapFunction, for the dependencies *between* those files.
//     Projected through common/projection.ProjectEdges it keeps exactly the edges both of whose ends
//     that selection holds, which is what the scope means for a rule about dependencies rather than
//     about files.
//
// Both are pure — a graph, and either compiled filters or the identifiers they selected, in — so the
// meaning of a rule's scope can be tested against a hand-built graph before any project is extracted at
// all.
package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// SelectFiles returns the identifiers of the project's own files that every selector accepts, sorted.
//
// A file is a node of the graph with a self-edge, so that is what this reads: a file that depends on
// nothing is still selected, and an import path the project depends on is not a file and is never
// selected, however well it matches. Which nodes have a self-edge is the extractor's promise —
// extraction.ExtractGraph emits one per file of the project.
//
// The selectors are combined with AND, in whatever order they were chained: a file is in scope only
// when it satisfies all of them, so each verb narrows the selection and the order cannot matter. No
// selector at all is every file of the project, which is `project files` with nothing chained onto it.
//
// Selecting nothing is a perfectly ordinary answer here, and not an error: whether an empty selection
// is a failure is the question assertion.GatherEmptyTestViolations exists to answer, and only a rule
// that judges something gets to ask it.
func SelectFiles(graph extraction.Graph, selectors ...matching.Filter) []string {
	selected := make([]string, 0, len(graph))
	for _, edge := range graph.SelfEdges() {
		if matchesEvery(edge.Source, selectors) {
			selected = append(selected, edge.Source)
		}
	}
	// Sorted here rather than inherited from the graph, so that the result is reproducible even for a
	// hand-written graph literal that never went through extraction.NewGraph.
	slices.Sort(selected)
	return selected
}

// matchesEvery reports whether every selector accepts this identifier. It is the AND the scope verbs
// are combined with, and an identifier no selector has anything to say about is accepted.
func matchesEvery(identifier string, selectors []matching.Filter) bool {
	for _, selector := range selectors {
		if !selector.Matches(identifier) {
			return false
		}
	}
	return true
}
