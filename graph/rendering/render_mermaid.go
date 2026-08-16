package rendering

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// RenderMermaid renders the snapshot as a Mermaid flowchart, which is the format to reach for when the diagram
// belongs in a document — a README, a pull request, a wiki page — because the places that render Markdown
// render this without a tool being installed anywhere:
//
//	%% the modules of this project
//	%% 3 nodes, 2 edges, 7 dependencies
//	flowchart LR
//		n0["internal/api"]
//		n1["internal/db"]
//		n2(["net/http"])
//		n0 -->|6| n1
//		n0 --> n2
//
// The nodes are `n0`, `n1`, ... because a Mermaid identifier is syntax and a label is a file path; the label is
// the text the box is drawn with, and nodeIdentifiers says why the numbering is stable. What is outside the
// project is drawn as a stadium instead of a box, which is where the difference is worth a shape: an arrow into
// somebody else's code is an arrow into a rounded box, and it needs no second notation on the arrow itself.
//
// The headline and the summary are comments rather than a title block. Mermaid's frontmatter is newer than a
// good deal of what renders Mermaid, and a diagram that fails to draw at all in a reader's wiki is worse than
// one whose headline is only in the source — while a `%%` comment has been in every version of the format.
//
// The result ends in a newline and holds no timestamp, so exporting the same report twice writes the same file.
func RenderMermaid(snapshot projection.Snapshot) string {
	nodes, edges := snapshot.Nodes(), snapshot.Edges()
	identifiers := nodeIdentifiers(snapshot)
	lines := make([]string, 0, len(nodes)+len(edges)+3)
	lines = append(lines,
		"%% "+mermaidEscaped(headline(snapshot)),
		"%% "+mermaidEscaped(snapshot.Summary().String()),
		"flowchart LR",
	)
	for _, node := range nodes {
		opening, closing := `["`, `"]`
		if node.IsExternal() {
			opening, closing = `(["`, `"])`
		}
		lines = append(lines, "\t"+identifiers[node.Label()]+opening+mermaidEscaped(node.Label())+closing)
	}
	for _, edge := range edges {
		arrow := "-->"
		if label := arrowLabel(edge); label != "" {
			arrow += "|" + mermaidEscaped(label) + "|"
		}
		lines = append(lines, "\t"+identifiers[edge.SourceLabel()]+" "+arrow+" "+identifiers[edge.TargetLabel()])
	}
	return strings.Join(append(lines, ""), "\n")
}

// mermaidEscaped is a label with what Mermaid reads as syntax written as the entities it accepts instead: a
// double quote would close the text of a box, a `#` would open one of those entities, and a line break would end
// the statement the box is declared by.
func mermaidEscaped(value string) string {
	return strings.NewReplacer("#", "#35;", `"`, "#quot;", "\n", " ", "\r", "").Replace(value)
}
