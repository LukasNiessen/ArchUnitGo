package rendering

import (
	"encoding/json"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// RenderJSON renders the snapshot as a JSON document, which is the format to reach for when another program is
// the reader — a dashboard, a script that fails a build when the coupling grows, a tool that draws the diagram
// its own way:
//
//	{
//	  "title": "the modules of this project",
//	  "summary": {
//	    "nodes": 3,
//	    "edges": 2,
//	    "dependencies": 7,
//	    "externalNodes": 1,
//	    "externalEdges": 1
//	  },
//	  "nodes": [
//	    {
//	      "label": "internal/api",
//	      "external": false
//	    }
//	  ],
//	  "edges": [
//	    {
//	      "source": "internal/api",
//	      "target": "internal/db",
//	      "dependencies": 6,
//	      "external": false,
//	      "importKinds": [
//	        "plain",
//	        "blank"
//	      ]
//	    }
//	  ]
//	}
//
// It is the one format that carries the whole snapshot — the title, the summary, every node, every dependency
// with its count and its import kinds — because it is the one whose reader is a program and cannot be trusted to
// have wanted only what a picture shows. The counts are the snapshot's own, so a consumer never has to add up
// the edges to learn how many dependencies a collapsed arrow stands for.
//
// The keys are the vocabulary of the data model rather than the field names of this library's Go types, and an
// edge's count is called `dependencies` for the reason the CSV column is: it is how many of the project's
// dependencies the arrow stands for. An empty report has `[]` for both lists, never `null`, so a consumer can
// range over them without asking first.
//
// The document is indented, ends in a newline and holds no timestamp, so exporting the same report twice writes
// the same file and a checked-in one diffs a line at a time.
func RenderJSON(snapshot projection.Snapshot) string {
	report := jsonReport{
		Title:   snapshot.Title(),
		Summary: jsonSummary(snapshot.Summary()),
		Nodes:   make([]jsonNode, 0, snapshot.Summary().Nodes),
		Edges:   make([]jsonEdge, 0, snapshot.Summary().Edges),
	}
	for _, node := range snapshot.Nodes() {
		report.Nodes = append(report.Nodes, jsonNode{Label: node.Label(), External: node.IsExternal()})
	}
	for _, edge := range snapshot.Edges() {
		report.Edges = append(report.Edges, jsonEdge{
			Source:       edge.SourceLabel(),
			Target:       edge.TargetLabel(),
			Dependencies: edge.Count(),
			External:     edge.IsExternal(),
			ImportKinds:  importKindNames(edge),
		})
	}

	document, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		// Unreachable, and checked rather than discarded so that it stays unreachable: encoding/json fails
		// only on a value it has no encoding for — a channel, a function, a cycle, a NaN — and a report is
		// strings, ints, bools and slices of those. Nothing is the honest answer if one of the four ever
		// reaches here, because half a document would be parsed by whatever reads it.
		return ""
	}
	return string(document) + "\n"
}

// jsonReport is the shape of the document: the snapshot, in the keys a consumer reads it by.
//
// It is a type of its own rather than tags on projection.Snapshot, and that is the point of it — the wire format
// of a report is this package's decision, so renaming a field of the projection cannot silently rename a key
// every consumer of every report is written against. The title is omitted when the query gave none, since a
// report has no headline to state rather than an empty one.
type jsonReport struct {
	Title   string      `json:"title,omitempty"`
	Summary jsonSummary `json:"summary"`
	Nodes   []jsonNode  `json:"nodes"`
	Edges   []jsonEdge  `json:"edges"`
}

// jsonSummary is projection.Summary's five counts under their wire names. Its fields are in the same order and
// of the same types, so the conversion is a cast and a count added to the projection is a compile error here
// rather than a key that quietly went missing from every report.
type jsonSummary struct {
	Nodes         int `json:"nodes"`
	Edges         int `json:"edges"`
	Dependencies  int `json:"dependencies"`
	ExternalNodes int `json:"externalNodes"`
	ExternalEdges int `json:"externalEdges"`
}

// jsonNode is one box of the report: what it is drawn as, and whether it is somebody else's code.
type jsonNode struct {
	Label    string `json:"label"`
	External bool   `json:"external"`
}

// jsonEdge is one arrow of the report: the two labels it runs between, how many of the project's dependencies
// were merged into it, whether following it leaves the project, and the kinds of import behind it.
type jsonEdge struct {
	Source       string   `json:"source"`
	Target       string   `json:"target"`
	Dependencies int      `json:"dependencies"`
	External     bool     `json:"external"`
	ImportKinds  []string `json:"importKinds"`
}
