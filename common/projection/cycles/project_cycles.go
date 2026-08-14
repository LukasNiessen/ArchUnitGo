// Package cycles finds the cyclic dependencies in a projected structure. It is the last of the three
// reshapings the PROJECT stage offers — projected edges, projected nodes, projected cycles — and it is
// where a `should have no cycles` rule gets the thing it reports.
//
// It works on the projected edges of any vocabulary, so one implementation serves files, slices,
// layers and packages alike: the projection has already decided what a node is, and a cycle is a cycle
// whatever the nodes are called. Like the rest of projection it is pure, and it is deterministic — the
// same edges give the same cycles in the same order, because a report has to be reproducible.
package cycles

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// ProjectCycles groups projected edges into the cyclic components of the projection: one entry per
// group of labels that depend on each other in a circle, holding every projected edge between labels of
// that group.
//
// A group is a strongly connected component of two or more labels, and every edge returned inside one
// is part of some cycle: for an edge `a -> b` inside a component there is a path from b back to a, which
// closes it. That is why the component, rather than one path through it, is what a report names — a
// component of five labels can hold dozens of simple cycles, listing them is exponential, and fixing
// any one edge of the component is what breaks it. The labels of a cycle are the source labels of its
// edges: inside a component every label has an outgoing edge that stays in it, so none is missed.
//
// A projected self-edge is ignored, which is the same convention as everywhere else in the PROJECT
// stage: a node depending on itself is not a dependency, so it is not a cycle either. ProjectEdges has
// already dropped those; passing a projection that kept them changes nothing.
//
// The result is ordered by the first label of each component, and the edges inside each component keep
// the order they arrived in — which for the output of ProjectEdges is by source label then target
// label. No cycles is an empty result, and that is the pass a rule reports; the edges themselves are
// not copied, because a ProjectedEdge is immutable.
func ProjectCycles(edges []projection.ProjectedEdge) [][]projection.ProjectedEdge {
	labels, successors := adjacency(edges)

	components := tarjanSCC(labels, successors)
	cyclic := make([][]string, 0, len(components))
	for _, component := range components {
		if len(component) < 2 {
			continue
		}
		cyclic = append(cyclic, component)
	}
	// Tarjan reports components in reverse topological order, which is an order about the graph rather
	// than about the report. Each component's labels are sorted and the components are disjoint, so the
	// first label orders them totally.
	slices.SortFunc(cyclic, slices.Compare)

	projected := make([][]projection.ProjectedEdge, 0, len(cyclic))
	for _, component := range cyclic {
		projected = append(projected, edgesWithin(component, edges))
	}
	return projected
}

// adjacency reads a projection as a plain directed graph of labels: the labels it mentions, sorted, and
// each label's successors, sorted and deduplicated. Sorting both is what makes the search's answer a
// function of the edges alone.
//
// Self-edges are left out entirely, so a label whose only edge is its own is a label with no successors
// — present in the graph, and never in a cycle.
func adjacency(edges []projection.ProjectedEdge) ([]string, map[string][]string) {
	labels := make([]string, 0, len(edges))
	seen := make(map[string]struct{}, len(edges))
	remember := func(label string) {
		if _, found := seen[label]; found {
			return
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
	}

	successors := make(map[string][]string, len(edges))
	for _, edge := range edges {
		remember(edge.SourceLabel())
		remember(edge.TargetLabel())
		if edge.IsSelfEdge() {
			continue
		}
		// (SourceLabel, TargetLabel) is unique in a projection, but this function is handed a slice
		// rather than that promise, and a duplicated successor would only cost the search work.
		if !slices.Contains(successors[edge.SourceLabel()], edge.TargetLabel()) {
			successors[edge.SourceLabel()] = append(successors[edge.SourceLabel()], edge.TargetLabel())
		}
	}

	slices.Sort(labels)
	for _, targets := range successors {
		slices.Sort(targets)
	}
	return labels, successors
}

// edgesWithin selects the projected edges whose both ends are labels of one component. Those are the
// dependencies that hold the cycle together; an edge leaving the component or arriving from outside it
// is a dependency the component has, not a dependency that closes it.
func edgesWithin(component []string, edges []projection.ProjectedEdge) []projection.ProjectedEdge {
	members := make(map[string]struct{}, len(component))
	for _, label := range component {
		members[label] = struct{}{}
	}

	within := make([]projection.ProjectedEdge, 0, len(component))
	for _, edge := range edges {
		if edge.IsSelfEdge() {
			continue
		}
		if _, source := members[edge.SourceLabel()]; !source {
			continue
		}
		if _, target := members[edge.TargetLabel()]; !target {
			continue
		}
		within = append(within, edge)
	}
	return within
}
