package fluentapi

import (
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

// MetricBuilder is a metrics rule whose metric has been chosen and whose number has not been judged yet:
// `metrics, in folder "internal/**", count, lines of code`.
//
// It is what every metric verb of every group returns — the eight counts, the five distance metrics and the
// custom metric a user defines themselves alike — and it is the stage the six threshold predicates —
// `should be below`, `should be above`, `should be`, `should be below or equal`, `should be above or equal`,
// `should satisfy` — are chained onto, so a rule about a number is this value plus a comparison. Those six are
// the whole of the family's grammar and no synonym joins them. Measure is what this stage can do on its own:
// hand back the numbers themselves.
//
// One type serves every group because the group is a word of the sentence rather than a kind of rule: what
// differs between `count, lines of code` and `distance, abstractness` is which population the metric reads
// and what it calls itself, and calculation.Metric is where both of those live. A builder per group would
// be a threshold predicate per group.
//
// A MetricBuilder is immutable and carries the stages it was built from unchanged, so storing one and
// branching from it is safe:
//
//	size := archunit.Metrics(nil).InFolder("internal/**").Count().LinesOfCode()
//	measurements, err := size.Measure(nil)
type MetricBuilder struct {
	// scope is the rule as it was described before the group and the metric were named.
	scope MetricsBuilder
	// group is the word the metric was chosen out of — `count`, `distance`, `custom metric` — kept so that
	// the rule renders as the sentence the user typed rather than as the scope and the metric with the group
	// missing.
	group string
	// description is what the number means, in the user's own words, and it is empty for every metric the
	// library names: `lines of code` describes itself and a custom metric's name cannot, which is why
	// CustomMetric asks for these words and no other verb does.
	//
	// It is kept here rather than on the metric because it is a word of the sentence, like the group above:
	// what calculation.CustomMetric holds is the name a measurement is reported under and the function that
	// reads it, and the prose beside them belongs where the rest of the rule's prose already is.
	description string
	// metric is the number this rule is about, and how it is read off the subjects the scope named.
	metric calculation.Metric
}

// Measure resolves this rule against the project and reads its metric off every subject the scope named: one
// measurement per selected file for a metric about a file, one per selected class for a metric about a class,
// and one per selected package for a metric about a component, in the order the subjects were selected in. A
// nil *CheckOptions means the defaults.
//
// It is the half of every rule about numbers that runs before the mood — locate the project, extract it, keep
// the files and classes every scope verb accepts, count — so the threshold predicates are written over it,
// and it is how a user can see the numbers a rule is about instead of only whether they pass:
//
//	measurements, err := archunit.Metrics(nil).
//		InFolder("internal/**").
//		Count().
//		LinesOfCode().
//		Measure(nil)
//
// Measuring nothing is neither an error nor a violation here. Whether an empty selection is a failure is a
// question only a rule that judges something can ask, so the empty-test guard belongs with the threshold
// predicates and Selectors is the data it reports with.
//
// The error is a pattern a scope verb rejected — a UserError naming the verb, returned before the project is
// read — or a project that cannot be located, extracted or read. It is never a rule failure.
func (b MetricBuilder) Measure(options *kernel.CheckOptions) ([]calculation.Measurement, error) {
	subjects, err := b.scope.resolve(options)
	if err != nil {
		return nil, err
	}
	return b.readings(subjects), nil
}

// Selectors are the compiled scope verbs this rule was built from, in the order they were chained. They are
// the scope's own, unchanged: choosing a metric selects nothing.
func (b MetricBuilder) Selectors() []matching.Filter {
	return b.scope.Selectors()
}

// String renders the rule for logs and test failures, as `metrics, path without filename matches
// "internal/**", count, lines of code` — and, for the metric a user defined themselves, with the words they
// described it with: `metrics, custom metric, public surface ("how many methods and fields a type exposes")`.
func (b MetricBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.scope.rejected()
}

// stages are the parts of the sentence this rule has been built from: the scope's, then the group, then the
// metric — with the words describing it in brackets, for the one metric the library did not define.
//
// A part that is not there is left out rather than rendered as an empty word, so that the zero MetricBuilder
// — no group and no metric — reads as `metrics` instead of as a sentence with two gaps in it.
func (b MetricBuilder) stages() []string {
	stages := b.scope.stages()
	if b.group != "" {
		stages = append(stages, b.group)
	}
	if b.metric != nil {
		named := b.metric.Name()
		if b.description != "" {
			named += ` ("` + b.description + `")`
		}
		stages = append(stages, named)
	}
	return stages
}

// readings are this rule's numbers, read off the subjects a resolved scope came to: one measurement per
// subject the metric is about, in the order they were selected.
//
// It is the one place the metric is asked for anything, so Measure and every predicate chained onto this
// stage read the same numbers the same way. The zero MetricBuilder names no metric, and a rule with no
// number to read measures nothing — which is the answer the zero calculation.CountMetric gives to the same
// question, one layer down.
func (b MetricBuilder) readings(subjects projection.Subjects) []calculation.Measurement {
	if b.metric == nil {
		return nil
	}
	return b.metric.Measure(subjects)
}
