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
// edge in, same answer out, with nothing remembered in between. Write one with the `per <thing>`
// factories here, or with a module's own `slice by <thing>` factory.
type MapFunction func(edge extraction.Edge) (MappedEdge, bool)

// PerEdge is the MapFunction that relabels nothing: every edge of the graph, under the identifiers it
// already carries. It is the projection a file-level rule speaks, where a node is a file, and it is
// the base the narrower ones are written as.
//
// It keeps external edges, and it keeps self-edges: dropping a self-edge from the dependencies is
// ProjectEdges' job, and ProjectToNodes needs the self-edge to know that a file depending on nothing
// is a node at all.
func PerEdge() MapFunction {
	return func(edge extraction.Edge) (MappedEdge, bool) {
		return MappedEdge{SourceLabel: edge.Source, TargetLabel: edge.Target}, true
	}
}

// PerInternalEdge is PerEdge without the dependencies that leave the project: an edge to the standard
// library or to a third-party module is dropped, and everything inside the project is kept as it is.
//
// It is what a rule about the project's own structure is projected through — cycles, layering, slice
// dependencies — because an external target is not code this project can restructure. A rule that is
// about external modules wants PerEdge and the External flag the raw edges still carry.
func PerInternalEdge() MapFunction {
	perEdge := PerEdge()
	return func(edge extraction.Edge) (MappedEdge, bool) {
		if edge.External {
			return MappedEdge{}, false
		}
		return perEdge(edge)
	}
}
