// Package extraction turns a Go project into the dependency graph every rule is evaluated
// against. It is the SOURCE and EXTRACT stages of the pipeline, and the only part of the library
// that is Go-specific: everything downstream works on the Edge and Graph values defined here and
// never sees an import declaration, a file path or the toolchain.
//
// A Graph built with NewGraph carries four invariants that downstream code relies on: identifiers
// are normalised, parallel edges are merged with their import kinds unioned so that
// (Source, Target) is unique, an edge from a node to itself is the one self-edge shape SelfEdge
// builds, and edges are ordered so that reports are reproducible.
//
// The stages run in order, and each one is a function of the last: LocateProject finds the project
// root by walking up to a go.mod, ExtractSourceFiles enumerates the Go files under it, ExtractImports
// reads the imports of one of them, and ExtractGraph resolves those imports against what the Go
// toolchain says the project is made of. Everything before the graph deals in host paths; everything
// from the graph onwards deals in the normalised identifiers of identifier.go, and nothing mixes the
// two.
package extraction

import (
	"slices"
	"strings"
)

// Graph is the whole extracted dependency structure: a list of Edge, nothing more. It is a slice
// so that reading it is ordinary Go, and the invariants downstream code relies on are established
// by NewGraph:
//
//   - identifiers are normalised;
//   - parallel edges are merged, unioning their import kinds, so (Source, Target) is unique;
//   - an edge from a node to itself is the self-edge SelfEdge builds, carrying no import kinds and
//     not external, so SelfEdges and Dependencies partition a graph into the edges that carry a
//     node and the edges that carry a dependency;
//   - edges are ordered by source then target, so reports built from a graph are reproducible.
//
// Build one with NewGraph or extend one with Add. A hand-written Graph literal is fine in tests as
// long as it already satisfies the invariants.
//
// Which nodes a graph is guaranteed to hold a self-edge for is the extractor's promise, not this
// type's: ExtractGraph emits one per file of the project.
type Graph []Edge

// NewGraph canonicalises and merges edges into a graph. Edges without a source or without a target
// carry no identifier and are dropped.
func NewGraph(edges ...Edge) Graph {
	merged := make(map[edgeKey]Edge, len(edges))
	for _, edge := range edges {
		canonical := edge.canonical()
		if canonical.Source == "" || canonical.Target == "" {
			continue
		}
		key := canonical.key()
		if existing, found := merged[key]; found {
			merged[key] = existing.merge(canonical)
			continue
		}
		merged[key] = canonical
	}

	graph := make(Graph, 0, len(merged))
	for _, edge := range merged {
		graph = append(graph, edge)
	}
	// (Source, Target) is unique in merged, so sorting by it makes the order total — the random
	// map iteration above cannot leak into the result.
	slices.SortFunc(graph, compareEdges)
	return graph
}

// Add returns a new graph holding this graph's edges and the given ones, merged. The receiver is
// not modified: values in this library are immutable.
func (g Graph) Add(edges ...Edge) Graph {
	return NewGraph(append(slices.Clone(g), edges...)...)
}

// Nodes lists every identifier the graph mentions, as a source or as a target, deduplicated and
// sorted. A file with no dependencies is in here because of its self-edge.
func (g Graph) Nodes() []string {
	seen := make(map[string]struct{}, len(g)*2)
	nodes := make([]string, 0, len(g))
	for _, edge := range g {
		for _, node := range [2]string{edge.Source, edge.Target} {
			if _, found := seen[node]; found {
				continue
			}
			seen[node] = struct{}{}
			nodes = append(nodes, node)
		}
	}
	slices.Sort(nodes)
	return nodes
}

// SelfEdges returns the self-edge of every node that has one, keeping this graph's order. Together
// with Dependencies it partitions the graph: every edge carries either a node or a dependency, and
// IsSelfEdge is which.
//
// This is what a node projection is built from. A file with no dependencies has nothing else in the
// graph, so reading the nodes off the dependencies alone would silently leave it out of a report —
// which for a rule about naming or placement is the whole population.
func (g Graph) SelfEdges() Graph {
	return g.filter(Edge.IsSelfEdge)
}

// Dependencies returns every edge that is not a self-edge, keeping this graph's order. It is what a
// projection reshapes by default: a node depending on itself is not a dependency any rule can be
// broken by, so a self-edge is carried for its node and dropped for its edge.
//
// The result satisfies the invariants in this type's doc, except that it no longer holds a self-edge
// per node — SelfEdges is where those went.
func (g Graph) Dependencies() Graph {
	return g.filter(func(edge Edge) bool { return !edge.IsSelfEdge() })
}

// Find returns the single edge between two nodes, if the graph has one.
func (g Graph) Find(source, target string) (Edge, bool) {
	wanted := edgeKey{source: NormalizeIdentifier(source), target: NormalizeIdentifier(target)}
	for _, edge := range g {
		if edge.key() == wanted {
			return edge, true
		}
	}
	return Edge{}, false
}

// String renders the graph one edge per line, for logs and test failures.
func (g Graph) String() string {
	lines := make([]string, 0, len(g))
	for _, edge := range g {
		lines = append(lines, edge.String())
	}
	return strings.Join(lines, "\n")
}

// filter selects the edges a predicate keeps. The receiver's order is inherited rather than
// re-established, because a subsequence of a sorted graph is sorted and merging is already done: a
// subset of a graph whose (Source, Target) is unique has unique (Source, Target) too.
func (g Graph) filter(keep func(Edge) bool) Graph {
	filtered := make(Graph, 0, len(g))
	for _, edge := range g {
		if keep(edge) {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

func compareEdges(left, right Edge) int {
	if bySource := strings.Compare(left.Source, right.Source); bySource != 0 {
		return bySource
	}
	return strings.Compare(left.Target, right.Target)
}
