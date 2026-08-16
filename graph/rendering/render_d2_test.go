package rendering_test

import (
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestRenderD2RendersTheWholeReportAsADeclaration(t *testing.T) {
	want := `# the modules of this project
# 3 nodes, 2 edges, 7 dependencies, 1 external node, 1 external edge
direction: right
n0: {label: "internal/api"}
n1: {label: "internal/db"}
n2: {label: "net/http"; style.stroke-dash: 3}
n0 -> n1: "6"
n0 -> n2
`

	if got := rendering.RenderD2(fixtureSnapshot()); got != want {
		t.Errorf("RenderD2() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderD2CarriesTheLabelAsAnAttributeRatherThanAsTheKey(t *testing.T) {
	// A D2 key is a path: an unquoted `.` nests the key under another, so a file declared as one would draw a box
	// called `go` inside a box called `handler`.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("internal/api/handler.go")}, nil)

	document := rendering.RenderD2(snapshot)

	if !strings.Contains(document, `n0: {label: "internal/api/handler.go"}`) {
		t.Errorf("RenderD2() =\n%s\nwant the file's identifier carried as a label", document)
	}
	if strings.Contains(document, "internal/api/handler.go:") {
		t.Errorf("RenderD2() =\n%s\nwant no file path used as a key", document)
	}
}

func TestRenderD2StrokesWhatIsOutsideTheProjectAsADashedBox(t *testing.T) {
	snapshot := projection.NewSnapshot("", []projection.Node{
		projection.NewNode("main.go"),
		projection.NewExternalNode("net/http"),
	}, nil)

	document := rendering.RenderD2(snapshot)

	if !strings.Contains(document, `n1: {label: "net/http"; style.stroke-dash: 3}`) {
		t.Errorf("RenderD2() =\n%s\nwant the external node stroked as a dashed box", document)
	}
	if !strings.Contains(document, `n0: {label: "main.go"}`) {
		t.Errorf("RenderD2() =\n%s\nwant the project's own node left plain", document)
	}
}

func TestRenderD2WritesTheCountOnlyOnAConnectionThatStandsForMoreThanOne(t *testing.T) {
	document := rendering.RenderD2(fixtureSnapshot())

	if !strings.Contains(document, `n0 -> n1: "6"`) {
		t.Errorf("RenderD2() =\n%s\nwant the aggregated connection to say it stands for six dependencies", document)
	}
	if !strings.Contains(document, "n0 -> n2\n") {
		t.Errorf("RenderD2() =\n%s\nwant no count on the connection that stands for a single dependency", document)
	}
}

func TestRenderD2EscapesWhatD2WouldReadAsSyntax(t *testing.T) {
	snapshot := projection.NewSnapshot("a \"quoted\"\r\nreport", []projection.Node{
		projection.NewNode(`back\slash`),
		projection.NewNode(`weird"name`),
	}, nil)

	document := rendering.RenderD2(snapshot)

	if !strings.Contains(document, `n0: {label: "back\\slash"}`) {
		t.Errorf("RenderD2() =\n%s\nwant the backslash in the label escaped", document)
	}
	if !strings.Contains(document, `n1: {label: "weird\"name"}`) {
		t.Errorf("RenderD2() =\n%s\nwant the quote in the label escaped", document)
	}
	if !strings.Contains(document, "# a \\\"quoted\\\" report\n") {
		t.Errorf("RenderD2() =\n%s\nwant the headline on one line", document)
	}
	if strings.Contains(document, "\r") {
		t.Errorf("RenderD2() =\n%q\nwant no carriage return left anywhere in the document", document)
	}
}
