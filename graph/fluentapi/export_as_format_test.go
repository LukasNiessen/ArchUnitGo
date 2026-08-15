package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
)

func TestTheSixExportsWriteTheDocumentTheirStringFormWouldHaveHandedBack(t *testing.T) {
	// The promise that makes the twelve terminals six: `export as <format>` is `to <format>` plus a file, so a
	// diagram committed beside the code and one rendered in a test are the same document, byte for byte.
	report := fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t))).
		IncludingExternalDependencies().
		CollapseToFolderDepth(2).
		Titled("the modules of this project")
	folder := t.TempDir()

	tests := []struct {
		format    string
		extension string
		exported  func(string) error
		rendered  func() (string, error)
	}{
		{format: "dot", extension: ".dot", exported: report.ExportAsDot, rendered: report.ToDot},
		{format: "mermaid", extension: ".mmd", exported: report.ExportAsMermaid, rendered: report.ToMermaid},
		{format: "d2", extension: ".d2", exported: report.ExportAsD2, rendered: report.ToD2},
		{format: "csv", extension: ".csv", exported: report.ExportAsCSV, rendered: report.ToCSV},
		{format: "json", extension: ".json", exported: report.ExportAsJSON, rendered: report.ToJSON},
		{format: "html", extension: ".html", exported: report.ExportAsHTML, rendered: report.ToHTML},
	}

	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			path := filepath.Join(folder, "architecture"+test.extension)

			if err := test.exported(path); err != nil {
				t.Fatalf("exporting the report failed: %v", err)
			}

			want, err := test.rendered()
			if err != nil {
				t.Fatalf("rendering the report failed: %v", err)
			}
			if got := readExportedReport(t, path); got != want {
				t.Errorf("the exported file holds\n%s\nwant the document the string form renders\n%s", got, want)
			}
		})
	}
}

func TestAnExportCreatesTheFoldersOfThePathItWasGiven(t *testing.T) {
	// A report exported into `docs/diagrams/` should not fail because nobody had made `docs/diagrams/` yet: the
	// path a user typed says where the file goes, and creating two folders is not a decision worth an error.
	report := fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t)))
	path := filepath.Join(t.TempDir(), "docs", "diagrams", "architecture.dot")

	if err := report.ExportAsDot(path); err != nil {
		t.Fatalf("ExportAsDot failed: %v", err)
	}
	if readExportedReport(t, path) == "" {
		t.Error("the exported file is empty, want the report in it")
	}
}

func TestAnExportOverwritesTheReportThatWasThereBefore(t *testing.T) {
	// A report is the current answer about the project. A stale one left beside it would be read as a second
	// answer, and appending to it would be a document in no format at all.
	root := writeFixtureProject(t)
	path := filepath.Join(t.TempDir(), "architecture.json")

	if err := fluentapi.ProjectGraph(fixtureLocator(t, root)).Titled("yesterday").ExportAsJSON(path); err != nil {
		t.Fatalf("the first ExportAsJSON failed: %v", err)
	}
	if err := fluentapi.ProjectGraph(fixtureLocator(t, root)).Titled("today").ExportAsJSON(path); err != nil {
		t.Fatalf("the second ExportAsJSON failed: %v", err)
	}

	exported := readExportedReport(t, path)
	if !strings.Contains(exported, `"title": "today"`) {
		t.Errorf("the exported file holds\n%s\nwant the report as it is now", exported)
	}
	if strings.Contains(exported, "yesterday") {
		t.Errorf("the exported file holds\n%s\nwant nothing of the report that was there before", exported)
	}
}

func TestAnExportWithNowhereToWriteToNamesTheTerminalAtFault(t *testing.T) {
	// There is nothing an empty path could have meant — the working directory is a folder, not a file — so this is
	// a call that cannot be run rather than a disk that refused, and the error says which of the six to go and fix.
	report := fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t)))

	tests := []struct {
		terminal string
		exported func(string) error
	}{
		{terminal: "export as dot", exported: report.ExportAsDot},
		{terminal: "export as mermaid", exported: report.ExportAsMermaid},
		{terminal: "export as d2", exported: report.ExportAsD2},
		{terminal: "export as csv", exported: report.ExportAsCSV},
		{terminal: "export as json", exported: report.ExportAsJSON},
		{terminal: "export as html", exported: report.ExportAsHTML},
	}

	for _, test := range tests {
		t.Run(test.terminal, func(t *testing.T) {
			err := test.exported("")

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.terminal {
				t.Errorf("UserError.Operation = %q, want the terminal %q", user.Operation, test.terminal)
			}
			if !errors.Is(err, fluentapi.ErrMissingExportPath) {
				t.Errorf("error = %v, want it to wrap ErrMissingExportPath", err)
			}
		})
	}
}

func TestAnExportOfAReportThatCannotBeRenderedWritesNoFileAtAll(t *testing.T) {
	// The order inside the terminal, made visible: the whole document is rendered before anything touches the disk.
	// A half-written diagram is the worse failure, because it looks like a report of a project with two files in it.
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
			path := filepath.Join(t.TempDir(), "never-written", "architecture.dot")

			if err := test.report.ExportAsDot(path); !errors.Is(err, test.want) {
				t.Fatalf("ExportAsDot error = %v, want it to wrap %v", err, test.want)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("os.Stat(%q) error = %v, want the file never to have been created", path, err)
			}
			if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the folder of the report was created for a report that was never rendered")
			}
		})
	}
}

func TestAnExportThatTheFileSystemRefusesIsATechnicalError(t *testing.T) {
	// The library and the query are both fine here and the code has not been judged: a path whose parent is a file
	// is the environment refusing, which is the other of the two error types and names the path it failed on.
	blocking := filepath.Join(t.TempDir(), "architecture.dot")
	if err := os.WriteFile(blocking, []byte("in the way\n"), 0o600); err != nil {
		t.Fatalf("writing the file in the way failed: %v", err)
	}
	report := fluentapi.ProjectGraph(fixtureLocator(t, writeFixtureProject(t)))

	err := report.ExportAsDot(filepath.Join(blocking, "architecture.dot"))

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExportAsDot error = %v, want a *archerror.TechnicalError", err)
	}
	if !strings.Contains(technical.Subject, "architecture.dot") {
		t.Errorf("TechnicalError.Subject = %q, want the path it failed on", technical.Subject)
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
