package fluentapi_test

import (
	"math"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

func TestEachDistanceVerbNamesOneOfTheFivePackageMetrics(t *testing.T) {
	// The five verbs the distance group holds, each measured over the whole fixture project, whose three
	// folders are:
	//
	//	.              0 types,               depends on internal/api          A 0,   I 1
	//	internal/api   2 types, 1 interface,  between the other two            A 0.5, I 0.5
	//	internal/db    1 type,                depended on by internal/api      A 0,   I 0
	scope := fluentapi.Metrics(measuredProject(t))
	components := []string{".", "internal/api", "internal/db"}

	tests := []struct {
		name       string
		rule       fluentapi.MetricBuilder
		wantValues []float64
	}{
		{name: "abstractness", rule: scope.Distance().Abstractness(), wantValues: []float64{0, 0.5, 0}},
		{name: "instability", rule: scope.Distance().Instability(), wantValues: []float64{1, 0.5, 0}},
		{name: "normalized distance", rule: scope.Distance().NormalizedDistance(), wantValues: []float64{0, 0, 1}},
		{
			name:       "distance from the main sequence",
			rule:       scope.Distance().DistanceFromMainSequence(),
			wantValues: []float64{0, 0, 1 / math.Sqrt(2)},
		},
		// Each of the three is coupled to one of the other two in one direction, except internal/api, which is
		// coupled to both: 1/(2*2) and 2/(2*2).
		{name: "coupling factor", rule: scope.Distance().CouplingFactor(), wantValues: []float64{0.25, 0.5, 0.25}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			measurements := measure(t, test.rule, nil)

			if got := subjectsOf(measurements); !slices.Equal(got, components) {
				t.Errorf("`%s` measures %v, want the folders of the project, %v", test.name, got, components)
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

func TestADistanceMetricIsMeasuredOverThePackagesTheScopeSelected(t *testing.T) {
	// A component's coupling is about the packages the rule selected, so narrowing the scope narrows what a
	// package is measured against: `internal/db` alone depends on nothing selected and nothing selected depends
	// on it, so it is maximally stable and its coupling factor is 0.
	rule := fluentapi.Metrics(measuredProject(t)).InFolder("internal/db")

	instability := measure(t, rule.Distance().Instability(), nil)
	coupling := measure(t, rule.Distance().CouplingFactor(), nil)

	if got := subjectsOf(instability); !slices.Equal(got, []string{"internal/db"}) {
		t.Errorf("`in folder \"internal/db\"` measures %v, want the one folder it named", got)
	}
	if got := valuesOf(instability); !slices.Equal(got, []float64{0}) {
		t.Errorf("instability = %v, want 0 for the only package the rule selected", got)
	}
	if got := valuesOf(coupling); !slices.Equal(got, []float64{0}) {
		t.Errorf("coupling factor = %v, want 0 when there is no other package to be coupled to", got)
	}
}

func TestADistanceMetricIsNarrowedByAClassVerbToo(t *testing.T) {
	// `for classes matching` narrows the files, and a package is the package of the files that were kept — so a
	// rule naming an interface measures the folders declaring one, counted over those files alone.
	rule := fluentapi.Metrics(measuredProject(t)).ForClassesMatching("Router")

	measurements := measure(t, rule.Distance().Abstractness(), nil)

	if got := subjectsOf(measurements); !slices.Equal(got, []string{"internal/api"}) {
		t.Errorf("`for classes matching \"Router\"` measures %v, want the folder declaring it", got)
	}
	if got := valuesOf(measurements); !slices.Equal(got, []float64{1}) {
		t.Errorf("abstractness = %v, want 1: the one type the rule kept is an interface", got)
	}
}

func TestDistanceOpensTheGroupWithoutReadingAnything(t *testing.T) {
	// `distance` is the word that says which kind of number the rule means, and nothing more: it selects
	// nothing, so a scope and the same scope with `distance` chained onto it describe the same subjects.
	scope := fluentapi.Metrics(nil).InFolder("internal/**")

	group := scope.Distance()

	if selectors := scope.Selectors(); len(selectors) != 1 {
		t.Errorf("the scope's Selectors() = %v after Distance(), want the one verb it was built with", selectors)
	}
	if rendered := group.String(); rendered != `metrics, path without filename matches "internal/**", distance` {
		t.Errorf("String() = %q, want the scope with `distance` appended", rendered)
	}
	if entry := fluentapi.Metrics(nil).Distance().String(); entry != "metrics, distance" {
		t.Errorf("String() = %q, want `metrics, distance`", entry)
	}
}

func TestADistanceMetricRendersTheWholeSentenceItWasBuiltFrom(t *testing.T) {
	rule := fluentapi.Metrics(nil).InFolder("internal/**").Distance().NormalizedDistance()

	want := `metrics, path without filename matches "internal/**", distance, normalized distance`
	if rendered := rule.String(); rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
}

func TestDistanceRendersTheRejectedPatternOfTheScopeItWasAskedOf(t *testing.T) {
	// A rejected pattern narrowed nothing, so a stage that hid it would render as the rule the user thought
	// they wrote.
	group := fluentapi.Metrics(nil).InFolder("[unclosed").Distance()

	rendered := group.String()

	if want := "metrics, distance (rejected: "; len(rendered) < len(want) || rendered[:len(want)] != want {
		t.Errorf("String() = %q, want it to start with %q", rendered, want)
	}
}

func TestTheDistanceGroupCanBeStoredAndBranchedFrom(t *testing.T) {
	// The stage is a value like every other, so one scope's `distance` can be asked for two numbers.
	group := fluentapi.Metrics(measuredProject(t)).InFolder("internal/api").Distance()

	abstractness := measure(t, group.Abstractness(), nil)
	instability := measure(t, group.Instability(), nil)

	if got := valuesOf(abstractness); !slices.Equal(got, []float64{0.5}) {
		t.Errorf("abstractness = %v, want the 0.5 of a folder with one interface among two types", got)
	}
	if got := valuesOf(instability); !slices.Equal(got, []float64{0}) {
		t.Errorf("instability = %v, want 0 for the only package the rule selected", got)
	}
}

func TestADistanceMetricOfAScopeThatSelectedNothingMeasuresNothing(t *testing.T) {
	// Measuring nothing is neither an error nor a violation here: whether an empty selection is a failure is a
	// question only a rule that judges something can ask, which is what the zone checks do.
	rule := fluentapi.Metrics(measuredProject(t)).InFolder("nowhere/**").Distance().Abstractness()

	if measurements := measure(t, rule, nil); len(measurements) != 0 {
		t.Errorf("%s measured %+v, want nothing", rule, measurements)
	}
}
