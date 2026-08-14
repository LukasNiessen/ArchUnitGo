package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// GatherDependencyViolations judges `project files, ..., should, depend on files, ...` and its negation: one
// DependencyViolation per selected file the rule does not hold for, in the order the files arrived, which is
// the order projection.SelectFiles sorted them into.
//
// No offending file is no violations, which is the pass. A rule whose scope, or whose object, selected
// nothing at all is the empty-test guard's answer rather than this one's: a file that depends on none of an
// empty set breaks the positive mood and satisfies the negated one, so a renamed folder would otherwise turn
// half the rules written about it green forever and the other half red for every file.
//
// The dependencies arrive as the projected edges of projection.PerDependencyEdge — from the files the scope
// selected, to the files the object named — so the question this function asks of one file is already
// answered by whether it is the source of any of them:
//
//	should      violates when the file depends on none of the object's files
//	should not  violates when it depends on any of them
//
// That is one walk for both moods, with assertion.Mood.Holds the only comparison between them, and it is why
// the negated rule needs no code of its own. The predicate is deliberately existential per file rather than
// per import: `should depend on files` is satisfied by one dependency and `should not depend on files` is
// broken by one, so the file is the subject either way and the edges it was broken by travel on the violation
// instead of multiplying it.
//
// The required filters are the object selectors, passed through untouched for the violation to carry. Nothing
// here matches with them — the population they describe was resolved before the projection, because the AND
// they are combined with lives in projection.SelectFiles and an object is a set of files rather than of
// strings.
func GatherDependencyViolations(
	files []string,
	dependencies []kernelprojection.ProjectedEdge,
	required []matching.Filter,
	mood kernel.Mood,
) []kernel.Violation {
	found := make(map[string][]string, len(files))
	for _, dependency := range dependencies {
		source := dependency.SourceLabel()
		found[source] = append(found[source], dependency.TargetLabel())
	}

	violations := make([]kernel.Violation, 0, len(files))
	for _, file := range files {
		if mood.Holds(len(found[file]) > 0) {
			continue
		}
		violations = append(violations, NewDependencyViolation(file, required, found[file], mood))
	}
	return violations
}
