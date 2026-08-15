package rendering

import (
	"html"
	"strconv"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// htmlStyle is the whole stylesheet of an exported page, inlined into it. It is a const rather than a file
// beside this one because a self-contained report is a promise: one file, no asset next to it, nothing fetched
// when it is opened.
const htmlStyle = `body { font: 15px/1.5 system-ui, sans-serif; margin: 2rem auto; max-width: 60rem; color: #222; }
h1 { font-size: 1.5rem; margin-bottom: 0.25rem; }
h2 { font-size: 1.1rem; margin-top: 2rem; text-transform: lowercase; }
p.summary { color: #666; margin-top: 0; }
p.note { color: #666; font-style: italic; }
ul.nodes { list-style: none; padding: 0; display: flex; flex-wrap: wrap; gap: 0.5rem; }
ul.nodes li { border: 1px solid #bbb; border-radius: 0.25rem; padding: 0.2rem 0.5rem; font-family: ui-monospace, monospace; }
ul.nodes li.external { border-style: dashed; color: #666; }
table { border-collapse: collapse; width: 100%; }
th, td { border-bottom: 1px solid #ddd; padding: 0.3rem 0.5rem; text-align: left; }
th { color: #666; font-weight: 600; }
td.count { text-align: right; font-variant-numeric: tabular-nums; }
tr.external td { color: #666; }
pre { background: #f6f6f6; border-radius: 0.25rem; padding: 0.75rem; overflow-x: auto; }
summary { cursor: pointer; margin-top: 0.5rem; }`

// RenderHTML renders the snapshot as one self-contained HTML page, which is the format to reach for when the
// report is for a person who has no tool installed: a file a build attaches to its output, or one a reviewer
// double-clicks.
//
// Self-contained is the whole constraint, and it is what shapes the page. The stylesheet is inlined, there is no
// script, and nothing is fetched when the file is opened — a report that needs a CDN is a report that renders as
// a blank page on a machine with no network, which is the machine a build runs on. So the page states the
// report in the two ways HTML alone can: the headline and the summary, the nodes as labeled boxes with what is
// outside the project drawn dashed, and the dependencies as a table of who depends on whom, how many times, and
// through which kinds of import.
//
// What it does not do is lay a graph out. Drawing the arrows would mean shipping a layout engine or calling one
// over the network, and neither belongs in a library's output file — so the DOT and Mermaid documents are
// included at the foot of the page instead, each in a collapsed block, for a reader who wants the picture to
// paste into a tool that draws it. The two are RenderDot and RenderMermaid, so the page cannot disagree with
// what those two formats would have exported beside it.
//
// The result ends in a newline and holds no timestamp, so exporting the same report twice writes the same file.
func RenderHTML(snapshot projection.Snapshot) string {
	title := htmlEscaped(headline(snapshot))
	lines := []string{
		"<!DOCTYPE html>",
		`<html lang="en">`,
		"<head>",
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		"<title>" + title + "</title>",
		"<style>",
		htmlStyle,
		"</style>",
		"</head>",
		"<body>",
		"<h1>" + title + "</h1>",
		`<p class="summary">` + htmlEscaped(snapshot.Summary().String()) + "</p>",
		"<h2>nodes</h2>",
	}
	lines = append(lines, htmlNodes(snapshot)...)
	lines = append(lines, "<h2>dependencies</h2>")
	lines = append(lines, htmlEdges(snapshot)...)
	lines = append(lines,
		"<h2>diagram source</h2>",
		"<details>",
		"<summary>DOT</summary>",
		"<pre>"+htmlEscaped(RenderDot(snapshot))+"</pre>",
		"</details>",
		"<details>",
		"<summary>Mermaid</summary>",
		"<pre>"+htmlEscaped(RenderMermaid(snapshot))+"</pre>",
		"</details>",
		"</body>",
		"</html>",
		"",
	)
	return strings.Join(lines, "\n")
}

// htmlNodes are the report's nodes as the page lists them: one labeled box each, dashed and marked when the
// node is outside the project. A report with no node says so in a sentence, because an empty list on a page
// looks like a page that failed to render.
func htmlNodes(snapshot projection.Snapshot) []string {
	nodes := snapshot.Nodes()
	if len(nodes) == 0 {
		return []string{`<p class="note">no node in this report</p>`}
	}
	lines := make([]string, 0, len(nodes)+2)
	lines = append(lines, `<ul class="nodes">`)
	for _, node := range nodes {
		item := "<li>" + htmlEscaped(node.Label()) + "</li>"
		if node.IsExternal() {
			item = `<li class="external">` + htmlEscaped(node.Label()) + " (external)</li>"
		}
		lines = append(lines, item)
	}
	return append(lines, "</ul>")
}

// htmlEdges are the report's dependencies as the page tabulates them: who depends on whom, how many of the
// project's dependencies the arrow stands for, and the kinds of import behind it. A report of nodes and no
// arrow says so in a sentence — a set of files that depend on nothing is a real answer, and often a good one.
func htmlEdges(snapshot projection.Snapshot) []string {
	edges := snapshot.Edges()
	if len(edges) == 0 {
		return []string{`<p class="note">no dependency between these nodes</p>`}
	}
	lines := make([]string, 0, len(edges)+6)
	lines = append(lines,
		"<table>",
		"<thead>",
		"<tr><th>from</th><th>to</th><th>dependencies</th><th>import kinds</th></tr>",
		"</thead>",
		"<tbody>",
	)
	for _, edge := range edges {
		row := "<tr>"
		if edge.IsExternal() {
			row = `<tr class="external">`
		}
		lines = append(lines, row+
			"<td>"+htmlEscaped(edge.SourceLabel())+"</td>"+
			"<td>"+htmlEscaped(edge.TargetLabel())+"</td>"+
			`<td class="count">`+strconv.Itoa(edge.Count())+"</td>"+
			"<td>"+htmlEscaped(strings.Join(importKindNames(edge), ", "))+"</td>"+
			"</tr>")
	}
	return append(lines, "</tbody>", "</table>")
}

// htmlEscaped is a label as text of the page rather than markup of it. Every label, title and embedded document
// on the page goes through it, because a label is whatever a folder may be called and a report that renders a
// file named with an angle bracket as an element is a report that draws the wrong page.
func htmlEscaped(value string) string {
	return html.EscapeString(value)
}
