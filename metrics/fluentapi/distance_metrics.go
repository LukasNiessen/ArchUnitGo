package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// distanceGroup is the word this group is called in the sentence a rule renders as, stated once for the reason
// countGroup is.
const distanceGroup = "distance"

// MetricsDistanceBuilder is the stage between a metrics rule's scope and a number about a package: `metrics,
// in folder "internal/**", distance`, waiting for which of the five the rule is about — or for one of the two
// zone checks, which are the rules this group can close with on its own.
//
// It is the group of Robert C. Martin's package metrics: `abstractness` and `instability`, which are the two
// axes of a plane, `distance from the main sequence` and `normalized distance`, which say how far from the
// line A + I = 1 a package sits, and `coupling factor` beside them. All five are about a component — a folder
// of the project, with the types it declares and the packages it depends on — which is why they are a group of
// their own rather than five more verbs under `count`: a count is about one file or one class, and none of
// these has an answer for either.
//
// The two zone checks are here for the same reason. A zone is a corner of the plane the first two metrics
// span, so `should not be in zone of pain` is a rule about exactly this group's numbers, and it needs no
// metric verb before it because it is about both axes at once.
//
// A MetricsDistanceBuilder is immutable and carries the scope it was asked of unchanged. Every metric verb
// hands back a MetricBuilder, which is the stage that can be resolved; the two zone checks hand back a
// MetricsZoneCondition, which is a rule that can be checked.
type MetricsDistanceBuilder struct {
	// scope is the rule as it was described before the group was opened.
	scope MetricsBuilder
}

// Distance opens the group of metrics about a package: the five numbers this library takes of a component, and
// the two zone checks over the plane the first two of them span. It reads nothing and decides nothing on its
// own — the verb after it is what picks the number or the rule.
//
// A component is a folder holding files the scope selected, and its numbers are about those files: how many
// types they declare, how many of those are interfaces, and which other selected folders they depend on. A
// rule about one package therefore has to select enough of the project for that package's dependencies to be
// visible, which metrics/projection.PerComponentEdge says in full.
func (b MetricsBuilder) Distance() MetricsDistanceBuilder {
	return MetricsDistanceBuilder{scope: b}
}

// Abstractness measures A of each selected package: the share of the types it declares that are interfaces, 0
// for a package of nothing but concrete types and 1 for a package of nothing but interfaces.
//
// Go has no abstract class, so an interface is what an abstract type is here. It is one axis of the plane the
// zone checks are about, and the metric a rule about a package that promises nothing is written with.
func (b MetricsDistanceBuilder) Abstractness() MetricBuilder {
	return b.measuring(calculation.Abstractness())
}

// Instability measures I of each selected package: the share of its couplings that are its own outgoing ones,
// 0 for a package nothing but its own dependents can break and 1 for one nothing depends on.
//
// It is the other axis of the plane, and it counts only the packages the scope selected — a dependency on a
// folder the rule left out is not a coupling this number knows about.
func (b MetricsDistanceBuilder) Instability() MetricBuilder {
	return b.measuring(calculation.Instability())
}

// DistanceFromMainSequence measures D of each selected package: how far it sits from the line where
// abstractness and instability balance, as the perpendicular distance to it.
//
// 0 is a package on the line and 1/√2 ≈ 0.707 is the worst there is, at either corner the line misses.
// NormalizedDistance is the same question on a scale of 0 to 1, and it is usually the one to write a threshold
// against.
func (b MetricsDistanceBuilder) DistanceFromMainSequence() MetricBuilder {
	return b.measuring(calculation.DistanceFromMainSequence())
}

// NormalizedDistance measures D' of each selected package: the distance from the main sequence stated on a
// scale of 0 to 1, where 0 is a package on the line and 1 is one at either corner off it.
func (b MetricsDistanceBuilder) NormalizedDistance() MetricBuilder {
	return b.measuring(calculation.NormalizedDistance())
}

// CouplingFactor measures the share of the couplings each selected package could have that it actually has:
// how many of the other selected packages it depends on or is depended on by, over twice how many others there
// are.
//
// 0 is a package connected to nothing and 1 is one coupled to every other package in both directions. It is
// the MOOD coupling factor asked of one package at a time, because a measurement is a number about a subject a
// report can name.
func (b MetricsDistanceBuilder) CouplingFactor() MetricBuilder {
	return b.measuring(calculation.CouplingFactor())
}

// String renders the rule as far as it has been described, as `metrics, path without filename matches
// "internal/**", distance`.
func (b MetricsDistanceBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.scope.rejected()
}

// stages are the parts of the sentence this stage has been built from: the scope's, then `distance`.
func (b MetricsDistanceBuilder) stages() []string {
	return append(b.scope.stages(), distanceGroup)
}

// measuring is every metric verb of this group: the scope it was asked of, the group it belongs to, and the
// metric the verb named. Which number a verb is, is the calculation.DistanceMetric it passes in, so no formula
// is written twice.
func (b MetricsDistanceBuilder) measuring(metric calculation.DistanceMetric) MetricBuilder {
	return MetricBuilder{scope: b.scope, group: distanceGroup, metric: metric}
}
