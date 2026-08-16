package rendering_test

import (
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestRenderHTMLStatesTheWholeReportOnOnePage(t *testing.T) {
	document := rendering.RenderHTML(fixtureSnapshot())

	want := []string{
		"<!DOCTYPE html>",
		"<title>the modules of this project</title>",
		"<h1>the modules of this project</h1>",
		`<p class="summary">3 nodes, 2 edges, 7 dependencies, 1 external node, 1 external edge</p>`,
		"<li>internal/api</li>",
		`<li class="external">net/http (external)</li>`,
		`<tr><td>internal/api</td><td>internal/db</td><td class="count">6</td><td>plain, blank</td></tr>`,
		`<tr class="external"><td>internal/api</td><td>net/http</td><td class="count">1</td><td>plain</td></tr>`,
		"</html>",
	}
	for _, line := range want {
		if !strings.Contains(document, line) {
			t.Errorf("the page does not hold %q:\n%s", line, document)
		}
	}
}

func TestRenderHTMLHeadlinesAnUntitledReportWithTheLibrarysOwn(t *testing.T) {
	// An untitled snapshot leaves the headline to the format, and this one puts it in two places a reader sees: the
	// page's own heading and the tab of the browser it is opened in. Both are the page's markup rather than the
	// diagram sources at its foot, which carry a headline of their own.
	document := rendering.RenderHTML(projection.NewSnapshot("", fixtureNodes(), fixtureEdges()))

	for _, want := range []string{"<title>dependency graph</title>", "<h1>dependency graph</h1>"} {
		if !strings.Contains(document, want) {
			t.Errorf("the page does not hold %q:\n%s", want, document)
		}
	}
}

func TestRenderHTMLListsANodeThatNoDependencyTouches(t *testing.T) {
	// The page lists the report's nodes rather than the ends of its dependencies, which is the difference between a
	// page that shows a file depending on nothing and one that has quietly dropped it.
	document := rendering.RenderHTML(projection.NewSnapshot("",
		[]projection.Node{projection.NewNode("internal/util/log.go")}, nil))

	if !strings.Contains(document, "<li>internal/util/log.go</li>") {
		t.Errorf("the page does not list the isolated node:\n%s", document)
	}
}

func TestRenderHTMLIsSelfContained(t *testing.T) {
	// The whole constraint of this format. A page that fetches a stylesheet or a script renders as a blank one on
	// a machine with no network, which is the machine a build runs on — so the style is inlined and there is
	// nothing to fetch.
	document := rendering.RenderHTML(fixtureSnapshot())

	for _, forbidden := range []string{"<script", "http://", "https://", "<link"} {
		if strings.Contains(document, forbidden) {
			t.Errorf("the page holds %q, want one file that needs nothing beside it:\n%s", forbidden, document)
		}
	}
	if !strings.Contains(document, "<style>") {
		t.Errorf("the page has no inlined stylesheet:\n%s", document)
	}
}

func TestRenderHTMLCarriesTheDiagramSourcesTheOtherFormatsWouldHaveExported(t *testing.T) {
	// Laying a graph out would mean shipping a layout engine or calling one over the network, so the page hands
	// the reader the two documents to paste into a tool that draws it — and they are the very documents those two
	// formats export, so the page cannot disagree with a diagram exported beside it.
	snapshot := fixtureSnapshot()

	document := rendering.RenderHTML(snapshot)

	for _, source := range []string{rendering.RenderDot(snapshot), rendering.RenderMermaid(snapshot)} {
		if !strings.Contains(document, escapedForHTML(source)) {
			t.Errorf("the page does not carry the diagram source\n%s\nin\n%s", source, document)
		}
	}
}

func TestRenderHTMLEscapesWhatWouldOtherwiseBeReadAsMarkup(t *testing.T) {
	// A label is whatever a folder may be called, and one holding an angle bracket must be text of the page rather
	// than an element of it.
	snapshot := projection.NewSnapshot("<b>bold</b>", []projection.Node{projection.NewNode("odd<name>.go")}, nil)

	document := rendering.RenderHTML(snapshot)

	if !strings.Contains(document, "<h1>&lt;b&gt;bold&lt;/b&gt;</h1>") {
		t.Errorf("the page does not escape the report's title:\n%s", document)
	}
	if !strings.Contains(document, "<li>odd&lt;name&gt;.go</li>") {
		t.Errorf("the page does not escape the node's label:\n%s", document)
	}
	if strings.Contains(document, "<b>bold</b>") {
		t.Errorf("the page renders the title as markup:\n%s", document)
	}
}

func TestRenderHTMLSaysSoInWordsWhenThereIsNothingToShow(t *testing.T) {
	// An empty list and an empty table both look like a page that failed to render, and a set of files that depend
	// on nothing is a real answer that deserves a sentence.
	isolated := projection.NewSnapshot("", []projection.Node{projection.NewNode("main.go")}, nil)

	if !strings.Contains(rendering.RenderHTML(isolated), "no dependency between these nodes") {
		t.Errorf("the page does not say the nodes depend on nothing:\n%s", rendering.RenderHTML(isolated))
	}
	if !strings.Contains(rendering.RenderHTML(projection.Snapshot{}), "no node in this report") {
		t.Errorf("the page does not say the report is empty:\n%s", rendering.RenderHTML(projection.Snapshot{}))
	}
}

// escapedForHTML is a document as the page embeds it: the three characters that would otherwise be markup,
// written as entities. It is the assertion's own escaping rather than the renderer's, so that a test proving the
// page carries the DOT source does not pass by asking the code under test what escaping means.
func escapedForHTML(document string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;").Replace(document)
}
