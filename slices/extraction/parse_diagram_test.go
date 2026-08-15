package extraction_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/slices/extraction"
)

func TestParseDiagramReadsAWholeComponentDiagram(t *testing.T) {
	// The document an architect draws, with everything the dialect holds in it: the frame, a comment, the
	// declarations and the arrows. This is the one test that says what the whole grammar comes to.
	text := `@startuml
' the architecture we agreed on
component [api]
component [domain]
component [db]

[api] --> [domain]
[api] --> [db]
@enduml
`

	diagram, err := extraction.ParseDiagram(text)
	if err != nil {
		t.Fatalf("reading the diagram failed with %v, want it read", err)
	}

	wantComponents := []string{"api", "domain", "db"}
	if got := diagram.Components(); !slices.Equal(got, wantComponents) {
		t.Errorf("the components are %v, want %v in the order they were declared", got, wantComponents)
	}
	wantDependencies := []extraction.Dependency{{From: "api", To: "domain"}, {From: "api", To: "db"}}
	if got := diagram.Dependencies(); !slices.Equal(got, wantDependencies) {
		t.Errorf("the dependencies are %v, want %v", got, wantDependencies)
	}
}

func TestParseDiagramReadsEveryFormOfTheDialect(t *testing.T) {
	// One case per clause of the documented grammar, each reading the same one dependency, so that a form the
	// parser stops accepting fails here by name.
	want := extraction.Dependency{From: "api", To: "db"}
	tests := []struct {
		name string
		text string
	}{
		{"the long arrow", "[api] --> [db]"},
		{"the short arrow", "[api] -> [db]"},
		{"a longer arrow still", "[api] ----> [db]"},
		{"an arrow with no space around it", "[api]-->[db]"},
		{"bare endpoints", "api --> db"},
		{"one bare end", "[api] --> db"},
		{"a labeled arrow", "[api] --> [db] : reads"},
		{"declarations before the arrow", "component [api]\ncomponent [db]\n[api] --> [db]"},
		{"a declaration with no keyword", "[api]\n[db]\n[api] --> [db]"},
		{"a bare declaration behind the keyword", "component api\ncomponent db\n[api] --> [db]"},
		{"the keyword in another case", "Component [api]\nCOMPONENT [db]\n[api] --> [db]"},
		{"the frame in another case", "@StartUml\n[api] --> [db]\n@EndUml"},
		{"a named diagram", "@startuml the architecture\n[api] --> [db]\n@enduml"},
		{"a line comment", "' the api reads the db\n[api] --> [db]"},
		{"a block comment over two lines", "/' the api reads\n   the db '/\n[api] --> [db]"},
		{"a block comment on one line", "/' agreed in ADR 7 '/\n[api] --> [db]"},
		{"windows line endings", "component [api]\r\ncomponent [db]\r\n[api] --> [db]\r\n"},
		{"indentation and blank lines", "\n\t[api] --> [db]   \n\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagram, err := extraction.ParseDiagram(test.text)
			if err != nil {
				t.Fatalf("reading %q failed with %v, want it read", test.text, err)
			}

			if got := diagram.Dependencies(); !slices.Equal(got, []extraction.Dependency{want}) {
				t.Errorf("%q draws %v, want exactly %v", test.text, got, want)
			}
		})
	}
}

func TestParseDiagramKeepsANameThatHoldsWhatAFolderNameHolds(t *testing.T) {
	// The names in a diagram are slice names, and a slice is a folder often enough that a dash and a dot have
	// to survive the arrow. The arrow is found from its head backwards for exactly this reason: scanning
	// forwards for the first dash would cut `my-api` in half.
	diagram, err := extraction.ParseDiagram("[my-api] --> [db.v2]\ncomponent [a name with spaces]")
	if err != nil {
		t.Fatalf("reading the diagram failed with %v, want it read", err)
	}

	// Declared components come before the ends of an arrow nobody declared, which is NewDiagram's order
	// rather than the text's.
	want := []string{"a name with spaces", "my-api", "db.v2"}
	if got := diagram.Components(); !slices.Equal(got, want) {
		t.Errorf("the components are %v, want %v", got, want)
	}
	if !diagram.Draws("my-api", "db.v2") {
		t.Errorf("the diagram draws %v, want my-api -> db.v2", diagram.Dependencies())
	}
}

func TestParseDiagramDeclaresTheEndsOfAnArrowNobodyDeclared(t *testing.T) {
	// A diagram of nothing but arrows is a diagram, in PlantUML and here: the declarations are what a diagram
	// may leave out, so `adhere to diagram` over two arrows is a rule about three components.
	diagram, err := extraction.ParseDiagram("[api] --> [domain]\n[api] --> [db]")
	if err != nil {
		t.Fatalf("reading the diagram failed with %v, want it read", err)
	}

	want := []string{"api", "domain", "db"}
	if got := diagram.Components(); !slices.Equal(got, want) {
		t.Errorf("the components are %v, want %v", got, want)
	}
}

func TestParseDiagramRefusesALineItWasNotTaught(t *testing.T) {
	// The one decision this parser is built around: a line it does not understand is a refusal, not a line to
	// skip. A skipped line is a dependency nobody checks, and a diagram that is half-read passes rules it never
	// stated — so every form below is a rejected rule the user can see and fix.
	tests := []struct {
		name string
		text string
	}{
		{"an arrow pointing backwards", "[db] <-- [api]"},
		{"an arrow with a direction hint", "[api] -down-> [db]"},
		{"a dotted arrow", "[api] ..> [db]"},
		{"two arrows on one line", "[api] --> [domain] --> [db]"},
		{"an arrow with no head", "[api] -- [db]"},
		{"an arrow with nothing at its tail", "--> [db]"},
		{"an arrow with nothing at its head", "[api] -->"},
		{"an arrow with an unclosed end", "[api --> [db]"},
		{"an arrow with a bracket inside a name", "[api]] --> [db]"},
		{"an arrow with two components at one end", "[api] --> [domain] [db]"},
		{"an alias", "component [api] as theAPI"},
		{"a stereotype", "component [db] <<database>>"},
		{"an interface", "() \"reads\" as reads"},
		{"a package block", "package internal {"},
		{"a title", "title the architecture we agreed on"},
		{"a styling directive", "skinparam monochrome true"},
		{"an include", "!include architecture.iuml"},
		{"a bare word on a line of its own", "api"},
		{"an empty pair of brackets", "[]"},
		{"a comment that was never opened", "the api reads the db '"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagram, err := extraction.ParseDiagram(test.text)

			if !errors.Is(err, extraction.ErrUnreadableDiagramLine) {
				t.Fatalf("reading %q failed with %v, want ErrUnreadableDiagramLine", test.text, err)
			}
			if !diagram.Empty() {
				t.Errorf("the refused diagram holds %v, want nothing: half a diagram is worse than none", diagram.Components())
			}
		})
	}
}

func TestARefusedLineIsReportedByItsNumberAndItsText(t *testing.T) {
	// What a user needs in order to fix it, and the reason the refusal is wrapped rather than bare: the line
	// number counts from one, over every line of the text including the ones that were skipped.
	text := "@startuml\n' a comment\ncomponent [api]\n[api] ..> [db]\n@enduml"

	_, err := extraction.ParseDiagram(text)
	if err == nil {
		t.Fatal("reading a diagram with a dotted arrow succeeded, want it refused")
	}

	message := err.Error()
	for _, want := range []string{"4", `"[api] ..> [db]"`} {
		if !strings.Contains(message, want) {
			t.Errorf("the refusal reads %q, want it to name %s", message, want)
		}
	}
}

func TestParseDiagramRefusesADiagramWithNoComponentInIt(t *testing.T) {
	// The empty-test guard's reasoning, applied to a diagram: one that declares nothing is not a diagram every
	// project adheres to, it is a diagram somebody lost in a copy-and-paste. Every slice of the project would
	// be one it does not declare, so the report would be about the project rather than about the drawing.
	tests := []struct {
		name string
		text string
	}{
		{"nothing at all", ""},
		{"whitespace", "\n\t\n   \n"},
		{"an empty frame", "@startuml\n@enduml\n"},
		{"nothing but comments", "@startuml\n' we will draw this later\n/' or not '/\n@enduml\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagram, err := extraction.ParseDiagram(test.text)

			if !errors.Is(err, extraction.ErrEmptyDiagram) {
				t.Fatalf("reading %q failed with %v, want ErrEmptyDiagram", test.text, err)
			}
			if !diagram.Empty() {
				t.Errorf("the refused diagram holds %v, want nothing", diagram.Components())
			}
		})
	}
}

func TestParseDiagramReadsABlockCommentToTheEndOfTheLineThatClosesIt(t *testing.T) {
	// The documented restriction of the dialect: a block comment ends with the line that closes it, so a
	// statement written after the `'/` is part of the comment. Reading half a line as a comment and half as a
	// statement is the kind of guessing that turns a diagram into rules nobody wrote.
	diagram, err := extraction.ParseDiagram("/' hidden\n'/ [api] --> [db]\ncomponent [api]")
	if err != nil {
		t.Fatalf("reading the diagram failed with %v, want it read", err)
	}

	if got := diagram.Dependencies(); len(got) != 0 {
		t.Errorf("the diagram draws %v, want nothing: the arrow is inside the comment", got)
	}
	if got := diagram.Components(); !slices.Equal(got, []string{"api"}) {
		t.Errorf("the components are %v, want only the one declared after the comment", got)
	}
}
