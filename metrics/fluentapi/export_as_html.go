package fluentapi

import (
	"errors"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
)

// ErrEmptyReport says the scope described a report with no number in it, which is this module's shape of the
// failure assertion.EmptyTestViolation exists for: a pattern that names nothing exports a page of empty tables,
// and a page of empty tables looks exactly like a project that has nothing to report.
//
// It is an error rather than a violation because a report has no violation list to put one in — the terminal
// writes an artifact and judges nothing — and it is reported at all because the alternative is a stale glob
// quietly producing a blank report for a year. Set AllowEmptyTests on the check options to permit it, the same
// knob that opts a rule out of the same guard.
var ErrEmptyReport = errors.New("no measurement in the report")

// ExportAsHTML measures every count of this group over the scope and writes them to this path as one
// self-contained HTML page: `export as html`. A nil *CheckOptions means the defaults.
//
//	err := archunit.Metrics(nil).InFolder("internal/**").Count().ExportAsHTML("build/counts.html", nil)
//
// It is the group rather than a metric that closes this way, and that is the point of it: a report is read to see
// how a project is shaped, and which of the eight counts that takes is not a question somebody has already
// answered when they ask for the page. So all eight are measured over the one scope and the page holds a group
// per count — the same numbers LinesOfCode().Measure(nil) and its seven siblings would have handed back, in one
// document, with the project read once.
//
// The page is titled with the rule's own sentence — `metrics, path without filename matches "internal/**", count`
// — so that a report found in a build's output says which scope produced it. MetricsExporter is the way to a
// title of the caller's own, to a timestamp on the page and to a stylesheet beside the library's: this terminal
// is the shorthand for the common case, and it is written over that exporter.
//
// The path behaves as MetricsExporter.ExportAsHTML describes: relative to the working directory, folders created
// if they are not there yet, an existing report overwritten.
//
// The error is the scope's — a pattern a scope verb rejected, a locator naming no Go project, a project that will
// not load — or ErrEmptyReport when the scope selected nothing to measure, ErrMissingExportPath for an empty
// path, or a technical error naming the file when the disk refused it. It is never a rule failure: a report
// judges no code, so it has no violation to report.
func (b MetricsCountBuilder) ExportAsHTML(path string, options *kernel.CheckOptions) error {
	return b.scope.exportedAsHTML(path, b.String(), b.reported(), options)
}

// reported are the metrics this group's report holds: all eight counts, each named by the verb that names it, so
// that a count added to the group is in the report the day the verb lands and no metric is enumerated twice.
func (b MetricsCountBuilder) reported() []MetricBuilder {
	return []MetricBuilder{
		b.LinesOfCode(),
		b.Statements(),
		b.Imports(),
		b.Functions(),
		b.Classes(),
		b.Interfaces(),
		b.MethodCount(),
		b.FieldCount(),
	}
}

// ExportAsHTML measures every number of this group over the scope and writes them to this path as one
// self-contained HTML page: `export as html`. A nil *CheckOptions means the defaults.
//
//	err := archunit.Metrics(nil).InFolder("internal/**").Distance().ExportAsHTML("build/distance.html", nil)
//
// It is MetricsCountBuilder.ExportAsHTML's twin over the five metrics about a package — abstractness,
// instability, the two distances from the main sequence and the coupling factor — and it behaves exactly as that
// terminal does about the title, the path, the folders and the errors.
//
// All five in one page is what makes this report worth reading: abstractness and instability are the two axes a
// package's distance from the main sequence is computed from, so a package the page reports as far off the line
// is a package the same page says why about. The two zone checks are the rules over that plane, and they report
// violations rather than writing a document.
func (b MetricsDistanceBuilder) ExportAsHTML(path string, options *kernel.CheckOptions) error {
	return b.scope.exportedAsHTML(path, b.String(), b.reported(), options)
}

// reported are the metrics this group's report holds: all five, named by their own verbs, for the reason
// MetricsCountBuilder.reported gives.
func (b MetricsDistanceBuilder) reported() []MetricBuilder {
	return []MetricBuilder{
		b.Abstractness(),
		b.Instability(),
		b.DistanceFromMainSequence(),
		b.NormalizedDistance(),
		b.CouplingFactor(),
	}
}

// exportedAsHTML is what a group's `export as html` terminal is: resolve the scope once, read every metric of the
// group off the subjects it came to, and hand the groups to an exporter titled with the rule's own sentence.
//
// One resolution for the whole page, which is the reason this is a terminal of the group rather than a loop a
// user writes over the metric verbs: the project is located, extracted and selected once, and the eight or five
// metrics are read off the same subjects, so a report cannot hold two numbers taken of two different readings of
// the same code.
//
// The order of the steps is the order the failures should be reported in. An empty path is refused before the
// project is read, because that call cannot be run whatever the code says; the empty-test guard is asked after
// the metrics have been read, because whether a scope selected anything is a question only the resolved scope can
// answer; and the file is written last, once the whole page exists.
func (b MetricsBuilder) exportedAsHTML(path, title string, metrics []MetricBuilder, options *kernel.CheckOptions) error {
	if path == "" {
		return archerror.NewUserError(htmlTerminal, path, ErrMissingExportPath)
	}
	subjects, err := b.resolve(options)
	if err != nil {
		return err
	}

	data := make(rendering.ReportData, len(metrics))
	for _, metric := range metrics {
		data[metric.metric.Name()] = metric.readings(subjects)
	}
	if data.Empty() && !options.WithDefaults().AllowEmptyTests {
		// A report of nothing is refused instead of being written, for the reason every terminal in this
		// library wires in the empty-test guard: a scope whose patterns have gone stale exports a page of
		// empty tables, and that is the one failure this library refuses to pass off as a pass.
		return archerror.NewUserError(htmlTerminal, title, ErrEmptyReport)
	}
	return NewMetricsExporter(&rendering.ReportOptions{Title: title}).ExportAsHTML(data, path)
}
