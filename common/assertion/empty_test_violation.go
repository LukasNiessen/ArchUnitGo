package assertion

import (
	"slices"
	"strings"

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

// String renders the violation as what the rule was selecting and the selectors that came to nothing —
// `files: path without filename matches "internal/apis/**" -> nothing` — for a log line or a test
// failure.
//
// It is the shape every violation in the library renders itself in: the subject that disagreed with the
// rule first, because it is the thing to go and look at, then the requirement in the words the rule was
// written in, then what was found. Here the requirement is the selection and what was found is nothing,
// which is the whole of what this violation says.
//
// The user-facing message is still the testing layer's to build, from Subject and Selectors — it is the
// one violation whose report also has to explain why an empty selection is a failure at all.
func (v EmptyTestViolation) String() string {
	subject := v.Subject
	if subject == "" {
		subject = "the rule's subject"
	}
	if len(v.Selectors) == 0 {
		return subject + " -> nothing"
	}
	clauses := make([]string, 0, len(v.Selectors))
	for _, selector := range v.Selectors {
		clauses = append(clauses, selector.String())
	}
	return subject + ": " + strings.Join(clauses, ", ") + " -> nothing"
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
