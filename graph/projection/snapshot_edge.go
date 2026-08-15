package projection

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// Edge is one dependency of a snapshot: the two nodes it runs between, how many of the project's own
// dependencies it stands for, whether it leaves the project, and the union of the import kinds behind it.
//
// The count is what a collapse does to a diagram made honest. Collapsing forty files onto two folders turns
// hundreds of dependencies into one arrow, and an arrow that does not say `312 dependencies` invites the
// reader to think the two folders are barely coupled. It is the aggregation count, so it counts the raw
// dependencies that were merged into this edge, not the nodes at either end.
//
// The import kinds are the union of the kinds of those raw dependencies, for the same reason
// extraction.Graph unions them when it merges parallel edges: an arrow that is one blank import and one real
// one is a different fact about a codebase than an arrow that is forty real ones, and unioning is the only
// merge that loses neither.
//
// An edge is external when any dependency behind it leaves the project. It cannot be `partly external`: the
// flag says whether following this arrow takes the reader out of the codebase, and once one of the merged
// dependencies does, it does.
//
// Build one with NewEdge. The zero Edge is a dependency between two unnamed nodes standing for nothing,
// which no projection produces.
type Edge struct {
	// source is the label of the depending node.
	source string
	// target is the label of the depended-on node.
	target string
	// count is how many raw dependencies were merged into this edge, and it is at least one for every edge
	// a projection builds.
	count int
	// external says at least one of those dependencies leaves the project.
	external bool
	// importKinds is the union of the kinds of those dependencies.
	importKinds extraction.ImportKindSet
}

// NewEdge is the snapshot edge between these two labels, standing for these extracted dependencies.
//
// Everything but the labels is derived from the dependencies rather than passed alongside them — the count
// is how many there are, the edge is external when one of them is, the import kinds are their union — so an
// edge cannot claim a count that does not match what it was built from.
//
// The dependencies are read and not kept: they go through extraction.NewGraph first, so the same raw
// dependency handed in twice is counted once and the invariant that `(source, target)` is unique holds here
// too. A caller with no dependencies to hand over gets an edge standing for none, which is what a
// hand-built snapshot in a test that is only about labels wants.
func NewEdge(sourceLabel, targetLabel string, dependencies ...extraction.Edge) Edge {
	merged := extraction.NewGraph(dependencies...)
	edge := Edge{source: sourceLabel, target: targetLabel, count: len(merged)}
	for _, dependency := range merged {
		edge.external = edge.external || dependency.External
		edge.importKinds = edge.importKinds.Union(dependency.ImportKinds)
	}
	return edge
}

// SourceLabel is the label of the depending node. It is a label rather than an identifier — the vocabulary
// projection.ProjectedEdge uses — because after a collapse it names a folder or a group and not a file.
func (e Edge) SourceLabel() string {
	return e.source
}

// TargetLabel is the label of the depended-on node.
func (e Edge) TargetLabel() string {
	return e.target
}

// Count is how many of the project's raw dependencies this edge stands for: one for an uncollapsed
// file-to-file dependency, and as many as were merged after a collapse.
func (e Edge) Count() int {
	return e.count
}

// IsExternal reports whether following this dependency leaves the project.
func (e Edge) IsExternal() bool {
	return e.external
}

// ImportKinds is the union of the import kinds of the dependencies this edge stands for.
func (e Edge) ImportKinds() extraction.ImportKindSet {
	return e.importKinds
}

// IsSelfDependency reports whether the edge runs from a node to itself. That is never a raw dependency — a
// file does not depend on itself — but it is a real and useful one after a collapse, where it says the
// files inside a folder depend on each other. Which is why it is a query option rather than an invariant:
// see SnapshotOptions.IncludeSelfDependencies.
func (e Edge) IsSelfDependency() bool {
	return e.source == e.target
}

// String renders the dependency as `internal/api -> internal/db [3 dependencies] [plain, blank]`, for logs
// and test failures. The kinds are omitted when there are none, which is what an edge built with no
// dependencies behind it has.
func (e Edge) String() string {
	description := e.source + " -> " + e.target + " [" + pluralize(e.count, "dependency", "dependencies") + "]"
	if e.external {
		description += " (external)"
	}
	if !e.importKinds.Empty() {
		description += " " + e.importKinds.String()
	}
	return description
}

// compareEdges orders dependencies by source label and then target label, which is the order a snapshot
// keeps them in and the same order extraction.Graph keeps its edges in. The pair of labels is unique in a
// snapshot, so the order is total.
func compareEdges(left, right Edge) int {
	if bySource := strings.Compare(left.source, right.source); bySource != 0 {
		return bySource
	}
	return strings.Compare(left.target, right.target)
}
