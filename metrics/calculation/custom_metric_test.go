package calculation_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

func TestACustomMetricReadsTheUsersOwnNumberOffEveryClass(t *testing.T) {
	// The escape hatch: a number this library has no verb for, reported per class identifier like every other
	// metric about a class, so that everything downstream is unaware it was not the library's own.
	subjects := projection.Subjects{Classes: fixtureFile().Classes}
	surface := calculation.NewCustomMetric("public surface", func(class extraction.ClassInfo) float64 {
		return float64(class.MethodCount + class.FieldCount)
	})

	measurements := surface.Measure(subjects)

	want := []calculation.Measurement{
		{Metric: "public surface", Subject: "internal/api.Handler", Value: 6},
		{Metric: "public surface", Subject: "internal/api.Router", Value: 1},
		{Metric: "public surface", Subject: "internal/api.ID", Value: 0},
	}
	if len(measurements) != len(want) {
		t.Fatalf("%s produced %+v, want one measurement per selected class", surface, measurements)
	}
	for index, wanted := range want {
		if measurements[index] != wanted {
			t.Errorf("measurement %d is %+v, want %+v", index, measurements[index], wanted)
		}
	}
}

func TestACustomMetricIsHandedTheWholeClass(t *testing.T) {
	// The point of the escape hatch is what the function may read: everything this library extracted about the
	// class, down to which of its fields each of its methods reaches.
	class := extraction.ClassInfo{
		Name: "Handler", Identifier: "internal/api.Handler", Path: "internal/api/handler.go",
		FieldCount: 2, MethodCount: 2,
		Fields: []extraction.FieldInfo{
			{Name: "name", AccessedBy: []string{"Handle"}},
			{Name: "log"},
		},
		Methods: []extraction.MethodInfo{
			{Name: "Handle", AccessedFields: []string{"name"}},
			{Name: "Close"},
		},
	}
	var seen extraction.ClassInfo
	touched := calculation.NewCustomMetric("touched fields", func(class extraction.ClassInfo) float64 {
		seen = class
		reached := 0
		for _, field := range class.Fields {
			reached += len(field.AccessedBy)
		}
		return float64(reached)
	})

	measurements := touched.Measure(projection.Subjects{Classes: []extraction.ClassInfo{class}})

	if len(measurements) != 1 || measurements[0].Value != 1 {
		t.Fatalf("touched fields produced %+v, want the one number the function computed", measurements)
	}
	if seen.Name != class.Name || seen.Path != class.Path || len(seen.Methods) != 2 {
		t.Errorf("the function was handed %+v, want the class as it was extracted", seen)
	}
}

func TestACustomMetricIgnoresThePopulationsItIsNotAbout(t *testing.T) {
	// Both populations arrive at once so that nothing upstream branches on the kind of metric, and this one is
	// about a class: a custom metric over a scope that selected files but no class measures nothing.
	files := projection.Subjects{Files: []extraction.FileInfo{{Path: "main.go", LinesOfCode: 3}}}
	always := calculation.NewCustomMetric("always one", func(extraction.ClassInfo) float64 { return 1 })

	if measurements := always.Measure(files); len(measurements) != 0 {
		t.Errorf("a custom metric over files alone measured %+v, want nothing", measurements)
	}
	if measurements := always.Measure(projection.Subjects{}); len(measurements) != 0 {
		t.Errorf("a custom metric of no subjects measured %+v, want nothing", measurements)
	}
}

func TestACustomMetricWithoutAFunctionMeasuresNothing(t *testing.T) {
	// Calling a nil function would take the host test process down. A rule the fluent API built never has one
	// — a missing function is returned as the user's error — so nothing here has to fail.
	subjects := projection.Subjects{Classes: fixtureFile().Classes}

	if measurements := calculation.NewCustomMetric("nameless", nil).Measure(subjects); len(measurements) != 0 {
		t.Errorf("a custom metric with no function measured %+v, want nothing", measurements)
	}
	if measurements := (calculation.CustomMetric{}).Measure(subjects); len(measurements) != 0 {
		t.Errorf("the zero CustomMetric measured %+v, want nothing", measurements)
	}
}

func TestACustomMetricIsCalledWhatTheUserCalledIt(t *testing.T) {
	// The name is the word a measurement is reported under, the word a violation quotes and the word the
	// fluent sentence renders, so it is the user's and never rephrased.
	metric := calculation.NewCustomMetric("branch count", func(extraction.ClassInfo) float64 { return 0 })

	if metric.Name() != "branch count" {
		t.Errorf("the metric is called %q, want %q", metric.Name(), "branch count")
	}
	if metric.String() != "branch count" {
		t.Errorf("the metric renders as %q, want %q", metric.String(), "branch count")
	}
	if (calculation.CustomMetric{}).Name() != "" {
		t.Errorf("the zero CustomMetric is called %q, want no name at all", (calculation.CustomMetric{}).Name())
	}
}

func TestACustomMetricIsAMetricLikeTheRest(t *testing.T) {
	// The whole reason the escape hatch costs the rest of the library nothing: the fluent layer holds a
	// calculation.Metric, and a number a user defined is one.
	var metric calculation.Metric = calculation.NewCustomMetric("public surface", func(extraction.ClassInfo) float64 {
		return 1
	})

	if measurements := metric.Measure(projection.Subjects{Classes: fixtureFile().Classes}); len(measurements) != 3 {
		t.Errorf("read through the interface it measured %+v, want one measurement per class", measurements)
	}
}
