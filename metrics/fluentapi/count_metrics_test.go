package fluentapi_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

func TestEachCountVerbNamesOneOfTheEightCounts(t *testing.T) {
	// The eight verbs the count group holds, each measured over the whole fixture project: which number a
	// verb is, is the only thing that distinguishes it, so every count of the fixture differs from the rest.
	scope := fluentapi.Metrics(measuredProject(t))
	files := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go", "main.go"}
	classes := []string{"internal/api.Handler", "internal/api.Router", "internal/db.Connection"}

	tests := []struct {
		name         string
		rule         fluentapi.MetricBuilder
		wantSubjects []string
		wantValues   []float64
	}{
		{name: "lines of code", rule: scope.Count().LinesOfCode(), wantSubjects: files, wantValues: []float64{9, 4, 3, 3}},
		{name: "statements", rule: scope.Count().Statements(), wantSubjects: files, wantValues: []float64{3, 0, 0, 1}},
		{name: "imports", rule: scope.Count().Imports(), wantSubjects: files, wantValues: []float64{1, 0, 0, 1}},
		{name: "functions", rule: scope.Count().Functions(), wantSubjects: files, wantValues: []float64{1, 0, 1, 1}},
		{name: "classes", rule: scope.Count().Classes(), wantSubjects: files, wantValues: []float64{1, 1, 1, 0}},
		{name: "interfaces", rule: scope.Count().Interfaces(), wantSubjects: files, wantValues: []float64{0, 1, 0, 0}},
		{name: "method count", rule: scope.Count().MethodCount(), wantSubjects: classes, wantValues: []float64{2, 1, 0}},
		{name: "field count", rule: scope.Count().FieldCount(), wantSubjects: classes, wantValues: []float64{2, 0, 0}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			measurements := measure(t, test.rule, nil)

			if got := subjectsOf(measurements); !slices.Equal(got, test.wantSubjects) {
				t.Errorf("`%s` measures %v, want %v", test.name, got, test.wantSubjects)
			}
			if got := valuesOf(measurements); !slices.Equal(got, test.wantValues) {
				t.Errorf("`%s` = %v, want %v", test.name, got, test.wantValues)
			}
			for _, measurement := range measurements {
				if measurement.Metric != test.name {
					t.Errorf("a measurement names the metric %q, want %q", measurement.Metric, test.name)
				}
			}
		})
	}
}

func TestTheSixCountsAboutAFileAndTheTwoAboutAClassReportDifferentSubjects(t *testing.T) {
	// What a metric is about decides what its measurements are named by, and that is the whole reason both
	// populations are carried through one scope: a file identifier for the six, a class identifier for the two.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/api")

	perFile := measure(t, scope.Count().Classes(), nil)
	perClass := measure(t, scope.Count().MethodCount(), nil)

	if got := subjectsOf(perFile); !slices.Equal(got, []string{"internal/api/handler.go", "internal/api/router.go"}) {
		t.Errorf("`classes` measures %v, want the files of the folder", got)
	}
	if got := subjectsOf(perClass); !slices.Equal(got, []string{"internal/api.Handler", "internal/api.Router"}) {
		t.Errorf("`method count` measures %v, want the classes those files declare", got)
	}
}

func TestCountOpensTheGroupWithoutReadingAnything(t *testing.T) {
	// `count` is the word that says which kind of number the rule means, and nothing more: it selects nothing,
	// so a scope and the same scope with `count` chained onto it describe the same set of subjects.
	scope := fluentapi.Metrics(nil).InFolder("internal/**")

	group := scope.Count()

	if selectors := scope.Selectors(); len(selectors) != 1 {
		t.Errorf("the scope's Selectors() = %v after Count(), want the one verb it was built with", selectors)
	}
	if rendered := group.String(); rendered != `metrics, path without filename matches "internal/**", count` {
		t.Errorf("String() = %q, want the scope with `count` appended", rendered)
	}
	if entry := fluentapi.Metrics(nil).Count().String(); entry != "metrics, count" {
		t.Errorf("String() = %q, want `metrics, count`", entry)
	}
}

func TestCountRendersTheRejectedPatternOfTheScopeItWasAskedOf(t *testing.T) {
	// A rejected pattern narrowed nothing, so a stage that hid it would render as the rule the user thought
	// they wrote — and this stage is what a failure prints before a metric has been chosen.
	group := fluentapi.Metrics(nil).InFolder("[unclosed").Count()

	rendered := group.String()

	if want := "metrics, count (rejected: "; len(rendered) < len(want) || rendered[:len(want)] != want {
		t.Errorf("String() = %q, want it to start with %q", rendered, want)
	}
}

func TestTheCountGroupCanBeStoredAndBranchedFrom(t *testing.T) {
	// The stage is a value like every other, so one scope's `count` can be asked for two numbers.
	group := fluentapi.Metrics(measuredProject(t)).InFolder("internal/db").Count()

	lines := measure(t, group.LinesOfCode(), nil)
	functions := measure(t, group.Functions(), nil)

	if got := valuesOf(lines); !slices.Equal(got, []float64{3}) {
		t.Errorf("lines of code = %v, want the 3 the fixture was written with", got)
	}
	if got := valuesOf(functions); !slices.Equal(got, []float64{1}) {
		t.Errorf("functions = %v, want the 1 the fixture was written with", got)
	}
}
