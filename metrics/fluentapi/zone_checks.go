package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
	metricsassertion "github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// MetricsZoneCondition is the terminal of a rule about where a project's packages sit in the abstractness/
// instability plane — `metrics, in folder "internal/**", distance, should not be in zone of pain` — and it is a
// fluentapi.Checkable, which is the one thing every consumer of a rule programs against:
//
//	rule := archunit.Metrics(nil).InFolder("internal/**").Distance().ShouldNotBeInZoneOfPain()
//	violations, err := rule.Check(nil)
//
// It is what the two zone verbs of MetricsDistanceBuilder return, it carries the scope it was asked of
// unchanged, and it is immutable like every stage before it — so a rule can be stored, passed to a helper and
// checked as often as it is useful. Nothing has been read when it is built: the project is located, extracted,
// projected and judged by Check, and by nothing else.
//
// It is the first rule of the metrics family, and the only one that closes the chain without a number in it:
// the threshold predicates judge one measurement against one figure the user typed, and a zone is a region of
// the plane two metrics span, so it is written as a predicate rather than as a comparison. Which of the two
// zones a rule means is the calculation.Zone in here, which is why one type serves both.
//
// The predicate has no object stage. `should not be in zone of pain` is a sentence on its own — the packages it
// is about are the ones the scope named — so this terminal is the end of the chain.
type MetricsZoneCondition struct {
	// scope is the rule as it was described before the zone was named. The group is not kept beside it: there
	// is one group these verbs hang off, and render below names it as the constant the group itself is named
	// from.
	scope MetricsBuilder
	// zone is the region of the plane the rule forbids, and the word a report calls it by.
	zone calculation.Zone
}

// ShouldNotBeInZoneOfPain forbids the corner of the plane where a package is concrete and depended upon:
// abstractness and instability both near 0, so it is rigid — much of the project breaks when it changes — and
// it offers no interface to depend on instead.
//
//	rule := archunit.Metrics(nil).InFolder("internal/**").Distance().ShouldNotBeInZoneOfPain()
//
// The corner is a quarter-circle rather than the point itself, so a package that is nearly all concrete and
// nearly all depended upon is reported too; calculation.ZoneOfPain says how wide.
//
// It exists in the negative mood alone, which is why there is no `ShouldBeInZoneOfPain`: that rule would demand
// that every selected package be badly designed, and nothing a user could write it for is worth the second
// verb. The mood still travels into the assertion rather than being assumed there, which
// metrics/assertion.GatherZoneViolations says the whole of why.
func (b MetricsDistanceBuilder) ShouldNotBeInZoneOfPain() MetricsZoneCondition {
	return b.forbidding(calculation.ZoneOfPain())
}

// ShouldNotBeInZoneOfUselessness forbids the opposite corner: abstractness and instability both near 1, a
// package of interfaces nothing depends on. Everything it declares is an abstraction with no implementation
// reaching for it, which is usually a design somebody built for a caller that never arrived, or one whose last
// caller was deleted.
//
//	rule := archunit.Metrics(nil).InFolder("internal/**").Distance().ShouldNotBeInZoneOfUselessness()
//
// It is negative-only for the same reason its sibling is, and it is the same walk with a different corner —
// calculation.ZoneOfUselessness is the whole difference between the two rules.
func (b MetricsDistanceBuilder) ShouldNotBeInZoneOfUselessness() MetricsZoneCondition {
	return b.forbidding(calculation.ZoneOfUselessness())
}

// Check runs the rule: one violation per selected package that sits in the forbidden zone, carrying its
// abstractness and its instability, and an empty result when none does, which is the pass. A nil *CheckOptions
// means the defaults.
//
// It is the whole pipeline in three steps — locate and extract the project, select the scope's files and the
// packages they make up, judge where each package sits — and, with Measure, one of the two stages of this family
// that read anything. The violations are the metrics module's own assertion.ZoneViolation values, or the one
// EmptyTestViolation of a scope that selected no file at all.
//
// It runs under kernel.CheckOptions.LoggedCheck, so a check that was asked for a log writes the rule, the count
// each of those steps came to, every violation and the outcome. With no log asked for, which is the default,
// nothing is written and nothing else about the check changes.
//
// A package's numbers are about the files the scope selected, so a rule about one folder measures that folder's
// dependencies on the rest of the selection and no others: a scope narrow enough to hide a package's dependents
// makes it look stable, and metrics/projection.PerComponentEdge is that reading written out. `metrics` with no
// scope verb is the whole project, which is the rule this predicate is usually written as.
//
// The error is technical or the user's — a pattern a scope verb could not compile, a locator naming no Go
// project, a project that will not load, a log this check was asked for that could not be opened, written or
// closed — and never a failing rule. When it is non-nil the violations say nothing.
func (c MetricsZoneCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	return options.LoggedCheck(c, func(log *logging.Logger) ([]assertion.Violation, error) {
		subjects, err := c.scope.resolve(options)
		if err != nil {
			return nil, err
		}
		log.LogProgress("selected components", len(subjects.Components))

		if empty := options.GatherEmptyTestViolations(c.population(len(subjects.Components))); len(empty) > 0 {
			// A rule with no subject is reported instead of being judged: no package selected means no package
			// in a zone, so every such rule would otherwise pass forever.
			return empty, nil
		}

		return metricsassertion.GatherZoneViolations(subjects.Components, c.zone, assertion.ShouldNot), nil
	})
}

// String renders the whole rule as the sentence the user typed, as `metrics, path without filename matches
// "internal/**", distance, should not be in zone of pain`.
func (c MetricsZoneCondition) String() string {
	stages := append(c.scope.stages(), distanceGroup, assertion.ShouldNot.String()+" be in "+c.zone.Name())
	return strings.Join(stages, ", ") + c.scope.rejected()
}

// population is the one population this rule selected, as the empty-test guard is asked about it. The subject is
// `components` and not `files`: a scope selecting a file whose folder holds nothing else still selects a
// package, so what a reader has to be told is the vocabulary the rule judged in.
func (c MetricsZoneCondition) population(matched int) kernel.EmptyTestPopulation {
	return kernel.EmptyTestPopulation{Subject: "components", Matched: matched, Selectors: c.scope.selectors}
}

// forbidding is both zone verbs: the scope they were asked of, and the region the verb named. The mood is not
// among them, because both verbs spell it and there is nothing else it could be; Check hands assertion.ShouldNot
// to the assertion, which is where the mood does its one job.
func (b MetricsDistanceBuilder) forbidding(zone calculation.Zone) MetricsZoneCondition {
	return MetricsZoneCondition{scope: b.scope, zone: zone}
}
