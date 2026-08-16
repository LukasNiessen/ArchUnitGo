package projection

import "strings"

// Summary is a snapshot in numbers: what a report prints above a diagram, and the whole of what a caller who
// only wants the shape of a project needs.
//
// The fields are exported because every one of them is a number a caller legitimately wants on its own — a
// test asserting that this project has no external dependency reads `summary.ExternalEdges != 0`, and making
// it call five accessors instead would buy nothing. A Summary is derived, never assembled: NewSnapshot
// counts it from the nodes and edges it was given, so it always describes them.
//
// Nodes and Edges count the snapshot as it is drawn, after the query filtered and collapsed it.
// Dependencies counts the project's raw dependencies behind those edges, so it is the number that does not
// shrink when a collapse merges arrows — the pair `4 edges, 312 dependencies` is the coupling a diagram
// alone cannot show.
type Summary struct {
	// Nodes is how many nodes the snapshot holds, external ones included.
	Nodes int
	// Edges is how many dependencies are drawn, after aggregation.
	Edges int
	// Dependencies is how many of the project's raw dependencies those edges stand for, which is the sum of
	// their counts and never less than Edges.
	Dependencies int
	// ExternalNodes is how many of the nodes are outside the project. It is zero unless the query included
	// external dependencies.
	ExternalNodes int
	// ExternalEdges is how many of the drawn dependencies leave the project.
	ExternalEdges int
}

// String renders the counts as `3 nodes, 2 edges, 7 dependencies, 1 external node, 1 external edge`, which
// is the line a report prints above a diagram. The two external counts are omitted when they are zero,
// because a report about a project with no external dependency should not spend a third of its headline
// saying so.
func (s Summary) String() string {
	parts := make([]string, 0, 5)
	parts = append(parts,
		pluralize(s.Nodes, "node", "nodes"),
		pluralize(s.Edges, "edge", "edges"),
		pluralize(s.Dependencies, "dependency", "dependencies"),
	)
	if s.ExternalNodes > 0 {
		parts = append(parts, pluralize(s.ExternalNodes, "external node", "external nodes"))
	}
	if s.ExternalEdges > 0 {
		parts = append(parts, pluralize(s.ExternalEdges, "external edge", "external edges"))
	}
	return strings.Join(parts, ", ")
}

// newSummary counts a snapshot's nodes and edges. It is the one place the counts are produced, which is what
// keeps a headline from disagreeing with the diagram under it.
func newSummary(nodes []Node, edges []Edge) Summary {
	summary := Summary{Nodes: len(nodes), Edges: len(edges)}
	for _, node := range nodes {
		if node.IsExternal() {
			summary.ExternalNodes++
		}
	}
	for _, edge := range edges {
		summary.Dependencies += edge.Count()
		if edge.IsExternal() {
			summary.ExternalEdges++
		}
	}
	return summary
}
