package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// EmptyTestPopulation is one of the populations a rule selected, as the empty-test guard is asked about
// it: what was being selected, how many nodes the selection came to, and the selectors that described
// them.
//
// A rule about one vocabulary has one population — the files, the slices, the layers its scope named. A
// relational rule usually has two, because `should not depend on files in folder "internal/db/**"` selects a
// subject and an object and either pattern can be the stale one; Subject is what tells them apart in a
// report, in the entry point's own words.
//
// Usually, because a relational rule whose object is not a population of the project's own nodes hands over
// its subject alone: the object of `should not depend on external modules matching "*.*/**"` *is* a set of
// dependencies, so "no module matched" and "no file depends on such a module" are one statement, and for the
// negated mood that statement is the pass. Which populations a rule has is the terminal's to say, and it says
// it by which ones it passes here.
//
// The selectors are read and not kept: CheckOptions.GatherEmptyTestViolations clones them on the way to
// the violation, for the reason assertion.NewEmptyTestViolation gives.
type EmptyTestPopulation struct {
	// Subject is what the rule was selecting, in its entry point's own vocabulary — `files`, `slices`,
	// `layers`, `files to depend on` — and it goes straight onto the violation. It is what lets a report
	// of a relational rule say which half of the sentence the user has to go and fix.
	Subject string
	// Matched is how many nodes this population came to. Zero is what the guard reports on, which is why
	// a terminal counts the selection rather than handing the guard the nodes themselves.
	Matched int
	// Selectors are the filters that, taken together, described this population, in the order the user
	// chained them. They are the data a reader needs in order to see which pattern was wrong, and they go
	// straight onto the violation.
	Selectors []matching.Filter
}

// GatherEmptyTestViolations is the empty-test guard as a terminal wires it in: one
// assertion.EmptyTestViolation per population of the rule that matched nothing, and no violations when
// every population matched something — or when the user opted out with AllowEmptyTests.
//
// It is the door every terminal in every module goes through, so that the highest-value defensive
// decision in the library — zero matches is a violation, not a pass — is made once and no rule family
// gets to be quietly lenient about it:
//
//	if empty := options.GatherEmptyTestViolations(rule.selection(len(selected))); len(empty) > 0 {
//		// A rule with no subject is reported instead of being judged.
//		return empty, nil
//	}
//
// Every empty population is reported rather than only the first, because both patterns of a relational
// rule are then wrong and a reader fixing one would come back for the other. A population the terminal
// does not pass is not guarded, and a terminal that passes none is not guarded at all — the guard cannot
// know what a rule was about, which is why AGENTS.md makes wiring it in a step of adding a rule and why
// the library holds its own terminals to having done it.
//
// The short-circuit stays with the terminal: this returns the violations of the empty populations, and it
// is the caller that reports them *instead of* judging anything, so that a report never says a rule both
// did and did not have a subject.
func (o *CheckOptions) GatherEmptyTestViolations(populations ...EmptyTestPopulation) []assertion.Violation {
	var violations []assertion.Violation
	for _, population := range populations {
		// Through EmptyTestOptions, so AllowEmptyTests is copied out of the user's bag in exactly one
		// place, and through the kernel's own guard, so the decision itself is made in exactly one.
		guard := o.EmptyTestOptions(population.Subject, population.Selectors...)
		violations = append(violations, assertion.GatherEmptyTestViolations(population.Matched, guard)...)
	}
	return violations
}
