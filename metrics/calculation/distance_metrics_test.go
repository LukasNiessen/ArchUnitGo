package calculation_test

import (
	"math"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

func TestEachMetricAboutAPackageReadsItsOwnRatio(t *testing.T) {
	// The five numbers about a package, each read off the same hand-built component: `internal/api` declares
	// four types of which one is an interface, depends on one other component and is depended on by three, in
	// a rule that selected five components.
	//
	//	A  = 1/4                        = 0.25
	//	I  = 1/(1+3)                    = 0.25
	//	D' = |0.25 + 0.25 - 1|          = 0.5
	//	D  = 0.5 / √2                   ≈ 0.3536
	//	CF = (3 + 1) / (2 * (5 - 1))    = 0.5
	subjects := fixtureComponents()

	tests := []struct {
		metric calculation.DistanceMetric
		want   float64
	}{
		{metric: calculation.Abstractness(), want: 0.25},
		{metric: calculation.Instability(), want: 0.25},
		{metric: calculation.NormalizedDistance(), want: 0.5},
		// math.Sqrt(2) rather than the math.Sqrt2 constant: the constant would make this an exact
		// arbitrary-precision division at compile time, and what the metric does is a float64 one.
		{metric: calculation.DistanceFromMainSequence(), want: 0.5 / math.Sqrt(2)},
		{metric: calculation.CouplingFactor(), want: 0.5},
	}

	for _, test := range tests {
		t.Run(test.metric.Name(), func(t *testing.T) {
			measurements := test.metric.Measure(subjects)

			if len(measurements) != len(subjects.Components) {
				t.Fatalf("%s produced %+v, want one measurement per selected component", test.metric, measurements)
			}
			if measurements[0].Value != test.want {
				t.Errorf("%s = %g, want %g", test.metric, measurements[0].Value, test.want)
			}
			if measurements[0].Subject != "internal/api" {
				t.Errorf("%s was reported about %q, want the folder identifier", test.metric, measurements[0].Subject)
			}
			if measurements[0].Metric != test.metric.Name() {
				t.Errorf("measurement names the metric %q, want %q", measurements[0].Metric, test.metric.Name())
			}
		})
	}
}

func TestAbstractnessIsTheShareOfDeclaredTypesThatAreInterfaces(t *testing.T) {
	tests := []struct {
		name      string
		component projection.Component
		want      float64
	}{
		{
			name:      "nothing but concrete types",
			component: projection.Component{Label: "internal/db", Classes: 3},
			want:      0,
		},
		{
			name:      "nothing but interfaces",
			component: projection.Component{Label: "internal/port", Classes: 2, Interfaces: 2},
			want:      1,
		},
		{
			name:      "half and half",
			component: projection.Component{Label: "internal/api", Classes: 4, Interfaces: 2},
			want:      0.5,
		},
		{
			// A folder of nothing but functions is as concrete as code gets, and it is also the reading that
			// keeps the formula from dividing by zero rather than saying so.
			name:      "a package declaring no type at all",
			component: projection.Component{Label: "cmd/tool"},
			want:      0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := calculation.AbstractnessOf(test.component); got != test.want {
				t.Errorf("AbstractnessOf(%+v) = %g, want %g", test.component, got, test.want)
			}
		})
	}
}

func TestInstabilityIsTheShareOfCouplingsThatAreOutgoing(t *testing.T) {
	tests := []struct {
		name      string
		component projection.Component
		want      float64
	}{
		{
			name:      "depended upon and depending on nothing",
			component: projection.Component{Label: "internal/db", DependedOnBy: []string{"internal/api", "."}},
			want:      0,
		},
		{
			name:      "depending on others and depended on by nothing",
			component: projection.Component{Label: ".", DependsOn: []string{"internal/api"}},
			want:      1,
		},
		{
			name: "as much of one as of the other",
			component: projection.Component{
				Label: "internal/api", DependsOn: []string{"internal/db"}, DependedOnBy: []string{"."},
			},
			want: 0.5,
		},
		{
			// Maximally stable in the sense the formula is about: nothing that depends on it can be broken by
			// a change to it, because nothing depends on it.
			name:      "no coupling at all",
			component: projection.Component{Label: "internal/util"},
			want:      0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := calculation.InstabilityOf(test.component); got != test.want {
				t.Errorf("InstabilityOf(%+v) = %g, want %g", test.component, got, test.want)
			}
		})
	}
}

func TestTheTwoDistancesAreTheSameQuestionOnTwoScales(t *testing.T) {
	// D is D' divided by √2, so a component's two distances can never disagree about how far off the line it
	// is — which is what one formula behind both names buys.
	components := []projection.Component{
		{Label: "on the line", Classes: 2, Interfaces: 1, DependsOn: []string{"a"}, DependedOnBy: []string{"b"}},
		{Label: "in the concrete corner", Classes: 2},
		{Label: "in the abstract corner", Classes: 2, Interfaces: 2, DependsOn: []string{"a"}},
	}
	wantNormalized := []float64{0, 1, 1}

	subjects := projection.Subjects{Components: components}
	normalized := calculation.NormalizedDistance().Measure(subjects)
	perpendicular := calculation.DistanceFromMainSequence().Measure(subjects)

	for index, want := range wantNormalized {
		if normalized[index].Value != want {
			t.Errorf("the normalized distance of %q is %g, want %g",
				normalized[index].Subject, normalized[index].Value, want)
		}
		if got := perpendicular[index].Value; got != want/math.Sqrt2 {
			t.Errorf("the distance from the main sequence of %q is %g, want %g",
				perpendicular[index].Subject, got, want/math.Sqrt2)
		}
	}
}

func TestTheCouplingFactorIsAShareOfThePopulationItWasSelectedWith(t *testing.T) {
	// The one metric of the five that is about a component *in a rule* rather than about a component: the same
	// package coupled the same way is a bigger share of a small selection than of a large one.
	component := projection.Component{
		Label: "internal/api", DependsOn: []string{"internal/db"}, DependedOnBy: []string{"."},
	}

	tests := []struct {
		name       string
		components int
		want       float64
	}{
		{name: "coupled to both of the other two", components: 3, want: 0.5},
		{name: "the same coupling among five", components: 5, want: 0.25},
		// There is no other component for it to be coupled to, which is also the reading that keeps the
		// formula from dividing by zero.
		{name: "the only component the rule selected", components: 1, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected := make([]projection.Component, 0, test.components)
			selected = append(selected, component)
			for len(selected) < test.components {
				selected = append(selected, projection.Component{Label: "filler"})
			}

			measurements := calculation.CouplingFactor().Measure(projection.Subjects{Components: selected})

			if len(measurements) == 0 {
				t.Fatalf("the coupling factor of %d components measured nothing", test.components)
			}
			if measurements[0].Value != test.want {
				t.Errorf("the coupling factor of %q among %d = %g, want %g",
					measurements[0].Subject, test.components, measurements[0].Value, test.want)
			}
		})
	}
}

func TestAMetricAboutAPackageIgnoresTheSelectedFilesAndClasses(t *testing.T) {
	// Every population is handed over at once so that nothing upstream branches on the kind of metric, and
	// each metric takes only the one it is about.
	subjects := fixtureComponents()
	subjects.Files = []extraction.FileInfo{fixtureFile()}
	subjects.Classes = fixtureFile().Classes

	measurements := calculation.Abstractness().Measure(subjects)

	if len(measurements) != len(subjects.Components) {
		t.Fatalf("abstractness produced %+v, want one measurement per component", measurements)
	}
	for _, measurement := range measurements {
		if measurement.Subject == "internal/api/handler.go" || measurement.Subject == "internal/api.Handler" {
			t.Errorf("abstractness was reported about %q, want a folder", measurement.Subject)
		}
	}
}

func TestAMetricAboutAPackageMeasuresItsComponentsInOrder(t *testing.T) {
	// The order of the report is the order of the selection, which SelectComponents already sorted by label,
	// so a rule prints the same list twice.
	measurements := calculation.Abstractness().Measure(fixtureComponents())

	want := []string{"internal/api", "internal/db"}
	for index, label := range want {
		if measurements[index].Subject != label {
			t.Errorf("measurement %d is about %q, want %q", index, measurements[index].Subject, label)
		}
	}
}

func TestAMetricAboutAPackageWithNoComponentsMeasuresNothing(t *testing.T) {
	// A rule whose scope selected nothing is the empty-test guard's business, asked where the rule is judged.
	for _, metric := range everyDistanceMetric() {
		if measurements := metric.Measure(projection.Subjects{}); len(measurements) != 0 {
			t.Errorf("%s of no components = %+v, want nothing measured", metric, measurements)
		}
	}
}

func TestTheZeroDistanceMetricMeasuresNothing(t *testing.T) {
	// A metric that was never built has no formula, so it has no number to report.
	if measurements := (calculation.DistanceMetric{}).Measure(fixtureComponents()); len(measurements) != 0 {
		t.Errorf("the zero DistanceMetric measured %+v, want nothing", measurements)
	}
}

func TestEveryDistanceMetricIsNamedAsTheFamilySpellsIt(t *testing.T) {
	// The five names are the words the fluent sentence and every report are printed with, so they are part of
	// the surface and not an implementation detail.
	want := []string{
		"abstractness", "instability", "distance from the main sequence",
		"normalized distance", "coupling factor",
	}

	metrics := everyDistanceMetric()
	if len(metrics) != len(want) {
		t.Fatalf("there are %d distance metrics, want the %d the issue names", len(metrics), len(want))
	}
	for index, name := range want {
		if metrics[index].Name() != name {
			t.Errorf("metric %d is called %q, want %q", index, metrics[index].Name(), name)
		}
		if metrics[index].String() != name {
			t.Errorf("metric %d renders as %q, want %q", index, metrics[index].String(), name)
		}
	}
	if (calculation.DistanceMetric{}).Name() != "" {
		t.Errorf("the zero DistanceMetric is called %q, want no name at all", (calculation.DistanceMetric{}).Name())
	}
}

// everyDistanceMetric is the five, in the order the issue lists them.
func everyDistanceMetric() []calculation.DistanceMetric {
	return []calculation.DistanceMetric{
		calculation.Abstractness(),
		calculation.Instability(),
		calculation.DistanceFromMainSequence(),
		calculation.NormalizedDistance(),
		calculation.CouplingFactor(),
	}
}

// fixtureComponents is a selection of five components whose first one has a different number in every field, so
// that a metric reading the wrong one cannot pass. The three fillers are what makes the population size — which
// only the coupling factor reads — a number of its own.
func fixtureComponents() projection.Subjects {
	return projection.Subjects{Components: []projection.Component{
		{
			Label: "internal/api", Classes: 4, Interfaces: 1,
			DependsOn:    []string{"internal/db"},
			DependedOnBy: []string{".", "cmd/serve", "internal/worker"},
		},
		{Label: "internal/db", Classes: 2, DependedOnBy: []string{"internal/api"}},
		{Label: "internal/worker", DependsOn: []string{"internal/api"}},
		{Label: "cmd/serve", DependsOn: []string{"internal/api"}},
		{Label: ".", DependsOn: []string{"internal/api"}},
	}}
}
