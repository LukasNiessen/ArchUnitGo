// Package projection is the metrics module's half of the PROJECT stage: it says which of the project's
// files a metrics rule is measured over, and which of the classes those files declare.
//
// Two functions, one per side of the read that sits between them:
//
//   - SelectFiles answers which files a rule is talking about, out of the extracted graph. It is where
//     `metrics, in folder "internal/api/**"` stops being a sentence and becomes a list of file
//     identifiers for metrics/extraction to read.
//   - SelectSubjects answers what is measured, out of what was read: the files a metric about a file is
//     measured over, and the classes a metric about a class is measured over, from one set of selectors so
//     the two cannot disagree about which classes the rule is about.
//
// Both are pure, so the meaning of a metrics rule's scope can be tested against a hand-built graph and
// hand-built file information before any project is extracted at all.
package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// SelectFiles returns the identifiers of the project's own files that every selector accepts, sorted.
//
// A file is a node of the graph with a self-edge, so that is what this reads: a file that depends on
// nothing is still selected and still measured, and an import path the project depends on is not a file of
// it and is never selected, however well it matches. Which nodes have a self-edge is the extractor's
// promise — extraction.ExtractGraph emits one per file of the project.
//
// The selectors are combined with AND, in whatever order they were chained: a file is in scope only when
// it satisfies all of them, so each verb narrows the selection and the order cannot matter. No selector at
// all is every file of the project, which is `metrics` with nothing chained onto it.
//
// A selector about a classname is none of this function's business and has to be left out by the caller —
// this is the population of files to *read*, and which class a file declares is only known afterwards.
// SelectSubjects is where such a selector applies.
//
// Selecting nothing is a perfectly ordinary answer here, and not an error: whether an empty selection is a
// failure is the question assertion.GatherEmptyTestViolations exists to answer, and only a rule that
// judges something gets to ask it.
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

// matchesEvery reports whether every selector accepts this identifier. It is the AND the scope verbs are
// combined with, and an identifier no selector has anything to say about is accepted.
func matchesEvery(identifier string, selectors []matching.Filter) bool {
	for _, selector := range selectors {
		if !selector.Matches(identifier) {
			return false
		}
	}
	return true
}
