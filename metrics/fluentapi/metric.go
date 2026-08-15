package fluentapi

import (
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// MetricBuilder is a metrics rule whose metric has been chosen and whose number has not been judged yet:
// `metrics, in folder "internal/**", count, lines of code`.
//
// It is what every metric verb of every group returns — the eight counts and the five distance metrics
// alike — and it is the stage the six threshold predicates — `should be below`, `should be above`, `should
// be`, `should be below or equal`, `should be above or equal`, `should satisfy` — are chained onto, so a
// rule about a number is this value plus a comparison. Measure is what it can do on its own: hand back the
// numbers themselves.
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
	// group is the word the metric was chosen out of — `count`, `distance` — kept so that the rule renders
	// as the sentence the user typed rather than as the scope and the metric with the group missing.
	group string
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
	if b.metric == nil {
		// The zero MetricBuilder names no metric, and a rule with no number to read measures nothing —
		// which is the answer the zero calculation.CountMetric gives to the same question, one layer down.
		return nil, nil
	}
	return b.metric.Measure(subjects), nil
}

// Selectors are the compiled scope verbs this rule was built from, in the order they were chained. They are
// the scope's own, unchanged: choosing a metric selects nothing.
func (b MetricBuilder) Selectors() []matching.Filter {
	return b.scope.Selectors()
}

// String renders the rule for logs and test failures, as `metrics, path without filename matches
// "internal/**", count, lines of code`.
func (b MetricBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.scope.rejected()
}

// stages are the parts of the sentence this rule has been built from: the scope's, then the group, then the
// metric.
//
// A part that is not there is left out rather than rendered as an empty word, so that the zero MetricBuilder
// — no group and no metric — reads as `metrics` instead of as a sentence with two gaps in it.
func (b MetricBuilder) stages() []string {
	stages := b.scope.stages()
	if b.group != "" {
		stages = append(stages, b.group)
	}
	if b.metric != nil {
		stages = append(stages, b.metric.Name())
	}
	return stages
}
