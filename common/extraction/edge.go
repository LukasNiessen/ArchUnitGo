package extraction

import "fmt"

// Edge is the atom of the library: one dependency, from one node to another, as found in the
// source. Everything downstream — projection, assertion, reporting — is a function of a list of
// these.
//
// Source and Target are identifiers in the canonical form described in identifier.go. An Edge is
// comparable, so it can be a map key and compared with ==.
type Edge struct {
	// Source is the node the dependency starts at, always a node inside the project.
	Source string
	// Target is the node the dependency points at: a project node, or a Go import path when
	// External is true.
	Target string
	// External marks a target outside the project — the standard library or a dependency module.
	External bool
	// ImportKinds is the set of import flavors that produced this edge. Parallel imports are
	// merged into one Edge whose kinds are the union of theirs.
	ImportKinds ImportKindSet
}

// NewEdge builds an edge between two nodes, normalising both identifiers.
func NewEdge(source, target string, external bool, kinds ...ImportKind) Edge {
	return Edge{
		Source:      NormalizeIdentifier(source),
		Target:      NormalizeIdentifier(target),
		External:    external,
		ImportKinds: NewImportKindSet(kinds...),
	}
}

// SelfEdge builds the edge every file gets from itself to itself. It is how a file with no
// dependencies still appears as a node: projections drop self-edges by default, and node
// projection is built out of them. It carries no import kinds and is not external, because no
// import produced it — and NewGraph reduces every edge from a node to itself to exactly this, so
// that is true of a self-edge found in a graph and not only of one built here.
func SelfEdge(identifier string) Edge {
	normalized := NormalizeIdentifier(identifier)
	return Edge{Source: normalized, Target: normalized}
}

// IsSelfEdge reports whether the edge is a node's edge to itself. It is how projections tell the
// edge that carries a node from the edges that carry a dependency.
func (e Edge) IsSelfEdge() bool {
	return e.Source == e.Target
}

// canonical returns the edge in the form a Graph holds it: both identifiers normalised, and an edge
// from a node to itself reduced to the self-edge SelfEdge would have built.
//
// The second half is what keeps one shape of self-edge in a graph rather than two. A file may import
// its own package — illegal Go, but a string a file can write and one the toolchain resolves to that
// file among the others, so extraction does emit the edge — and it would otherwise land as a
// self-edge claiming a plain import. Downstream code drops self-edges without reading either field,
// so the two shapes would differ only where a report looked closely enough to disagree with itself.
func (e Edge) canonical() Edge {
	normalized := Edge{
		Source:      NormalizeIdentifier(e.Source),
		Target:      NormalizeIdentifier(e.Target),
		External:    e.External,
		ImportKinds: e.ImportKinds,
	}
	if normalized.IsSelfEdge() {
		return Edge{Source: normalized.Source, Target: normalized.Target}
	}
	return normalized
}

// key is the identity of an edge for merging purposes. Downstream code may assume that
// (source, target) is unique within a Graph.
func (e Edge) key() edgeKey {
	return edgeKey{source: e.Source, target: e.Target}
}

// merge folds another edge between the same two nodes into this one, unioning their import kinds.
// Two edges disagreeing about externality means one of them saw a target the other did not resolve,
// so externality is unioned too.
func (e Edge) merge(other Edge) Edge {
	return Edge{
		Source:      e.Source,
		Target:      e.Target,
		External:    e.External || other.External,
		ImportKinds: e.ImportKinds.Union(other.ImportKinds),
	}
}

// String renders the edge for logs and test failures. User-facing violation messages are built in
// the testing layer, not here.
func (e Edge) String() string {
	if e.IsSelfEdge() {
		return fmt.Sprintf("%s -> itself", e.Source)
	}
	target := e.Target
	if e.External {
		target += " (external)"
	}
	return fmt.Sprintf("%s -> %s %s", e.Source, target, e.ImportKinds)
}

type edgeKey struct {
	source string
	target string
}
