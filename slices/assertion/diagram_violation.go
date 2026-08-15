package assertion

import (
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// KindSliceDiagram is the kind of DiagramViolation: a project and the component diagram somebody drew of
// it disagree.
//
// It is a kind of its own rather than a second shape of KindSliceDependency, because the two answer
// different questions. A dependency rule is one pair of slices a user typed; a diagram is the whole
// architecture, so a check against one reports as many disagreements as there are, and each of them may
// be about a dependency, about a slice or about a component. The kind is what the testing layer picks a
// phrasing by, and the finding below is what it picks the words inside that phrasing by.
const KindSliceDiagram kernel.ViolationKind = "slice-diagram"

// DiagramFinding is which of the three ways a project and a diagram of it can disagree a DiagramViolation
// reports.
//
// There are three because a diagram states two things at once — which components exist and which of them
// may depend on which — and a project can contradict either of them, in one direction or the other. They
// are one violation type under one kind rather than three types, because a reader checking a project
// against a drawing wants one list of the ways the two do not match.
type DiagramFinding uint8

// The three ways a project and a diagram of it can disagree.
const (
	// FindingUndrawnDependency is a dependency the project has and the diagram does not draw: both slices
	// are in the drawing, and the arrow between them is not. It is the finding a diagram is drawn for, and
	// the only one no modifier switches off.
	FindingUndrawnDependency DiagramFinding = iota
	// FindingUndeclaredSlice is a slice the project has and the diagram does not declare. Every dependency
	// of such a slice would be undrawn too, so it is reported once, about the slice, and the arrows it is an
	// end of are left out — the drawing is missing a component, not a hundred arrows. `ignoring orphan
	// slices` leaves out the ones no dependency reaches in either direction.
	FindingUndeclaredSlice
	// FindingAbsentComponent is a component the diagram declares and the project has no slice for: a folder
	// that was renamed or deleted after the diagram was drawn, or one that was drawn before it was written.
	// `ignoring external slices` leaves it out, for a diagram that deliberately draws more than this one
	// project.
	FindingAbsentComponent
)

// String names the finding as reports refer to it — `undrawn dependency`, `undeclared slice`, `absent
// component` — and `unknown finding` for a value that is none of the three.
//
// It is the vocabulary and not the message: the sentence a user reads is the testing layer's, built from
// this violation's fields, so that one place controls phrasing, numbering and color.
func (f DiagramFinding) String() string {
	switch f {
	case FindingUndrawnDependency:
		return "undrawn dependency"
	case FindingUndeclaredSlice:
		return "undeclared slice"
	case FindingAbsentComponent:
		return "absent component"
	default:
		return "unknown finding"
	}
}

// DiagramViolation says that a project and the component diagram somebody drew of it disagree in one
// place: a dependency the drawing does not have, a slice the drawing does not declare, or a component the
// project does not have.
//
// It is what `project slices, defined by "internal/(**)/**", should, adhere to diagram(text)` reports, and
// there is one per disagreement rather than one per rule — a diagram is a statement about the whole
// project, so a check against one is a list of everything that does not match, in one run.
//
// Which of the three a violation is, is Finding, and it is what a report has to read first: the other
// fields mean what that finding says they mean. A dependency the diagram does not draw is about two
// slices and carries the files that made it; the other two are about one name and carry no files, because
// what they report is that something is not there at all.
type DiagramViolation struct {
	// Finding is which of the three disagreements this is, and the field a report reads before the others.
	Finding DiagramFinding
	// Slice is the name the finding is about: the depending slice of an undrawn dependency, the slice the
	// diagram does not declare, or the component the project has no slice for. It is spelled as the side
	// that has it spells it — the slicing for the first two, the diagram for the third — and the two are
	// matched exactly, so a diagram and a slicing that disagree about capitals disagree about everything.
	Slice string
	// DependsOn is the name of the slice depended upon, for FindingUndrawnDependency, and the empty string
	// for the other two findings: a slice nobody drew and a component nobody wrote are about one name.
	DependsOn string
	// Dependencies are the concrete file dependencies an undrawn dependency stands for: every extracted
	// edge from a file of Slice to a file of DependsOn, in the graph's own order.
	//
	// They are what a reader has to go and look at in order to decide whether to draw the arrow or delete
	// the import, and they are the reason a projected edge cumulates the raw edges it was built from —
	// after relabelling, the files are nowhere else. The other two findings carry none, and so does a
	// violation built by hand.
	Dependencies extraction.Graph
}

// NewUndrawnDependencyViolation records that the project depends from slice on dependsOn, through these
// extracted edges, and that the diagram draws no such arrow.
//
// The edges are copied through extraction.NewGraph, for the reason NewDependencyViolation copies its own:
// a violation that has been reported must not change when the projection it was found in is walked on,
// and a violation built from a hand-written list has to read exactly like one built from a projection.
func NewUndrawnDependencyViolation(slice, dependsOn string, dependencies ...extraction.Edge) DiagramViolation {
	return DiagramViolation{
		Finding:      FindingUndrawnDependency,
		Slice:        slice,
		DependsOn:    dependsOn,
		Dependencies: extraction.NewGraph(dependencies...),
	}
}

// NewUndeclaredSliceViolation records that the project has this slice and the diagram does not declare it.
//
// It carries no dependencies, and that is the finding rather than a gap in it: what a reader has to do is
// draw the component or fix the slicing, and the arrows it is an end of are not reported until it is one
// of the components the diagram is about.
func NewUndeclaredSliceViolation(slice string) DiagramViolation {
	return DiagramViolation{Finding: FindingUndeclaredSlice, Slice: slice}
}

// NewAbsentComponentViolation records that the diagram declares this component and the project has no
// slice by that name.
//
// It carries no dependencies either: there is no code to point at, which is the whole of what it says. The
// name is the diagram's own spelling, because that is the text the reader has to go and change.
func NewAbsentComponentViolation(component string) DiagramViolation {
	return DiagramViolation{Finding: FindingAbsentComponent, Slice: component}
}

// Kind is KindSliceDiagram.
func (DiagramViolation) Kind() kernel.ViolationKind {
	return KindSliceDiagram
}

// String renders the violation as what it is about, the finding, and the dependencies it was found
// through — `api -> db: undrawn dependency (internal/api/handler.go -> internal/db/conn.go)`, `ui:
// undeclared slice`, `cache: absent component` — for a log line or a test failure.
//
// It is the finding's own vocabulary rather than a sentence to show a user: the message is the testing
// layer's to build, from Finding, Slice, DependsOn and Dependencies.
func (v DiagramViolation) String() string {
	subject := v.Slice
	if v.DependsOn != "" {
		subject += " -> " + v.DependsOn
	}
	return subject + ": " + v.Finding.String() + v.drawn()
}

// drawn renders the concrete dependencies an undrawn arrow stands for, in parentheses, and the empty
// string when there are none — the two findings about a name alone, or a violation built by hand.
func (v DiagramViolation) drawn() string {
	if len(v.Dependencies) == 0 {
		return ""
	}
	rendered := make([]string, 0, len(v.Dependencies))
	for _, edge := range v.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return " (" + strings.Join(rendered, ", ") + ")"
}
