package calculation

import (
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

// ClassMeasure is a number the library has no verb for, as the user writes it: one question about one
// class, answered with a figure.
//
// It is the third argument of `custom metric`, and everything it may read is on the extraction.ClassInfo
// it is handed — the class's name, the file it was declared in, whether it is an interface, its fields,
// its methods, and which of those fields each of those methods reaches:
//
//	func(class archunit.MetricsClassInfo) float64 { return float64(len(class.Methods) + len(class.Fields)) }
//
// It is asked once per selected class and nothing else is asked of it: it must not depend on how often or
// in which order it is called, because that is the library's business and both may change. A count, a
// ratio and a score are all one type here, for the reason Measurement.Value is a float64.
type ClassMeasure func(class extraction.ClassInfo) float64

// CustomMetric is a number this library has no verb for: the name the user calls it, and their own
// function for reading it off one class.
//
// It is the escape hatch of the metrics module, and the reason the module does not have to be exhaustive.
// The counts and the distance metrics are the numbers the family names; this is any other number a user
// can compute from a declared type — a score over its methods, a cohesion measure the fluent API has no
// group for yet, a house rule about how many fields a value object may carry.
//
// It is a metric about a class, so its subjects are the classes a rule selected and its measurements are
// reported per class identifier — `internal/api.Handler`. What a class is here, and what the function is
// handed of one, is extraction.ClassInfo's to say.
//
// A CustomMetric is immutable: build one with NewCustomMetric and read it through its methods. The zero
// value has no name and no function, and measures nothing, exactly as the zero CountMetric does.
type CustomMetric struct {
	// name is what the user calls this number, and what a report prints it under.
	name string
	// measure is the user's own function, kept as it was given.
	measure ClassMeasure
}

// NewCustomMetric is the number a user names and computes themselves: what it is called, and how it is
// read off one class.
//
// It is the only way a metric of this kind is made, and it is what the fluent API's `custom metric` verb
// passes the user's own function into. Neither argument is checked here — a metric with no name or no
// function measures nothing rather than failing, and rejecting a rule the library cannot run is the
// fluent stage's business, where there is a user to report it to.
func NewCustomMetric(name string, measure ClassMeasure) CustomMetric {
	return CustomMetric{name: name, measure: measure}
}

// Name is what this metric is called in a report, in the user's own words — `branch count`. It is the word
// a violation quotes and the one the fluent sentence renders, and the zero CustomMetric has none.
func (m CustomMetric) Name() string {
	return m.name
}

// Measure reads this metric off every class the rule selected, one Measurement each, in the order the
// classes arrived.
//
// The user's function is called exactly once per class, with the class as this library extracted it, and
// its answer is carried through untouched: a number nobody here can interpret is still a number a
// threshold predicate can judge.
//
// The files and the components are handed over and ignored, as they are by every metric about a class,
// because which population a metric reads is an answer this package gives rather than a question its
// caller has to ask. A CustomMetric with no function measures nothing, for the same reason the zero
// CountMetric does — and no rule the fluent API builds has one, because a missing function is returned
// to the user as their error instead.
//
// No classes at all is an empty result rather than an error. Whether a rule that measured nothing is a
// failure is the empty-test guard's question, asked where the rule is judged.
func (m CustomMetric) Measure(subjects projection.Subjects) []Measurement {
	if m.measure == nil {
		return nil
	}
	measurements := make([]Measurement, 0, len(subjects.Classes))
	for _, class := range subjects.Classes {
		measurements = append(measurements, Measurement{
			Metric:  m.name,
			Subject: class.Identifier,
			Value:   m.measure(class),
		})
	}
	return measurements
}

// String renders the metric as its name, for logs and test failures, like every other metric here. The
// words the user described it with are the fluent stage's to render: they are part of the sentence a rule
// was written as rather than of the number itself.
func (m CustomMetric) String() string {
	return m.name
}
