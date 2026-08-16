package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
)

func TestEachGroupsReportHoldsEveryMetricTheGroupNames(t *testing.T) {
	// The reason the terminal is the group's rather than a metric's: which of the eight counts a page should show
	// is not a question somebody has already answered when they ask for the page. The verbs are enumerated by
	// reflection, so a metric added to a group and left out of its report is a failing test rather than a page
	// somebody notices is missing a column.
	scope := fluentapi.Metrics(measuredProject(t))
	folder := t.TempDir()

	tests := []struct {
		group   string
		metrics []fluentapi.MetricBuilder
		export  func(string, *kernel.CheckOptions) error
	}{
		{group: "count", metrics: metricVerbsOf(t, scope.Count()), export: scope.Count().ExportAsHTML},
		{group: "distance", metrics: metricVerbsOf(t, scope.Distance()), export: scope.Distance().ExportAsHTML},
	}

	for _, test := range tests {
		t.Run(test.group, func(t *testing.T) {
			path := filepath.Join(folder, test.group+".html")

			if err := test.export(path, nil); err != nil {
				t.Fatalf("`%s, export as html` failed: %v", test.group, err)
			}

			exported := readExportedReport(t, path)
			for _, metric := range test.metrics {
				if heading := "<h2>" + measuredMetricOf(metric) + "</h2>"; !strings.Contains(exported, heading) {
					t.Errorf("the page does not hold the group %q:\n%s", heading, exported)
				}
			}
			if headings := strings.Count(exported, "<h2>"); headings != len(test.metrics) {
				t.Errorf("the page holds %d groups, want the %d verbs of `%s`", headings, len(test.metrics), test.group)
			}
		})
	}
}

func TestAGroupsReportHoldsTheNumbersItsMetricsWouldHaveMeasured(t *testing.T) {
	// The promise that makes the terminal a shorthand rather than a second way of measuring: the page holds the
	// numbers Measure hands back, one group per metric, with the project read once for all of them.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/**")
	path := filepath.Join(t.TempDir(), "counts.html")

	if err := scope.Count().ExportAsHTML(path, nil); err != nil {
		t.Fatalf("`count, export as html` failed: %v", err)
	}

	data := rendering.ReportData{}
	for _, metric := range metricVerbsOf(t, scope.Count()) {
		data[measuredMetricOf(metric)] = measure(t, metric, nil)
	}
	want := rendering.RenderHTML(data, &rendering.ReportOptions{Title: scope.Count().String()})
	if got := readExportedReport(t, path); got != want {
		t.Errorf("the exported page holds\n%s\nwant the measured numbers rendered\n%s", got, want)
	}
}

func TestAGroupsReportIsTitledWithTheRuleItWasExportedFrom(t *testing.T) {
	// A report found in a build's output has to say which scope produced it, and the rule already renders as the
	// sentence the user typed. MetricsExporter is the way to a title of the caller's own.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/api")
	folder := t.TempDir()

	tests := map[string]struct {
		export func(string, *kernel.CheckOptions) error
		want   string
	}{
		"count":    {export: scope.Count().ExportAsHTML, want: scope.Count().String()},
		"distance": {export: scope.Distance().ExportAsHTML, want: scope.Distance().String()},
	}

	for group, test := range tests {
		path := filepath.Join(folder, group+".html")
		if err := test.export(path, nil); err != nil {
			t.Fatalf("`%s, export as html` failed: %v", group, err)
		}

		exported := readExportedReport(t, path)
		if headline := "<h1>" + escapedForHTML(test.want) + "</h1>"; !strings.Contains(exported, headline) {
			t.Errorf("the page does not hold the headline %q:\n%s", headline, exported)
		}
	}
}

func TestAGroupsReportCreatesTheFoldersOfThePathItWasGiven(t *testing.T) {
	scope := fluentapi.Metrics(measuredProject(t))
	path := filepath.Join(t.TempDir(), "build", "reports", "counts.html")

	if err := scope.Count().ExportAsHTML(path, nil); err != nil {
		t.Fatalf("`count, export as html` failed: %v", err)
	}
	if readExportedReport(t, path) == "" {
		t.Error("the exported file is empty, want the report in it")
	}
}

func TestTheCheckOptionsReachTheProjectAGroupsReportIsMeasuredFrom(t *testing.T) {
	// The same options every other terminal of the family takes, passed at the terminal as Measure and Check take
	// theirs — so a report can be taken of the test files as well.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/api")
	folder := t.TempDir()

	byDefault := filepath.Join(folder, "production.html")
	if err := scope.Count().ExportAsHTML(byDefault, nil); err != nil {
		t.Fatalf("`count, export as html` failed: %v", err)
	}
	withTests := filepath.Join(folder, "tests.html")
	if err := scope.Count().ExportAsHTML(withTests, &kernel.CheckOptions{IncludeTestFiles: true}); err != nil {
		t.Fatalf("`count, export as html` with IncludeTestFiles failed: %v", err)
	}

	if exported := readExportedReport(t, byDefault); strings.Contains(exported, "handler_test.go") {
		t.Errorf("the page reports the test file by default:\n%s", exported)
	}
	if exported := readExportedReport(t, withTests); !strings.Contains(exported, "handler_test.go") {
		t.Errorf("the page with IncludeTestFiles does not report the test file:\n%s", exported)
	}
}

func TestAGroupsReportOfNothingIsRefusedRatherThanWritten(t *testing.T) {
	// The empty-test guard, in the shape a terminal that writes an artifact can report it: a scope whose patterns
	// have gone stale exports a page of empty tables, which looks exactly like a project with nothing to report.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/transport/**")
	folder := t.TempDir()

	tests := map[string]func(string, *kernel.CheckOptions) error{
		"count":    scope.Count().ExportAsHTML,
		"distance": scope.Distance().ExportAsHTML,
	}

	for group, export := range tests {
		path := filepath.Join(folder, group, "report.html")

		err := export(path, nil)

		var user *archerror.UserError
		if !errors.As(err, &user) {
			t.Fatalf("`%s, export as html` error = %v, want a *archerror.UserError", group, err)
		}
		if !errors.Is(err, fluentapi.ErrEmptyReport) {
			t.Errorf("`%s, export as html` error = %v, want it to wrap ErrEmptyReport", group, err)
		}
		if user.Subject != scope.String()+", "+group {
			t.Errorf("UserError.Subject = %q, want the rule that reported nothing", user.Subject)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("os.Stat(%q) error = %v, want the file never to have been created", path, err)
		}
	}
}

func TestAGroupsReportOfNothingIsWrittenWhenTheUserOptedOutOfTheGuard(t *testing.T) {
	// The one knob that changes what a terminal reports, threaded in here as it is in every other terminal: a
	// caller who means an empty report says so, and gets a page saying the groups hold nothing.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/transport/**")
	path := filepath.Join(t.TempDir(), "counts.html")

	if err := scope.Count().ExportAsHTML(path, &kernel.CheckOptions{AllowEmptyTests: true}); err != nil {
		t.Fatalf("`count, export as html` with AllowEmptyTests failed: %v", err)
	}

	exported := readExportedReport(t, path)
	if !strings.Contains(exported, "no measurement in this group") {
		t.Errorf("the page does not say its groups hold nothing:\n%s", exported)
	}
	if !strings.Contains(exported, "<h2>lines of code</h2>") {
		t.Errorf("the page drops the groups nothing was measured for:\n%s", exported)
	}
}

func TestAGroupsExportWithNowhereToWriteToNamesTheTerminalAtFault(t *testing.T) {
	// An empty path is a call that cannot be run whatever the code says, so it is refused before the project is
	// read — which is why the locator here names no project at all and the error is still about the path.
	scope := fluentapi.Metrics(&extraction.ProjectLocator{Directory: t.TempDir()})

	tests := map[string]func(string, *kernel.CheckOptions) error{
		"count":    scope.Count().ExportAsHTML,
		"distance": scope.Distance().ExportAsHTML,
	}

	for group, export := range tests {
		err := export("", nil)

		var user *archerror.UserError
		if !errors.As(err, &user) {
			t.Fatalf("`%s, export as html` error = %v, want a *archerror.UserError", group, err)
		}
		if user.Operation != "export as html" {
			t.Errorf("UserError.Operation = %q, want the terminal `export as html`", user.Operation)
		}
		if !errors.Is(err, fluentapi.ErrMissingExportPath) {
			t.Errorf("`%s, export as html` error = %v, want it to wrap ErrMissingExportPath", group, err)
		}
		if errors.Is(err, extraction.ErrModuleFileNotFound) {
			t.Errorf("`%s, export as html` error = %v, want the project left unread", group, err)
		}
	}
}

func TestAGroupsExportThatCannotBeMeasuredWritesNoFileAtAll(t *testing.T) {
	// The order inside the terminal, made visible: the whole page is rendered before anything touches the disk, so
	// a failure leaves no half-written report behind for the next reader to trust.
	tests := []struct {
		failure string
		scope   fluentapi.MetricsBuilder
		want    error
	}{
		{
			failure: "a pattern that will not compile",
			scope:   fluentapi.Metrics(nil).InFolder("[unclosed"),
			want:    matching.ErrInvalidPattern,
		},
		{
			failure: "a locator naming no project",
			scope:   fluentapi.Metrics(&extraction.ProjectLocator{Directory: t.TempDir()}),
			want:    extraction.ErrModuleFileNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.failure, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "never-written", "counts.html")

			if err := test.scope.Count().ExportAsHTML(path, nil); !errors.Is(err, test.want) {
				t.Fatalf("`count, export as html` error = %v, want it to wrap %v", err, test.want)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("os.Stat(%q) error = %v, want the file never to have been created", path, err)
			}
			if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
				t.Error("the folder of the report was created for a report that was never rendered")
			}
		})
	}
}

// metricVerbsOf are the metric verbs a group holds, as the group itself declares them: every exported method that
// takes nothing and hands back a MetricBuilder.
//
// Reflection rather than a list written out here, because a list would be the very thing under test — the report
// has to hold every metric of the group, and a second enumeration could agree with the code and disagree with the
// group. The zone checks are not among them: they hand back a rule rather than a metric.
func metricVerbsOf(t *testing.T, group any) []fluentapi.MetricBuilder {
	t.Helper()

	value := reflect.ValueOf(group)
	measured := reflect.TypeOf(fluentapi.MetricBuilder{})
	metrics := make([]fluentapi.MetricBuilder, 0, value.NumMethod())
	for index := range value.NumMethod() {
		method := value.Method(index)
		if method.Type().NumIn() != 0 || method.Type().NumOut() != 1 || method.Type().Out(0) != measured {
			continue
		}
		metric, ok := method.Call(nil)[0].Interface().(fluentapi.MetricBuilder)
		if !ok {
			t.Fatalf("the method %d of %T does not hand back a MetricBuilder", index, group)
		}
		metrics = append(metrics, metric)
	}
	if len(metrics) == 0 {
		t.Fatalf("%T holds no metric verb, want the group's own", group)
	}
	return metrics
}

// measuredMetricOf is the name of the metric a rule ends with, which is the heading its group is reported under:
// the last stage of the sentence the rule renders as.
func measuredMetricOf(metric fluentapi.MetricBuilder) string {
	stages := strings.Split(metric.String(), ", ")
	return stages[len(stages)-1]
}

// escapedForHTML is a piece of the page's text as the page holds it: the characters that would otherwise be
// markup, written as entities. It is the assertion's own escaping rather than the renderer's, so that a test
// proving the page is titled with the rule does not ask the code under test what escaping means.
func escapedForHTML(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(text)
}
