package projection

import (
	"fmt"
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// ProjectedNode is one node of a projection: its label, the projected edges that point at it and the
// projected edges that leave it.
//
// It is the shape a rule about the nodes themselves is judged against — naming, placement, counting,
// fan-in and fan-out — as opposed to a rule about the dependencies between them, which reads
// ProjectedEdge directly. Both are views of one projection, so a node's edges are the very same
// values ProjectEdges returns, raw edges included.
//
// A ProjectedNode is immutable: get one from ProjectToNodes and read it through its methods.
type ProjectedNode struct {
	label    string
	incoming []ProjectedEdge
	outgoing []ProjectedEdge
}

// Label is what this node is called in the projection: a file identifier, a slice name, a layer name.
func (n ProjectedNode) Label() string {
	return n.label
}

// Incoming are the projected edges that depend on this node, ordered as the projection is. It is empty
// for a node nothing depends on, which for a file-level projection is most of them.
//
// The result is the caller's own copy: a node that has been reported must not change afterwards.
func (n ProjectedNode) Incoming() []ProjectedEdge {
	return slices.Clone(n.incoming)
}

// Outgoing are the projected edges this node depends through, ordered as the projection is. It is
// empty for a node that depends on nothing — which is precisely the node only a self-edge put in the
// projection, and the reason node projection reads self-edges at all.
//
// The result is the caller's own copy: a node that has been reported must not change afterwards.
func (n ProjectedNode) Outgoing() []ProjectedEdge {
	return slices.Clone(n.outgoing)
}

// String renders the node for logs and test failures, as `internal/api/handler.go [1 in, 2 out]`.
// User-facing violation messages are built in the testing layer, not here.
func (n ProjectedNode) String() string {
	return fmt.Sprintf("%s [%d in, %d out]", n.label, len(n.incoming), len(n.outgoing))
}

// ProjectToNodes reshapes a graph into the nodes of the vocabulary mapper speaks: one node per label
// the projection mentions, each carrying the projected edges that arrive at it and leave it.
//
// It is the other half of ProjectEdges, over the same MapFunction and the same merge, and the
// difference is the self-edges. ProjectEdges drops an edge whose two labels are equal because it is no
// dependency; here that edge is what names a node, so a file that imports nothing is still a node, and
// a slice whose files only depend on each other is still a slice. Such an edge appears in neither
// Incoming nor Outgoing, because a node depending on itself is not a dependency — every edge in this
// result is an edge ProjectEdges returned too.
//
// A node the projection only ever saw as a target is in the result as well, with no outgoing edges: for
// Identity that is every external module the project depends on, and a mapper that drops those leaves
// the projection with the project's own nodes only.
//
// Identity, not PerEdge, is the file-level mapper this function wants. Every `per <thing> edge` factory
// is about dependencies and so drops the self-edges, which here means dropping every file that depends
// on nothing — for a rule about naming or placement, most of the population.
//
// The result is ordered by label, so a report built from it is reproducible, and every label appears
// once. A nil mapper projects nothing, which the empty-test guard then reports rather than passing
// silently.
func ProjectToNodes(graph extraction.Graph, mapper MapFunction) []ProjectedNode {
	projected := projectAll(graph, mapper)

	nodes := make(map[string]*ProjectedNode, len(projected))
	labels := make([]string, 0, len(projected))
	node := func(label string) *ProjectedNode {
		if existing, found := nodes[label]; found {
			return existing
		}
		fresh := &ProjectedNode{label: label}
		nodes[label] = fresh
		labels = append(labels, label)
		return fresh
	}

	for _, edge := range projected {
		source := node(edge.sourceLabel)
		target := node(edge.targetLabel)
		if edge.IsSelfEdge() {
			continue
		}
		source.outgoing = append(source.outgoing, edge)
		target.incoming = append(target.incoming, edge)
	}

	slices.Sort(labels)
	result := make([]ProjectedNode, 0, len(labels))
	for _, label := range labels {
		result = append(result, *nodes[label])
	}
	return result
}
