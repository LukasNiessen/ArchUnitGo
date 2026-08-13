package assertion

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// EmptyTestViolation says a rule selected nothing: its scope matched no node at all, so there was
// nothing to judge and a pass would have meant nothing.
//
// It is the most valuable defensive check in the library, because the alternative is silence. A
// selector that matches nothing is almost always a mistake — a renamed folder, a stale glob, a rule
// still pointed at code that has moved — and a rule with an empty subject is green forever. Every
// terminal wires the guard in; `allowEmptyTests` on the check options is how a user who really means
// an empty selection opts out.
type EmptyTestViolation struct {
	// Subject is what the rule was selecting, in its entry point's own vocabulary: `files`,
	// `slices`, `layers`. It is empty when the caller has nothing more precise to say.
	Subject string
	// Selectors are the filters that, taken together, described the empty set, in the order the
	// user chained them. They are the data a reader needs in order to see which pattern was wrong,
	// and each one already knows the part of an identifier it looks at.
	Selectors []matching.Filter
}

// NewEmptyTestViolation records that selectors, taken together, selected no subject at all.
//
// The selectors are copied: spreading a caller's slice into a variadic parameter shares its backing
// array, and a violation that has been reported must not change afterwards.
func NewEmptyTestViolation(subject string, selectors ...matching.Filter) EmptyTestViolation {
	return EmptyTestViolation{Subject: subject, Selectors: slices.Clone(selectors)}
}

// Kind is KindEmptyTest.
func (EmptyTestViolation) Kind() ViolationKind {
	return KindEmptyTest
}

// EmptyTestOptions are the knobs on the empty-test guard: what the rule was selecting, what it
// selected with, and whether an empty selection is allowed at all. A nil *EmptyTestOptions means the
// defaults — the guard on, with nothing to say about what was selected.
type EmptyTestOptions struct {
	// Subject is what the rule was selecting, and goes straight onto the violation.
	Subject string
	// Selectors are the filters the rule's scope was built from, and go straight onto the
	// violation.
	Selectors []matching.Filter
	// AllowEmptyTests switches the guard off, making an empty selection a pass. It comes straight
	// from the user's check options and is false by default, because a rule that selected nothing
	// is far more often a typo than an intention.
	AllowEmptyTests bool
}

// GatherEmptyTestViolations is the empty-test guard: given how many nodes a rule's scope matched, it
// returns one EmptyTestViolation when that count is zero, and no violations otherwise.
//
// It lives in the kernel because the decision that zero matches is a violation rather than a pass is
// made once, for every rule family, and no terminal gets to be quietly lenient about it. Every
// terminal calls it before judging anything.
func GatherEmptyTestViolations(matched int, options *EmptyTestOptions) []Violation {
	if matched > 0 {
		return nil
	}
	if options == nil {
		return []Violation{NewEmptyTestViolation("")}
	}
	if options.AllowEmptyTests {
		return nil
	}
	return []Violation{NewEmptyTestViolation(options.Subject, options.Selectors...)}
}
