// Package rendering is the metrics module's REPORT stage: it turns the numbers a rule measured into the
// self-contained HTML page a metrics report is read as.
//
// Rendering a report is two steps and this package is the second of them. metrics/calculation answers what a
// number is and reads it off a subject; RenderHTML is a function of the resulting measurements and of the
// options bag beside them, and of nothing else. The split is what the two packages are for: a page laid out
// here says everything about every metric that was ever written, and a metric added there is in the page the
// day it lands.
//
// One exported function, one options bag and one type for the numbers it renders — ReportData, which is the
// measurements grouped under the heading each group is listed under. There is no error, because a pure function
// over an in-memory value has nothing to fail at, and there is no file: writing the page somewhere is
// metrics/fluentapi's business, where MetricsExporter and the `export as html` terminals are, since writing a
// file is the one part of a report that touches a disk.
//
// The page is deterministic. The groups are rendered in the sorted order of their headings, the measurements
// inside a group in the order the rule measured them, and the only clock a page carries is the one the caller
// put on the options bag — so the same numbers render the same bytes, which is what makes a report a thing a
// reviewer can read a diff of and a test can assert on.
package rendering

import (
	"html"
	"strconv"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// htmlStyle is the whole stylesheet of an exported page, inlined into it. It is a const rather than a file
// beside this one because a self-contained report is a promise: one file, no asset next to it, nothing fetched
// when it is opened.
const htmlStyle = `body { font: 15px/1.5 system-ui, sans-serif; margin: 2rem auto; max-width: 60rem; color: #222; }
h1 { font-size: 1.5rem; margin-bottom: 0.25rem; }
h2 { font-size: 1.1rem; margin-top: 2rem; text-transform: lowercase; }
p.summary { color: #666; margin-top: 0; }
p.taken { color: #666; font-variant-numeric: tabular-nums; }
p.spread { color: #666; margin: 0 0 0.5rem; font-variant-numeric: tabular-nums; }
p.note { color: #666; font-style: italic; }
table { border-collapse: collapse; width: 100%; }
th, td { border-bottom: 1px solid #ddd; padding: 0.3rem 0.5rem; text-align: left; }
th { color: #666; font-weight: 600; }
td.value { text-align: right; font-variant-numeric: tabular-nums; }
td.subject { font-family: ui-monospace, monospace; }`

// RenderHTML renders these measurements as one self-contained HTML page, which is the format to reach for when
// the numbers of a project are for a person rather than for a threshold: a page a build attaches to its output,
// or one a reviewer double-clicks to see which files have grown.
//
// Self-contained is the whole constraint, and it is what shapes the page. The stylesheet is inlined, there is no
// script, and nothing is fetched when the file is opened — a report that needs a CDN is a report that renders as
// a blank page on a machine with no network, which is the machine a build runs on. So the page states the report
// in the ways HTML alone can: the headline, how much was measured, the timestamp when the options bag carries
// one, and then one section per group — its spread, and a table of what was measured, which metric read it and
// what came out.
//
// The spread above each table is the arithmetic a reader of a report wants and a list of numbers does not give:
// how many measurements the group holds, the smallest, the largest and the average of them. A rule's threshold
// is not among them, because a page is written from the numbers rather than from what was required of them —
// what a rule required is in the violations it reported.
//
// A nil *ReportOptions means the defaults: the library's own headline, no timestamp and no styling but the one
// built in. A report with no group in it renders as a page saying so rather than as a blank one, and so does a
// group with no measurement under it — a metric whose population a scope did not select is a real answer, and a
// section quietly left out would read as a metric nobody asked for.
func RenderHTML(data ReportData, options *ReportOptions) string {
	resolved := options.WithDefaults()
	title := htmlEscaped(resolved.Headline())
	lines := []string{
		"<!DOCTYPE html>",
		`<html lang="en">`,
		"<head>",
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		"<title>" + title + "</title>",
		"<style>",
	}
	lines = append(lines, htmlStyleSheet(resolved)...)
	lines = append(lines,
		"</style>",
		"</head>",
		"<body>",
		"<h1>"+title+"</h1>",
		`<p class="summary">`+htmlEscaped(summary(data))+"</p>",
	)
	if stamp := resolved.Stamp(); stamp != "" {
		lines = append(lines, `<p class="taken">taken `+htmlEscaped(stamp)+"</p>")
	}
	lines = append(lines, htmlGroups(data)...)
	return strings.Join(append(lines, "</body>", "</html>", ""), "\n")
}

// htmlStyleSheet is the page's stylesheet: the library's own, and then the caller's when they brought one. The
// order is the whole of what `Style` promises, since CSS resolves a tie in favor of the later rule.
func htmlStyleSheet(options ReportOptions) []string {
	if options.Style == "" {
		return []string{htmlStyle}
	}
	return []string{htmlStyle, options.Style}
}

// htmlGroups are the report's groups as the page lists them: a heading, the spread of the numbers under it, and
// a table of them. A report with no group at all says so in a sentence, because an empty page looks like a page
// that failed to render.
func htmlGroups(data ReportData) []string {
	headings := data.Headings()
	if len(headings) == 0 {
		return []string{`<p class="note">no measurement in this report</p>`}
	}
	lines := make([]string, 0, data.Measured()+len(headings)*8)
	for _, heading := range headings {
		lines = append(lines, "<h2>"+htmlEscaped(heading)+"</h2>")
		lines = append(lines, htmlMeasurements(data[heading])...)
	}
	return lines
}

// htmlMeasurements are one group's numbers as the page tabulates them: the spread above, then a row per
// measurement naming what was measured, the metric that read it and the number itself.
//
// A group with nothing in it says so in a sentence. That is the shape a metric whose population a scope did not
// select arrives in — no class selected means no method count — and it is worth a line on the page, because the
// alternative is a reader concluding that the metric does not exist.
func htmlMeasurements(measurements []calculation.Measurement) []string {
	if len(measurements) == 0 {
		return []string{`<p class="note">no measurement in this group</p>`}
	}
	lines := make([]string, 0, len(measurements)+7)
	lines = append(lines,
		`<p class="spread">`+htmlEscaped(spread(measurements))+"</p>",
		"<table>",
		"<thead>",
		"<tr><th>subject</th><th>metric</th><th>value</th></tr>",
		"</thead>",
		"<tbody>",
	)
	for _, measurement := range measurements {
		lines = append(lines, "<tr>"+
			`<td class="subject">`+htmlEscaped(measurement.Subject)+"</td>"+
			"<td>"+htmlEscaped(measurement.Metric)+"</td>"+
			`<td class="value">`+number(measurement.Value)+"</td>"+
			"</tr>")
	}
	return append(lines, "</tbody>", "</table>")
}

// summary is what the page states under its headline: how much was measured, and how many groups it was
// measured into — `12 measurements in 3 groups`.
func summary(data ReportData) string {
	return pluralize(data.Measured(), "measurement", "measurements") + " in " +
		pluralize(len(data), "group", "groups")
}

// spread is what one group's numbers add up to, above the table of them: `4 measurements, min 3, max 9, average
// 4.75`.
//
// The three figures are the whole of what a reader of a report asks a column of numbers, and they are computed
// here rather than in metrics/calculation because they are about a page rather than about a project: what a
// metric is and how it is read off a subject is that package's business, and the average of a column somebody
// grouped is this one's. It is never asked of an empty group — a group with no number has no smallest one — and
// htmlMeasurements is where that case is answered instead.
func spread(measurements []calculation.Measurement) string {
	lowest, highest, total := measurements[0].Value, measurements[0].Value, 0.0
	for _, measurement := range measurements {
		lowest = min(lowest, measurement.Value)
		highest = max(highest, measurement.Value)
		total += measurement.Value
	}
	return pluralize(len(measurements), "measurement", "measurements") +
		", min " + number(lowest) +
		", max " + number(highest) +
		", average " + number(total/float64(len(measurements)))
}

// number is one measured value as the page prints it, with as many digits as it takes to say exactly which
// float64 it is and no more — so a count reads as `120` rather than `120.000000`, and a ratio is never quietly
// rounded into a different number on a page a reader is trying to explain. It is what calculation.Measurement
// prints itself with, so a page and a test failure about the same number agree.
func number(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// pluralize is `1 measurement` and `2 measurements`, the one place this package decides how a count reads.
// Counts are all over a page — the summary, the spread of every group — and English plurals are not worth
// getting wrong in three places.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(count) + " " + plural
}

// htmlEscaped is a piece of text of the page rather than markup of it. Every heading, subject, metric name and
// title goes through it, because a subject is whatever a folder may be called and a report that renders a file
// named with an angle bracket as an element is a report that draws the wrong page. The caller's own stylesheet
// is the one thing that does not, for the reason ReportOptions.Style gives.
func htmlEscaped(value string) string {
	return html.EscapeString(value)
}
