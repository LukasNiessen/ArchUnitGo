package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// GatherNamingViolations judges the three self-contained rules about how the files a scope selected are
// named and where they live — `have name`, `be in folder`, `be in path` — in either mood: one
// NamingViolation per selected file the rule does not hold for, in the order the files arrived, which is
// the order projection.SelectFiles sorted them into.
//
// No offending file is no violations, which is the pass. A rule that selected nothing at all is the
// empty-test guard's answer rather than this one's: every file of an empty selection satisfies every
// requirement, so a stale glob would otherwise be green forever whichever mood it was written in.
//
// One function serves all three predicates and both moods, and it is the same walk either way. Which part
// of an identifier is looked at is the required filter's own match target, so `have name "*.go"` and
// `be in folder "internal/**"` differ here by nothing but the value passed in — and the mood is
// assertion.Mood.Holds over the one question the filter answers, so there is no negative code path to keep
// in step with the positive one:
//
//	should      violates when the file does not match the pattern
//	should not  violates when it does
//
// The filter is used exactly as the fluent stage compiled it — never inverted for the negated mood, which
// is what keeps Mood.Holds the one place anything is inverted, and never re-derived from the pattern
// string, because a domain module does not compile patterns. A zero matching.Filter matches nothing, so a
// rule written with one reports every selected file under `should`; a pattern a predicate could not compile
// never reaches here, because the terminal returns that as the user's error before the project is read.
func GatherNamingViolations(files []string, required matching.Filter, mood kernel.Mood) []kernel.Violation {
	violations := make([]kernel.Violation, 0, len(files))
	for _, file := range files {
		if mood.Holds(required.Matches(file)) {
			continue
		}
		violations = append(violations, NewNamingViolation(file, required, mood))
	}
	return violations
}
