package rendering_test

import (
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestRenderMermaidRendersTheWholeReportAsAFlowchart(t *testing.T) {
	want := `%% the modules of this project
%% 3 nodes, 2 edges, 7 dependencies, 1 external node, 1 external edge
flowchart LR
	n0["internal/api"]
	n1["internal/db"]
	n2(["net/http"])
	n0 -->|6| n1
	n0 --> n2
`

	if got := rendering.RenderMermaid(fixtureSnapshot()); got != want {
		t.Errorf("RenderMermaid() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderMermaidNumbersTheNodesInTheSnapshotsOrder(t *testing.T) {
	// A Mermaid identifier is syntax and a label is a file path, so the identifiers are minted — and they are
	// minted from the position in the sorted snapshot, which is what keeps two renderings of one report identical.
	snapshot := projection.NewSnapshot("", []projection.Node{
		projection.NewNode("internal/api/handler.go"),
		projection.NewNode("main.go"),
	}, []projection.Edge{projection.NewEdge("main.go", "internal/api/handler.go")})

	want := []string{
		`	n0["internal/api/handler.go"]`,
		`	n1["main.go"]`,
		"	n1 --> n0",
	}
	document := rendering.RenderMermaid(snapshot)
	for _, line := range want {
		if !strings.Contains(document, line) {
			t.Errorf("RenderMermaid() =\n%s\nwant the line %q in it", document, line)
		}
	}
}

func TestRenderMermaidDrawsWhatIsOutsideTheProjectAsAStadium(t *testing.T) {
	// The one place this format spends notation on: an arrow into somebody else's code is an arrow into a rounded
	// box, so a reader sees where the codebase ends without a second notation on the arrow itself.
	snapshot := projection.NewSnapshot("", []projection.Node{
		projection.NewNode("main.go"),
		projection.NewExternalNode("net/http"),
	}, nil)

	document := rendering.RenderMermaid(snapshot)

	if !strings.Contains(document, `n1(["net/http"])`) {
		t.Errorf("RenderMermaid() =\n%s\nwant the external node drawn as a stadium", document)
	}
	if !strings.Contains(document, `n0["main.go"]`) {
		t.Errorf("RenderMermaid() =\n%s\nwant the project's own node drawn as a box", document)
	}
}

func TestRenderMermaidEscapesWhatMermaidWouldReadAsSyntax(t *testing.T) {
	// A quote would close the text of a box and a `#` would open an entity, so both travel as the entities
	// Mermaid accepts instead. A newline in a title would end the comment the headline is written as.
	snapshot := projection.NewSnapshot("a #tagged\r\nreport", []projection.Node{projection.NewNode(`weird"name`)}, nil)

	document := rendering.RenderMermaid(snapshot)

	if !strings.Contains(document, `n0["weird#quot;name"]`) {
		t.Errorf("RenderMermaid() =\n%s\nwant the quote in the label written as an entity", document)
	}
	if !strings.Contains(document, "%% a #35;tagged report\n") {
		t.Errorf("RenderMermaid() =\n%s\nwant the headline on one line with its hash escaped", document)
	}
	if strings.Contains(document, "\r") {
		t.Errorf("RenderMermaid() =\n%q\nwant no carriage return left anywhere in the document", document)
	}
}
