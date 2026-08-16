package projection

import "strings"

// Node is one node of a snapshot: the label a report draws it under, and whether it is code this project
// owns or something outside it.
//
// A label is not always an identifier, which is why a snapshot's nodes are Node values instead of the
// strings extraction deals in. Without a collapse it is the file identifier or the import path the graph
// carried; with one it is the folder or the group name the query collapsed that node into, and then a single
// Node stands for a hundred files.
//
// A node is external when it is outside the project — the standard library, or a module the project depends
// on. Renderers use the flag to draw those differently, and a summary counts them separately, because `this
// project has nine nodes` means something quite different depending on how many of them are somebody else's
// code.
//
// The zero Node is a node of this project with no label, which no projection produces.
type Node struct {
	// label is what the node is drawn as: an identifier, a folder, or a collapse group's name.
	label string
	// external says the node is not this project's code.
	external bool
}

// NewNode is a node of the project's own code, drawn under this label.
func NewNode(label string) Node {
	return Node{label: label}
}

// NewExternalNode is a node outside the project — the standard library or a dependency module — drawn under
// this label.
//
// It is a constructor of its own rather than a boolean argument to NewNode, because `NewNode("net/http",
// true)` at a call site says nothing about what the flag means, and a report where internal and external
// have been swapped is a report that lies.
func NewExternalNode(label string) Node {
	return Node{label: label, external: true}
}

// Label is what the node is drawn as.
func (n Node) Label() string {
	return n.label
}

// IsExternal reports whether the node is outside the project.
func (n Node) IsExternal() bool {
	return n.external
}

// String renders the node as `internal/api`, or as `net/http (external)` when it is outside the project.
func (n Node) String() string {
	if n.external {
		return n.label + " (external)"
	}
	return n.label
}

// compareNodes orders nodes by label, which is the order a snapshot keeps them in. Labels are unique in a
// snapshot, so this is a total order and a snapshot built from a map cannot leak that map's iteration order
// into a rendered diagram.
func compareNodes(left, right Node) int {
	return strings.Compare(left.label, right.label)
}
