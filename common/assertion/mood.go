package assertion

// Mood is which of the library's two moods a rule was written in — `should` or `should not` — and it
// is a bool because that is the whole of it.
//
// The two moods are one code path. A `gather <thing> violations` function takes a Mood, asks its
// predicate the positive question about each subject — does this file depend on that folder, does this
// slice have a cycle — and hands the answer to Holds; the negated rule is then the same walk over the
// same structure with one comparison inverted. That is why the negative half of the fluent API costs
// almost nothing to maintain, and why no assertion is allowed to branch on the mood itself: a
// duplicated negative path would double the bug surface of every rule family in the library.
//
// There are exactly two moods, in every ArchUnit port, and they have no synonyms — no `must`, no
// `may`, no `never`, no `is not`. A fluent API stops sounding like one language the moment the same
// sentence can be written two ways.
type Mood bool

const (
	// Should is the positive mood: the rule holds for a subject its predicate is satisfied by.
	Should Mood = false
	// ShouldNot is the negated mood: the rule holds for a subject its predicate is *not* satisfied
	// by. It is the same rule, read the other way round, and not a second rule.
	ShouldNot Mood = true
)

// Negated reports whether this is the negated mood, `should not`. It is the flag itself, for a
// report that has to say which mood a rule was written in; the question an assertion asks of a
// subject is Holds.
func (m Mood) Negated() bool {
	return bool(m)
}

// Holds answers the whole of what a mood does: given that a subject satisfies the rule's predicate,
// does the rule hold for that subject?
//
// `should` holds where the predicate is satisfied and `should not` holds where it is not, so this is
// one comparison — and it is the only place in the library that inverts anything. Every gather
// function has the shape
//
//	for _, file := range selected {
//		if mood.Holds(dependsOnTheDatabase(file)) {
//			continue
//		}
//		violations = append(violations, NewFileDependencyViolation(file))
//	}
//
// so a subject the rule does not hold for is one violation, and the predicate is written once for
// both moods.
func (m Mood) Holds(satisfied bool) bool {
	return satisfied != m.Negated()
}

// String is the mood in the two words the user typed, `should` or `should not`, for a rule that
// renders itself into a log line or a test failure.
//
// This is not the prose of a violation message — that is the testing layer's, so that one place
// controls phrasing. It is the sentence the user wrote, read back.
func (m Mood) String() string {
	if m.Negated() {
		return "should not"
	}
	return "should"
}
