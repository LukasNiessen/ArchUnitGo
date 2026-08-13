// Package extraction turns a Go project into the dependency graph every rule is evaluated
// against. It is the SOURCE and EXTRACT stages of the pipeline, and the only part of the library
// that is Go-specific: everything downstream works on the Edge and Graph values defined here and
// never sees an import declaration, a file path or the toolchain.
//
// A Graph built with NewGraph carries three invariants that downstream code relies on:
// identifiers are normalised, parallel edges are merged with their import kinds unioned so that
// (Source, Target) is unique, and edges are ordered so that reports are reproducible.
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
//   - edges are ordered by source then target, so reports built from a graph are reproducible.
//
// Build one with NewGraph or extend one with Add. A hand-written Graph literal is fine in tests as
// long as it already satisfies the invariants.
type Graph []Edge

// NewGraph normalises and merges edges into a graph. Edges without a source or without a target
// carry no identifier and are dropped.
func NewGraph(edges ...Edge) Graph {
	merged := make(map[edgeKey]Edge, len(edges))
	for _, edge := range edges {
		normalized := Edge{
			Source:      NormalizeIdentifier(edge.Source),
			Target:      NormalizeIdentifier(edge.Target),
			External:    edge.External,
			ImportKinds: edge.ImportKinds,
		}
		if normalized.Source == "" || normalized.Target == "" {
			continue
		}
		key := normalized.key()
		if existing, found := merged[key]; found {
			merged[key] = existing.merge(normalized)
			continue
		}
		merged[key] = normalized
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

func compareEdges(left, right Edge) int {
	if bySource := strings.Compare(left.Source, right.Source); bySource != 0 {
		return bySource
	}
	return strings.Compare(left.Target, right.Target)
}
