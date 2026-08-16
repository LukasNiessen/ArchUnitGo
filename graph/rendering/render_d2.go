package rendering

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// RenderD2 renders the snapshot as a D2 declaration, which is the format to reach for when the diagram is the
// artifact — D2 lays a large graph out more readably than the other two, and it is what a document that has to
// show four hundred files as nine modules is usually rendered with:
//
//	# the modules of this project
//	# 3 nodes, 2 edges, 7 dependencies
//	direction: right
//	n0: {label: "internal/api"}
//	n1: {label: "internal/db"}
//	n2: {label: "net/http"; style.stroke-dash: 3}
//	n0 -> n1: "6"
//	n0 -> n2
//
// The nodes are `n0`, `n1`, ... and carry their label as an attribute, because a D2 key is a path: an
// unquoted `.` in one nests the key under another, so `internal/api/handler.go` declared as a key would draw a
// box called `go` inside a box called `handler`. nodeIdentifiers says why the numbering is stable.
//
// What is outside the project is stroked as a dashed box, and an arrow says how many dependencies were merged
// into it once there is more than one — the same two facts the other two diagram formats state in their own
// notation.
//
// The result ends in a newline and holds no timestamp, so exporting the same report twice writes the same file.
func RenderD2(snapshot projection.Snapshot) string {
	nodes, edges := snapshot.Nodes(), snapshot.Edges()
	identifiers := nodeIdentifiers(snapshot)
	lines := make([]string, 0, len(nodes)+len(edges)+3)
	lines = append(lines,
		"# "+d2Escaped(headline(snapshot)),
		"# "+d2Escaped(snapshot.Summary().String()),
		"direction: right",
	)
	for _, node := range nodes {
		attributes := []string{"label: " + d2Quoted(node.Label())}
		if node.IsExternal() {
			attributes = append(attributes, "style.stroke-dash: 3")
		}
		lines = append(lines, identifiers[node.Label()]+": {"+strings.Join(attributes, "; ")+"}")
	}
	for _, edge := range edges {
		connection := identifiers[edge.SourceLabel()] + " -> " + identifiers[edge.TargetLabel()]
		if label := arrowLabel(edge); label != "" {
			connection += ": " + d2Quoted(label)
		}
		lines = append(lines, connection)
	}
	return strings.Join(append(lines, ""), "\n")
}

// d2Quoted is a label as D2 wants it written: escaped, in double quotes. Every label this format writes goes
// through it, so that a label is never read as the key path it looks like.
func d2Quoted(value string) string {
	return `"` + d2Escaped(value) + `"`
}

// d2Escaped is a label with what D2 reads as syntax inside a quoted string made literal: a backslash, a double
// quote, and a line break that would end the declaration the label sits in.
func d2Escaped(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ", "\r", "").Replace(value)
}
