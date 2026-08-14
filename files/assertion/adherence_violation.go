package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// KindFileAdherence is the kind of AdherenceViolation: a file that does not satisfy a predicate the user
// wrote themselves.
//
// It names the vocabulary as well as the failure, the way KindFileNaming does, because every vocabulary
// the library grows can be judged by a user's own function — a slice, a layer, a package — and each of
// them reports its own type. The kind is what the testing layer picks a phrasing by.
const KindFileAdherence kernel.ViolationKind = "file-adherence"

// AdherenceViolation says that one of the files a rule selected does not satisfy the predicate the user
// gave it — or does satisfy it, where the rule forbade it.
//
// It is what `project files, ..., should, adhere to` reports, one per offending file, and it carries the
// file, the requirement as the user phrased it and the mood.
//
// The requirement is prose, and this is the one violation in the library where that is unavoidable rather
// than a mistake: the rule was a Go function, so there is nothing else to carry. That is why `adhere to`
// takes the message alongside the function — a report cannot describe a closure, and a violation with
// nothing to say would leave a reader with only a file name and no idea what was asked of it.
type AdherenceViolation struct {
	// File is the identifier of the file the rule does not hold for, as the graph spells it —
	// `internal/db/conn.go`.
	File string
	// Requirement is what the rule asked of each selected file, in the user's own words: the message
	// argument of `adhere to`, phrased as a bare infinitive so that the mood reads onto it — `be at most
	// 400 lines long`.
	Requirement string
	// Mood is which way round the requirement was written. Satisfying the predicate is what `should`
	// demands and what `should not` forbids, so without the mood a report could not tell one failure from
	// the other — and the same file and requirement describe both.
	Mood kernel.Mood
}

// NewAdherenceViolation records that this file does not satisfy the predicate of a rule written in this
// mood, and what that predicate was asking for.
//
// It is the only way a violation of this family is made, and every field of it is immutable: two strings
// and a flag. The user's function is deliberately not among them — a violation is a value a report reads,
// and a closure is neither printable nor safe to call a second time.
func NewAdherenceViolation(file, requirement string, mood kernel.Mood) AdherenceViolation {
	return AdherenceViolation{File: file, Requirement: requirement, Mood: mood}
}

// Kind is KindFileAdherence.
func (AdherenceViolation) Kind() kernel.ViolationKind {
	return KindFileAdherence
}

// String renders the violation as the offending file and then the requirement it broke, in the words the
// rule was written in — `internal/db/conn.go: should, adhere to "be at most 400 lines long"` — for a log
// line or a test failure.
//
// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. The user-facing message is
// still the testing layer's to build, from File, Requirement and Mood.
func (v AdherenceViolation) String() string {
	return v.File + ": " + v.Mood.String() + `, adhere to "` + v.Requirement + `"`
}
