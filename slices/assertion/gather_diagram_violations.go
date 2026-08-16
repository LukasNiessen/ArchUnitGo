package assertion

import (
	"slices"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	slicesextraction "github.com/LukasNiessen/ArchUnitGo/slices/extraction"
)

// DiagramOptions are the two ways a diagram may be read less strictly than it is drawn. The zero value is
// the strict reading, in which every disagreement between the project and the drawing is reported.
//
// They are modifiers of the rule rather than of the diagram: the same drawing means the same thing, and
// what these change is which of the three findings a check reports. Neither of them can switch off
// FindingUndrawnDependency, because that is the finding a diagram is drawn for — a diagram that no longer
// says what may depend on what is not a diagram anybody should be checking against.
type DiagramOptions struct {
	// IgnoreOrphanSlices leaves out FindingUndeclaredSlice for a slice that no dependency reaches in either
	// direction: it imports nothing in the project and nothing in the project imports it.
	//
	// It is for the diagram that draws the architecture rather than the folder tree. A slice nothing depends
	// on and that depends on nothing says nothing about what may depend on what, so an architect may
	// reasonably leave it out of the drawing — while a slice that is an end of an arrow and is not in the
	// diagram is a hole in it.
	IgnoreOrphanSlices bool
	// IgnoreExternalSlices leaves out FindingAbsentComponent: a component the diagram declares that this
	// project has no slice for.
	//
	// It is for the diagram that is about more than this one project — a drawing of a system of several
	// modules, checked against each of them in turn — where a component this project does not have is not a
	// folder that went missing but somebody else's. Without it, such a drawing reports every component of
	// every sibling.
	IgnoreExternalSlices bool
}

// GatherDiagramViolations judges a project's slices against the component diagram somebody drew of them:
// one DiagramViolation per disagreement between the two, and none at all when the drawing and the code say
// the same thing.
//
// Three questions are asked, in this order, and the order is the order of the result:
//
//  1. Every projected dependency between two components the diagram declares: is the arrow drawn? An
//     undrawn one is FindingUndrawnDependency, carrying the files it was found through, in the
//     projection's own order.
//  2. Every slice the project has: does the diagram declare it? One that it does not is
//     FindingUndeclaredSlice, in alphabetical order, and DiagramOptions.IgnoreOrphanSlices leaves out the
//     ones no dependency reaches.
//  3. Every component the diagram declares: does the project have a slice by that name? One it has none
//     for is FindingAbsentComponent, in the order the diagram declared them, unless
//     DiagramOptions.IgnoreExternalSlices leaves them out.
//
// A dependency with an end the diagram does not declare is not reported as undrawn. The undeclared end is
// reported once instead, by the second question, because the drawing is missing a component rather than
// every arrow that component is at the end of — and a reader who adds it can then see which of its arrows
// are genuinely missing.
//
// This is where a diagram differs from `contain dependency`, and it is why the mood is not a parameter
// here: a diagram is a closed statement about a whole project, so `should adhere to diagram` is the only
// sentence there is. Its negation would be a rule asking that a project disagree with its own
// documentation somewhere, which is nothing anybody wants to be told is true — the same reason `have no
// cycles` is offered on the positive mood alone.
//
// The dependencies arrive as the projected edges of projection.SliceByCapture and its siblings, so a
// dependency inside one slice is not among them and neither is one with an end in no slice at all. present
// are the slices the project has — the keys of projection.SelectSliceFiles — in any order: they are sorted
// here, so the result is the same list however the caller came by them. A slicing that found no slice at
// all is the empty-test guard's answer rather than this one's, and that guard runs before this function, in
// the rule's terminal.
func GatherDiagramViolations(
	diagram slicesextraction.Diagram,
	dependencies []kernelprojection.ProjectedEdge,
	present []string,
	options DiagramOptions,
) []kernel.Violation {
	violations := make([]kernel.Violation, 0, len(dependencies))

	for _, dependency := range dependencies {
		from, to := dependency.SourceLabel(), dependency.TargetLabel()
		if !diagram.Declares(from) || !diagram.Declares(to) || diagram.Draws(from, to) {
			continue
		}
		violations = append(violations, NewUndrawnDependencyViolation(from, to, dependency.CumulatedEdges()...))
	}

	sliced := slices.Compact(slices.Sorted(slices.Values(present)))
	for _, slice := range sliced {
		if diagram.Declares(slice) || (options.IgnoreOrphanSlices && orphaned(slice, dependencies)) {
			continue
		}
		violations = append(violations, NewUndeclaredSliceViolation(slice))
	}

	if options.IgnoreExternalSlices {
		return violations
	}
	for _, component := range diagram.Components() {
		if slices.Contains(sliced, component) {
			continue
		}
		violations = append(violations, NewAbsentComponentViolation(component))
	}
	return violations
}

// orphaned says that no projected dependency reaches this slice in either direction: nothing in the
// project imports it and it imports nothing in the project.
//
// It is asked only of a slice the diagram does not declare, and only when the rule was told to ignore
// orphans, so the walk happens for the few slices a report is about rather than for every one of them.
// Both directions count, because a slice that is drawn nowhere and is an end of an arrow somewhere is the
// hole in a diagram that this modifier must not hide.
func orphaned(slice string, dependencies []kernelprojection.ProjectedEdge) bool {
	for _, dependency := range dependencies {
		if dependency.SourceLabel() == slice || dependency.TargetLabel() == slice {
			return false
		}
	}
	return true
}
