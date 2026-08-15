package fluentapi

import (
	"errors"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// customGroup is the word this group is called in the sentence a rule renders as, stated once for the reason
// countGroup is. It is a group of one verb, because the verb *is* the group: what a custom metric measures is
// the user's own function rather than one of a family the library names.
const customGroup = "custom metric"

// ErrNoMetricName is the reason `custom metric` was rejected: the number has no name. Every other metric in
// this library calls itself something — `lines of code`, `abstractness` — and a measurement, a rule's sentence
// and a violation all quote that word, so a nameless custom metric is a report saying that nothing came to 40.
// The fix is to name it.
var ErrNoMetricName = errors.New("no name given to the custom metric")

// ErrNoMetricDescription is the reason `custom metric` was rejected: the words saying what the number means
// are missing or blank. They are not decoration — the library cannot describe a number it did not define, so
// those words are what a rule renders as and the only thing that tells a reader of a failure what was
// counted.
var ErrNoMetricDescription = errors.New("no description given to the custom metric")

// ErrNoMeasure is the reason `custom metric` was rejected: there is no function to read the number with, so
// the rule measures nothing at all. The fix is to pass the function.
var ErrNoMeasure = errors.New("no measure function given to the custom metric")

// CustomMetric is the metric a user defines themselves: a name, the words saying what the number means, and
// their own function for reading it off one class.
//
//	rule := archunit.Metrics(nil).
//		ForClassesMatching("*Service").
//		CustomMetric("public surface", "how many methods and fields a type exposes",
//			func(class archunit.MetricsClassInfo) float64 {
//				return float64(class.MethodCount + class.FieldCount)
//			})
//
//	measurements, err := rule.Measure(nil)
//
// It is the escape hatch of the metrics module, and the reason the module does not have to be exhaustive: the
// counts under `count` and the package metrics under `distance` are the numbers the family names, and this is
// any other number a user can compute from a declared type. Everything after it is unchanged — the same
// Measure, the same threshold predicates — because a custom metric is a metric and not a second kind of rule.
//
// The function is handed an archunit.MetricsClassInfo: the class's name, its identifier, the file it was
// declared in, whether it is an interface, how many fields and methods it has, and which of its fields each
// of its methods reaches. It is asked once per selected class, so the subjects are class identifiers —
// `internal/api.Handler` — and a rule usually pairs this verb with ForClassesMatching.
//
// The description is the second half of the metric rather than a nicety. Everything else in this library
// renders itself — a pattern quotes the glob the user typed, a count names the family's own word for it — but
// a closure is not printable and `public surface` alone does not say what was counted, so these words are what
// the rule reads as in a log line and in the heading of a test failure. Phrase them as a noun phrase
// completing "the number of": `how many methods and fields a type exposes`.
//
// Three things are rejected rather than measured, in the order the arguments are given, and none of them is a
// rule failure: a metric with no name, one with no description, and one with no function. Each is returned by
// the resolving stage as a UserError naming `custom metric`, before the project is read.
//
// It opens no group of its own. `count` and `distance` are groups because they hold eight verbs and five, and
// a group holding one verb would be a word the user typed twice.
func (b MetricsBuilder) CustomMetric(name, description string, measure calculation.ClassMeasure) MetricBuilder {
	rule := MetricBuilder{
		scope:       b,
		group:       customGroup,
		description: description,
		metric:      calculation.NewCustomMetric(name, measure),
	}
	switch {
	case strings.TrimSpace(name) == "":
		rule.scope = b.rejecting(customGroup, name, ErrNoMetricName)
	case strings.TrimSpace(description) == "":
		rule.scope = b.rejecting(customGroup, name, ErrNoMetricDescription)
	case measure == nil:
		rule.scope = b.rejecting(customGroup, name, ErrNoMeasure)
	}
	return rule
}
