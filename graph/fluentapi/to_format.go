package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

// ToDot renders the report as a Graphviz DOT digraph: `to dot`.
//
//	diagram, err := archunit.ProjectGraph(nil).
//		CollapseToFolderDepth(2).
//		Titled("the modules of this project").
//		ToDot()
//
// It is one of the six `to <format>` terminals, and each of them has an `export as <format>` twin that writes
// the same document to a file. All twelve go through Snapshot, so a rendered report is the report this chain
// describes and nothing else, and the error is the one Snapshot returns — a pattern that would not compile, a
// project that will not load, or ErrEmptySnapshot when the query described nothing. When it is non-nil the
// document is empty: half a diagram is worse than none, because a reader would believe it.
//
// DOT is the format for a diagram a tool lays out and somebody looks at. The document holds the headline over
// the summary, a dashed box for whatever is outside the project, and the number of merged dependencies on every
// arrow that stands for more than one. `.dot` is what such a file is conventionally called, and `dot -Tsvg` is
// what turns it into a picture.
func (b GraphBuilder) ToDot() (string, error) {
	return b.rendered(rendering.RenderDot)
}

// ToMermaid renders the report as a Mermaid flowchart: `to mermaid`.
//
//	diagram, err := archunit.ProjectGraph(nil).CollapseToFolderDepth(1).ToMermaid()
//
// Mermaid is the format for a diagram that belongs in a document, because the places that render Markdown — a
// README, a pull request, a wiki — render it with no tool installed anywhere. Nodes outside the project are
// drawn as stadiums rather than boxes, and the headline is a `%%` comment, so the document draws in every
// version of the format rather than only in the newest. `.mmd` is what such a file is conventionally called.
func (b GraphBuilder) ToMermaid() (string, error) {
	return b.rendered(rendering.RenderMermaid)
}

// ToD2 renders the report as a D2 declaration: `to d2`.
//
//	diagram, err := archunit.ProjectGraph(nil).CollapseByPattern("api", "internal/api/**").ToD2()
//
// D2 is the format for a diagram that is the artifact: it lays a large graph out more readably than the other
// two, which is what a picture of four hundred files collapsed onto nine modules needs. `.d2` is what such a
// file is conventionally called.
func (b GraphBuilder) ToD2() (string, error) {
	return b.rendered(rendering.RenderD2)
}

// ToCSV renders the report as one comma-separated table: `to csv`.
//
//	table, err := archunit.ProjectGraph(nil).CollapseToFolderDepth(2).ToCSV()
//
// CSV is the format for a report that is to be counted rather than looked at — sorted in a spreadsheet, grouped
// by a script, diffed between two commits to see which coupling grew. The nodes are rows of the same table as
// the dependencies, told apart by the first column, so that a node with no arrow on it is in the file too. The
// title is not: a headline above the header row would stop the file being a table. `.csv` is what such a file is
// conventionally called.
func (b GraphBuilder) ToCSV() (string, error) {
	return b.rendered(rendering.RenderCSV)
}

// ToJSON renders the report as a JSON document: `to json`.
//
//	report, err := archunit.ProjectGraph(nil).IncludingExternalDependencies().ToJSON()
//
// JSON is the format for a reader that is another program — a dashboard, a script that fails a build when the
// coupling grows, a tool that draws the diagram its own way. It is the one format that carries the whole
// snapshot: the title, the summary's five counts, every node and every dependency with its own count and its
// import kinds. `.json` is what such a file is conventionally called.
func (b GraphBuilder) ToJSON() (string, error) {
	return b.rendered(rendering.RenderJSON)
}

// ToHTML renders the report as one self-contained HTML page: `to html`.
//
//	page, err := archunit.ProjectGraph(nil).CollapseToFolderDepth(2).ToHTML()
//
// HTML is the format for a person with no tool installed: a file a build attaches to its output, or one a
// reviewer double-clicks. It is self-contained in the strict sense — the stylesheet is inlined, there is no
// script and nothing is fetched when it is opened — so it renders the same on a machine with no network, which
// is the machine a build runs on. The page states the summary, the nodes and a table of the dependencies, and
// carries the DOT and Mermaid documents at its foot for a reader who wants the picture laid out. `.html` is what
// such a file is conventionally called.
func (b GraphBuilder) ToHTML() (string, error) {
	return b.rendered(rendering.RenderHTML)
}

// rendered is what every `to <format>` terminal is: build the snapshot this chain describes, then render it.
//
// The two steps are one line each and they are worth having in one place. Every format is a function of a
// projection.Snapshot alone, so this is the whole of what a format needs from the chain — which is why adding
// the next one is a method of three lines here and a function of its own in graph/rendering, and why none of the
// six can differ in what it reports about a query that described nothing.
func (b GraphBuilder) rendered(render func(projection.Snapshot) string) (string, error) {
	snapshot, err := b.Snapshot()
	if err != nil {
		return "", err
	}
	return render(snapshot), nil
}
