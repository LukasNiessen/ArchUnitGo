package fluentapi_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

func TestACustomMetricMeasuresTheUsersOwnNumberOffEveryClass(t *testing.T) {
	// The escape hatch through the public chain: a number this library has no verb for, measured the way every
	// class metric is — one per class the scope selected, in the order they were selected.
	rule := fluentapi.Metrics(measuredProject(t)).
		CustomMetric("public surface", "how many methods and fields a type exposes",
			func(class extraction.ClassInfo) float64 { return float64(class.MethodCount + class.FieldCount) })

	measurements := measure(t, rule, nil)

	wantSubjects := []string{"internal/api.Handler", "internal/api.Router", "internal/db.Connection"}
	if got := subjectsOf(measurements); !slices.Equal(got, wantSubjects) {
		t.Errorf("the rule measures %v, want %v", got, wantSubjects)
	}
	if got := valuesOf(measurements); !slices.Equal(got, []float64{4, 1, 0}) {
		t.Errorf("public surface = %v, want the numbers the fixture was written with", got)
	}
	for _, measurement := range measurements {
		if measurement.Metric != "public surface" {
			t.Errorf("a measurement names the metric %q, want the user's own name", measurement.Metric)
		}
	}
}

func TestACustomMetricIsHandedTheWholeClassOfTheProject(t *testing.T) {
	// What the function may read is the point of the verb, and it is the class as this library extracted it:
	// the fixture's Handler reaches one of its two fields from each of its two methods.
	rule := fluentapi.Metrics(measuredProject(t)).
		ForClassesMatching("Handler").
		CustomMetric("reached fields", "how many of a type's fields its own methods read",
			func(class extraction.ClassInfo) float64 {
				reached := 0
				for _, field := range class.Fields {
					if len(field.AccessedBy) > 0 {
						reached++
					}
				}
				return float64(reached)
			})

	measurements := measure(t, rule, nil)

	if got := subjectsOf(measurements); !slices.Equal(got, []string{"internal/api.Handler"}) {
		t.Errorf("the rule measures %v, want the one class the scope selected", got)
	}
	if got := valuesOf(measurements); !slices.Equal(got, []float64{2}) {
		t.Errorf("reached fields = %v, want the 2 the fixture's methods read", got)
	}
}

func TestACustomMetricIsNarrowedByTheScopeItWasAskedOf(t *testing.T) {
	// A custom metric is a metric and not a second kind of rule, so the four scope verbs mean here exactly what
	// they mean for the counts.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/db")
	always := func(extraction.ClassInfo) float64 { return 1 }

	measurements := measure(t, scope.CustomMetric("one", "the number one", always), nil)

	if got := subjectsOf(measurements); !slices.Equal(got, []string{"internal/db.Connection"}) {
		t.Errorf("the rule measures %v, want the classes of the selected folder", got)
	}
}

func TestACustomMetricOfAScopeWithNoClassMeasuresNothing(t *testing.T) {
	// The verb is about a class, so a scope selecting files that declare none has nothing to measure — and
	// whether that is a failure is the empty-test guard's question, asked where the rule is judged.
	rule := fluentapi.Metrics(measuredProject(t)).
		InFolder(".").
		CustomMetric("one", "the number one", func(extraction.ClassInfo) float64 { return 1 })

	measurements, err := rule.Measure(nil)
	if err != nil {
		t.Fatalf("Measure failed: %v", err)
	}

	if len(measurements) != 0 {
		t.Errorf("Measure produced %+v, want nothing: the root package declares no class", measurements)
	}
}

func TestACustomMetricRendersTheWordsItWasDescribedWith(t *testing.T) {
	// The description is what a log line and the heading of a test failure say the number is, because a closure
	// is not printable and a name the library never defined does not describe itself.
	tests := []struct {
		name string
		rule fluentapi.MetricBuilder
		want string
	}{
		{
			name: "a scope and a custom metric",
			rule: fluentapi.Metrics(nil).ForClassesMatching("*Service").
				CustomMetric("public surface", "how many methods and fields a type exposes", zeroMeasure),
			want: `metrics, classname matches "*Service", custom metric, ` +
				`public surface ("how many methods and fields a type exposes")`,
		},
		{
			name: "no scope verb at all",
			rule: fluentapi.Metrics(nil).CustomMetric("branch count", "how many branches a type's methods take", zeroMeasure),
			want: `metrics, custom metric, branch count ("how many branches a type's methods take")`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.rule.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestACustomMetricWithNothingMissingIsRejectedForTheFirstThingThatIs(t *testing.T) {
	// Three things the library cannot run a rule without, each returned as the user's own error naming the verb
	// — and none of them a rule failure, because a rule that says nothing has not been broken by the code.
	tests := []struct {
		name string
		rule fluentapi.MetricBuilder
		want error
	}{
		{
			name: "no name",
			rule: fluentapi.Metrics(nil).CustomMetric("  ", "how many branches a type's methods take", zeroMeasure),
			want: fluentapi.ErrNoMetricName,
		},
		{
			name: "no description",
			rule: fluentapi.Metrics(nil).CustomMetric("branch count", "\t", zeroMeasure),
			want: fluentapi.ErrNoMetricDescription,
		},
		{
			name: "no function",
			rule: fluentapi.Metrics(nil).CustomMetric("branch count", "how many branches a type's methods take", nil),
			want: fluentapi.ErrNoMeasure,
		},
		{
			// Nothing at all given: the name is the first argument, so it is the one the user is told about, and
			// the two rejections behind it are the ones they would be told about next.
			name: "no name, no description and no function",
			rule: fluentapi.Metrics(nil).CustomMetric("", "", nil),
			want: fluentapi.ErrNoMetricName,
		},
		{
			// The name is there and the two arguments after it are not, so the rejection moves on to the second
			// of the three rather than to the last.
			name: "no description and no function",
			rule: fluentapi.Metrics(nil).CustomMetric("branch count", "", nil),
			want: fluentapi.ErrNoMetricDescription,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			measurements, err := test.rule.Measure(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Measure error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != "custom metric" {
				t.Errorf("UserError.Operation = %q, want the verb `custom metric`", user.Operation)
			}
			if !errors.Is(err, test.want) {
				t.Errorf("Measure error = %v, want it to wrap %v", err, test.want)
			}
			if len(measurements) != 0 {
				t.Errorf("Measure returned %+v beside the error, want nothing", measurements)
			}
			if !strings.Contains(test.rule.String(), "rejected") {
				t.Errorf("%s renders without the rejection, want it visible in a test failure", test.rule)
			}
		})
	}
}

func TestACustomMetricIsRejectedBeforeTheProjectIsRead(t *testing.T) {
	// What the user left out is missing whatever the project turns out to be, and reading it first would answer
	// a missing function with a complaint about the locator.
	rule := fluentapi.Metrics(measuredProject(t)).CustomMetric("branch count", "how many branches", nil)

	_, err := rule.Measure(nil)

	if !errors.Is(err, fluentapi.ErrNoMeasure) {
		t.Errorf("Measure error = %v, want the missing function", err)
	}
}

func TestACustomMetricKeepsTheScopesOwnRejection(t *testing.T) {
	// The first thing the user got wrong is the one they are told about, so a pattern the scope could not
	// compile is not overwritten by the verb chained onto it.
	rule := fluentapi.Metrics(nil).InFolder("[unclosed").CustomMetric("", "", nil)

	_, err := rule.Measure(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Measure error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the scope verb `in folder`", user.Operation)
	}
}

func TestACustomMetricSelectsNothingAndBranchesFromOneScope(t *testing.T) {
	// The stage is a value like every other: naming a metric does not narrow the scope, and one scope can be
	// asked for two numbers of the user's own without either writing into the other's sentence.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/**")

	methods := scope.CustomMetric("methods", "how many methods a type has",
		func(class extraction.ClassInfo) float64 { return float64(class.MethodCount) })
	fields := scope.CustomMetric("fields", "how many fields a type has",
		func(class extraction.ClassInfo) float64 { return float64(class.FieldCount) })

	if got := valuesOf(measure(t, methods, nil)); !slices.Equal(got, []float64{2, 1, 0}) {
		t.Errorf("methods = %v, want the counts the fixture was written with", got)
	}
	if got := valuesOf(measure(t, fields, nil)); !slices.Equal(got, []float64{2, 0, 0}) {
		t.Errorf("fields = %v, want the counts the fixture was written with", got)
	}
	if selectors := scope.Selectors(); len(selectors) != 1 {
		t.Errorf("the scope's Selectors() = %v, want the one verb it was built with", selectors)
	}
	if got := methods.String(); got != `metrics, path without filename matches "internal/**", `+
		`custom metric, methods ("how many methods a type has")` {
		t.Errorf("the first rule renders as %q, want its own metric", got)
	}
}

func TestACustomMetricIsAMetricTheLibraryHoldsLikeTheRest(t *testing.T) {
	// The reason the escape hatch costs the stages after it nothing: what the fluent layer keeps is a
	// calculation.Metric, and the user's number is one.
	var metric calculation.Metric = calculation.NewCustomMetric("public surface", zeroMeasure)

	if metric.Name() != "public surface" {
		t.Errorf("Name() = %q, want the user's own name", metric.Name())
	}
}

// zeroMeasure is a function that reads nothing, for the tests that are about the words of a rule rather than
// about its numbers.
func zeroMeasure(extraction.ClassInfo) float64 {
	return 0
}
