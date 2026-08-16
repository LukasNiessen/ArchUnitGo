package rendering_test

import (
	"strings"
	"testing"
	"time"

	"github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
)

func TestRenderHTMLStatesTheWholeReportOnOnePage(t *testing.T) {
	document := rendering.RenderHTML(fixtureReport(), &rendering.ReportOptions{
		Title:     "the numbers of this project",
		Timestamp: time.Date(2026, time.August, 15, 9, 30, 0, 0, time.UTC),
	})

	want := []string{
		"<!DOCTYPE html>",
		"<title>the numbers of this project</title>",
		"<h1>the numbers of this project</h1>",
		`<p class="summary">5 measurements in 2 groups</p>`,
		`<p class="taken">taken 2026-08-15T09:30:00Z</p>`,
		"<h2>imports</h2>",
		"<h2>lines of code</h2>",
		`<p class="spread">3 measurements, min 3, max 9, average 5.333333333333333</p>`,
		"<tr><th>subject</th><th>metric</th><th>value</th></tr>",
		`<td class="subject">internal/api/handler.go</td><td>lines of code</td><td class="value">9</td>`,
		`<td class="subject">main.go</td><td>imports</td><td class="value">1</td>`,
		"</html>",
	}
	for _, line := range want {
		if !strings.Contains(document, line) {
			t.Errorf("the page does not hold %q:\n%s", line, document)
		}
	}
}

func TestRenderHTMLRendersTheGroupsSortedAndTheirMeasurementsAsMeasured(t *testing.T) {
	// Two orders, and they are different questions. The groups are sorted, because a map has no order to keep. The
	// numbers inside a group keep the order the rule measured them in, because that order is the caller's own —
	// and a page that re-sorted them would disagree with the violations reported beside it.
	document := rendering.RenderHTML(fixtureReport(), nil)

	imports, linesOfCode := strings.Index(document, "<h2>imports</h2>"), strings.Index(document, "<h2>lines of code</h2>")
	if imports > linesOfCode {
		t.Errorf("the groups are not rendered sorted:\n%s", document)
	}
	rows := []string{"main.go", "internal/api/handler.go", "internal/db/conn.go"}
	at := 0
	for _, row := range rows {
		found := strings.Index(document[at:], `<td class="subject">`+row+"</td><td>lines of code</td>")
		if found < 0 {
			t.Fatalf("the page does not hold the row of %q in the measured order:\n%s", row, document)
		}
		at += found
	}
}

func TestRenderHTMLHeadlinesAnUntitledReportWithTheLibrarysOwn(t *testing.T) {
	document := rendering.RenderHTML(fixtureReport(), nil)

	for _, want := range []string{"<title>metrics report</title>", "<h1>metrics report</h1>"} {
		if !strings.Contains(document, want) {
			t.Errorf("the page does not hold %q:\n%s", want, document)
		}
	}
}

func TestRenderHTMLCarriesNoStampWhenTheCallerBroughtNoTime(t *testing.T) {
	// A page reads no clock of its own, so the same numbers render the same bytes: a report committed beside the
	// code does not show up in every diff, and a test can assert on a page at all.
	document := rendering.RenderHTML(fixtureReport(), nil)

	if strings.Contains(document, `<p class="taken">`) {
		t.Errorf("the page stamps itself without being given a time:\n%s", document)
	}
	if again := rendering.RenderHTML(fixtureReport(), nil); again != document {
		t.Error("two renderings of the same report differ, want the page to be deterministic")
	}
}

func TestRenderHTMLIsSelfContained(t *testing.T) {
	// The whole constraint of this format. A page that fetches a stylesheet or a script renders as a blank one on
	// a machine with no network, which is the machine a build runs on — so the style is inlined and there is
	// nothing to fetch.
	document := rendering.RenderHTML(fixtureReport(), nil)

	for _, forbidden := range []string{"<script", "http://", "https://", "<link"} {
		if strings.Contains(document, forbidden) {
			t.Errorf("the page holds %q, want one file that needs nothing beside it:\n%s", forbidden, document)
		}
	}
	if !strings.Contains(document, "<style>") {
		t.Errorf("the page has no inlined stylesheet:\n%s", document)
	}
	// And the tag holds the library's own sheet rather than nothing: a caller who brought no style of their own
	// still gets a styled page, so an empty <style> is not what self-contained means.
	if !strings.Contains(document, "border-collapse: collapse") {
		t.Errorf("the page inlines an empty stylesheet, want the library's own:\n%s", document)
	}
}

func TestRenderHTMLAddsTheCallersOwnStylesheetAfterTheLibrarysOwn(t *testing.T) {
	// After, and not instead of: CSS resolves a tie in favor of the later rule, so a caller who restyles the
	// heading wins it and keeps everything they did not name. The stylesheet is the one thing on the page that is
	// written as it is, because a stylesheet that has been escaped is not one any more.
	own := "h1 { color: #b00; content: \"a > b\"; }"

	document := rendering.RenderHTML(fixtureReport(), &rendering.ReportOptions{Style: own})

	if !strings.Contains(document, own) {
		t.Errorf("the page does not hold the caller's own stylesheet as it is:\n%s", document)
	}
	if library := strings.Index(document, "border-collapse"); library < 0 || library > strings.Index(document, own) {
		t.Errorf("the caller's own stylesheet does not come after the library's own:\n%s", document)
	}
}

func TestRenderHTMLEscapesWhatWouldOtherwiseBeReadAsMarkup(t *testing.T) {
	// A subject is whatever a folder may be called, a title is whatever a rule reads as, and a metric name is
	// whatever a caller of `custom metric` named theirs. One holding an angle bracket must be text of the page
	// rather than an element of it, wherever on the page it lands.
	document := rendering.RenderHTML(rendering.ReportData{
		"<em>count</em>": {measurement("<em>count</em>", "odd<name>.go", 1)},
	}, &rendering.ReportOptions{Title: "<b>bold</b>"})

	for _, want := range []string{
		"<title>&lt;b&gt;bold&lt;/b&gt;</title>",
		"<h1>&lt;b&gt;bold&lt;/b&gt;</h1>",
		"<h2>&lt;em&gt;count&lt;/em&gt;</h2>",
		`<td class="subject">odd&lt;name&gt;.go</td>`,
		"<td>&lt;em&gt;count&lt;/em&gt;</td>",
	} {
		if !strings.Contains(document, want) {
			t.Errorf("the page does not hold %q:\n%s", want, document)
		}
	}
	for _, markup := range []string{"<b>bold</b>", "<em>count</em>"} {
		if strings.Contains(document, markup) {
			t.Errorf("the page renders %q as markup:\n%s", markup, document)
		}
	}
}

func TestRenderHTMLSaysSoInWordsWhenThereIsNothingToShow(t *testing.T) {
	// A blank page looks like a page that failed to render, and both shapes of nothing are real answers: a project
	// with nothing measured, and a metric whose population a scope did not select.
	empty := rendering.RenderHTML(rendering.ReportData{}, nil)
	if !strings.Contains(empty, "no measurement in this report") {
		t.Errorf("the page does not say the report is empty:\n%s", empty)
	}
	if !strings.Contains(empty, `<p class="summary">0 measurements in 0 groups</p>`) {
		t.Errorf("the empty page does not count what it holds:\n%s", empty)
	}

	unselected := rendering.RenderHTML(rendering.ReportData{"method count": nil}, nil)
	if !strings.Contains(unselected, "<h2>method count</h2>") {
		t.Errorf("the page drops the group nothing was measured for:\n%s", unselected)
	}
	if !strings.Contains(unselected, "no measurement in this group") {
		t.Errorf("the page does not say the group is empty:\n%s", unselected)
	}
}

func TestRenderHTMLCountsOneMeasurementInWordsThatAgreeWithIt(t *testing.T) {
	document := rendering.RenderHTML(rendering.ReportData{
		"imports": {measurement("imports", "main.go", 1)},
	}, nil)

	for _, want := range []string{
		`<p class="summary">1 measurement in 1 group</p>`,
		`<p class="spread">1 measurement, min 1, max 1, average 1</p>`,
	} {
		if !strings.Contains(document, want) {
			t.Errorf("the page does not hold %q:\n%s", want, document)
		}
	}
}

func TestRenderHTMLPrintsARatioAsTheNumberItIs(t *testing.T) {
	// Half the family is a ratio, and a ratio rounded on a page is a number nobody can explain — so a value is
	// printed with as many digits as it takes to say exactly which float64 it is, the way a measurement prints
	// itself in a test failure.
	measurements := fixtureReport()["lines of code"]
	document := rendering.RenderHTML(rendering.ReportData{
		"abstractness": {measurement("abstractness", "internal/api", 0.25), measurement("abstractness", "internal/db", 0)},
	}, nil)

	for _, want := range []string{
		`<td class="value">0.25</td>`,
		`<td class="value">0</td>`,
		`<p class="spread">2 measurements, min 0, max 0.25, average 0.125</p>`,
	} {
		if !strings.Contains(document, want) {
			t.Errorf("the page does not hold %q:\n%s", want, document)
		}
	}
	if strings.Contains(rendering.RenderHTML(rendering.ReportData{"lines of code": measurements}, nil), "3.000000") {
		t.Error("the page pads a whole count with decimals")
	}
}

// fixtureReport is the report the tests of this package render: two groups of numbers a rule could have measured
// over a small project, hand-built so that the page under test is the only thing being read.
//
// The `lines of code` group is deliberately in an order that is neither its subjects sorted nor its values
// sorted, so that a page which re-sorted a group's rows would render them differently from the way they were
// measured — which is the promise TestRenderHTMLRendersTheGroupsSortedAndTheirMeasurementsAsMeasured is about,
// and a fixture already in sorted order could not catch.
func fixtureReport() rendering.ReportData {
	return rendering.ReportData{
		"lines of code": {
			measurement("lines of code", "main.go", 4),
			measurement("lines of code", "internal/api/handler.go", 9),
			measurement("lines of code", "internal/db/conn.go", 3),
		},
		"imports": {
			measurement("imports", "internal/api/handler.go", 2),
			measurement("imports", "main.go", 1),
		},
	}
}
