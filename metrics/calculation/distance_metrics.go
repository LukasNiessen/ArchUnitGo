package calculation

import (
	"math"

	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

// DistanceMetric is one of the five numbers this library takes of a package: what it is called, and how it
// is read off one component.
//
// It is Robert C. Martin's family — abstractness, instability, the distance from the main sequence and the
// normalized distance — plus the coupling factor, and all five are ratios rather than counts, which is what
// they have in common beside their population. A component is a folder, so these are the numbers a rule
// about the shape of a project's packages is written with, where the counts are the numbers a rule about one
// file is written with.
//
// A DistanceMetric is immutable: get one from a factory below and read it through its methods. The zero
// DistanceMetric reads nothing and measures nothing, like the zero CountMetric.
type DistanceMetric struct {
	// name is what the metric is called in a report, spelled as the family spells it — `abstractness`.
	name string
	// read is the formula, over one component and how many components the rule selected.
	//
	// The population size is passed in because one of the five needs it: a coupling factor is a share of the
	// couplings a component could possibly have, and how many that is, is how many other components there
	// are. The four Martin metrics ignore it, and taking it as a parameter rather than reading it off a
	// field is what keeps a Component a fact about one package instead of a fact about one package in one
	// rule.
	read func(component projection.Component, components int) float64
}

// Abstractness is A, the share of a component's declared types that are interfaces.
//
//	A = interfaces / classes
//
// 0 is a package of nothing but concrete types and 1 is a package of nothing but interfaces. It is one axis
// of the plane the main sequence runs through, and the question it asks is how much of what this package
// says is a promise rather than a mechanism.
//
// Go has no abstract class, so an interface is what an abstract type is here — the declaration that names
// what a package does without saying how — and `classes` is every type the package declares, which is the
// population `count, classes` counts and `for classes matching` selects.
//
// A package that declares no type at all is 0: a folder of nothing but functions is as concrete as code
// gets, and it is also the reading that keeps the formula from dividing by zero rather than saying so.
func Abstractness() DistanceMetric {
	return DistanceMetric{name: "abstractness", read: func(component projection.Component, _ int) float64 {
		return AbstractnessOf(component)
	}}
}

// Instability is I, the share of a component's couplings that are its own outgoing ones.
//
//	I = Ce / (Ce + Ca)
//
// Ce is the efferent coupling, how many other components this one depends on, and Ca the afferent coupling,
// how many depend on it. 0 is a component nothing but other packages' choices can break — every coupling it
// has points at it — and 1 is a component that nothing depends on and that reaches for everything, which is
// free to change because nobody would notice.
//
// It is the other axis of the plane, and it is a fact about the components the rule selected: a dependency
// on a package the scope left out is not counted, which is metrics/projection.PerComponentEdge's trade.
//
// A component with no coupling at all is 0. It is maximally stable in the sense the formula is about —
// nothing that depends on it can be broken by a change to it, because nothing depends on it — which is worth
// knowing before writing a rule that reads a whole project one folder at a time.
func Instability() DistanceMetric {
	return DistanceMetric{name: "instability", read: func(component projection.Component, _ int) float64 {
		return InstabilityOf(component)
	}}
}

// DistanceFromMainSequence is D, how far a component sits from the line where abstractness and instability
// balance.
//
//	D = |A + I - 1| / √2
//
// The main sequence is the line A + I = 1, running from the abstract and stable corner to the concrete and
// unstable one, and a component on it is as abstract as its dependents' need for stability demands. 0 is a
// component on the line, and the largest this number can be is 1/√2 ≈ 0.707, at either of the two corners
// the line misses — which are the zone of pain and the zone of uselessness.
//
// It is the perpendicular distance to the line, which is why it is bounded by 1/√2 rather than by 1.
// NormalizedDistance is the same question with the bound divided out, and it is usually the one to write a
// threshold against, because 0.7 means "as bad as it gets" there and "impossible" here.
func DistanceFromMainSequence() DistanceMetric {
	return DistanceMetric{name: "distance from the main sequence", read: func(component projection.Component, _ int) float64 {
		return normalizedDistance(component) / math.Sqrt2
	}}
}

// NormalizedDistance is D', the distance from the main sequence stated on a scale of 0 to 1.
//
//	D' = |A + I - 1|
//
// 0 is a component on the main sequence and 1 is one at either corner off it: maximally abstract and
// maximally unstable, or maximally concrete and maximally stable. It is DistanceFromMainSequence multiplied
// by √2, and both names are kept for the reason LCOM2 and LCOM96b both are — a reader arriving from the
// literature looks for the one they know, and one function behind both is what stops them drifting apart.
func NormalizedDistance() DistanceMetric {
	return DistanceMetric{name: "normalized distance", read: func(component projection.Component, _ int) float64 {
		return normalizedDistance(component)
	}}
}

// CouplingFactor is the share of the couplings a component could have that it actually has.
//
//	CF = (Ca + Ce) / (2 * (n - 1))
//
// n is how many components the rule selected, so 2 * (n - 1) is every coupling one component could possibly
// carry: it could depend on each of the others and be depended on by each of them. 0 is a package that is
// connected to nothing and 1 is one that is coupled to every other package in both directions.
//
// It is the MOOD coupling factor asked of one component rather than of the whole system, which is what makes
// it a Measurement like the other four: every number in this library is about a subject a report can name
// and a rule can be broken by, and "the project" is neither. Averaging these over the components is the
// system-wide figure, and a caller that wants it has every term of the average in the result.
//
// A rule that selected a single component is 0, because there is no other component for it to be coupled to
// — which is also the reading that keeps the formula from dividing by zero.
func CouplingFactor() DistanceMetric {
	return DistanceMetric{name: "coupling factor", read: couplingFactor}
}

// Name is what this metric is called in a report, spelled as the family spells it — `abstractness`,
// `coupling factor`. It is the word the fluent sentence renders, and the zero DistanceMetric has none.
func (m DistanceMetric) Name() string {
	return m.name
}

// Measure reads this metric off every component the rule selected, one Measurement each, in the order
// SelectComponents sorted them into — which is by folder identifier, so a report of a project's packages
// comes out the same way twice.
//
// The files and the classes are handed over with them and ignored: a distance metric is about a package, and
// which population a metric reads is the answer this package gives rather than a question its caller asks.
// The zero DistanceMetric measures nothing, because a metric with no formula has no number to report.
//
// No components at all is an empty result rather than an error. Whether a rule that measured nothing is a
// failure is the empty-test guard's question, asked where the rule is judged.
func (m DistanceMetric) Measure(subjects projection.Subjects) []Measurement {
	if m.read == nil {
		return nil
	}
	measurements := make([]Measurement, 0, len(subjects.Components))
	for _, component := range subjects.Components {
		measurements = append(measurements, Measurement{
			Metric:  m.name,
			Subject: component.Label,
			Value:   m.read(component, len(subjects.Components)),
		})
	}
	return measurements
}

// String renders the metric as its name, for logs and test failures.
func (m DistanceMetric) String() string {
	return m.name
}

// AbstractnessOf is A read off one component, as Abstractness measures it and as its doc describes it.
//
// It is exported for the same reason InstabilityOf is: the two of them are the coordinates of the plane a
// Zone is a region of, so a rule that reports a component in the zone of pain has to name them — the numbers
// are the whole diagnosis — and a second implementation of an axis of that plane would be a plane whose axes
// could disagree. The two distances are metrics and nothing else, so they have no such function.
func AbstractnessOf(component projection.Component) float64 {
	if component.Classes == 0 {
		return 0
	}
	return float64(component.Interfaces) / float64(component.Classes)
}

// InstabilityOf is I read off one component, as Instability measures it and as its doc describes it: its
// efferent coupling over its total coupling, and 0 for a component with neither.
func InstabilityOf(component projection.Component) float64 {
	efferent, afferent := len(component.DependsOn), len(component.DependedOnBy)
	if efferent+afferent == 0 {
		return 0
	}
	return float64(efferent) / float64(efferent+afferent)
}

// normalizedDistance is D', |A + I - 1|. It is the one place the two distances are calculated, so that D is
// this number scaled and the pair cannot disagree about which side of the main sequence a component is on —
// neither of them says, because a distance is unsigned.
func normalizedDistance(component projection.Component) float64 {
	return math.Abs(AbstractnessOf(component) + InstabilityOf(component) - 1)
}

// couplingFactor is CF, read off one component and the size of the population it was selected with. It is a
// named function rather than a closure because it is the one of the five that reads that second argument.
func couplingFactor(component projection.Component, components int) float64 {
	if components < 2 {
		return 0
	}
	coupled := len(component.DependsOn) + len(component.DependedOnBy)
	return float64(coupled) / float64(2*(components-1))
}
