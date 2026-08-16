package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	slicesassertion "github.com/LukasNiessen/ArchUnitGo/slices/assertion"
	slicesextraction "github.com/LukasNiessen/ArchUnitGo/slices/extraction"
	"github.com/LukasNiessen/ArchUnitGo/slices/fluentapi"
)

// A rule about a diagram is a Checkable like every other rule in the library, so a suite that collects its
// rules in one list can hold this one beside them.
var _ kernel.Checkable = fluentapi.SlicesDiagramCondition{}

// fixtureDiagramText is the drawing the fixture project adheres to: its three folders under internal/, and the
// two dependencies the api has. It is written the way an architect would write it, with a comment and the frame
// around it, so that the dialect the parser reads is exercised through the public chain too.
const fixtureDiagramText = `
' the architecture of the fixture project
@startuml
component [api]
component [db]
component [domain]
[api] --> [db]
[api] --> [domain]
@enduml
`

func TestAProjectThatAdheresToItsDiagramReportsNothing(t *testing.T) {
	// The pass, and what makes a diagram worth drawing: one rule, and it says that every dependency the project
	// has is an arrow somebody drew, every slice is a component, and every component is a slice.
	rule := fixtureSlicing(t, writeSlicedFixtureProject(t)).Should().AdhereToDiagram(fixtureDiagramText)

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 0 {
		t.Errorf("the project disagrees with its diagram in %v, want it to adhere", messages(t, violations))
	}
}

func TestADependencyTheDiagramDoesNotDrawIsReportedWithTheFilesThatMadeIt(t *testing.T) {
	// The finding a diagram is drawn for. Both slices are in the drawing and the arrow between them is not, so
	// the violation carries the concrete imports: that list is what a reader decides between drawing the arrow
	// and deleting the import by, and after the relabelling it lives nowhere else.
	rule := fixtureSlicing(t, writeSlicedFixtureProject(t)).Should().AdhereToDiagram(`
		@startuml
		component [api]
		component [db]
		component [domain]
		[api] --> [domain]
		@enduml
	`)

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("the rule reported %v, want the one dependency the diagram does not draw", messages(t, violations))
	}
	violation := diagramViolation(t, violations[0])
	if violation.Finding != slicesassertion.FindingUndrawnDependency {
		t.Errorf("the violation is a %s, want an %s", violation.Finding, slicesassertion.FindingUndrawnDependency)
	}
	if violation.Slice != "api" || violation.DependsOn != "db" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", violation.Slice, violation.DependsOn, "api", "db")
	}
	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if made := madeBy(violation); !slices.Equal(made, want) {
		t.Errorf("the dependency was made by %v, want %v", made, want)
	}
	if kind := violation.Kind(); kind != slicesassertion.KindSliceDiagram {
		t.Errorf("the violation is of kind %q, want %q", kind, slicesassertion.KindSliceDiagram)
	}
}

func TestTheDrawingsThisRuleDisagreesWithAProjectAbout(t *testing.T) {
	// The three findings, each through the public chain: an arrow nobody drew, a slice nobody declared, and a
	// component the project has no slice for. One rule reports all of them in one run, which is the whole
	// difference from `contain dependency` — forty of those are forty rules nobody keeps up to date.
	root := writeSlicedFixtureProject(t)
	tests := []struct {
		name    string
		diagram string
		want    []string
	}{
		{
			name:    "an arrow the drawing is missing",
			diagram: "@startuml\ncomponent [api]\ncomponent [db]\ncomponent [domain]\n[api] --> [db]\n@enduml",
			want:    []string{"api -> domain: undrawn dependency (internal/api/handler.go -> internal/domain/order.go)"},
		},
		{
			name:    "a slice the drawing does not declare",
			diagram: "@startuml\ncomponent [api]\ncomponent [db]\n[api] --> [db]\n@enduml",
			want:    []string{"domain: undeclared slice"},
		},
		{
			name:    "a component the project has no slice for",
			diagram: fixtureDiagramText + "\ncomponent [cache]\n",
			want:    []string{"cache: absent component"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule := fixtureSlicing(t, root).Should().AdhereToDiagram(test.diagram)

			violations, err := rule.Check(nil)
			if err != nil {
				t.Fatalf("checking %s failed: %v", rule, err)
			}
			if got := diagramFindings(t, violations); !slices.Equal(got, test.want) {
				t.Errorf("the project disagrees with its diagram in %v, want %v", got, test.want)
			}
		})
	}
}

func TestADiagramInAFileJudgesTheProjectTheSameWayTheSameTextDoes(t *testing.T) {
	// The two forms of one predicate: a drawing that is an artifact beside the code and the same drawing inside
	// the test. A file that judged a project differently from its own contents would make the artifact the form
	// nobody could trust.
	root := writeSlicedFixtureProject(t)
	drawing := "@startuml\ncomponent [api]\ncomponent [db]\ncomponent [domain]\n[api] --> [domain]\n@enduml\n"
	path := writeFixtureDiagram(t, drawing)

	inFile, err := fixtureSlicing(t, root).Should().AdhereToDiagramInFile(path).Check(nil)
	if err != nil {
		t.Fatalf("checking the diagram in a file failed: %v", err)
	}
	asText, err := fixtureSlicing(t, root).Should().AdhereToDiagram(drawing).Check(nil)
	if err != nil {
		t.Fatalf("checking the same diagram as text failed: %v", err)
	}

	if got, want := diagramFindings(t, inFile), diagramFindings(t, asText); !slices.Equal(got, want) {
		t.Errorf("the diagram in a file reported %v, want what the same text reports, %v", got, want)
	}
}

func TestIgnoringOrphanSlicesLeavesOutTheSlicesNoDependencyReaches(t *testing.T) {
	// The modifier is for the diagram that draws the architecture rather than the folder tree: tools imports
	// nothing and nothing imports it, which is an isolated folder and says nothing about what may depend on what.
	// mail is the other case, and the one the modifier must not hide: it imports nothing either, but the api
	// reaches it, so a drawing that does not declare it is missing a component that is at the end of an arrow.
	root := writeSlicedFixtureProject(t)
	writeFixtureFiles(t, root, map[string]string{
		"internal/tools/gen.go": "package tools\n\nfunc Generate() {}\n",
		"internal/mail/send.go": "package mail\n\nfunc Send() {}\n",
		"internal/api/report.go": "package api\n\nimport \"example.com/sliced/internal/mail\"\n\n" +
			"func Report() { mail.Send() }\n",
	})
	drawing := fixtureDiagramText

	reported, err := fixtureSlicing(t, root).Should().AdhereToDiagram(drawing).Check(nil)
	if err != nil {
		t.Fatalf("checking the drawing failed: %v", err)
	}
	ignored, err := fixtureSlicing(t, root).Should().AdhereToDiagram(drawing).IgnoringOrphanSlices().Check(nil)
	if err != nil {
		t.Fatalf("checking the drawing with the orphans ignored failed: %v", err)
	}

	if want := []string{"mail: undeclared slice", "tools: undeclared slice"}; !slices.Equal(
		diagramFindings(t, reported), want) {
		t.Errorf("the strict rule reported %v, want %v", diagramFindings(t, reported), want)
	}
	if want := []string{"mail: undeclared slice"}; !slices.Equal(diagramFindings(t, ignored), want) {
		t.Errorf("the modified rule reported %v, want %v: tools is an orphan and mail is at the end of an arrow",
			diagramFindings(t, ignored), want)
	}
}

func TestIgnoringExternalSlicesLeavesOutTheComponentsThisProjectHasNoSliceFor(t *testing.T) {
	// For the drawing that is about more than this one project. Without the modifier such a diagram reports
	// every component of every sibling module; with it, this project is judged by the arrows that are about it —
	// and by nothing else, so the arrow it does not draw is still reported.
	root := writeSlicedFixtureProject(t)
	drawing := "@startuml\ncomponent [api]\ncomponent [db]\ncomponent [domain]\ncomponent [billing]\n" +
		"[api] --> [domain]\n[api] --> [billing]\n@enduml\n"

	reported, err := fixtureSlicing(t, root).Should().AdhereToDiagram(drawing).Check(nil)
	if err != nil {
		t.Fatalf("checking the drawing failed: %v", err)
	}
	ignored, err := fixtureSlicing(t, root).Should().AdhereToDiagram(drawing).IgnoringExternalSlices().Check(nil)
	if err != nil {
		t.Fatalf("checking the drawing with the external components ignored failed: %v", err)
	}

	want := []string{
		"api -> db: undrawn dependency (internal/api/handler.go -> internal/db/conn.go)",
		"billing: absent component",
	}
	if got := diagramFindings(t, reported); !slices.Equal(got, want) {
		t.Errorf("the strict rule reported %v, want %v", got, want)
	}
	if got := diagramFindings(t, ignored); !slices.Equal(got, want[:1]) {
		t.Errorf("the modified rule reported %v, want %v: only the component is somebody else's", got, want[:1])
	}
}

func TestARuleAboutADiagramThreadsTheCheckOptionsIntoTheExtraction(t *testing.T) {
	// AllowEmptyTests travels its own way to the guard, so every other knob on the bag reaches this terminal
	// through the extraction — and a project extracted differently is a project the drawing was not made about.
	// IncludeTestFiles is the cheapest to observe: the fixture's db slice reaches the api slice through its test
	// file and through nothing else, so that arrow is missing from the drawing only when the knob is on.
	rule := fixtureSlicing(t, writeSlicedFixtureProject(t)).Should().AdhereToDiagram(fixtureDiagramText)

	byDefault, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}
	withTests, err := rule.Check(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("checking %s with IncludeTestFiles failed: %v", rule, err)
	}

	if got := diagramFindings(t, byDefault); len(got) != 0 {
		t.Errorf("the project disagrees with its diagram in %v by default, want it to adhere", got)
	}
	want := []string{
		"db -> api: undrawn dependency (internal/db/conn_test.go -> internal/api/handler.go, " +
			"internal/db/conn_test.go -> internal/api/router.go)",
	}
	if got := diagramFindings(t, withTests); !slices.Equal(got, want) {
		t.Errorf("the project disagrees with its diagram in %v with IncludeTestFiles, want %v", got, want)
	}
}

func TestASlicingThatFoundNoSliceIsReportedRatherThanJudgedAgainstADiagram(t *testing.T) {
	// The empty-test guard on this terminal, and why it is not optional here: a slicing whose folder was renamed
	// would report every component of the drawing as one the project has no slice for, which is a report about
	// the pattern told as a report about the architecture.
	rule := fluentapi.ProjectSlices(fixtureLocator(t, writeSlicedFixtureProject(t))).
		DefinedBy("modules/(**)/**").
		Should().
		AdhereToDiagram(fixtureDiagramText)

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("the rule reported %v, want the one slicing that found nothing", messages(t, violations))
	}
	empty := emptyTestViolation(t, violations[0])
	if empty.Subject != "slices" {
		t.Errorf("the guard reports %q, want the vocabulary the entry point names", empty.Subject)
	}
	if len(empty.Selectors) != 1 || !strings.Contains(empty.Selectors[0].String(), "modules/(**)/**") {
		t.Errorf("the guard reports the selectors %v, want the slicing's own pattern", empty.Selectors)
	}
}

func TestATextThatIsNotADiagramIsAUserError(t *testing.T) {
	// A drawing this library cannot read is a rule that cannot run, so it is the treatment a pattern that will
	// not compile gets: rejected where it was written, reported by the terminal, and never a violation. A line
	// the dialect does not have is refused rather than skipped, because a skipped line is a dependency nobody
	// checks.
	tests := []struct {
		name    string
		diagram string
		want    error
	}{
		{
			name:    "a line the dialect does not have",
			diagram: "@startuml\ntitle the architecture\ncomponent [api]\n@enduml",
			want:    slicesextraction.ErrUnreadableDiagramLine,
		},
		{
			name:    "a document with no component in it",
			diagram: "@startuml\n' nothing was ever drawn here\n@enduml",
			want:    slicesextraction.ErrEmptyDiagram,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule := fixtureSlicing(t, writeSlicedFixtureProject(t)).Should().AdhereToDiagram(test.diagram)

			violations, err := rule.Check(nil)

			if !errors.Is(err, test.want) {
				t.Fatalf("checking %s returned %v, want it to wrap %v", rule, err, test.want)
			}
			if operation := userError(t, err).Operation; operation != "adhere to diagram" {
				t.Errorf("UserError.Operation = %q, want the predicate at fault, %q", operation, "adhere to diagram")
			}
			if len(violations) != 0 {
				t.Errorf("the rule reported %v alongside the error, want nothing: there was no runnable rule",
					messages(t, violations))
			}
			if !strings.Contains(rule.String(), "rejected") {
				t.Errorf("%s renders without the rejection, want it visible in a test failure", rule)
			}
		})
	}
}

func TestAFileThatIsNotADiagramIsTheSameUserErrorUnderThePredicateThatReadIt(t *testing.T) {
	// The file form fails the way the text form does, because it is the same reader: the drawing is wrong
	// wherever it was written down. The predicate the message names is the one the user typed.
	path := writeFixtureDiagram(t, "@startuml\nskinparam monochrome true\ncomponent [api]\n@enduml\n")
	rule := fixtureSlicing(t, writeSlicedFixtureProject(t)).Should().AdhereToDiagramInFile(path)

	_, err := rule.Check(nil)

	if !errors.Is(err, slicesextraction.ErrUnreadableDiagramLine) {
		t.Fatalf("checking %s returned %v, want it to wrap ErrUnreadableDiagramLine", rule, err)
	}
	user := userError(t, err)
	if user.Operation != "adhere to diagram in file" {
		t.Errorf("UserError.Operation = %q, want %q", user.Operation, "adhere to diagram in file")
	}
	if user.Subject != path {
		t.Errorf("UserError.Subject = %q, want the file the user named, %q", user.Subject, path)
	}
}

func TestADiagramFileThatCannotBeReadIsATechnicalError(t *testing.T) {
	// A file that is not there is the environment's fault rather than the rule's, so the error travels as the
	// technical one it is: there is nothing in the sentence the user typed to fix.
	path := filepath.Join(t.TempDir(), "architecture.puml")
	rule := fixtureSlicing(t, writeSlicedFixtureProject(t)).Should().AdhereToDiagramInFile(path)

	_, err := rule.Check(nil)

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("checking %s returned a %T, want a *archerror.TechnicalError: %v", rule, err, err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the error is %v, want it to wrap os.ErrNotExist", err)
	}
}

func TestADiagramWithNoPathIsAUserError(t *testing.T) {
	// There is nothing the empty string could have meant: a diagram has to be somewhere, and the working
	// directory is a folder rather than a file.
	rule := fixtureSlicing(t, writeSlicedFixtureProject(t)).Should().AdhereToDiagramInFile("")

	_, err := rule.Check(nil)

	if !errors.Is(err, fluentapi.ErrMissingDiagramPath) {
		t.Fatalf("checking %s returned %v, want ErrMissingDiagramPath", rule, err)
	}
	if operation := userError(t, err).Operation; operation != "adhere to diagram in file" {
		t.Errorf("UserError.Operation = %q, want %q", operation, "adhere to diagram in file")
	}
}

func TestARuleAboutADiagramRendersAsTheSentenceTheUserTyped(t *testing.T) {
	// A reader needs the whole sentence, and a drawing given as text renders as its two sizes rather than as
	// itself: a rule renders as one line, and a whole document inside one would bury it.
	slicing := fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**")
	tests := []struct {
		name string
		rule fluentapi.SlicesDiagramCondition
		want string
	}{
		{
			name: "a diagram given as text",
			rule: slicing.Should().AdhereToDiagram(fixtureDiagramText),
			want: `project slices, path matches "internal/(**)/**", should, ` +
				`adhere to diagram (3 components, 2 dependencies)`,
		},
		{
			name: "a diagram given as a file",
			rule: slicing.Should().AdhereToDiagramInFile("docs/architecture.puml").IgnoringOrphanSlices(),
			want: `project slices, path matches "internal/(**)/**", should, ` +
				`adhere to diagram in file "docs/architecture.puml", ignoring orphan slices`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if rendered := test.rule.String(); rendered != test.want {
				t.Errorf("the rule reads\n%q, want\n%q", rendered, test.want)
			}
		})
	}
}

func TestTheModifiersOfADiagramReadTheSameWayHoweverTheyWereChained(t *testing.T) {
	// They are a pair of flags rather than steps, so two rules that were typed differently and mean the same
	// thing read the same way — and a rule stays a value: chaining a modifier leaves the rule it came from as it
	// was.
	rule := fluentapi.ProjectSlices(nil).
		DefinedBy("internal/(**)/**").
		Should().
		AdhereToDiagramInFile("docs/architecture.puml")

	oneWay := rule.IgnoringOrphanSlices().IgnoringExternalSlices()
	otherWay := rule.IgnoringExternalSlices().IgnoringOrphanSlices()

	if oneWay.String() != otherWay.String() {
		t.Errorf("the two orders read\n%q and\n%q, want one sentence", oneWay, otherWay)
	}
	if want := "ignoring orphan slices, ignoring external slices"; !strings.HasSuffix(oneWay.String(), want) {
		t.Errorf("the modified rule reads %q, want it to end in %q", oneWay, want)
	}
	if strings.Contains(rule.String(), "ignoring") {
		t.Errorf("the rule the modifiers were chained from now reads %q, want it left alone", rule)
	}
}

// writeFixtureDiagram writes a drawing to a file of its own and hands back the path, for the rule that names
// one instead of carrying it.
func writeFixtureDiagram(t *testing.T, drawing string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "architecture.puml")
	if err := os.WriteFile(path, []byte(drawing), 0o600); err != nil {
		t.Fatalf("writing the fixture diagram failed: %v", err)
	}
	return path
}

// diagramViolation is this violation as the diagram assertion's own type, failing the test when a rule
// reported anything else.
func diagramViolation(t *testing.T, violation assertion.Violation) slicesassertion.DiagramViolation {
	t.Helper()

	reported, ok := violation.(slicesassertion.DiagramViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want a DiagramViolation", violation)
	}
	return reported
}

// diagramFindings are these violations as the lines they render as, which is what a failing test has to show:
// the finding, the names it is about and, for an undrawn dependency, the files that made it.
func diagramFindings(t *testing.T, violations []assertion.Violation) []string {
	t.Helper()

	rendered := make([]string, 0, len(violations))
	for _, violation := range violations {
		rendered = append(rendered, diagramViolation(t, violation).String())
	}
	return rendered
}

// madeBy are the file dependencies an undrawn dependency was made by, as `a.go -> b.go`.
func madeBy(violation slicesassertion.DiagramViolation) []string {
	rendered := make([]string, 0, len(violation.Dependencies))
	for _, edge := range violation.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}
