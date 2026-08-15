package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// countGroup is the word this group is called in the sentence a rule renders as. It is stated once, because a
// group that spelled itself in the stage it names and again in the builder it hands back could disagree with
// itself.
const countGroup = "count"

// MetricsCountBuilder is the stage between a metrics rule's scope and its number: `metrics, in folder
// "internal/**", count`, waiting for which of the eight counts the rule is about.
//
// It exists so that the eight verbs below are a group rather than eight more methods on the scope. `count`
// is what the family calls this group, and the families beside it — the distance metrics of
// MetricsDistanceBuilder, the cohesion metrics — are groups of their own, so a rule says which kind of number
// it means before it says which number: `count, method count` reads as one phrase, and the scope stage stays
// the stage about *where* the rule looks.
//
// A MetricsCountBuilder is immutable and carries the scope it was asked of unchanged. Every verb hands back
// a MetricBuilder, which is the stage that can be resolved.
type MetricsCountBuilder struct {
	// scope is the rule as it was described before the metric was chosen.
	scope MetricsBuilder
}

// Count opens the group of counting metrics: the eight numbers this library can take of a project as it is
// written. It reads nothing and decides nothing on its own — the verb after it is what picks the number.
func (b MetricsBuilder) Count() MetricsCountBuilder {
	return MetricsCountBuilder{scope: b}
}

// LinesOfCode counts the lines of each selected file that carry code: blank lines and comment-only lines
// left out, a line holding code and a trailing comment counted. It is the size of a file as a compiler
// would judge it, and the metric a rule about files that have grown too big is written with.
func (b MetricsCountBuilder) LinesOfCode() MetricBuilder {
	return b.measuring(calculation.LinesOfCode())
}

// Statements counts the statements of each selected file, at every depth. It is how much a file does rather
// than how long it is, so a rule written with it is not paid off by moving code onto fewer lines.
func (b MetricsCountBuilder) Statements() MetricBuilder {
	return b.measuring(calculation.Statements())
}

// Imports counts the import specs of each selected file, blank and dot imports included. It is how much of
// the rest of the world one file reaches for, which is the fan-out a rule about a file's coupling means.
func (b MetricsCountBuilder) Imports() MetricBuilder {
	return b.measuring(calculation.Imports())
}

// Functions counts the functions each selected file declares at package level. A method belongs to its type
// and is MethodCount's business, so the two never count the same declaration twice.
func (b MetricsCountBuilder) Functions() MetricBuilder {
	return b.measuring(calculation.Functions())
}

// Classes counts the types each selected file declares — structs, interfaces and names given to other types
// alike. It is the population `for classes matching` selects, counted per file.
func (b MetricsCountBuilder) Classes() MetricBuilder {
	return b.measuring(calculation.Classes())
}

// Interfaces counts how many of the types each selected file declares are interfaces. It is Classes narrowed
// to one kind, which is why a rule about interfaces needs no scope verb of its own.
func (b MetricsCountBuilder) Interfaces() MetricBuilder {
	return b.measuring(calculation.Interfaces())
}

// MethodCount counts the methods of each selected class: an interface's own members, and for every other type
// the functions declared with it as their receiver.
//
// It is the first of the two metrics about a class rather than a file, so its subjects are class identifiers —
// `internal/api.Handler` — and a rule usually pairs it with ForClassesMatching.
func (b MetricsCountBuilder) MethodCount() MetricBuilder {
	return b.measuring(calculation.MethodCount())
}

// FieldCount counts the fields each selected class declares, which is 0 for a class that is not a struct. It
// is a metric about a class, like MethodCount.
func (b MetricsCountBuilder) FieldCount() MetricBuilder {
	return b.measuring(calculation.FieldCount())
}

// String renders the rule as far as it has been described, as `metrics, path without filename matches
// "internal/**", count`.
func (b MetricsCountBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.scope.rejected()
}

// stages are the parts of the sentence this stage has been built from: the scope's, then `count`.
func (b MetricsCountBuilder) stages() []string {
	return append(b.scope.stages(), countGroup)
}

// measuring is every count verb: the scope it was asked of, the group it belongs to, and the metric the verb
// named. Which number a verb is, is the calculation.CountMetric it passes in, so no metric is defined twice —
// the eight are the eight factories in metrics/calculation and this stage only names them.
func (b MetricsCountBuilder) measuring(metric calculation.CountMetric) MetricBuilder {
	return MetricBuilder{scope: b.scope, group: countGroup, metric: metric}
}
