// Package rendering is the graph module's REPORT stage: it turns the snapshot a query produced into the six
// formats a dependency-graph report is written in — DOT, Mermaid, D2, CSV, JSON and self-contained HTML.
//
// Rendering a graph is two steps and this package is the second of them. graph/projection answers which nodes
// a report is about, what they are drawn as and how many dependencies each arrow stands for; every function
// here is a function of that one projection.Snapshot and of nothing else. The split is what the two packages
// are for: a format added here understands every query option that was ever written, and a query option added
// there is in all six formats the day it lands.
//
// Six exported functions, one per format, and all of them have the same signature — a snapshot in, a document
// out. There is no options bag, because a renderer decides nothing that a query has not already decided, and
// there is no error, because a pure function over an in-memory value has nothing to fail at. The file a user
// keeps is the fluent API's business: `to <format>` hands back one of these documents and `export as <format>`
// writes it, and both of those live in graph/fluentapi, since writing a file is the one part of a report that
// touches a disk.
//
// Every document is deterministic. The snapshot arrives sorted, nothing here reads a map in iteration order,
// and no format carries a timestamp — so the same graph and the same query render the same bytes, which is
// what makes a checked-in diagram a thing a reviewer can read a diff of.
//
// The formats are not interchangeable, and which one to reach for is the whole choice a caller makes here:
// DOT for a diagram Graphviz lays out, Mermaid for one a Markdown file renders inline, D2 for one D2 lays out,
// CSV for a spreadsheet or a script, JSON for another program, HTML for a page somebody opens.
package rendering

import (
	"strconv"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// headline is what a report is called: the snapshot's title, or `dependency graph` when the query did not say.
//
// An untitled snapshot leaves the headline to the format on purpose — projection.Snapshot does not invent one —
// so this is the single place the formats that print a headline decide what it is, and they cannot disagree
// about it. It is the same fallback projection.Snapshot.String uses, so a report and a test failure about it
// are called the same thing.
func headline(snapshot projection.Snapshot) string {
	if title := snapshot.Title(); title != "" {
		return title
	}
	return "dependency graph"
}

// arrowLabel is what an arrow says about the dependencies behind it: the count when more than one was merged
// into it, and nothing at all when it stands for a single dependency.
//
// It is the trade the three diagram formats make. A collapsed diagram has to say `312`, or it invites the
// reader to believe two folders are barely coupled; an uncollapsed one with a `1` written on every one of four
// hundred arrows is a diagram nobody reads twice. So an arrow with no number on it is one dependency, and it is
// documented here rather than three times. The tabular formats have room for the number and always print it.
func arrowLabel(edge projection.Edge) string {
	if edge.Count() < 2 {
		return ""
	}
	return strconv.Itoa(edge.Count())
}

// nodeIdentifiers are the identifiers Mermaid and D2 refer to a node by — `n0`, `n1` — one per label of the
// snapshot, minted in the snapshot's own sorted order.
//
// Those two formats need them because a node identifier is syntax there and a label is a file path: a `/` ends
// an identifier in Mermaid and a `.` nests a key in D2, so `internal/api/handler.go` cannot be one. The label
// travels as the text the box is drawn with instead, and the identifier is derived from the position in the
// sorted snapshot, which is what keeps a rendered diagram byte-identical between two runs. DOT needs none of
// this: a quoted DOT identifier may hold anything, so it is drawn under the label itself and stays readable in
// a diff.
//
// The labels the dependencies run between are minted after the nodes, so a hand-built snapshot carrying an
// arrow to a node it never declared still renders as valid syntax rather than as a dangling reference.
func nodeIdentifiers(snapshot projection.Snapshot) map[string]string {
	identifiers := make(map[string]string, snapshot.Summary().Nodes)
	mint := func(label string) {
		if _, minted := identifiers[label]; !minted {
			identifiers[label] = "n" + strconv.Itoa(len(identifiers))
		}
	}
	for _, node := range snapshot.Nodes() {
		mint(node.Label())
	}
	for _, edge := range snapshot.Edges() {
		mint(edge.SourceLabel())
		mint(edge.TargetLabel())
	}
	return identifiers
}

// importKindNames are the kinds of import behind one dependency, named, in the declaration order
// extraction.ImportKindSet keeps them in.
//
// The three tabular formats print them and the three diagram formats do not, which is the difference between a
// row that can carry a column nobody reads and an arrow whose label is the one thing on it: `plain, blank`
// written along an arrow costs the diagram more than it tells the reader.
func importKindNames(edge projection.Edge) []string {
	kinds := edge.ImportKinds().Kinds()
	names := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		names = append(names, kind.String())
	}
	return names
}
