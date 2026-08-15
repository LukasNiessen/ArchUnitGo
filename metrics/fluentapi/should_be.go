package fluentapi

import (
	"errors"
	"math"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	metricsassertion "github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// The five predicates this file adds to the grammar, spelled once each: they are the words a rejection names and
// the words the rendered sentence ends with, and those two must not be able to disagree.
//
// These five and `should satisfy` are the whole of this family's threshold predicates. AGENTS.md fixes the six,
// and the reason it fixes them is that a synonym — `should equal`, `should be at most`, `should be less than` —
// is how a fluent API stops sounding like one language: two spellings of one comparison mean every reader of a
// suite has to learn which of them the author picked.
const (
	belowVerb        = "should be below"
	aboveVerb        = "should be above"
	exactlyVerb      = "should be"
	belowOrEqualVerb = "should be below or equal"
	aboveOrEqualVerb = "should be above or equal"
)

// ErrLimitNotANumber is the reason a threshold predicate was rejected: the figure it was given is not a number,
// so the rule compares its measurements against nothing.
//
// NaN is the one float64 that is on no side of itself: every comparison against it is false, so a rule written
// with one would report every number it measured and never say why. That is a rule nobody could act on rather
// than a rule the code has broken, which is why it is the user's own error. An infinite figure is not rejected —
// `should be below +Inf` is the rule that a count is finite at all, and somebody could mean it.
var ErrLimitNotANumber = errors.New("the figure to compare against is not a number")

// MetricsThresholdCondition is the terminal of a metrics rule that holds its numbers to a figure the user typed
// — `metrics, in folder "internal/**", count, lines of code, should be below 400` — and it is a
// fluentapi.Checkable, which is the one thing every consumer of a rule programs against:
//
//	rule := archunit.Metrics(nil).
//		InFolder("internal/**").
//		Count().
//		LinesOfCode().
//		ShouldBeBelow(400)
//	violations, err := rule.Check(nil)
//
// It is what five of the six threshold predicates return — `should be below`, `should be above`, `should be`,
// `should be below or equal`, `should be above or equal` — because what differs between them is the comparison
// and not the rule: which side of the figure satisfies is the calculation.Threshold in here, exactly as which of
// the two corners a zone rule means is the calculation.Zone in MetricsZoneCondition. The sixth predicate,
// `should satisfy`, has its own terminal, because a comparison the user wrote is a function rather than a figure.
//
// It carries the rule it was asked of unchanged — the scope, the group and the metric — plus the comparison, and
// it is immutable like every stage before it, so a rule can be stored, passed to a helper and checked as often as
// it is useful. Nothing is read when it is built: the project is located, extracted, measured and judged by
// Check, and by nothing else.
//
// There is no object stage and no mood stage. Each of the five verbs spells its own mood, as all six threshold
// predicates do, and the figure is the verb's argument — so this terminal ends the chain.
type MetricsThresholdCondition struct {
	// rule is the scope, the group and the metric the comparison was asked of: everything that says which
	// numbers this rule is about.
	rule MetricBuilder
	// threshold is the figure the numbers are held to and which side of it satisfies. The verb is not kept
	// beside it: the comparison already carries the words the sentence renders, so a rule cannot render as one
	// verb and be judged as another.
	threshold calculation.Threshold
}

// ShouldBeBelow requires every number the rule measured to be strictly under this figure: `should be below`.
//
//	rule := archunit.Metrics(nil).InFolder("internal/**").Count().LinesOfCode().ShouldBeBelow(400)
//
// `below 400` holds for 399 and not for 400, which is the reading of a limit somebody wrote as a maximum plus
// one. ShouldBeBelowOrEqual is the same rule stated as the maximum itself, and one of the two is what a limit
// already written down in a style guide means — this library does not guess which, and offers no third spelling
// of either.
func (b MetricBuilder) ShouldBeBelow(limit float64) MetricsThresholdCondition {
	return b.comparing(belowVerb, calculation.Below(limit))
}

// ShouldBeAbove requires every number the rule measured to be strictly over this figure: `should be above`.
//
//	rule := archunit.Metrics(nil).InFolder("internal/**").Count().Statements().ShouldBeAbove(0)
//
// It is the floor half of the family, and the rule a number that must not collapse is written with — a package
// with no statements in it, a class nothing reaches. ShouldBeAboveOrEqual is the same rule stated as the minimum
// itself.
func (b MetricBuilder) ShouldBeAbove(limit float64) MetricsThresholdCondition {
	return b.comparing(aboveVerb, calculation.Above(limit))
}

// ShouldBe requires every number the rule measured to be this figure exactly: `should be`.
//
//	rule := archunit.Metrics(nil).InFolder("internal/port").Distance().Abstractness().ShouldBe(1)
//
// It is the comparison for the numbers that are a decision rather than a budget — a package of nothing but
// interfaces is abstractness 1, a file that declares one class counts one — and it is spelled `should be` and
// never `should equal`, which is the synonym AGENTS.md names first. The figure follows the verb with no
// comparison word between them, because the equality is the comparison.
//
// It compares float64 values as they are, so a rule written over a ratio asks for a number that divides out
// exactly. `should be 1` and `should be 0` are the two figures a ratio reaches on purpose; for anything in
// between, the rule somebody means is one of the four inequalities.
func (b MetricBuilder) ShouldBe(limit float64) MetricsThresholdCondition {
	return b.comparing(exactlyVerb, calculation.Exactly(limit))
}

// ShouldBeBelowOrEqual requires every number the rule measured to be under this figure or at it: `should be
// below or equal`.
//
//	rule := archunit.Metrics(nil).ForClassesMatching("*Service").Count().MethodCount().ShouldBeBelowOrEqual(10)
//
// `below or equal 10` holds for 10 and not for 11, which is the reading of a limit written as a maximum. It is
// spelled this way and not `should be at most`, which is the synonym AGENTS.md names second: the four
// inequalities are one pair of words apart from each other here, and a reader who has learned one has learned
// all four.
func (b MetricBuilder) ShouldBeBelowOrEqual(limit float64) MetricsThresholdCondition {
	return b.comparing(belowOrEqualVerb, calculation.BelowOrEqual(limit))
}

// ShouldBeAboveOrEqual requires every number the rule measured to be over this figure or at it: `should be above
// or equal`.
//
//	rule := archunit.Metrics(nil).InFolder("internal/**").Distance().Instability().ShouldBeAboveOrEqual(0.2)
//
// `above or equal 0.2` holds for 0.2 and not for 0.19, which is the reading of a minimum. It is the mirror of
// ShouldBeBelowOrEqual and the fifth of the six predicates the family has.
func (b MetricBuilder) ShouldBeAboveOrEqual(limit float64) MetricsThresholdCondition {
	return b.comparing(aboveOrEqualVerb, calculation.AboveOrEqual(limit))
}

// Check runs the rule: one violation per measurement that is not on the side of the figure the comparison
// requires, carrying the number that was found and the figure it was held to, and an empty result when every
// number is where it should be, which is the pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, select the scope's files and the
// classes and packages they make up, read the metric off every subject, compare each number against the figure —
// and the only stage of the chain that reads anything.
//
// The metric is read only once the project has been selected, and the numbers are judged only once the selection
// is known to have produced one: a rule whose glob matches nothing compares nothing at all and is reported as the
// empty test it is.
//
// The violations are the metrics module's own assertion.ThresholdViolation values, each carrying the subject, the
// metric, the number, the comparison in the words the rule was written in, the figure and the mood — or the one
// EmptyTestViolation of a rule that measured nothing.
//
// The error is technical or the user's — a figure that is not a number, a pattern a scope verb could not compile,
// a locator naming no Go project, a project that will not load — and never a failing rule. When it is non-nil the
// violations say nothing.
func (c MetricsThresholdCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	subjects, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}

	measurements := c.rule.readings(subjects)
	if empty := options.GatherEmptyTestViolations(c.population(len(measurements))); len(empty) > 0 {
		// A rule with no number is reported instead of being judged: no measurement means no measurement on the
		// wrong side of the figure, so every such rule would otherwise pass forever.
		return empty, nil
	}

	return metricsassertion.GatherThresholdViolations(measurements, c.threshold, assertion.Should), nil
}

// String renders the whole rule as the sentence the user typed, as `metrics, path without filename matches
// "internal/**", count, lines of code, should be below 400` — and, for the comparison that is the equality
// itself, as `..., should be 1`, because the figure is the whole of what follows `be` there.
func (c MetricsThresholdCondition) String() string {
	stages := append(c.rule.stages(), assertion.Should.String()+" be "+c.threshold.String())
	return strings.Join(stages, ", ") + c.rule.scope.rejected()
}

// population is the one population this rule selected, as the empty-test guard is asked about it.
//
// The subject is `measurements` and not `files` or `classes`, for the reason MetricsSatisfactionCondition's is:
// which population a metric reads is the metric's own business, so what a reader has to be told is that the rule
// ended up with no number to compare, whichever of the two its scope selected.
func (c MetricsThresholdCondition) population(matched int) kernel.EmptyTestPopulation {
	return kernel.EmptyTestPopulation{
		Subject:   "measurements",
		Matched:   matched,
		Selectors: c.rule.scope.selectors,
	}
}

// comparing is all five verbs: the rule they were asked of, and the comparison the verb named. The mood is not
// among them, because each of the five spells it and there is nothing else it could be; Check hands
// assertion.Should to the assertion, which is where the mood does its one job.
//
// The one thing rejected rather than judged is a figure that is not a number, and it is not a rule failure: it is
// returned by Check as a UserError naming the verb the user typed, before the project is read.
func (b MetricBuilder) comparing(verb string, threshold calculation.Threshold) MetricsThresholdCondition {
	condition := MetricsThresholdCondition{rule: b, threshold: threshold}
	if math.IsNaN(threshold.Limit()) {
		condition.rule.scope = b.scope.rejecting(verb, threshold.String(), ErrLimitNotANumber)
	}
	return condition
}
