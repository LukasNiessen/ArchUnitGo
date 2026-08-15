package rendering_test

import (
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestEveryFormatRendersTheSameSnapshotTwiceTheSameWay(t *testing.T) {
	// The promise that makes a checked-in diagram reviewable: no format reads a map in iteration order and none
	// of them carries a timestamp, so a report exported in two commits differs only where the project did.
	snapshot := fixtureSnapshot()

	for name, render := range formats() {
		t.Run(name, func(t *testing.T) {
			if first, second := render(snapshot), render(snapshot); first != second {
				t.Errorf("the same snapshot rendered\n%s\nand then\n%s", first, second)
			}
		})
	}
}

func TestEveryFormatEndsInExactlyOneNewline(t *testing.T) {
	// A document is written to a file by `export as <format>`, and a file that does not end in a newline is the
	// one every other tool in a pipeline complains about.
	snapshot := fixtureSnapshot()

	for name, render := range formats() {
		t.Run(name, func(t *testing.T) {
			document := render(snapshot)
			if !strings.HasSuffix(document, "\n") {
				t.Errorf("the document ends %q, want a trailing newline", lastCharacters(document))
			}
			if strings.HasSuffix(document, "\n\n") {
				t.Errorf("the document ends %q, want exactly one trailing newline", lastCharacters(document))
			}
		})
	}
}

func TestEveryFormatRendersTheReportsTitleExceptTheOneWithNowhereToPutIt(t *testing.T) {
	// The title is the query's, and CSV is the one format that cannot carry it: a headline above the header row
	// would stop the file being a table a spreadsheet or a script can read.
	titled := projection.NewSnapshot("what the api layer touches", fixtureNodes(), fixtureEdges())

	for name, render := range formats() {
		t.Run(name, func(t *testing.T) {
			document := rendered(name, render, titled)
			holdsTheTitle := strings.Contains(document, "what the api layer touches")
			if name == "csv" && holdsTheTitle {
				t.Errorf("the table carries the report's title:\n%s", document)
			}
			if name != "csv" && !holdsTheTitle {
				t.Errorf("the document does not say what the report is called:\n%s", document)
			}
		})
	}
}

func TestTheFormatsWithAHeadlineFallBackToTheLibrarysOwnForAnUntitledReport(t *testing.T) {
	// An untitled snapshot leaves the headline to the format — what a diagram says at the top is not the
	// projection's business — and the four formats a person reads supply the same fallback rather than four.
	//
	// The two formats a program reads supply none: CSV has no headline at all, and JSON omits the key, because a
	// consumer asking whether the report was titled deserves the answer rather than a name this library invented.
	untitled := projection.NewSnapshot("", fixtureNodes(), fixtureEdges())

	for name, render := range formats() {
		t.Run(name, func(t *testing.T) {
			document := rendered(name, render, untitled)
			headlined := strings.Contains(document, "dependency graph")
			if machineReadable := name == "csv" || name == "json"; machineReadable && headlined {
				t.Errorf("the document invents a headline for an untitled report:\n%s", document)
			} else if !machineReadable && !headlined {
				t.Errorf("the document renders no headline at all for an untitled report:\n%s", document)
			}
		})
	}
}

func TestEveryFormatRendersASnapshotWithNothingInIt(t *testing.T) {
	// The zero snapshot is what a terminal hands back beside an error, and a renderer is a pure function that has
	// no way to refuse it. Every format has to come out as a document that is still valid in its own syntax.
	for name, render := range formats() {
		t.Run(name, func(t *testing.T) {
			if document := render(projection.Snapshot{}); document == "" {
				t.Error("the empty snapshot renders as nothing at all, want an empty report")
			}
		})
	}
}

func TestEveryFormatDrawsANodeThatNoDependencyTouches(t *testing.T) {
	// A file that depends on nothing and that nothing depends on is a node of the report, and for a report about
	// how a project is arranged it is often the interesting one. A format built from the edges alone would lose
	// it, which is why the snapshot carries its nodes separately.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("internal/util/log.go")}, nil)

	for name, render := range formats() {
		t.Run(name, func(t *testing.T) {
			if document := rendered(name, render, snapshot); !strings.Contains(document, "internal/util/log.go") {
				t.Errorf("the isolated node is not in the document:\n%s", document)
			}
		})
	}
}

func TestEveryFormatDrawsAFolderThatDependsOnItself(t *testing.T) {
	// A collapse turns the dependencies between the files of one folder into an arrow from a node to itself, which
	// is exactly what `including self dependencies` asks a report to show — that the files inside the folder are
	// coupled. A format that skipped it would answer a query the user did not write.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("internal/api")}, []projection.Edge{
		projection.NewEdge("internal/api", "internal/api",
			extraction.NewEdge("internal/api/handler.go", "internal/api/router.go", false, extraction.ImportKindPlain),
			extraction.NewEdge("internal/api/router.go", "internal/api/handler.go", false, extraction.ImportKindPlain)),
	})

	arrows := map[string][]string{
		"dot":     {`"internal/api" -> "internal/api" [label="2"];`},
		"mermaid": {"\tn0 -->|2| n0"},
		"d2":      {`n0 -> n0: "2"`},
		"csv":     {"edge,internal/api,internal/api,2,false,plain\n"},
		"json":    {`"source": "internal/api",`, `"target": "internal/api",`, `"dependencies": 2,`},
		"html":    {`<tr><td>internal/api</td><td>internal/api</td><td class="count">2</td>`},
	}
	for name, render := range formats() {
		t.Run(name, func(t *testing.T) {
			document := rendered(name, render, snapshot)
			for _, arrow := range arrows[name] {
				if !strings.Contains(document, arrow) {
					t.Errorf("the document does not draw the folder's arrow to itself as %q:\n%s", arrow, document)
				}
			}
		})
	}
}

func TestTheFormatsWithMintedIdentifiersNameANodeOnlyADependencyDeclares(t *testing.T) {
	// nodeIdentifiers mints an identifier for the labels the dependencies run between as well as for the declared
	// nodes, so a hand-built snapshot carrying an arrow to a node it never declared still renders as valid syntax
	// — a Mermaid arrow with an empty identifier at one end is a diagram that fails to draw at all.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("main.go")},
		[]projection.Edge{projection.NewEdge("main.go", "internal/db")})

	if document := rendering.RenderMermaid(snapshot); !strings.Contains(document, "\tn0 --> n1\n") {
		t.Errorf("RenderMermaid() =\n%s\nwant the undeclared node's end of the arrow named `n1`", document)
	}
	if document := rendering.RenderD2(snapshot); !strings.Contains(document, "n0 -> n1\n") {
		t.Errorf("RenderD2() =\n%s\nwant the undeclared node's end of the arrow named `n1`", document)
	}
}

// formats are the six output formats under the names the fluent API's terminals give them, so that every test
// above states one promise about all of them rather than six times about one.
func formats() map[string]func(projection.Snapshot) string {
	return map[string]func(projection.Snapshot) string{
		"dot":     rendering.RenderDot,
		"mermaid": rendering.RenderMermaid,
		"d2":      rendering.RenderD2,
		"csv":     rendering.RenderCSV,
		"json":    rendering.RenderJSON,
		"html":    rendering.RenderHTML,
	}
}

// rendered is the document a format renders, and for HTML the page's own markup rather than the whole file.
//
// The page embeds the DOT and the Mermaid documents at its foot, and those hold every label and every headline the
// page holds — so a promise about what the page itself renders, asserted over the whole file, is satisfied by the
// copy inside a `<pre>` no matter what the markup above it says. Cutting at the heading of that section is what
// makes a cross-format promise mean the same thing for HTML as for the other five.
func rendered(name string, render func(projection.Snapshot) string, snapshot projection.Snapshot) string {
	document := render(snapshot)
	if name != "html" {
		return document
	}
	markup, _, _ := strings.Cut(document, "<h2>diagram source</h2>")
	return markup
}

// fixtureSnapshot is the report every test in this package renders: two folders of this project and one node
// outside it, one aggregated dependency standing for six and one leaving the project.
func fixtureSnapshot() projection.Snapshot {
	return projection.NewSnapshot("the modules of this project", fixtureNodes(), fixtureEdges())
}

// fixtureNodes are that report's nodes: two of the project's own and one that is somebody else's code.
func fixtureNodes() []projection.Node {
	return []projection.Node{
		projection.NewNode("internal/api"),
		projection.NewNode("internal/db"),
		projection.NewExternalNode("net/http"),
	}
}

// fixtureEdges are that report's dependencies: an aggregated one standing for six imports of two kinds, and a
// single one that leaves the project.
func fixtureEdges() []projection.Edge {
	return []projection.Edge{
		projection.NewEdge("internal/api", "internal/db", sixDependencies()...),
		projection.NewEdge("internal/api", "net/http", extraction.NewEdge("internal/api/handler.go", "net/http", true, extraction.ImportKindPlain)),
	}
}

// sixDependencies are six imports between the files of two folders, one of them a blank import, so that both the
// count an arrow carries and the union of the import kinds a row states are visible in a rendered report.
func sixDependencies() []extraction.Edge {
	dependencies := []extraction.Edge{
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindBlank),
	}
	for _, source := range []string{"a.go", "b.go", "c.go", "d.go", "e.go"} {
		dependencies = append(dependencies,
			extraction.NewEdge("internal/api/"+source, "internal/db/query.go", false, extraction.ImportKindPlain))
	}
	return dependencies
}

// lastCharacters are the tail of a document, for a failure message about how it ends.
func lastCharacters(document string) string {
	const tail = 12
	if len(document) <= tail {
		return document
	}
	return document[len(document)-tail:]
}
