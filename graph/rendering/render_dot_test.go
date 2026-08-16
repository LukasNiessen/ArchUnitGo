package rendering_test

import (
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestRenderDotRendersTheWholeReportAsADigraph(t *testing.T) {
	// The whole document, asserted as one string, because the whole point of a pure renderer is that there is
	// exactly one right answer for a given snapshot and a reviewer can read it.
	want := `digraph "the modules of this project" {
	rankdir=LR;
	labelloc=t;
	label="the modules of this project\n3 nodes, 2 edges, 7 dependencies, 1 external node, 1 external edge";
	node [shape=box];
	"internal/api" [label="internal/api"];
	"internal/db" [label="internal/db"];
	"net/http" [label="net/http", style=dashed];
	"internal/api" -> "internal/db" [label="6"];
	"internal/api" -> "net/http" [style=dashed];
}
`

	if got := rendering.RenderDot(fixtureSnapshot()); got != want {
		t.Errorf("RenderDot() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderDotDrawsNodesUnderTheirOwnLabels(t *testing.T) {
	// DOT identifiers are the labels themselves, unlike Mermaid's and D2's, because a quoted one may hold any
	// character a file path holds — so a diff between two exported diagrams names the folder that moved.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("internal/api/handler.go")}, nil)

	if !strings.Contains(rendering.RenderDot(snapshot), `"internal/api/handler.go" [label="internal/api/handler.go"];`) {
		t.Errorf("RenderDot() =\n%s\nwant the file drawn under its own identifier", rendering.RenderDot(snapshot))
	}
}

func TestRenderDotWritesTheCountOnlyOnAnArrowThatStandsForMoreThanOne(t *testing.T) {
	// The trade every diagram format makes: a collapsed arrow has to say how many dependencies it merged, and a
	// `1` on every arrow of four hundred is a diagram nobody reads twice.
	document := rendering.RenderDot(fixtureSnapshot())

	if !strings.Contains(document, `"internal/api" -> "internal/db" [label="6"]`) {
		t.Errorf("RenderDot() =\n%s\nwant the aggregated arrow to say it stands for six dependencies", document)
	}
	if strings.Contains(document, `label="1"`) {
		t.Errorf("RenderDot() =\n%s\nwant no count on the arrow that stands for a single dependency", document)
	}
}

func TestRenderDotEscapesWhatDotWouldReadAsSyntax(t *testing.T) {
	// A label is whatever a folder may be called. An unescaped quote in one would end the identifier and leave a
	// document Graphviz refuses, which is the failure a user would see as `the library exported nonsense`.
	snapshot := projection.NewSnapshot("a \"quoted\"\r\nreport", []projection.Node{
		projection.NewNode(`weird"name`),
		projection.NewNode(`back\slash`),
		projection.NewNode("line\r\nbreak"),
	}, nil)

	document := rendering.RenderDot(snapshot)

	if !strings.Contains(document, `"weird\"name" [label="weird\"name"];`) {
		t.Errorf("RenderDot() =\n%s\nwant the quote in the label escaped", document)
	}
	if !strings.Contains(document, `"back\\slash" [label="back\\slash"];`) {
		t.Errorf("RenderDot() =\n%s\nwant the backslash in the label escaped", document)
	}
	if !strings.Contains(document, `"line\nbreak" [label="line\nbreak"];`) {
		t.Errorf("RenderDot() =\n%s\nwant the line break in the label written as DOT's own escape", document)
	}
	if !strings.Contains(document, `digraph "a \"quoted\"\nreport" {`) {
		t.Errorf("RenderDot() =\n%s\nwant the quotes and the line break in the headline escaped", document)
	}
	if strings.Contains(document, "\r") {
		t.Errorf("RenderDot() =\n%q\nwant no carriage return left anywhere in the document", document)
	}
}

func TestRenderDotRendersASnapshotWithNothingInItAsAnEmptyDigraph(t *testing.T) {
	// The zero snapshot is what a terminal hands back beside an error, and `digraph {}` is what a report of
	// nothing looks like in this format — not a document Graphviz rejects.
	want := `digraph "dependency graph" {
	rankdir=LR;
	labelloc=t;
	label="dependency graph\n0 nodes, 0 edges, 0 dependencies";
	node [shape=box];
}
`

	if got := rendering.RenderDot(projection.Snapshot{}); got != want {
		t.Errorf("RenderDot() =\n%s\nwant\n%s", got, want)
	}
}
