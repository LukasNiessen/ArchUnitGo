// Package extraction is the slices module's own half of the EXTRACT stage, and what it extracts is not
// Go: it is the component diagram an architect drew. A diagram is a second source of truth about the
// same project — one a human wrote on purpose — and a rule about slices can be judged against it the
// way it is judged against a pattern.
//
// Two functions read one, and they are the same reader twice:
//
//   - ParseDiagram, for a diagram that is a string in the test that asserts on it.
//   - ExtractDiagram, for a diagram that is a file beside the code, which is where a diagram belongs
//     once more than one test asks about it.
//
// The result of both is a Diagram: the components it declares and the dependencies it draws between
// them, as data. Nothing here judges anything — a drawn arrow is not a violation and a missing one is
// not either until a rule says which of the two it is looking for — and nothing here knows what a slice
// is. That keeps the parser testable against a string, and it is why this package holds no pattern, no
// projection and no violation type.
//
// The dialect is the component-diagram subset of PlantUML and no more of it: component declarations,
// arrows between components, comments, and the `@startuml`/`@enduml` frame. A line this package was not
// taught is refused with its number and its text rather than skipped, because a skipped line is a
// dependency nobody checks — and a diagram whose arrows are quietly half-read is worse than no diagram
// at all. ParseDiagram documents the whole grammar.
//
// This is the module's only impure package: ExtractDiagram reads a file. The parser it reads through is
// a function of a string, so everything about the dialect is testable without a filesystem, and the one
// call that touches the disk is three lines long.
package extraction

import (
	"slices"
	"strconv"
	"strings"
)

// Dependency is one arrow of a component diagram: the component at its tail, and the one at its head.
//
// It is the diagram's shape of a dependency, not the graph's: two names a human wrote, with no file, no
// import kind and no direction to interpret. `[api] --> [db]` says the api may depend on the db, and it
// says nothing whatever about the other direction — which is the same reading as
// `contain dependency("api", "db")`, so a diagram is a list of allowed pairs and a rule about one pair
// is the same question asked once.
type Dependency struct {
	// From is the name of the depending component, as the diagram spells it — `api`. It is matched
	// against a slice name exactly, so the diagram and the slicing have to agree on spelling.
	From string
	// To is the name of the component depended upon — `db`.
	To string
}

// Diagram is a component diagram as data: the components it declares, in the order it declared them,
// and the dependencies it draws between them.
//
// It is what both readers of this package produce and what the assertion judges a projection against.
// The order is the diagram's own, kept rather than sorted, because a report about a component the
// project has no slice for should name them in the order a reader will find them in the file.
//
// A Diagram is immutable: build one with NewDiagram or get one from ParseDiagram, and read it through
// its methods. The zero value is the diagram nobody drew — it declares nothing, draws nothing, and is
// Empty.
type Diagram struct {
	components   []string
	dependencies []Dependency
}

// NewDiagram builds the diagram that declares these components and draws these dependencies.
//
// The ends of every arrow are declared too, whether or not they are in the list. That is PlantUML's own
// rule — `[api] --> [db]` is a complete diagram of two components — and having it here rather than in
// the parser is what makes a hand-built diagram read exactly like a parsed one, which is what lets the
// assertion be tested without going through the text at all.
//
// Both lists are copied and de-duplicated, keeping the first occurrence of each: a diagram that names a
// component twice is one component, and a diagram that draws the same arrow twice allows one dependency.
// Copying is for the reason assertion.NewEmptyTestViolation copies its selectors — a diagram that has
// been read must not change when the caller reuses the slice it was built from.
//
// An arrow from a component to itself declares the component and is not kept as a dependency, because
// projection.ProjectEdges drops self-edges: the diagram carries the shape a projection can be compared
// against, so that nothing downstream has to know that one of the two sides never has such an edge.
func NewDiagram(components []string, dependencies ...Dependency) Diagram {
	diagram := Diagram{
		components:   make([]string, 0, len(components)+2*len(dependencies)),
		dependencies: make([]Dependency, 0, len(dependencies)),
	}
	for _, component := range components {
		diagram.components = declaring(diagram.components, component)
	}
	for _, dependency := range dependencies {
		diagram.components = declaring(diagram.components, dependency.From)
		diagram.components = declaring(diagram.components, dependency.To)
		if dependency.From == dependency.To || slices.Contains(diagram.dependencies, dependency) {
			continue
		}
		diagram.dependencies = append(diagram.dependencies, dependency)
	}
	return diagram
}

// Components are the names the diagram declares, in the order it declared them, including the ends of
// every arrow it draws. The result is a copy, so a caller may sort it without changing the diagram.
func (d Diagram) Components() []string {
	return slices.Clone(d.components)
}

// Dependencies are the arrows the diagram draws, in the order it drew them. The result is a copy, for
// the reason Components hands one out.
func (d Diagram) Dependencies() []Dependency {
	return slices.Clone(d.dependencies)
}

// Declares says whether the diagram has this component, matched by name exactly.
//
// A diagram is a closed statement about a project: a slice the diagram does not declare is a slice
// nobody drew, which is a finding rather than a dependency to look up. That question is asked of this
// method, so the assertion never walks the component list itself.
func (d Diagram) Declares(component string) bool {
	return slices.Contains(d.components, component)
}

// Draws says whether the diagram has an arrow from this component to that one, in that direction.
//
// The direction is the whole meaning of an arrow, so `[api] --> [db]` is not drawn from `db` to `api`.
// A component is never drawn as depending on itself: a slice may always depend on itself and a
// projection does not even carry that dependency, so an arrow that says so would be a rule about
// nothing. It is the diagram's own answer, and it is what makes an undrawn dependency a finding.
func (d Diagram) Draws(from, to string) bool {
	return slices.Contains(d.dependencies, Dependency{From: from, To: to})
}

// Empty says the diagram declares no component at all — the zero value, or a text that was all comments.
// It is what makes an empty diagram a refusal in ParseDiagram rather than a rule that holds forever.
func (d Diagram) Empty() bool {
	return len(d.components) == 0
}

// String renders the diagram as its two sizes — `3 components, 2 dependencies` — which is how a rule
// that was given a diagram as a string says what it was given.
//
// It is deliberately not the diagram: a rule renders as one line of a sentence, and pasting a whole
// document into the middle of one would bury the rule it is part of. Drawing a diagram is
// rendering.RenderPlantUML's job, and phrasing a violation about one is the testing layer's.
func (d Diagram) String() string {
	return counted(len(d.components), "component") + ", " + counted(len(d.dependencies), "dependency")
}

// declaring appends a component to a declaration list unless it is already in it, keeping the first
// occurrence: the order is the diagram's own, and a name declared twice is one component.
func declaring(components []string, component string) []string {
	if slices.Contains(components, component) {
		return components
	}
	return append(components, component)
}

// counted renders a number and its noun in the right number — `1 component`, `3 components` — with the
// `y`/`ies` plural the word `dependency` needs. It is the small half of what the testing layer's own
// pluralization does, for the one sentence this package renders.
func counted(number int, noun string) string {
	if number == 1 {
		return "1 " + noun
	}
	if strings.HasSuffix(noun, "y") {
		return strconv.Itoa(number) + " " + strings.TrimSuffix(noun, "y") + "ies"
	}
	return strconv.Itoa(number) + " " + noun + "s"
}
