package projection

import "github.com/LukasNiessen/ArchUnitGo/common/extraction"

// MappedEdge is one extracted edge said in the vocabulary a rule is about: two labels and nothing
// else. A label is whatever the rule talks about — a file identifier, a slice name, a layer name —
// and it is taken verbatim, because it is the string the rule's own patterns were written against.
//
// It is deliberately smaller than ProjectedEdge: a MapFunction says what an edge is called, and
// ProjectEdges is what remembers which raw edges ended up under that name.
type MappedEdge struct {
	// SourceLabel is what the edge's source is called in the projected structure. An empty label is
	// not a label, and an edge carrying one is dropped.
	SourceLabel string
	// TargetLabel is what the edge's target is called in the projected structure. An empty label is
	// not a label, and an edge carrying one is dropped.
	TargetLabel string
}

// MapFunction is the hook every projection is written as: a function from one extracted edge to the
// labels it has in the vocabulary a rule speaks, where returning false means drop this edge.
//
// That one convention is why there is no separate filtering step. A projection that only relabels
// returns true for everything; a projection that only selects returns the identifiers unchanged for
// the edges it wants and false for the rest; and a slicing projection does both at once. Every domain
// module gets its own view of one shared graph by writing this function and nothing else — which is
// also why the reshaping layer is not Go-specific in any way.
//
// A MapFunction is called once per edge of the graph, self-edges included, and must be pure: same
// edge in, same answer out, with nothing remembered in between. Write one with Identity or the
// `per <thing> edge` factories here, or with a module's own `slice by <thing>` factory.
type MapFunction func(edge extraction.Edge) (MappedEdge, bool)

// Identity is the MapFunction that relabels nothing and drops nothing: every edge of the graph, under
// the identifiers it already carries. It is the projection of the whole graph into the vocabulary of
// files, where a node is a file and an external target is the import path the file wrote, and it is the
// base every other factory here is written as, so that the identity labeling is stated once.
//
// Alone among the factories here it keeps the self-edges, and that is what makes it the mapper a rule
// about nodes rather than about dependencies is projected through: ProjectToNodes needs the self-edge
// to know that a file depending on nothing is a node at all. For ProjectEdges it makes no difference,
// because an edge whose two labels are equal is dropped there anyway.
func Identity() MapFunction {
	return func(edge extraction.Edge) (MappedEdge, bool) {
		return MappedEdge{SourceLabel: edge.Source, TargetLabel: edge.Target}, true
	}
}

// PerEdge is every dependency of the graph under the identifiers it already carries: Identity without
// the self-edges. It is the projection a file-level rule about dependencies speaks, and it keeps the
// dependencies that leave the project.
//
// Dropping the self-edge is what the whole `per <thing> edge` family means — each of them is about
// dependencies, so none of them keeps the edge that exists only to name a node. It costs nothing for
// ProjectEdges, which drops an edge whose two labels are equal in any case, and it is exactly the
// difference that matters for ProjectToNodes: projected through PerEdge, a file that depends on nothing
// is not a node. Identity is the mapper that keeps it.
func PerEdge() MapFunction {
	identity := Identity()
	return func(edge extraction.Edge) (MappedEdge, bool) {
		if edge.IsSelfEdge() {
			return MappedEdge{}, false
		}
		return identity(edge)
	}
}

// PerInternalEdge is PerEdge without the dependencies that leave the project: an edge to the standard
// library or to a third-party module is dropped, and every dependency inside the project is kept as it
// is.
//
// It is what a rule about the project's own structure is projected through — cycles, layering, slice
// dependencies — because an external target is not code this project can restructure.
func PerInternalEdge() MapFunction {
	perEdge := PerEdge()
	return func(edge extraction.Edge) (MappedEdge, bool) {
		if edge.External {
			return MappedEdge{}, false
		}
		return perEdge(edge)
	}
}

// PerExternalEdge is PerEdge with nothing but the dependencies that leave the project: every edge to
// the standard library or to a third-party module, under the import path the file wrote, and nothing
// inside the project.
//
// It is what a rule about third-party dependencies is projected through, and it is the exact complement
// of PerInternalEdge — the two partition the dependencies PerEdge keeps, so no edge is in both and
// every one of them is in one. Neither decides what external means: both read
// extraction.Edge.External, which the extractor settled once, because it is the only layer that knows
// which code is this project's own.
func PerExternalEdge() MapFunction {
	perEdge := PerEdge()
	return func(edge extraction.Edge) (MappedEdge, bool) {
		if !edge.External {
			return MappedEdge{}, false
		}
		return perEdge(edge)
	}
}
