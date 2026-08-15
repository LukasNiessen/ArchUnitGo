// Package projection is the graph module's PROJECT stage: it turns an extracted dependency graph into the
// Snapshot every dependency-graph report is rendered from.
//
// Rendering a graph is two steps, and this package is the first of them — build a snapshot: keep the nodes
// the query asks about, collapse them onto the labels it asks for, aggregate the dependencies between those
// labels, count what came out. Rendering is the second, and every renderer consumes this one Snapshot. That
// split is the whole point of the package: a new output format is one function over a Snapshot, and a new
// query option is one field of SnapshotOptions that every format understands the day it lands.
//
// Three concepts, one of them a function. Snapshot, with its Node, Edge and Summary parts, is what a report
// is; SnapshotOptions, with its Focus and CollapseGroup parts, is the query that describes one; and
// ProjectSnapshot is the one way to get the second to produce the first.
//
// The package is pure — hand it a graph and a query, get a snapshot, no filesystem and no clock anywhere —
// so every query option is tested against hand-built graphs, and a snapshot is reproducible: the same graph
// and the same query render the same diagram, byte for byte, which is what makes a checked-in artifact
// reviewable in a pull request.
package projection

import (
	"slices"
	"strconv"
	"strings"
)

// Snapshot is a dependency graph as one report shows it: the nodes that survived the query, the
// dependencies between them, the title the report is rendered under, and the counts that summarize both.
//
// It is the seam the graph module hangs from, the way fluentapi.Checkable is the seam of a rule. Rendering
// is two steps — build a snapshot, then render it — so a renderer is a function of a Snapshot and of
// nothing else, and a query option nobody has written a renderer for is still in every one of them.
//
// A Snapshot is immutable and it is ordered: nodes by label, dependencies by source label and then target
// label. Build one with NewSnapshot, or get one from ProjectSnapshot, and read it through the methods
// below.
//
// The zero Snapshot is the empty graph — no node, no dependency, no title, a summary of zeros — and it is
// what a terminal returns beside an error.
type Snapshot struct {
	// nodes are the snapshot's nodes, sorted by label and unique in it.
	nodes []Node
	// edges are the aggregated dependencies between those nodes, sorted, and unique in the pair of labels
	// they run between.
	edges []Edge
	// title is what the report is called, and the empty string means the renderer picks a headline.
	title string
	// summary is the counts, derived in NewSnapshot rather than passed in, so that it cannot disagree
	// with the nodes and edges it counts.
	summary Summary
}

// NewSnapshot is the snapshot holding these nodes and these dependencies, rendered under this title.
//
// It is the one constructor, and it puts the snapshot in order: the nodes are sorted by label, the
// dependencies by source label and then target label. Both slices are copied, so a caller may keep and
// reuse the ones it passed in.
//
// The summary is counted here rather than taken as an argument. A report that says `12 nodes` above a
// diagram of eleven is worse than no report, and the only way to be sure it never happens is for the
// counts to have one source.
//
// Duplicates are the caller's business: the projection produces one node per label and one edge per pair
// of labels, and a hand-built snapshot in a test is expected to do the same.
func NewSnapshot(title string, nodes []Node, edges []Edge) Snapshot {
	snapshot := Snapshot{
		nodes: slices.Clone(nodes),
		edges: slices.Clone(edges),
		title: title,
	}
	slices.SortFunc(snapshot.nodes, compareNodes)
	slices.SortFunc(snapshot.edges, compareEdges)
	snapshot.summary = newSummary(snapshot.nodes, snapshot.edges)
	return snapshot
}

// Nodes are the snapshot's nodes, sorted by label. The result is the caller's own copy: a snapshot is a
// value that has already been decided and reading it must not be able to change it.
func (s Snapshot) Nodes() []Node {
	return slices.Clone(s.nodes)
}

// Edges are the snapshot's dependencies, sorted by source label and then target label, each one already
// carrying how many of the project's own dependencies it stands for. The result is the caller's own copy.
func (s Snapshot) Edges() []Edge {
	return slices.Clone(s.edges)
}

// Title is what the report is called — `the modules of this project` — and the empty string when the query
// did not say. A renderer that needs a headline anyway supplies its own; this type does not invent one.
func (s Snapshot) Title() string {
	return s.title
}

// Summary is the snapshot in numbers: how many nodes and dependencies it holds, how many raw dependencies
// those stand for, and how much of both is outside the project.
func (s Snapshot) Summary() Summary {
	return s.summary
}

// Empty reports whether the snapshot has no node in it, which is the graph-report shape of the failure the
// empty-test guard exists for: a query whose patterns describe nothing renders a blank diagram, and a blank
// diagram looks exactly like a project that is clean.
//
// A snapshot with nodes and no dependency is not empty. A set of files that depend on nothing is a real
// answer, and often a good one.
func (s Snapshot) Empty() bool {
	return len(s.nodes) == 0
}

// String renders the snapshot as plain text, for logs and test failures: the title and the summary on the
// first line, the nodes on the second, then one line per dependency.
//
//	the modules of this project [2 nodes, 1 edge, 3 dependencies]
//	nodes: internal/api, internal/db
//	internal/api -> internal/db [3 dependencies] [plain]
//
// It is deliberately not one of the output formats the issue after this one adds — those are files a user
// keeps, and they get their own functions and their own tests. This is the same debugging courtesy every
// other type in the library extends to whoever is reading a failure.
func (s Snapshot) String() string {
	headline := s.title
	if headline == "" {
		headline = "dependency graph"
	}
	lines := make([]string, 0, len(s.edges)+2)
	lines = append(lines, headline+" ["+s.summary.String()+"]")
	if labels := s.labels(); len(labels) > 0 {
		lines = append(lines, "nodes: "+strings.Join(labels, ", "))
	}
	for _, edge := range s.edges {
		lines = append(lines, edge.String())
	}
	return strings.Join(lines, "\n")
}

// labels are the nodes as String prints them, in order, each one saying whether it is outside the project.
// Isolated nodes are the reason a report lists its nodes at all: a node in no dependency is in none of the
// lines below it, and for a report about placement it is the whole answer.
func (s Snapshot) labels() []string {
	labels := make([]string, 0, len(s.nodes))
	for _, node := range s.nodes {
		labels = append(labels, node.String())
	}
	return labels
}

// pluralize is `1 node` and `2 nodes`, the one place this package decides how a count reads. Counts are all
// over a report — the summary, an aggregated edge, the hops of a focus — and English plurals are not worth
// getting wrong in five places.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(count) + " " + plural
}
