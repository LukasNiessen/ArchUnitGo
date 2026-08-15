package fluentapi

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
)

// ErrMissingExportPath says an export was given the empty string for a path. A report has to be written
// somewhere, and the working directory is a folder rather than a file, so there is nothing this could have
// meant.
//
// It is reported as an archerror.UserError naming the terminal at fault, the way a scope verb's rejected pattern
// is: the library is working and the code has not been judged, the call simply cannot be run as written.
var ErrMissingExportPath = errors.New("no path to export to")

// htmlTerminal is the word the three exports name themselves with when they reject a call, stated once for the
// reason countGroup is: a terminal that spelled its own name in two places could disagree with itself, and the
// name is what tells a reader which of the three calls to go and look at.
const htmlTerminal = "export as html"

const (
	// exportedFilePermissions are the mode an exported report is created with: readable by anyone, writable by
	// its owner. A report is an artifact meant to be opened, attached to a build's output or committed beside
	// the code, so the default of a file nobody but its owner can read would be the surprise.
	exportedFilePermissions = 0o644
	// exportedFolderPermissions are the mode a folder created for a report is given, which is the file's mode
	// plus the traversal bit a folder is useless without.
	exportedFolderPermissions = 0o755
)

// MetricsExporter writes a metrics report to a file: the numbers somebody has already measured, rendered as one
// self-contained HTML page and put where they asked for it.
//
//	exporter := archunit.NewMetricsExporter(&archunit.MetricsReportOptions{Title: "the numbers of this project"})
//	err := exporter.ExportAsHTML(archunit.MetricsReportData{"lines of code": measurements}, "build/metrics.html")
//
// It is the one stage of this module that is not a stage of a rule, and that is what it is for. Every other
// terminal here describes what to measure and then measures it, so the numbers it reports are the numbers of one
// scope read one way; an exporter takes the data as an argument, so a caller can group measurements from as many
// rules as they like, add numbers of their own, or export what they read from Measure last week — grouped however
// they mean them to be compared. MetricsCountBuilder.ExportAsHTML and MetricsDistanceBuilder.ExportAsHTML are the
// convenience for the common case, and they are written over this.
//
// A MetricsExporter is immutable and reads no clock: what a page says about itself — its title, the timestamp on
// it, the stylesheet beside the library's own — is the rendering.ReportOptions it was built with, and the same
// exporter given the same data writes the same bytes. Reuse one for every report of a suite.
//
// The zero value writes an untitled, unstamped, plainly styled page, which is what NewMetricsExporter(nil)
// returns.
type MetricsExporter struct {
	// options are what the exported page says about itself, already resolved, so the "nil means defaults"
	// contract is honored once here rather than at each export.
	options rendering.ReportOptions
}

// NewMetricsExporter builds an exporter for reports described by these options: what the page is called, the
// timestamp it carries, and the stylesheet added to the library's own. A nil *MetricsReportOptions means the
// defaults — an untitled, unstamped, plainly styled page.
//
//	plain := archunit.NewMetricsExporter(nil)
//	stamped := archunit.NewMetricsExporter(&archunit.MetricsReportOptions{Timestamp: time.Now()})
//
// The timestamp is the caller's own, because this library reads no clock: a page that stamped itself would
// render different bytes on every run, so a report committed beside the code would show up in every diff. A
// caller who wants the stamp passes time.Now() and owns that decision.
//
// The options are copied rather than kept, so a caller may reuse one struct to build an exporter per report and
// each exporter still means the options it was built with. Nothing is read or written here: ExportAsHTML is what
// touches the disk.
func NewMetricsExporter(options *rendering.ReportOptions) MetricsExporter {
	return MetricsExporter{options: options.WithDefaults()}
}

// ExportAsHTML writes these measurements to this path as one self-contained HTML page: `export as html`.
//
//	err := archunit.NewMetricsExporter(nil).
//		ExportAsHTML(archunit.MetricsReportData{"lines of code": measurements}, "build/metrics.html")
//
// The data is the report: each key is a heading of the page and the measurements under it are the group listed
// there, which for a report written off a rule is one group per metric. What the page then looks like is
// rendering.RenderHTML's to describe, and one file is the whole of it — nothing is fetched when it is opened,
// which is what makes it the report to attach to a build's output.
//
// The path is interpreted like any other path a test writes to, relative to the working directory the test runs
// in, and its folders are created if they are not there yet: a report exported into `build/` should not fail
// because nobody had made `build/` yet. An existing file is overwritten, because a report is the current answer
// about the project and a stale one beside it would be read as a second answer.
//
// A report with no measurement in it is written rather than refused. An exporter selected nothing, so it cannot
// tell a project with nothing to report from a pattern that has gone stale — the empty-test guard is asked by
// the terminals that resolved a scope, which is where ErrEmptyReport comes from.
//
// The error is ErrMissingExportPath for an empty path, or a technical error naming the file when the disk
// refused it. Nothing is written unless the whole page was rendered, so a failure leaves no half-written report
// behind for the next reader to trust.
func (e MetricsExporter) ExportAsHTML(data rendering.ReportData, path string) error {
	if path == "" {
		return archerror.NewUserError(htmlTerminal, path, ErrMissingExportPath)
	}
	return written(path, rendering.RenderHTML(data, &e.options))
}

// written is what an export is once the page exists: make sure the folder it is going into is there, write the
// file.
//
// The page is rendered before this is called and never while it is being written, which is the order that
// matters: a failure leaves no half-written report behind, where writing as the document was produced would
// leave a page that looks like a report of a project with three files in it.
func written(path, document string) error {
	if err := os.MkdirAll(filepath.Dir(path), exportedFolderPermissions); err != nil {
		return archerror.NewTechnicalError("create the folder of the report", path, err)
	}
	if err := os.WriteFile(path, []byte(document), exportedFilePermissions); err != nil {
		return archerror.NewTechnicalError("write the report", path, err)
	}
	return nil
}
