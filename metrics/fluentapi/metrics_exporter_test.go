package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
)

func TestTheExporterWritesTheDocumentTheRendererWouldHaveHandedBack(t *testing.T) {
	// An export is a rendering plus a file, so a report committed beside the code and one rendered in a test are
	// the same document, byte for byte. Everything the page says about itself comes from the options bag.
	options := &rendering.ReportOptions{
		Title:     "the numbers of this project",
		Timestamp: time.Date(2026, time.August, 15, 9, 30, 0, 0, time.UTC),
		Style:     "h1 { color: #b00; }",
	}
	data := reportedNumbers()
	path := filepath.Join(t.TempDir(), "metrics.html")

	if err := fluentapi.NewMetricsExporter(options).ExportAsHTML(data, path); err != nil {
		t.Fatalf("ExportAsHTML failed: %v", err)
	}

	if got, want := readExportedReport(t, path), rendering.RenderHTML(data, options); got != want {
		t.Errorf("the exported file holds\n%s\nwant the page the renderer renders\n%s", got, want)
	}
}

func TestTheExporterWithNoOptionsWritesThePlainPage(t *testing.T) {
	// The zero exporter and the nil bag are the same report: the library's own headline, no stamp, no styling but
	// the one built in — which is what an options bag in this library means by "every default is a zero value".
	folder := t.TempDir()

	for name, exporter := range map[string]fluentapi.MetricsExporter{
		"the nil bag":    fluentapi.NewMetricsExporter(nil),
		"the zero value": {},
	} {
		path := filepath.Join(folder, strings.ReplaceAll(name, " ", "-")+".html")
		if err := exporter.ExportAsHTML(reportedNumbers(), path); err != nil {
			t.Fatalf("ExportAsHTML with %s failed: %v", name, err)
		}

		exported := readExportedReport(t, path)
		if !strings.Contains(exported, "<h1>metrics report</h1>") {
			t.Errorf("the page exported with %s is not headlined by the library:\n%s", name, exported)
		}
		if strings.Contains(exported, `<p class="taken">`) {
			t.Errorf("the page exported with %s stamps itself:\n%s", name, exported)
		}
	}
}

func TestTheOptionsTheExporterWasBuiltWithCannotBeChangedAfterwards(t *testing.T) {
	// The half of immutability a value receiver cannot give: a caller reusing one struct to build an exporter per
	// report would otherwise leave every stored exporter meaning the options of the last.
	options := &rendering.ReportOptions{Title: "the report it was built for"}
	exporter := fluentapi.NewMetricsExporter(options)
	options.Title = "a report somebody built later"
	path := filepath.Join(t.TempDir(), "metrics.html")

	if err := exporter.ExportAsHTML(reportedNumbers(), path); err != nil {
		t.Fatalf("ExportAsHTML failed: %v", err)
	}

	exported := readExportedReport(t, path)
	if !strings.Contains(exported, "<h1>the report it was built for</h1>") {
		t.Errorf("the page holds\n%s\nwant the title the exporter was built with", exported)
	}
}

func TestTheExporterCreatesTheFoldersOfThePathItWasGiven(t *testing.T) {
	// A report exported into `build/reports/` should not fail because nobody had made `build/reports/` yet: the
	// path a user typed says where the file goes, and creating two folders is not a decision worth an error.
	path := filepath.Join(t.TempDir(), "build", "reports", "metrics.html")

	if err := fluentapi.NewMetricsExporter(nil).ExportAsHTML(reportedNumbers(), path); err != nil {
		t.Fatalf("ExportAsHTML failed: %v", err)
	}
	if readExportedReport(t, path) == "" {
		t.Error("the exported file is empty, want the report in it")
	}
}

func TestTheExporterOverwritesTheReportThatWasThereBefore(t *testing.T) {
	// A report is the current answer about the project. A stale one left beside it would be read as a second
	// answer, and appending to it would be a document in no format at all.
	path := filepath.Join(t.TempDir(), "metrics.html")
	yesterday := fluentapi.NewMetricsExporter(&rendering.ReportOptions{Title: "yesterday"})
	today := fluentapi.NewMetricsExporter(&rendering.ReportOptions{Title: "today"})

	if err := yesterday.ExportAsHTML(reportedNumbers(), path); err != nil {
		t.Fatalf("the first ExportAsHTML failed: %v", err)
	}
	if err := today.ExportAsHTML(reportedNumbers(), path); err != nil {
		t.Fatalf("the second ExportAsHTML failed: %v", err)
	}

	exported := readExportedReport(t, path)
	if !strings.Contains(exported, "<h1>today</h1>") {
		t.Errorf("the exported file holds\n%s\nwant the report as it is now", exported)
	}
	if strings.Contains(exported, "yesterday") {
		t.Errorf("the exported file holds\n%s\nwant nothing of the report that was there before", exported)
	}
}

func TestTheExporterWritesAReportWithNoMeasurementInIt(t *testing.T) {
	// The exporter selected nothing, so it cannot tell a project with nothing to report from a pattern that has
	// gone stale — the empty-test guard is asked by the terminals that resolved a scope. What it can do is write a
	// page that says it holds nothing rather than a blank one.
	path := filepath.Join(t.TempDir(), "metrics.html")

	if err := fluentapi.NewMetricsExporter(nil).ExportAsHTML(rendering.ReportData{}, path); err != nil {
		t.Fatalf("ExportAsHTML of an empty report failed: %v, want the caller's own data written", err)
	}

	if exported := readExportedReport(t, path); !strings.Contains(exported, "no measurement in this report") {
		t.Errorf("the exported file holds\n%s\nwant a page saying it holds nothing", exported)
	}
}

func TestTheExporterWithNowhereToWriteToNamesTheTerminalAtFault(t *testing.T) {
	// There is nothing an empty path could have meant — the working directory is a folder, not a file — so this is
	// a call that cannot be run rather than a disk that refused.
	err := fluentapi.NewMetricsExporter(nil).ExportAsHTML(reportedNumbers(), "")

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("ExportAsHTML error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "export as html" {
		t.Errorf("UserError.Operation = %q, want the terminal `export as html`", user.Operation)
	}
	if !errors.Is(err, fluentapi.ErrMissingExportPath) {
		t.Errorf("ExportAsHTML error = %v, want it to wrap ErrMissingExportPath", err)
	}
}

func TestAnExportThatTheFileSystemRefusesIsATechnicalError(t *testing.T) {
	// The library and the data are both fine here and no code has been judged: a path whose parent is a file is the
	// environment refusing, which is the other of the two error types and names the path it failed on.
	blocking := filepath.Join(t.TempDir(), "metrics.html")
	if err := os.WriteFile(blocking, []byte("in the way\n"), 0o600); err != nil {
		t.Fatalf("writing the file in the way failed: %v", err)
	}

	err := fluentapi.NewMetricsExporter(nil).ExportAsHTML(reportedNumbers(), filepath.Join(blocking, "metrics.html"))

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExportAsHTML error = %v, want a *archerror.TechnicalError", err)
	}
	if !strings.Contains(technical.Subject, "metrics.html") {
		t.Errorf("TechnicalError.Subject = %q, want the path it failed on", technical.Subject)
	}
}

// reportedNumbers are the measurements the exporter's tests hand over: two groups of numbers somebody has already
// measured, which is the shape of data this terminal takes rather than resolves.
func reportedNumbers() rendering.ReportData {
	return rendering.ReportData{
		"lines of code": {
			{Metric: "lines of code", Subject: "internal/api/handler.go", Value: 9},
			{Metric: "lines of code", Subject: "main.go", Value: 3},
		},
		"imports": {
			{Metric: "imports", Subject: "internal/api/handler.go", Value: 1},
		},
	}
}

// readExportedReport is an exported report read back as the text a user would open it as.
func readExportedReport(t *testing.T, path string) string {
	t.Helper()

	document, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the exported report failed: %v", err)
	}
	return string(document)
}
