package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/extraction"
)

// FilePredicate is the rule a user writes themselves: a question about one file, answered yes or no.
//
// It is the first argument of `adhere to`, and everything it may ask about is on the extraction.FileInfo it
// is handed — where the file is, what it is called, its full source text, how many of its lines carry
// something:
//
//	func(file archunit.FileInfo) bool { return strings.Contains(file.Source, "context.Context") }
//
// It is asked once per selected file and nothing else is asked of it: it must not depend on how often or
// in which order it is called, because that is the library's business and both may change. `should` demands
// that it answer yes for every selected file; `should not` demands that it never does.
type FilePredicate func(file extraction.FileInfo) bool

// GatherAdherenceViolations judges `project files, ..., should, adhere to` and its negation: one
// AdherenceViolation per selected file the user's own predicate does not hold for, in the order the files
// arrived, which is the order projection.SelectFiles sorted the selection into.
//
// No offending file is no violations, which is the pass. A rule that selected nothing at all is the
// empty-test guard's answer rather than this one's: every file of an empty selection satisfies every
// predicate, in either mood, so a stale glob would otherwise be green forever.
//
// One function serves both moods, and it is the same walk either way — assertion.Mood.Holds over the one
// question the predicate answers, so there is no negative code path to keep in step with the positive one:
//
//	should      violates when the predicate says no about the file
//	should not  violates when it says yes
//
// The predicate is called exactly once per file, with the file the fluent stage read, and its answer is
// never inverted here — inverting is Mood.Holds's job and nothing else's. The requirement travels along
// untouched, for the violation to carry: this function does not read it, because the words a rule was
// phrased in cannot be checked against anything.
//
// A nil predicate satisfies nothing, the way a zero matching.Filter matches nothing in
// GatherNamingViolations, so a rule written with one reports every selected file under `should` and none
// under `should not`. It is not reached from the fluent API, which returns a missing predicate as the
// user's error before the project is read.
func GatherAdherenceViolations(
	files []extraction.FileInfo,
	predicate FilePredicate,
	requirement string,
	mood kernel.Mood,
) []kernel.Violation {
	violations := make([]kernel.Violation, 0, len(files))
	for _, file := range files {
		if mood.Holds(satisfies(predicate, file)) {
			continue
		}
		violations = append(violations, NewAdherenceViolation(file.Path, requirement, mood))
	}
	return violations
}

// satisfies asks the user's own function about one file, and answers no when there is no function to ask.
// Calling a nil predicate would take the host test process down with a panic, which is the one thing a
// library judging someone else's code must never do to them.
func satisfies(predicate FilePredicate, file extraction.FileInfo) bool {
	if predicate == nil {
		return false
	}
	return predicate(file)
}
