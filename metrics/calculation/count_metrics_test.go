package calculation_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

func TestEachMetricAboutAFileReadsItsOwnCount(t *testing.T) {
	// The six counts about a file, each read off the same hand-built description: which number a metric is,
	// is the whole of what distinguishes it.
	subjects := projection.Subjects{Files: []extraction.FileInfo{fixtureFile()}}

	tests := []struct {
		metric calculation.CountMetric
		want   int
	}{
		{metric: calculation.LinesOfCode(), want: 40},
		{metric: calculation.Statements(), want: 12},
		{metric: calculation.Imports(), want: 3},
		{metric: calculation.Functions(), want: 2},
		{metric: calculation.Classes(), want: 3},
		{metric: calculation.Interfaces(), want: 1},
	}

	for _, test := range tests {
		t.Run(test.metric.Name(), func(t *testing.T) {
			measurements := test.metric.Measure(subjects)

			if len(measurements) != 1 {
				t.Fatalf("%s produced %+v, want one measurement per selected file", test.metric, measurements)
			}
			if measurements[0].Value != test.want {
				t.Errorf("%s = %d, want %d", test.metric, measurements[0].Value, test.want)
			}
			if measurements[0].Subject != "internal/api/handler.go" {
				t.Errorf("%s was reported about %q, want the file identifier", test.metric, measurements[0].Subject)
			}
			if measurements[0].Metric != test.metric.Name() {
				t.Errorf("measurement names the metric %q, want %q", measurements[0].Metric, test.metric.Name())
			}
		})
	}
}

func TestEachMetricAboutAClassReadsItsOwnCount(t *testing.T) {
	// The two counts about a class, reported per class identifier rather than per file, because that is the
	// subject a rule about a class means.
	subjects := projection.Subjects{Classes: fixtureFile().Classes}

	tests := []struct {
		metric calculation.CountMetric
		want   []int
	}{
		{metric: calculation.MethodCount(), want: []int{4, 1, 0}},
		{metric: calculation.FieldCount(), want: []int{2, 0, 0}},
	}

	for _, test := range tests {
		t.Run(test.metric.Name(), func(t *testing.T) {
			measurements := test.metric.Measure(subjects)

			if len(measurements) != len(test.want) {
				t.Fatalf("%s produced %+v, want one measurement per selected class", test.metric, measurements)
			}
			for index, want := range test.want {
				if measurements[index].Value != want {
					t.Errorf("%s of %q = %d, want %d", test.metric, measurements[index].Subject, measurements[index].Value, want)
				}
			}
			if measurements[0].Subject != "internal/api.Handler" {
				t.Errorf("%s was reported about %q, want the class identifier", test.metric, measurements[0].Subject)
			}
		})
	}
}

func TestAMetricAboutAFileIgnoresTheSelectedClasses(t *testing.T) {
	// Both populations are handed over at once so that nothing upstream branches on the kind of metric, and
	// each metric has to take only the one it is about — otherwise a class metric's subjects would leak into
	// a file metric's report.
	subjects := projection.Subjects{
		Files:   []extraction.FileInfo{fixtureFile()},
		Classes: fixtureFile().Classes,
	}

	files := calculation.LinesOfCode().Measure(subjects)
	classes := calculation.MethodCount().Measure(subjects)

	if len(files) != 1 || files[0].Subject != "internal/api/handler.go" {
		t.Errorf("lines of code produced %+v, want one measurement about the file", files)
	}
	if len(classes) != 3 {
		t.Errorf("method count produced %+v, want one measurement per class", classes)
	}
}

func TestAMetricMeasuresItsSubjectsInTheOrderTheyWereSelected(t *testing.T) {
	// The order of the report is the order of the selection, so a rule prints the same list twice.
	subjects := projection.Subjects{Files: []extraction.FileInfo{
		{Path: "main.go", LinesOfCode: 3},
		{Path: "internal/api/handler.go", LinesOfCode: 40},
	}}

	measurements := calculation.LinesOfCode().Measure(subjects)

	if len(measurements) != 2 || measurements[0].Subject != "main.go" || measurements[1].Subject != "internal/api/handler.go" {
		t.Errorf("measured %+v, want the order the files were selected in", measurements)
	}
}

func TestAMetricOfNoSubjectsMeasuresNothing(t *testing.T) {
	// A rule whose scope selected nothing is the empty-test guard's business, asked where the rule is
	// judged, so measuring nothing is an ordinary answer here rather than an error.
	for _, metric := range everyCountMetric() {
		if measurements := metric.Measure(projection.Subjects{}); len(measurements) != 0 {
			t.Errorf("%s of no subjects = %+v, want nothing measured", metric, measurements)
		}
	}
}

func TestTheZeroMetricMeasuresNothing(t *testing.T) {
	// A metric that was never built reads neither population, so it cannot quietly measure the wrong one.
	subjects := projection.Subjects{
		Files:   []extraction.FileInfo{fixtureFile()},
		Classes: fixtureFile().Classes,
	}

	if measurements := (calculation.CountMetric{}).Measure(subjects); len(measurements) != 0 {
		t.Errorf("the zero CountMetric measured %+v, want nothing", measurements)
	}
}

func TestEveryMetricIsNamedAsTheFamilySpellsIt(t *testing.T) {
	// The eight names are the words the fluent sentence and every report are printed with, so they are part
	// of the surface and not an implementation detail.
	want := []string{
		"lines of code", "statements", "imports", "functions",
		"classes", "interfaces", "method count", "field count",
	}

	metrics := everyCountMetric()
	if len(metrics) != len(want) {
		t.Fatalf("there are %d count metrics, want the %d the issue names", len(metrics), len(want))
	}
	for index, name := range want {
		if metrics[index].Name() != name {
			t.Errorf("metric %d is called %q, want %q", index, metrics[index].Name(), name)
		}
		if metrics[index].String() != name {
			t.Errorf("metric %d renders as %q, want %q", index, metrics[index].String(), name)
		}
	}
	if (calculation.CountMetric{}).Name() != "" {
		t.Errorf("the zero CountMetric is called %q, want no name at all", (calculation.CountMetric{}).Name())
	}
}

// everyCountMetric is the eight, in the order the issue lists the counts about a file and then a class.
func everyCountMetric() []calculation.CountMetric {
	return []calculation.CountMetric{
		calculation.LinesOfCode(),
		calculation.Statements(),
		calculation.Imports(),
		calculation.Functions(),
		calculation.Classes(),
		calculation.Interfaces(),
		calculation.MethodCount(),
		calculation.FieldCount(),
	}
}

// fixtureFile is one read file in the shape metrics/extraction describes one, with a different number for
// every count so that a metric reading the wrong field cannot pass.
func fixtureFile() extraction.FileInfo {
	return extraction.FileInfo{
		Path: "internal/api/handler.go", Directory: "internal/api",
		LinesOfCode: 40, StatementCount: 12, ImportCount: 3, FunctionCount: 2,
		Classes: []extraction.ClassInfo{
			{Name: "Handler", Identifier: "internal/api.Handler", Path: "internal/api/handler.go", FieldCount: 2, MethodCount: 4},
			{Name: "Router", Identifier: "internal/api.Router", Path: "internal/api/handler.go", Interface: true, MethodCount: 1},
			{Name: "ID", Identifier: "internal/api.ID", Path: "internal/api/handler.go"},
		},
	}
}
