package extraction

import (
	"errors"
	"fmt"
	"strings"
)

// The two ways a diagram can fail to be one, as sentinels a caller can recognize with errors.Is. Both
// are reported as an archerror.UserError naming the step of the chain that was given the diagram: the
// library is working and the code has not been judged, there is simply no diagram to judge it against.
var (
	// ErrEmptyDiagram says the text held no component at all — an empty string, or nothing but comments
	// and the `@startuml`/`@enduml` frame. Every slice of the project would be one the diagram does not
	// declare, so the rule would report the whole project rather than what a reader meant to check, and
	// a diagram whose contents were lost in a copy-and-paste is the likeliest way to get one.
	ErrEmptyDiagram = errors.New("no component in the diagram")
	// ErrUnreadableDiagramLine says a line is not one of the four things a component diagram is made of.
	// The number and the text of the line are wrapped around it, because that is what a user needs in
	// order to fix it.
	//
	// It is an error rather than a skipped line on purpose: a line quietly ignored is a dependency
	// nobody checks, and a diagram that is half-read passes rules it never stated.
	ErrUnreadableDiagramLine = errors.New("unreadable diagram line")
)

const (
	// componentKeyword is PlantUML's own word for a component declaration, accepted in any case because
	// nothing else in a diagram is spelled twice.
	componentKeyword = "component"
	// diagramStart and diagramEnd are the frame a PlantUML document is wrapped in. They are skipped
	// rather than required: a diagram pasted into a test as a string is a diagram whether or not the
	// frame came with it.
	diagramStart = "@startuml"
	diagramEnd   = "@enduml"
	// lineComment starts a comment that runs to the end of the line.
	lineComment = "'"
	// blockCommentStart and blockCommentEnd delimit a comment that may span lines.
	blockCommentStart = "/'"
	blockCommentEnd   = "'/"
)

// ParseDiagram reads a component diagram out of text: the components it declares and the dependencies it
// draws between them.
//
//	diagram, err := extraction.ParseDiagram(`
//		@startuml
//		' the architecture we agreed on
//		component [api]
//		component [domain]
//		component [db]
//		[api] --> [domain]
//		[api] --> [db]
//		@enduml
//	`)
//
// The dialect is the component-diagram subset of PlantUML, read one line at a time, and this is the
// whole grammar:
//
//   - A blank line, and one holding nothing but whitespace, is nothing.
//   - `'` starts a comment that runs to the end of the line; `/'` opens one that runs until a line
//     holding `'/`. A block comment ends with the line that closes it, so a statement written after the
//     `'/` is part of the comment — put it on its own line.
//   - `@startuml` and `@enduml` are skipped, with or without a name after them, in any case.
//   - `component [api]`, `component api` and `[api]` each declare the component `api`. The keyword is
//     optional in the bracketed form and required in the bare one, which is what keeps a stray word from
//     being read as a component. A bracketed name may hold spaces; a bare one may not.
//   - `[api] --> [db]` and `[api] -> [db]` draw a dependency from `api` to `db`. Any number of dashes is
//     one arrow, either end may be bare, and a `: label` after the arrow is read and dropped — the label
//     is prose about the dependency and this library judges the dependency.
//
// Everything else is ErrUnreadableDiagramLine, wrapped with the line's number and its text: a `<--`
// arrow pointing backwards, `-up->` and its direction hints, `[a] --> [b] --> [c]`, an alias
// (`component [a] as A`), a stereotype, an interface, a `package` block, and every styling directive —
// `title`, `skinparam`, `!include`. A reader that skipped what it did not understand would turn a
// diagram it half-read into rules nobody wrote, so the dialect is exactly what is documented above and
// a line outside it is a rejected rule rather than a silent one. A directive a diagram needs for its
// looks can be commented out with `'` for this library's benefit; the arrows are what a rule is about.
//
// A text with no component in it at all is ErrEmptyDiagram, for the reason the empty-test guard exists:
// a diagram that declares nothing is not a diagram every project adheres to, it is a diagram somebody
// lost.
//
// The arrows' ends are components whether or not they were declared, which is PlantUML's rule as well —
// so the frame, the keyword and the declarations are all optional, and `[api] --> [db]` on its own is a
// diagram of two components and one dependency.
func ParseDiagram(text string) (Diagram, error) {
	var (
		components   []string
		dependencies []Dependency
		commented    bool
	)
	for index, line := range strings.Split(text, "\n") {
		statement := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if commented {
			commented = !strings.Contains(statement, blockCommentEnd)
			continue
		}
		switch {
		case statement == "", strings.HasPrefix(statement, lineComment):
			continue
		case strings.HasPrefix(statement, blockCommentStart):
			commented = !strings.Contains(strings.TrimPrefix(statement, blockCommentStart), blockCommentEnd)
			continue
		case framed(statement):
			continue
		}
		if strings.ContainsRune(statement, '>') {
			// The line means to be an arrow: it holds the head of one. Whether it is a well-formed arrow
			// is parseArrow's answer, and a malformed one is refused rather than read as a declaration —
			// a user who typed `[a] <-- [b]` wrote a dependency, not a component called `[a] <`.
			dependency, drawn := parseArrow(statement)
			if !drawn {
				return Diagram{}, unreadable(index, statement)
			}
			dependencies = append(dependencies, dependency)
			continue
		}
		component, declared := parseDeclaration(statement)
		if !declared {
			return Diagram{}, unreadable(index, statement)
		}
		components = append(components, component)
	}

	diagram := NewDiagram(components, dependencies...)
	if diagram.Empty() {
		return Diagram{}, ErrEmptyDiagram
	}
	return diagram, nil
}

// framed says the line is the `@startuml` or `@enduml` the document is wrapped in, with or without the
// diagram name PlantUML allows after either of them.
func framed(statement string) bool {
	keyword, _, _ := strings.Cut(statement, " ")
	return strings.EqualFold(keyword, diagramStart) || strings.EqualFold(keyword, diagramEnd)
}

// parseArrow reads one dependency out of a line that holds an arrow, and says whether the line was one.
//
// The arrow is found from its head backwards — the `>`, then the run of dashes before it — rather than
// from the first dash forwards, because a component name may hold a dash and `[my-api] --> [db]` has to
// mean what it says. A second `>` or a `<` on either side of the arrow is refused: those are two arrows
// on one line, an arrow pointing the other way, or a stereotype, and none of them is a dependency this
// dialect states.
func parseArrow(statement string) (Dependency, bool) {
	head := strings.IndexByte(statement, '>')
	tail := head
	for tail > 0 && statement[tail-1] == '-' {
		tail--
	}
	if tail == head {
		return Dependency{}, false
	}

	source, target := statement[:tail], statement[head+1:]
	if labeled := strings.IndexByte(target, ':'); labeled >= 0 {
		target = target[:labeled]
	}
	if strings.ContainsAny(source, "<>") || strings.ContainsAny(target, "<>") {
		return Dependency{}, false
	}

	from, named := parseEndpoint(source)
	to, pointed := parseEndpoint(target)
	if !named || !pointed {
		return Dependency{}, false
	}
	return Dependency{From: from, To: to}, true
}

// parseDeclaration reads the component a declaration line names, and says whether the line was one:
// `component [api]`, `component api` or `[api]`.
//
// The keyword is optional only in front of a bracketed name. A bare word on a line of its own is refused
// instead, because every line of a diagram that this reader does not understand looks like one — and a
// mistyped directive read as a component would add a component nobody drew.
func parseDeclaration(statement string) (string, bool) {
	declaration := statement
	keyword, named, hasKeyword := strings.Cut(declaration, " ")
	switch {
	case hasKeyword && strings.EqualFold(keyword, componentKeyword):
		declaration = named
	case strings.HasPrefix(declaration, "["):
		// The brackets already say that the name is a component's, so the keyword is what a diagram may
		// leave out — and `[api]` alone is a declaration in PlantUML too.
	default:
		return "", false
	}
	return parseEndpoint(declaration)
}

// parseEndpoint reads the name of one component out of the text around it — `[api]`, `[my api]` or
// `api` — and says whether that text named one.
//
// The brackets are PlantUML's way of saying that a name is a component's, and they are what allows a
// space inside one. A bare name is therefore a single word, and a name that is empty, or that holds a
// bracket of its own, is nothing this reader can hand on as a component.
func parseEndpoint(text string) (string, bool) {
	name := strings.TrimSpace(text)
	if strings.HasPrefix(name, "[") {
		if !strings.HasSuffix(name, "]") {
			return "", false
		}
		name = strings.TrimSpace(name[1 : len(name)-1])
		if name == "" || strings.ContainsAny(name, "[]") {
			return "", false
		}
		return name, true
	}
	if name == "" || strings.ContainsAny(name, "[] \t") {
		return "", false
	}
	return name, true
}

// unreadable is the refusal of one line, as the user has to see it: the sentinel, the number of the line
// counted from one, and the line itself as it was written. The index is the one the reader walked with,
// so the arithmetic that turns it into a line number happens once.
func unreadable(index int, statement string) error {
	return fmt.Errorf("%w %d: %q", ErrUnreadableDiagramLine, index+1, statement)
}
