package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
)

// Satisfaction is the rule a user writes about a number themselves: one question about one measurement,
// answered yes or no.
//
// It is the first argument of `should satisfy`, and it is handed both halves of what the library knows
// about the number it is asked about:
//
//	func(measurement archunit.Measurement, class archunit.MetricsClassInfo) bool {
//		return measurement.Value <= 10 || class.Interface
//	}
//
// The measurement is the number and what it is about — the metric's own name, the subject it was read
// off, the value — so a predicate can exempt one subject or read the figure, whichever metric the rule
// was written over.
//
// The class is the declared type the number was read off, and it is the zero extraction.ClassInfo for a
// metric that is not about a class: `count, lines of code` is a number about a file and `distance,
// abstractness` one about a package, and neither has a class for this to be. A predicate written for a
// class metric — the custom metrics, `method count`, `field count` — gets the whole of what this library
// extracted about it, which is the point of being handed it at all.
//
// It is asked once per measurement and nothing else is asked of it: it must not depend on how often or in
// which order it is called, because that is the library's business and both may change. `should satisfy`
// requires it to answer yes about every measurement the rule took.
type Satisfaction func(measurement calculation.Measurement, class extraction.ClassInfo) bool

// GatherSatisfactionViolations judges `metrics, ..., should satisfy` in either mood: one
// SatisfactionViolation per measurement the user's own predicate does not hold for, in the order the
// measurements arrived, which is the order the metric read its subjects in.
//
// No offending measurement is no violations, which is the pass. A rule that measured nothing at all is the
// empty-test guard's answer rather than this one's: every measurement of an empty list satisfies every
// predicate, in either mood, so a stale glob would otherwise be green forever.
//
// One function serves both moods, and it is the same walk either way — assertion.Mood.Holds over the one
// question the predicate answers, so there is no negative code path to keep in step with the positive one:
//
//	should      violates when the predicate says no about the measurement
//	should not  violates when it says yes
//
// Only `should` is offered by the fluent API, because the six threshold predicates AGENTS.md fixes spell
// their own mood and `should satisfy` is one of them. The mood is still threaded through rather than
// assumed, for the reason the library threads it everywhere: the day a family wants the other polarity,
// the assertion is already written, and until then this function has no branch that only one caller ever
// takes.
//
// The classes are the population the predicate's second argument is looked up in, by the identifier a
// measurement names its subject with. A measurement about a file or a package matches none of them and the
// predicate is handed the zero extraction.ClassInfo, which is what Satisfaction says it means: the number
// was not read off a class. The lookup is by identifier rather than by position because the measurements
// of a metric about a file are not the classes, one per one.
//
// The requirement travels along untouched, for the violation to carry: this function does not read it,
// because the words a rule was phrased in cannot be checked against anything. A nil predicate satisfies
// nothing, the way a nil files/assertion.FilePredicate does, so a rule written with one reports every
// measurement under `should` and none under `should not`. It is not reached from the fluent API, which
// returns a missing predicate as the user's error before the project is read.
func GatherSatisfactionViolations(
	measurements []calculation.Measurement,
	classes []extraction.ClassInfo,
	predicate Satisfaction,
	requirement string,
	mood kernel.Mood,
) []kernel.Violation {
	measured := classesByIdentifier(classes)
	violations := make([]kernel.Violation, 0, len(measurements))
	for _, measurement := range measurements {
		if mood.Holds(satisfies(predicate, measurement, measured[measurement.Subject])) {
			continue
		}
		violations = append(violations, NewSatisfactionViolation(
			measurement.Subject, measurement.Metric, measurement.Value, requirement, mood))
	}
	return violations
}

// satisfies asks the user's own function about one measurement, and answers no when there is no function
// to ask. Calling a nil predicate would take the host test process down with a panic, which is the one
// thing a library judging someone else's code must never do to them.
func satisfies(predicate Satisfaction, measurement calculation.Measurement, class extraction.ClassInfo) bool {
	if predicate == nil {
		return false
	}
	return predicate(measurement, class)
}

// classesByIdentifier indexes the selected classes by the identifier a measurement about one names it by,
// so that the predicate is handed a class without the walk being quadratic in the size of the selection.
//
// The first class of an identifier wins. Two declarations cannot share one — an identifier is the folder
// and the declared name, and a package may not declare a name twice — so this is a tie that only a
// hand-built selection can produce, and answering it in the order the classes were selected is the same
// answer twice.
func classesByIdentifier(classes []extraction.ClassInfo) map[string]extraction.ClassInfo {
	indexed := make(map[string]extraction.ClassInfo, len(classes))
	for _, class := range classes {
		if _, seen := indexed[class.Identifier]; !seen {
			indexed[class.Identifier] = class
		}
	}
	return indexed
}
