package fluentapi_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

func TestMeasureHandsBackTheNumbersTheRuleIsAbout(t *testing.T) {
	// The resolution door of a rule about numbers, and the half every threshold predicate is written over:
	// locate, extract, select, count — one measurement per subject, each saying what it measured.
	rule := fluentapi.Metrics(measuredProject(t)).InFolder("internal/api").Count().LinesOfCode()

	measurements, err := rule.Measure(nil)
	if err != nil {
		t.Fatalf("Measure failed: %v", err)
	}

	if len(measurements) != 2 {
		t.Fatalf("Measure produced %+v, want one measurement per selected file", measurements)
	}
	want := "internal/api/handler.go: lines of code = 9"
	if got := measurements[0].String(); got != want {
		t.Errorf("the first measurement is %q, want %q", got, want)
	}
}

func TestMeasuringTwiceAnswersTheSameThing(t *testing.T) {
	// A rule is a value and resolving it is a read, so a stored rule measured twice cannot disagree with
	// itself — which is what lets one rule be reported and then judged.
	rule := fluentapi.Metrics(measuredProject(t)).Count().Statements()

	first := measure(t, rule, nil)
	second := measure(t, rule, nil)

	if !slices.Equal(subjectsOf(first), subjectsOf(second)) || !slices.Equal(valuesOf(first), valuesOf(second)) {
		t.Errorf("Measure answered %+v and then %+v, want the same numbers", first, second)
	}
}

func TestMeasuringNothingIsNeitherAnErrorNorAViolation(t *testing.T) {
	// Whether a rule that selected nothing is a failure is a question only a rule that judges something can
	// ask, so the empty-test guard belongs with the threshold predicates and this door reports the empty list.
	rule := fluentapi.Metrics(measuredProject(t)).InFolder("nowhere/**").Count().LinesOfCode()

	measurements, err := rule.Measure(nil)
	if err != nil {
		t.Fatalf("Measure failed: %v", err)
	}

	if len(measurements) != 0 {
		t.Errorf("Measure produced %+v, want nothing measured", measurements)
	}
	if selectors := rule.Selectors(); len(selectors) != 1 {
		t.Errorf("Selectors() = %v, want the verb a report would name as the one that selected nothing", selectors)
	}
}

func TestAMetricBuildersSelectorsAreTheScopesOwn(t *testing.T) {
	// Choosing a metric selects nothing, so the data a report needs in order to say which pattern matched
	// nothing is the same at both stages.
	scope := fluentapi.Metrics(nil).InFolder("internal/**").ForClassesMatching("*Service")

	rule := scope.Count().MethodCount()

	selectors := rule.Selectors()
	if len(selectors) != 2 {
		t.Fatalf("Selectors() = %v, want the two verbs the scope was built with", selectors)
	}
	if selectors[0].Target() != matching.TargetPathWithoutFilename || selectors[1].Target() != matching.TargetClassname {
		t.Errorf("Selectors() = %v, want the folder verb and then the class verb", selectors)
	}
	if got := scope.Selectors(); len(got) != len(selectors) {
		t.Errorf("the scope's Selectors() = %v, want the rule's %v", got, selectors)
	}
}

func TestAMetricBuilderRendersTheWholeSentence(t *testing.T) {
	tests := []struct {
		name string
		rule fluentapi.MetricBuilder
		want string
	}{
		{
			name: "a metric about a file",
			rule: fluentapi.Metrics(nil).InFolder("internal/**").Count().LinesOfCode(),
			want: `metrics, path without filename matches "internal/**", count, lines of code`,
		},
		{
			name: "a metric about a class",
			rule: fluentapi.Metrics(nil).ForClassesMatching("*Service").Count().MethodCount(),
			want: `metrics, classname matches "*Service", count, method count`,
		},
		{
			name: "no scope verb at all",
			rule: fluentapi.Metrics(nil).Count().Interfaces(),
			want: "metrics, count, interfaces",
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

func TestAMetricBuilderRendersTheRejectedPatternOfItsScope(t *testing.T) {
	rule := fluentapi.Metrics(nil).WithName("[unclosed").Count().Imports()

	rendered := rule.String()

	if want := "metrics, count, imports (rejected: "; len(rendered) < len(want) || rendered[:len(want)] != want {
		t.Errorf("String() = %q, want it to start with %q", rendered, want)
	}
}

func TestTwoMetricsOfOneScopeDoNotWriteIntoEachOthersSentence(t *testing.T) {
	// The append trap once more, one stage further on: both metrics grow the same scope's stages, so a stage
	// list shared between them would leave one rule rendering as the other.
	group := fluentapi.Metrics(nil).InFolder("internal/**").Count()

	lines := group.LinesOfCode()
	imports := group.Imports()

	if got := lines.String(); got != `metrics, path without filename matches "internal/**", count, lines of code` {
		t.Errorf("the first rule renders as %q, want its own metric", got)
	}
	if got := imports.String(); got != `metrics, path without filename matches "internal/**", count, imports` {
		t.Errorf("the second rule renders as %q, want its own metric", got)
	}
}
