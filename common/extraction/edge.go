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
// projection is built out of them. It carries no import kinds, because no import produced it.
func SelfEdge(identifier string) Edge {
	normalized := NormalizeIdentifier(identifier)
	return Edge{Source: normalized, Target: normalized}
}

// IsSelfEdge reports whether the edge is a node's edge to itself.
func (e Edge) IsSelfEdge() bool {
	return e.Source == e.Target
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
