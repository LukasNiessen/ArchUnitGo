package fluentapi_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestTheSixFormatsRenderTheReportTheChainDescribes(t *testing.T) {
	// The wiring of all twelve terminals, end to end and through the public grammar: locate the fixture project,
	// extract it, project it, render it. What each format looks like is graph/rendering's business and is settled
	// there; what is settled here is that a terminal renders this query's own report and not some other one.
	report := fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t))).
		IncludingExternalDependencies().
		CollapseToFolderDepth(2).
		Titled("the modules of this project")

	tests := []struct {
		format   string
		rendered func() (string, error)
		want     []string
	}{
		{format: "dot", rendered: report.ToDot, want: []string{
			`digraph "the modules of this project" {`,
			`"net/http" [label="net/http", style=dashed];`,
			`"internal/api" -> "internal/db" [label="2"];`,
		}},
		{format: "mermaid", rendered: report.ToMermaid, want: []string{
			"%% the modules of this project",
			"flowchart LR",
			`n2["internal/api"]`,
			`n4(["net/http"])`,
			"n2 -->|2| n3",
		}},
		{format: "d2", rendered: report.ToD2, want: []string{
			"# the modules of this project",
			"direction: right",
			`n2: {label: "internal/api"}`,
			`n2 -> n3: "2"`,
		}},
		{format: "csv", rendered: report.ToCSV, want: []string{
			"kind,source,target,dependencies,external,import kinds",
			"node,internal/api,,,false,",
			"edge,internal/api,net/http,1,true,plain",
		}},
		{format: "json", rendered: report.ToJSON, want: []string{
			`"title": "the modules of this project"`,
			`"nodes": 5`,
			`"label": "internal/api"`,
		}},
		{format: "html", rendered: report.ToHTML, want: []string{
			"<!DOCTYPE html>",
			"<h1>the modules of this project</h1>",
			"<li>internal/api</li>",
			`<li class="external">net/http (external)</li>`,
		}},
	}

	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			document, err := test.rendered()
			if err != nil {
				t.Fatalf("rendering the report failed: %v", err)
			}
			for _, want := range test.want {
				if !strings.Contains(document, want) {
					t.Errorf("the document does not hold %q:\n%s", want, document)
				}
			}
		})
	}
}

func TestTheSixFormatsRenderTheSnapshotTheOtherTerminalHandsBack(t *testing.T) {
	// Rendering is two steps, and this is the seam between them: `to <format>` is the snapshot terminal followed by
	// a pure function of graph/rendering, so a format can neither see a different report nor render it its own way.
	report := fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t))).
		IncludingExternalDependencies().
		CollapseToFolderDepth(1)

	snapshot, err := report.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	tests := []struct {
		format   string
		rendered func() (string, error)
		want     string
	}{
		{format: "dot", rendered: report.ToDot, want: rendering.RenderDot(snapshot)},
		{format: "mermaid", rendered: report.ToMermaid, want: rendering.RenderMermaid(snapshot)},
		{format: "d2", rendered: report.ToD2, want: rendering.RenderD2(snapshot)},
		{format: "csv", rendered: report.ToCSV, want: rendering.RenderCSV(snapshot)},
		{format: "json", rendered: report.ToJSON, want: rendering.RenderJSON(snapshot)},
		{format: "html", rendered: report.ToHTML, want: rendering.RenderHTML(snapshot)},
	}

	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			document, err := test.rendered()
			if err != nil {
				t.Fatalf("rendering the report failed: %v", err)
			}
			if document != test.want {
				t.Errorf("the terminal rendered\n%s\nwant the snapshot rendered as\n%s", document, test.want)
			}
		})
	}
}

func TestTheSixFormatsReportWhatTheSnapshotWouldHaveReported(t *testing.T) {
	// A rendered report judges no code, so it has no violation to state and the only thing a terminal can report is
	// what stopped it reading the project. Every format reports the same failure, and none of them hands back half a
	// document beside it: a reader would believe it.
	tests := []struct {
		failure string
		report  fluentapi.GraphBuilder
		want    error
	}{
		{
			failure: "a query that describes nothing",
			report: fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t))).
				FocusOn("internal/transport/**", 2),
			want: fluentapi.ErrEmptySnapshot,
		},
		{
			failure: "a pattern that will not compile",
			report:  fluentapi.ProjectGraph(nil).FocusOn("[unclosed", 1),
			want:    matching.ErrInvalidPattern,
		},
		{
			failure: "a locator naming no project",
			report:  fluentapi.ProjectGraph(&extraction.ProjectLocator{Directory: t.TempDir()}),
			want:    extraction.ErrModuleFileNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.failure, func(t *testing.T) {
			for format, rendered := range renderingTerminals(test.report) {
				document, err := rendered()
				if !errors.Is(err, test.want) {
					t.Errorf("%s error = %v, want it to wrap %v", format, err, test.want)
				}
				if document != "" {
					t.Errorf("%s rendered %q beside the error, want nothing said about the project", format, document)
				}
			}
		})
	}
}

func TestARenderedReportIsTheSameDocumentEveryTimeItIsRendered(t *testing.T) {
	// The promise a checked-in diagram rests on. A report is a value and rendering reads it twice the same way, so
	// a stored query rendered in two runs differs only where the project did.
	report := fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t))).
		IncludingExternalDependencies().
		Titled("twice")

	for format, rendered := range renderingTerminals(report) {
		first, err := rendered()
		if err != nil {
			t.Fatalf("rendering the report as %s failed: %v", format, err)
		}
		second, err := rendered()
		if err != nil {
			t.Fatalf("rendering the report as %s a second time failed: %v", format, err)
		}
		if first != second {
			t.Errorf("the report rendered as %s\n%s\nand then\n%s", format, first, second)
		}
	}
}

// renderingTerminals are the six `to <format>` terminals of one described report, under the names the chain gives
// them, so that a test states one promise about all six rather than six times about one.
func renderingTerminals(report fluentapi.GraphBuilder) map[string]func() (string, error) {
	return map[string]func() (string, error){
		"ToDot":     report.ToDot,
		"ToMermaid": report.ToMermaid,
		"ToD2":      report.ToD2,
		"ToCSV":     report.ToCSV,
		"ToJSON":    report.ToJSON,
		"ToHTML":    report.ToHTML,
	}
}
