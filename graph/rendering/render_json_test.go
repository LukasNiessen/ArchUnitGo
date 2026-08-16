package rendering_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestRenderJSONRendersTheWholeReportAsADocument(t *testing.T) {
	want := `{
  "title": "the modules of this project",
  "summary": {
    "nodes": 3,
    "edges": 2,
    "dependencies": 7,
    "externalNodes": 1,
    "externalEdges": 1
  },
  "nodes": [
    {
      "label": "internal/api",
      "external": false
    },
    {
      "label": "internal/db",
      "external": false
    },
    {
      "label": "net/http",
      "external": true
    }
  ],
  "edges": [
    {
      "source": "internal/api",
      "target": "internal/db",
      "dependencies": 6,
      "external": false,
      "importKinds": [
        "plain",
        "blank"
      ]
    },
    {
      "source": "internal/api",
      "target": "net/http",
      "dependencies": 1,
      "external": true,
      "importKinds": [
        "plain"
      ]
    }
  ]
}
`

	if got := rendering.RenderJSON(fixtureSnapshot()); got != want {
		t.Errorf("RenderJSON() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderJSONIsADocumentAnotherProgramCanRead(t *testing.T) {
	// The format exists for a reader that is a program, so it is asserted the way that reader would consume it:
	// unmarshalled, and read by the keys the document promises rather than by the shape of its text.
	var report struct {
		Title   string `json:"title"`
		Summary struct {
			Nodes         int `json:"nodes"`
			Edges         int `json:"edges"`
			Dependencies  int `json:"dependencies"`
			ExternalNodes int `json:"externalNodes"`
			ExternalEdges int `json:"externalEdges"`
		} `json:"summary"`
		Nodes []struct {
			Label    string `json:"label"`
			External bool   `json:"external"`
		} `json:"nodes"`
		Edges []struct {
			Source       string   `json:"source"`
			Target       string   `json:"target"`
			Dependencies int      `json:"dependencies"`
			External     bool     `json:"external"`
			ImportKinds  []string `json:"importKinds"`
		} `json:"edges"`
	}

	if err := json.Unmarshal([]byte(rendering.RenderJSON(fixtureSnapshot())), &report); err != nil {
		t.Fatalf("the document does not parse as JSON: %v", err)
	}

	if report.Title != "the modules of this project" {
		t.Errorf("the report is titled %q, want what the query said", report.Title)
	}
	if report.Summary.Nodes != 3 || report.Summary.Edges != 2 || report.Summary.Dependencies != 7 {
		t.Errorf("the summary is %+v, want the snapshot's own counts", report.Summary)
	}
	if report.Summary.ExternalNodes != 1 || report.Summary.ExternalEdges != 1 {
		t.Errorf("the summary is %+v, want one node and one dependency outside the project", report.Summary)
	}
	if len(report.Nodes) != 3 || report.Nodes[2].Label != "net/http" || !report.Nodes[2].External {
		t.Errorf("the nodes are %+v, want the third of them to be the one outside the project", report.Nodes)
	}
	if len(report.Edges) != 2 || report.Edges[0].Dependencies != 6 {
		t.Fatalf("the dependencies are %+v, want the first to stand for six", report.Edges)
	}
	if want := []string{"plain", "blank"}; !slices.Equal(report.Edges[0].ImportKinds, want) {
		t.Errorf("the aggregated dependency's import kinds are %v, want %v", report.Edges[0].ImportKinds, want)
	}
}

func TestRenderJSONRendersASnapshotWithNothingInItAsEmptyListsAndNoTitle(t *testing.T) {
	// `null` for a list a consumer is about to range over is the one thing a machine-readable format must not do,
	// and a report with no headline says nothing rather than an empty something.
	want := `{
  "summary": {
    "nodes": 0,
    "edges": 0,
    "dependencies": 0,
    "externalNodes": 0,
    "externalEdges": 0
  },
  "nodes": [],
  "edges": []
}
`

	if got := rendering.RenderJSON(projection.Snapshot{}); got != want {
		t.Errorf("RenderJSON() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderJSONEscapesALabelThatWouldOtherwiseBreakTheDocument(t *testing.T) {
	// Escaping is encoding/json's, which is the reason this format goes through it rather than through a builder
	// of this package: a label is whatever a folder may be called, and a quote in one must not end a string.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode(`weird"name`)}, nil)

	document := rendering.RenderJSON(snapshot)

	if !strings.Contains(document, `"label": "weird\"name"`) {
		t.Errorf("RenderJSON() =\n%s\nwant the quote in the label escaped", document)
	}
	if !json.Valid([]byte(document)) {
		t.Errorf("RenderJSON() =\n%s\nwant a document that parses", document)
	}
}
