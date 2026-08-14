// Package projection is the PROJECT stage of the pipeline: it reshapes the graph the extractor
// produced into the vocabulary the rule is about — files, slices, layers — and drops everything the
// rule is not about. Nothing in here knows Go. A projection is a function of an extraction.Graph and
// one MapFunction, so every domain module gets its own view of one shared graph by writing that hook
// and nothing else.
//
// Three functions are the whole surface, and they line up with the three shapes a rule is judged
// against:
//
//   - ProjectEdges, for a rule about dependencies — the projected edges, self-edges dropped.
//   - ProjectToNodes, for a rule about the nodes themselves — every node of the projection with its
//     incoming and outgoing edges, including the nodes that have neither.
//   - cycles.ProjectCycles, for a rule about cyclic dependencies, over the edges ProjectEdges
//     returned.
//
// A ProjectedEdge keeps the raw edges it was built from, so a violation about slices or layers can
// still name the concrete files that broke it. That is what makes one projection serve both the rule
// and its report, and it is the field nothing here is allowed to drop.
//
// Everything in this package is pure and immutable: no filesystem, no clock, no globals, and every
// value is built once and read afterwards. That is what lets a rule's projection be tested against a
// hand-built graph before any project is extracted at all.
package projection

import (
	"fmt"
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// ProjectedEdge is one dependency in the vocabulary a rule speaks: a source label, a target label,
// and the raw extracted edges that were relabelled onto that pair.
//
// The cumulated edges are the reason a projected edge is not just two strings. A rule about slices
// reports that `api` must not depend on `db`, but the reader needs to know which files did it, and
// after relabelling the only place that survives is here. Every projection therefore carries them,
// and a violation built from a projected edge points at concrete files without going back to the
// graph.
//
// A ProjectedEdge is immutable: build one with NewProjectedEdge or get one from ProjectEdges, and
// read it through its methods. The zero value is an edge between two nameless labels and is what a
// dropped edge would have been.
type ProjectedEdge struct {
	sourceLabel    string
	targetLabel    string
	cumulatedEdges extraction.Graph
}

// NewProjectedEdge builds a projected edge between two labels out of the raw edges that produced it.
//
// The labels are used as given: they are the vocabulary the rule speaks, so a slice called `API` stays
// `API` and only an identifier-shaped label is already normalised, by the extractor that minted it.
// The raw edges go through extraction.NewGraph, which is what gives them this library's one edge
// order and one self-edge shape, and copies them away from the caller's slice.
func NewProjectedEdge(sourceLabel, targetLabel string, cumulatedEdges ...extraction.Edge) ProjectedEdge {
	return ProjectedEdge{
		sourceLabel:    sourceLabel,
		targetLabel:    targetLabel,
		cumulatedEdges: extraction.NewGraph(cumulatedEdges...),
	}
}

// SourceLabel is what the depending end of this edge is called in the projection.
func (e ProjectedEdge) SourceLabel() string {
	return e.sourceLabel
}

// TargetLabel is what the depended-on end of this edge is called in the projection.
func (e ProjectedEdge) TargetLabel() string {
	return e.targetLabel
}

// CumulatedEdges are the raw extracted edges this one projected edge stands for, in the graph's own
// order. They are what a violation names concrete files with.
//
// The result is the caller's own copy, because a Graph is a slice and a projected edge that has been
// reported must not change afterwards.
func (e ProjectedEdge) CumulatedEdges() extraction.Graph {
	return slices.Clone(e.cumulatedEdges)
}

// IsSelfEdge reports whether both ends of this edge carry the same label, which after projection
// means a node depending on itself: a file importing itself, or — far more usefully — two files of
// one slice depending on each other.
//
// It is how the consumers of a projection tell the edges that carry a dependency between nodes from
// the ones that only prove a node exists. ProjectEdges drops these, ProjectToNodes reads the label
// off them, and ProjectCycles ignores them.
func (e ProjectedEdge) IsSelfEdge() bool {
	return e.sourceLabel == e.targetLabel
}

// String renders the projected edge for logs and test failures, as `api -> db [2 edges]`. User-facing
// violation messages are built in the testing layer, not here.
func (e ProjectedEdge) String() string {
	target := e.targetLabel
	if e.IsSelfEdge() {
		target = "itself"
	}
	return fmt.Sprintf("%s -> %s [%d edges]", e.sourceLabel, target, len(e.cumulatedEdges))
}

// ProjectEdges reshapes a graph into the dependencies of the vocabulary mapper speaks: it calls mapper
// once per edge, drops what mapper drops, merges every edge that came out under the same pair of
// labels into one projected edge cumulating all of them, and drops the ones whose two labels are
// equal.
//
// That last step is the "projections filter self-edges out by default" rule of the data model. A node
// depending on itself is not a dependency any rule can be broken by, so a raw self-edge is carried for
// its node and dropped for its edge — and after relabelling the same is true of the edges inside one
// slice, which is why the drop is by label rather than by extraction.Edge.IsSelfEdge. ProjectToNodes
// is where those edges are still read, and it is the function a rule about nodes rather than
// dependencies wants.
//
// The result is ordered by source label then target label, so a report built from it is reproducible,
// and (SourceLabel, TargetLabel) is unique in it, the same way (Source, Target) is unique in a Graph.
// A nil mapper projects nothing, which the empty-test guard then reports rather than passing silently.
func ProjectEdges(graph extraction.Graph, mapper MapFunction) []ProjectedEdge {
	projected := projectAll(graph, mapper)
	dependencies := make([]ProjectedEdge, 0, len(projected))
	for _, edge := range projected {
		if edge.IsSelfEdge() {
			continue
		}
		dependencies = append(dependencies, edge)
	}
	return dependencies
}

// projectAll is ProjectEdges with the self-edges left in, which is what node projection is built out
// of: a node whose only edge is its own self-edge is a node the projection has to name, and dropping
// it here would silently leave a file that depends on nothing out of every rule about naming or
// placement.
//
// It is the one place a MapFunction is called, so relabelling, dropping and merging happen once each.
func projectAll(graph extraction.Graph, mapper MapFunction) []ProjectedEdge {
	if mapper == nil {
		return nil
	}

	cumulated := make(map[projectedKey][]extraction.Edge, len(graph))
	for _, edge := range graph {
		mapped, kept := mapper(edge)
		if !kept || mapped.SourceLabel == "" || mapped.TargetLabel == "" {
			continue
		}
		key := projectedKey{source: mapped.SourceLabel, target: mapped.TargetLabel}
		cumulated[key] = append(cumulated[key], edge)
	}

	projected := make([]ProjectedEdge, 0, len(cumulated))
	for key, edges := range cumulated {
		projected = append(projected, NewProjectedEdge(key.source, key.target, edges...))
	}
	// The key is unique in cumulated, so sorting by it makes the order total — the random map
	// iteration above cannot leak into the result.
	slices.SortFunc(projected, compareProjectedEdges)
	return projected
}

// projectedKey is the identity of a projected edge while it is being merged: two edges that map to the
// same pair of labels are the same projected edge, cumulating both.
type projectedKey struct {
	source string
	target string
}

func compareProjectedEdges(left, right ProjectedEdge) int {
	if bySource := strings.Compare(left.sourceLabel, right.sourceLabel); bySource != 0 {
		return bySource
	}
	return strings.Compare(left.targetLabel, right.targetLabel)
}
