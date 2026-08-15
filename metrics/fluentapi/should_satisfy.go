package fluentapi

import (
	"errors"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	metricsassertion "github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
)

// satisfyVerb is the predicate this file adds to the grammar, spelled once: it is the word a rejection names
// and the word the rendered sentence ends with, and those two must not be able to disagree.
const satisfyVerb = "should satisfy"

// ErrNoPredicate is the reason `should satisfy` was rejected: it is the one threshold predicate whose
// comparison *is* a Go function, so a chain that passes none has said nothing about the numbers it measured.
// The fix is to pass the function.
var ErrNoPredicate = errors.New("no predicate function given")

// ErrNoRequirement is the reason `should satisfy` was rejected: the message beside the function is missing or
// blank. It is not decoration — a report cannot print a closure, so those words are the whole of what a
// failure would be able to say about what the number should have been, and a rule nobody can read from its
// output is a rule nobody can fix.
var ErrNoRequirement = errors.New("no requirement given to describe the predicate")

// MetricsSatisfactionCondition is the terminal of the metrics rule a user writes the comparison of themselves
// — `metrics, for classes matching "*Service", count, method count, should satisfy "be at most 10 methods
// wide"` — and it is a fluentapi.Checkable, which is the one thing every consumer of a rule programs against:
//
//	rule := archunit.Metrics(nil).
//		ForClassesMatching("*Service").
//		Count().
//		MethodCount().
//		ShouldSatisfy(func(measurement archunit.Measurement, class archunit.MetricsClassInfo) bool {
//			return measurement.Value <= 10 || class.Interface
//		}, "be at most 10 methods wide unless it is an interface")
//	violations, err := rule.Check(nil)
//
// It is what ShouldSatisfy returns, and it is the escape hatch of this family's predicates: the other five
// threshold predicates compare one measurement against one figure the user typed, and this one hands the
// measurement over and lets them decide — an exemption, a rule that depends on the subject, a comparison
// between the number and something else about the class.
//
// It carries the rule it was asked of unchanged — the scope, the group and the metric — plus the function and
// the words describing it, and it is immutable like every stage before it, so a rule can be stored, passed to
// a helper and checked as often as it is useful. Nothing is read when it is built and the predicate is not
// called: the project is located, extracted, measured and judged by Check, and by nothing else.
//
// There is no object stage and no mood stage. `should satisfy` spells its own mood, as all six threshold
// predicates do, and what is asked of the numbers is the user's own function — so this terminal ends the
// chain.
type MetricsSatisfactionCondition struct {
	// rule is the scope, the group and the metric the predicate was asked of: everything that says which
	// numbers this rule is about.
	rule MetricBuilder
	// predicate is the user's own function, kept as it was given. It is nil only for a rule that was
	// rejected, which no check reaches: the rejection is returned as an error first.
	predicate metricsassertion.Satisfaction
	// requirement is the message the user wrote beside the function, for the sentence String renders and for
	// the violations to carry. It is what stands in for a comparison that cannot be printed.
	requirement string
}

// ShouldSatisfy is the threshold predicate that holds a rule's numbers to a comparison the user writes
// themselves: `should satisfy`.
//
//	rule := archunit.Metrics(nil).
//		InFolder("internal/**").
//		Count().
//		LinesOfCode().
//		ShouldSatisfy(func(measurement archunit.Measurement, _ archunit.MetricsClassInfo) bool {
//			return measurement.Value <= 400 || strings.HasSuffix(measurement.Subject, "_gen.go")
//		}, "be at most 400 lines long unless it is generated")
//
// The function is asked once about each number the metric read, and is handed both halves of what the library
// knows about it: the archunit.Measurement — the metric's name, the subject it was read off and the value —
// and the class it was read off, which is the zero archunit.MetricsClassInfo for a metric about a file or a
// package, because neither has a class. metrics/assertion.Satisfaction is where that pair is described in
// full. `should satisfy` requires the answer to be yes about every measurement the rule took.
//
// The message is the second half of the predicate rather than a nicety. The other five threshold predicates
// render the figure they compare against — `should be below 400` — but a Go function is not printable, so
// these words are what the rule reads as in a log line and what every violation carries. Phrase them as a bare
// infinitive, so that the mood reads onto them: `be at most 400 lines long`, not `file is too long`.
//
// This is the family's escape hatch, and it is deliberately not the first tool to reach for: a rule that
// compares one number against one figure is expressible as one of the other five threshold predicates, and
// those print the figure they wanted. Reach for this one when the comparison is not a threshold — an
// exemption for generated code, a limit that depends on the class, a rule over two of a class's numbers at
// once.
//
// It exists in the positive mood alone, which is why there is no `ShouldNotSatisfy`: the six threshold
// predicates AGENTS.md fixes are the whole of this family's grammar and each spells its own mood, and a
// predicate the user writes is negated by writing it the other way round. The mood still travels into the
// assertion rather than being assumed there, which metrics/assertion.GatherSatisfactionViolations says the
// whole of why.
//
// Two things are rejected rather than judged, and neither is a rule failure — a missing function, which is a
// rule that says nothing, and a missing message, which is a failure nobody could read. Both are returned by
// Check as a UserError naming `should satisfy`, before the project is read.
func (b MetricBuilder) ShouldSatisfy(predicate metricsassertion.Satisfaction, requirement string) MetricsSatisfactionCondition {
	condition := MetricsSatisfactionCondition{rule: b, predicate: predicate, requirement: requirement}
	switch {
	case predicate == nil:
		condition.rule.scope = b.scope.rejecting(satisfyVerb, requirement, ErrNoPredicate)
	case strings.TrimSpace(requirement) == "":
		condition.rule.scope = b.scope.rejecting(satisfyVerb, requirement, ErrNoRequirement)
	}
	return condition
}

// Check runs the rule: one violation per measurement the user's predicate does not hold for, carrying the
// number that was found, and an empty result when every one of them satisfies it, which is the pass. A nil
// *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, select the scope's files and the
// classes and packages they make up, read the metric off every subject, ask the user's function about each
// number — and the only stage of the chain that reads anything or calls the predicate.
//
// The metric is read only once the project has been selected, and the predicate is asked only once the
// selection is known to have produced a number: a rule whose glob matches nothing calls the user's function
// not at all and is reported as the empty test it is.
//
// The violations are the metrics module's own assertion.SatisfactionViolation values, each carrying the
// subject, the metric, the number, the requirement as the user phrased it and the mood — or the one
// EmptyTestViolation of a rule that measured nothing.
//
// The error is technical or the user's — a missing function or message, a pattern a scope verb could not
// compile, a locator naming no Go project, a project that will not load — and never a failing rule. When it
// is non-nil the violations say nothing.
func (c MetricsSatisfactionCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	subjects, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}

	measurements := c.rule.readings(subjects)
	if empty := options.GatherEmptyTestViolations(c.population(len(measurements))); len(empty) > 0 {
		// A rule with no number is reported instead of being judged: no measurement means no measurement
		// the predicate says no about, so every such rule would otherwise pass forever.
		return empty, nil
	}

	return metricsassertion.GatherSatisfactionViolations(
		measurements, subjects.Classes, c.predicate, c.requirement, assertion.Should), nil
}

// String renders the whole rule as the sentence the user typed, as `metrics, classname matches "*Service",
// count, method count, should satisfy "be at most 10 methods wide"`.
//
// The message is what stands in for the comparison, because a closure has no readable form: this is the one
// threshold predicate the library cannot print, and the reason ShouldSatisfy insists on being given words for
// it.
func (c MetricsSatisfactionCondition) String() string {
	stages := append(c.rule.stages(), assertion.Should.String()+` satisfy "`+c.requirement+`"`)
	return strings.Join(stages, ", ") + c.rule.scope.rejected()
}

// population is the one population this rule selected, as the empty-test guard is asked about it.
//
// The subject is `measurements` and not `files` or `classes`: which population a metric reads is the metric's
// own business — a count about a file reads the files and one about a class reads the classes — so what a
// reader has to be told is that the rule ended up with no number to judge, whichever of the two its scope
// selected. A scope that selected files declaring no class is exactly that case, and the selectors are the
// data that says which pattern to go and look at.
func (c MetricsSatisfactionCondition) population(matched int) kernel.EmptyTestPopulation {
	return kernel.EmptyTestPopulation{
		Subject:   "measurements",
		Matched:   matched,
		Selectors: c.rule.scope.selectors,
	}
}
