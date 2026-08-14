package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// GatherExternalDependencyViolations judges `project files, ..., should, depend on external modules, ...` and
// its negation: one ExternalDependencyViolation per selected file the rule does not hold for, in the order the
// files arrived, which is the order projection.SelectFiles sorted them into.
//
// No offending file is no violations, which is the pass. A rule whose scope selected nothing at all is the
// empty-test guard's answer rather than this one's, for the reason GatherDependencyViolations gives. Its
// object, though, is not guarded and is not meant to be: this rule's object *is* a set of dependencies, so a
// pattern that matched no module and a project that depends on no such module are one statement — and for the
// negated mood, which is the one nearly every third-party policy is written in, that statement is exactly the
// pass.
//
// The dependencies arrive as the projected edges of projection.PerExternalDependencyEdge — from the files the
// scope selected, out of the project to the modules the object named — so the question this function asks of
// one file is already answered by whether it is the source of any of them:
//
//	should      violates when the file depends on none of the named modules
//	should not  violates when it depends on any of them
//
// That is one walk for both moods, with assertion.Mood.Holds the only comparison between them. The predicate
// is existential per file rather than per import, as it is for `depend on files`: one dependency satisfies the
// positive mood and one breaks the negated one, so the file is the subject either way and the import paths it
// was broken by travel on the violation instead of multiplying it.
//
// The walk is written out here rather than shared with GatherDependencyViolations, which is the same shape
// over a different violation type. Each `gather <thing> violations` function in this package states its own
// judgement in full — the naming and adherence pair already do — because what a family shares with another is
// the mood flag, and folding two families into one walk would make a change to either of their objects a
// change to both.
//
// The required filters are the object selectors, passed through untouched for the violation to carry. Nothing
// here matches with them: the modules they describe were resolved before the projection, by
// projection.SelectExternalModules, which is where the OR they are combined with lives.
func GatherExternalDependencyViolations(
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
		violations = append(violations, NewExternalDependencyViolation(file, required, found[file], mood))
	}
	return violations
}
