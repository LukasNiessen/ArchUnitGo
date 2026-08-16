package calculation

import "github.com/LukasNiessen/ArchUnitGo/metrics/projection"

// Metric is what every number this library can take of a project has in common: a name a report prints it
// under, and a way of reading it off the populations a rule selected.
//
// It is the type the fluent layer holds, which is the whole reason it exists. `count, lines of code` and
// `distance, abstractness` are two groups of numbers about two different populations — one file at a time
// and one package at a time — and a builder that named the family in a field would have to branch on which
// group the user picked in every stage after it. Behind this interface it branches nowhere: a group verb
// picks a Metric, and the stage that resolves the rule calls Measure.
//
// An implementation reads the one population it is about out of the projection.Subjects it is handed and
// ignores the rest, exactly as CountMetric does across files and classes. That is what keeps the choice of
// population an answer this package gives rather than a question the caller has to ask.
//
// Implementations are immutable values: get one from a factory in this package and read it through its
// methods. CountMetric, DistanceMetric and CustomMetric are the three the library ships — the last of them
// being how a number the library never named is still a Metric like the rest.
type Metric interface {
	// Name is what the metric is called in a report, spelled as the family spells it — `lines of code`,
	// `abstractness`. It is the word the fluent sentence renders and a violation quotes.
	Name() string
	// Measure reads this metric off every subject it is about, one Measurement each, in the order the
	// subjects were selected. No subjects at all is an empty result rather than an error.
	Measure(subjects projection.Subjects) []Measurement
}
