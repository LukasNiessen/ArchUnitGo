// Package rendering is the slices module's half of the REPORT stage: it draws the slices of a project and
// the dependencies between them as a document somebody reads.
//
// One function is the whole of it, RenderPlantUML, and it is the reverse of the rule `adhere to diagram`:
// that rule reads a drawing and judges the project by it, this draws the project so that somebody can
// start from what is true. Between the two, a diagram is a format this library both reads and writes — which
// is what makes the drawn document a starting point for a rule rather than a picture beside one, and what
// the round-trip test in this package pins.
//
// It is pure, like every rendering package in the library: a function of the values the projection produced,
// with no filesystem, no clock and no globals in it. Writing the document to a file is the fluent API's
// `export as plantuml` terminal, and that seam is what lets the format be tested against a hand-built
// projection.
//
// The document is deterministic and holds no timestamp, so drawing the same project twice writes the same
// bytes and a diff between two of them names the dependency that appeared.
package rendering

import (
	"slices"
	"strconv"
	"strings"

	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// RenderPlantUML draws the slices of a project as a PlantUML component diagram:
//
//	@startuml
//	' 3 components, 2 dependencies
//	component [api]
//	component [db]
//	component [domain]
//	[api] --> [db]
//	[api] --> [domain]
//	@enduml
//
// The components are the slices the project has — the keys of projection.SelectSliceFiles — drawn in
// alphabetical order whatever order they arrive in, because they arrive as the keys of a map and a document
// that changed between two runs of the same suite could not be committed beside the code. A name that
// arrives twice is drawn once, for the same reason a diagram that declares one twice reads as one. The
// arrows are the projected dependencies in the order they arrive, which for projection.ProjectEdges is by
// source and then by target.
//
// What the document does not say is deliberate. There is no count on an arrow and no label: a component
// diagram is a statement about what may depend on what, and this one is meant to be read back — by
// extraction.ParseDiagram, whose dialect is exactly what this function draws, so a project's own diagram can
// be committed as the drawing the next run of `adhere to diagram` judges it against. A summary comment is
// the one thing above the components, because a diagram pasted into a document should say how big the thing
// it draws is.
//
// A dependency of a slice on itself is not drawn, because projection.ProjectEdges does not produce one, and
// a name is drawn as the projection produced it — with the brackets PlantUML reserves and anything that would
// break a line replaced, so that what is drawn can be read back.
//
// The result ends in a newline, holds no timestamp, and is a function of its two arguments alone.
func RenderPlantUML(components []string, dependencies []kernelprojection.ProjectedEdge) string {
	drawn := slices.Compact(slices.Sorted(slices.Values(components)))
	lines := make([]string, 0, len(drawn)+len(dependencies)+3)
	lines = append(lines,
		"@startuml",
		"' "+pluralize(len(drawn), "component", "components")+", "+
			pluralize(len(dependencies), "dependency", "dependencies"),
	)
	for _, component := range drawn {
		lines = append(lines, "component "+plantUMLComponent(component))
	}
	for _, dependency := range dependencies {
		lines = append(lines,
			plantUMLComponent(dependency.SourceLabel())+" --> "+plantUMLComponent(dependency.TargetLabel()))
	}
	lines = append(lines, "@enduml")
	return strings.Join(append(lines, ""), "\n")
}

// plantUMLComponent draws one component name in the brackets that say it is a component's.
//
// The brackets are what allows a space inside a name, and they are therefore what a name may not hold: a
// bracket of its own is drawn as a parenthesis, and so is an angle bracket, which PlantUML reads as the
// start of a stereotype or of an arrow. Anything that would break the line the name is drawn on — a newline,
// a tab, a carriage return — becomes a space, because a component per line is what makes the document
// readable back.
//
// It is this format's own escaping, in one place, the way every renderer in the library has one: a name that
// went out unescaped would draw a document that this library's own reader refuses.
func plantUMLComponent(name string) string {
	return "[" + strings.Map(plantUMLRune, name) + "]"
}

// plantUMLRune is the one substitution table plantUMLComponent maps a name through.
func plantUMLRune(character rune) rune {
	switch character {
	case '[', '<':
		return '('
	case ']', '>':
		return ')'
	case '\n', '\r', '\t':
		return ' '
	default:
		return character
	}
}

// pluralize is `1 component` and `2 components`, the one place this package decides how a count reads. It is
// the shape graph/projection's own has, for the reason given there: an English plural is not worth getting
// wrong twice, and a summary that says "1 components" is the first thing a reader distrusts.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(count) + " " + plural
}
