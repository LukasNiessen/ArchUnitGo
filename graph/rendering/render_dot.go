package rendering

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// RenderDot renders the snapshot as a Graphviz DOT digraph, which is the format to reach for when the diagram
// is to be laid out by a tool and looked at:
//
//	digraph "the modules of this project" {
//		rankdir=LR;
//		labelloc=t;
//		label="the modules of this project\n3 nodes, 2 edges, 7 dependencies";
//		node [shape=box];
//		"internal/api" [label="internal/api"];
//		"net/http" [label="net/http", style=dashed];
//		"internal/api" -> "internal/db" [label="6"];
//		"internal/api" -> "net/http" [style=dashed];
//	}
//
// The nodes are drawn under their own labels rather than under minted identifiers, because a quoted DOT
// identifier may hold any character a file path holds — so the document stays readable, and a diff between two
// of them names the folder that moved instead of reporting that `n7` became `n8`.
//
// Three things the diagram says beyond who depends on whom. What is outside the project is dashed, node and
// arrow alike, so a reader sees at a glance where the codebase ends. An arrow carries the number of
// dependencies merged into it once a collapse merged more than one, and no number when it stands for exactly
// one. And the headline is the graph's own label, over the summary, so a diagram pasted into a document says
// what it is a diagram of.
//
// The result ends in a newline and holds no timestamp, so exporting the same report twice writes the same file.
func RenderDot(snapshot projection.Snapshot) string {
	nodes, edges := snapshot.Nodes(), snapshot.Edges()
	lines := make([]string, 0, len(nodes)+len(edges)+6)
	lines = append(lines,
		"digraph "+dotQuoted(headline(snapshot))+" {",
		"\trankdir=LR;",
		"\tlabelloc=t;",
		// The two lines of the headline are joined with DOT's own line break rather than with a newline of
		// this file: a literal one inside a quoted attribute is legal and Graphviz draws it as a space.
		"\tlabel=\""+dotEscaped(headline(snapshot))+`\n`+dotEscaped(snapshot.Summary().String())+"\";",
		"\tnode [shape=box];",
	)
	for _, node := range nodes {
		attributes := []string{"label=" + dotQuoted(node.Label())}
		if node.IsExternal() {
			attributes = append(attributes, "style=dashed")
		}
		lines = append(lines, "\t"+dotQuoted(node.Label())+" ["+strings.Join(attributes, ", ")+"];")
	}
	for _, edge := range edges {
		lines = append(lines, "\t"+dotQuoted(edge.SourceLabel())+" -> "+dotQuoted(edge.TargetLabel())+dotAttributes(edge)+";")
	}
	return strings.Join(append(lines, "}", ""), "\n")
}

// dotAttributes are the attributes of one arrow, brackets included, and the empty string when it has none: the
// count of the dependencies behind it when there is more than one, and a dashed line when following it leaves
// the project.
func dotAttributes(edge projection.Edge) string {
	attributes := make([]string, 0, 2)
	if label := arrowLabel(edge); label != "" {
		attributes = append(attributes, "label="+dotQuoted(label))
	}
	if edge.IsExternal() {
		attributes = append(attributes, "style=dashed")
	}
	if len(attributes) == 0 {
		return ""
	}
	return " [" + strings.Join(attributes, ", ") + "]"
}

// dotQuoted is a label as DOT wants it written: escaped, in double quotes. Every identifier and every attribute
// value this format writes goes through it, so a label is never trusted to be a bare DOT identifier — `main.go`
// is not one, and neither is anything holding a space.
func dotQuoted(value string) string {
	return `"` + dotEscaped(value) + `"`
}

// dotEscaped is a label with what DOT reads as syntax inside a quoted string made literal: a backslash, a
// double quote, and a newline that would otherwise end up drawn as a line break the label never asked for.
func dotEscaped(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", "").Replace(value)
}
