package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/slices/fluentapi"
)

func TestToPlantUMLDrawsTheSlicesOfTheProjectTheSlicingWasResolvedAgainst(t *testing.T) {
	// The whole document, through the public chain: the fixture's three folders under internal/ are its three
	// components, and the two imports the api has are the two arrows. main.go is in no slice, so nothing it
	// depends on is drawn — a diagram of the slicing rather than of the project.
	document, err := fixtureSlicing(t, writeSlicedFixtureProject(t)).ToPlantUML(nil)
	if err != nil {
		t.Fatalf("`to plantuml` failed: %v", err)
	}

	want := `@startuml
' 3 components, 2 dependencies
component [api]
component [db]
component [domain]
[api] --> [db]
[api] --> [domain]
@enduml
`
	if document != want {
		t.Errorf("the project is drawn as\n%s\nwant\n%s", document, want)
	}
}

func TestExportAsPlantUMLWritesTheDocumentToPlantUMLWouldHaveHandedBack(t *testing.T) {
	// The promise that makes the two terminals one: `export as plantuml` is `to plantuml` plus a file, so a
	// diagram committed beside the code and one rendered in a test are the same document, byte for byte.
	slicing := fixtureSlicing(t, writeSlicedFixtureProject(t))
	path := filepath.Join(t.TempDir(), "architecture.puml")

	if err := slicing.ExportAsPlantUML(path, nil); err != nil {
		t.Fatalf("`export as plantuml` failed: %v", err)
	}

	want, err := slicing.ToPlantUML(nil)
	if err != nil {
		t.Fatalf("`to plantuml` failed: %v", err)
	}
	if got := readExportedDiagram(t, path); got != want {
		t.Errorf("the exported file holds\n%s\nwant the document the string form renders\n%s", got, want)
	}
}

func TestExportAsPlantUMLDrawsTheProjectUnderTheCheckOptionsItIsGiven(t *testing.T) {
	// The bag is what says which project is drawn, and a diagram of a differently-extracted project is a
	// diagram of something the user did not ask about. IncludeTestFiles is the cheapest to observe: the
	// fixture's only test file sits in the db folder and imports the api, so it is the one arrow back up — and
	// it exists only when the options say test files are part of the project.
	slicing := fixtureSlicing(t, writeSlicedFixtureProject(t))
	folder := t.TempDir()
	withTests := filepath.Join(folder, "with-tests.puml")
	byDefault := filepath.Join(folder, "by-default.puml")

	if err := slicing.ExportAsPlantUML(withTests, &kernel.CheckOptions{IncludeTestFiles: true}); err != nil {
		t.Fatalf("`export as plantuml` with IncludeTestFiles failed: %v", err)
	}
	if err := slicing.ExportAsPlantUML(byDefault, nil); err != nil {
		t.Fatalf("`export as plantuml` failed: %v", err)
	}

	arrow := "[db] --> [api]"
	if drawn := readExportedDiagram(t, withTests); !strings.Contains(drawn, arrow) {
		t.Errorf("the exported diagram is\n%s\nwant the arrow %q the fixture's test file makes", drawn, arrow)
	}
	if drawn := readExportedDiagram(t, byDefault); strings.Contains(drawn, arrow) {
		t.Errorf("the exported diagram is\n%s\nwant no arrow %q: the test file is not part of the project", drawn, arrow)
	}
}

func TestAnExportedDiagramIsTheDrawingTheRuleReadsBack(t *testing.T) {
	// The round trip, and why both halves of this feature are worth having in one library: the diagram a project
	// exports is the diagram the next run judges it against, so exporting one and committing it is how a project
	// starts being checked against its own architecture. A drawn document that failed its own rule would make
	// the export useless.
	root := writeSlicedFixtureProject(t)
	path := filepath.Join(t.TempDir(), "docs", "architecture.puml")

	if err := fixtureSlicing(t, root).ExportAsPlantUML(path, nil); err != nil {
		t.Fatalf("`export as plantuml` failed: %v", err)
	}

	violations, err := fixtureSlicing(t, root).Should().AdhereToDiagramInFile(path).Check(nil)
	if err != nil {
		t.Fatalf("checking the project against its own exported diagram failed: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("the project disagrees with its own exported diagram in %v, want it to adhere", violations)
	}
}

func TestAnExportedDiagramCreatesTheFoldersOfThePathItWasGiven(t *testing.T) {
	// A diagram exported into `docs/diagrams/` should not fail because nobody had made `docs/diagrams/` yet: the
	// path a user typed says where the file goes, and creating two folders is not a decision worth an error.
	path := filepath.Join(t.TempDir(), "docs", "diagrams", "architecture.puml")

	if err := fixtureSlicing(t, writeSlicedFixtureProject(t)).ExportAsPlantUML(path, nil); err != nil {
		t.Fatalf("`export as plantuml` failed: %v", err)
	}
	if readExportedDiagram(t, path) == "" {
		t.Error("the exported file is empty, want the diagram in it")
	}
}

func TestAnExportedDiagramOverwritesTheOneThatWasThereBefore(t *testing.T) {
	// A diagram is the current answer about the project, and a stale one beside it would be read as a second
	// answer — which is the very failure `adhere to diagram` exists to catch, so this terminal must not cause
	// it. Appending would be a document in no format at all.
	root := writeSlicedFixtureProject(t)
	path := filepath.Join(t.TempDir(), "architecture.puml")
	if err := os.WriteFile(path, []byte("@startuml\ncomponent [yesterday]\n@enduml\n"), 0o600); err != nil {
		t.Fatalf("writing the diagram of yesterday failed: %v", err)
	}

	if err := fixtureSlicing(t, root).ExportAsPlantUML(path, nil); err != nil {
		t.Fatalf("`export as plantuml` failed: %v", err)
	}

	drawn := readExportedDiagram(t, path)
	if strings.Contains(drawn, "yesterday") {
		t.Errorf("the exported file holds\n%s\nwant the diagram of yesterday overwritten", drawn)
	}
	if strings.Count(drawn, "@startuml") != 1 {
		t.Errorf("the exported file holds\n%s\nwant one document in it", drawn)
	}
}

func TestExportAsPlantUMLRejectsThePathThatIsNotOne(t *testing.T) {
	// There is nothing the empty string could have meant: a diagram has to be written somewhere, and the
	// working directory is a folder rather than a file.
	err := fixtureSlicing(t, writeSlicedFixtureProject(t)).ExportAsPlantUML("", nil)

	if !errors.Is(err, fluentapi.ErrMissingExportPath) {
		t.Fatalf("`export as plantuml` error = %v, want it to wrap ErrMissingExportPath", err)
	}
	if want := "export as plantuml"; userError(t, err).Operation != want {
		t.Errorf("the rejection blames %q, want the terminal at fault, %q", userError(t, err).Operation, want)
	}
}

func TestTheOutputTerminalsRefuseToDrawAProjectWithNoSliceInIt(t *testing.T) {
	// A slicing whose pattern has gone stale draws an empty frame, and an empty frame looks exactly like a
	// project with nothing in it. This is a report's shape of the empty-test guard: there is no violation list
	// to put a finding in, so the refusal is the error.
	root := writeSlicedFixtureProject(t)
	stale := fluentapi.ProjectSlices(fixtureLocator(t, root)).DefinedBy("services/(**)/**")

	if _, err := stale.ToPlantUML(nil); !errors.Is(err, fluentapi.ErrNothingToDraw) {
		t.Errorf("`to plantuml` error = %v, want it to wrap ErrNothingToDraw", err)
	}

	path := filepath.Join(t.TempDir(), "architecture.puml")
	err := stale.ExportAsPlantUML(path, nil)
	if !errors.Is(err, fluentapi.ErrNothingToDraw) {
		t.Fatalf("`export as plantuml` error = %v, want it to wrap ErrNothingToDraw", err)
	}
	if want := "export as plantuml"; userError(t, err).Operation != want {
		t.Errorf("the rejection blames %q, want the terminal at fault, %q", userError(t, err).Operation, want)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("the refused export left a file behind: %v", statErr)
	}
}

func TestAllowEmptyTestsIsHowAnEmptyDiagramIsAskedFor(t *testing.T) {
	// The same knob that opts a rule out of the guard, honored by the terminal that has no violation to report:
	// a suite drawing a project it knows may be empty says so once, on the bag.
	root := writeSlicedFixtureProject(t)
	stale := fluentapi.ProjectSlices(fixtureLocator(t, root)).DefinedBy("services/(**)/**")

	document, err := stale.ToPlantUML(&kernel.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("`to plantuml` failed with AllowEmptyTests: %v", err)
	}

	want := "@startuml\n' 0 components, 0 dependencies\n@enduml\n"
	if document != want {
		t.Errorf("a project with no slice is drawn as\n%s\nwant\n%s", document, want)
	}
}

func TestTheOutputTerminalsReportWhatTheChainRejected(t *testing.T) {
	// A report is a terminal like a check, so it is where the rejections of a chain arrive. A slicing nobody
	// typed is the one this module cannot leave to the type system, and a diagram drawn out of a rule that
	// cannot run would be a picture of a pattern nobody wrote.
	sliceless := fluentapi.ProjectSlices(fixtureLocator(t, writeSlicedFixtureProject(t)))

	document, err := sliceless.ToPlantUML(nil)
	if !errors.Is(err, fluentapi.ErrNoSlicing) {
		t.Fatalf("`to plantuml` error = %v, want it to wrap ErrNoSlicing", err)
	}
	if document != "" {
		t.Errorf("the failed terminal handed back\n%s\nwant no document at all", document)
	}

	path := filepath.Join(t.TempDir(), "architecture.puml")
	if err := sliceless.ExportAsPlantUML(path, nil); !errors.Is(err, fluentapi.ErrNoSlicing) {
		t.Errorf("`export as plantuml` error = %v, want it to wrap ErrNoSlicing", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the failed export left a file behind: %v", err)
	}
}

// readExportedDiagram is the file an export wrote, failing the test when it is not there: an export that wrote
// nowhere would otherwise pass every assertion about what it wrote.
func readExportedDiagram(t *testing.T, path string) string {
	t.Helper()

	document, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the exported diagram failed: %v", err)
	}
	return string(document)
}
