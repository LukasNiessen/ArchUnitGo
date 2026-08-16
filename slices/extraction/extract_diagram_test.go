package extraction_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/slices/extraction"
)

func TestExtractDiagramReadsTheDiagramInAFile(t *testing.T) {
	// A diagram in a file means exactly what the same diagram means as a string, which is the whole point of
	// this function being ParseDiagram over os.ReadFile and nothing more.
	path := writeDiagramFile(t, "architecture.puml", "@startuml\ncomponent [api]\n[api] --> [db]\n@enduml\n")

	diagram, err := extraction.ExtractDiagram(path)
	if err != nil {
		t.Fatalf("reading %s failed with %v, want it read", path, err)
	}

	want := []string{"api", "db"}
	if got := diagram.Components(); !slices.Equal(got, want) {
		t.Errorf("the components of the file are %v, want %v", got, want)
	}
	if !diagram.Draws("api", "db") {
		t.Errorf("the file draws %v, want api -> db", diagram.Dependencies())
	}
}

func TestExtractDiagramReportsAFileItCannotRead(t *testing.T) {
	// The environment failed and there is nothing in the rule to fix, so this is the technical half of the two
	// error types — and it names the path, because a diagram that was moved is the likeliest way to get here.
	path := filepath.Join(t.TempDir(), "moved.puml")

	_, err := extraction.ExtractDiagram(path)

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("reading a diagram that is not there failed with %v, want a TechnicalError", err)
	}
	if technical.Subject != path {
		t.Errorf("the failure is about %q, want it to name %q", technical.Subject, path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the failure does not wrap os.ErrNotExist (%v), want the reason the file could not be read", err)
	}
}

func TestExtractDiagramReportsAFileThatIsNotADiagramAsTheUsersMistake(t *testing.T) {
	// The other half of the distinction: the file was read, so the library and its environment worked. What is
	// wrong is what the user wrote, and the error is the parser's own so that the stage of the chain that was
	// given the path is what names itself around it.
	path := writeDiagramFile(t, "prose.puml", "@startuml\nthe api reads the db\n@enduml\n")

	_, err := extraction.ExtractDiagram(path)

	if !errors.Is(err, extraction.ErrUnreadableDiagramLine) {
		t.Fatalf("reading a file that is not a diagram failed with %v, want ErrUnreadableDiagramLine", err)
	}
	var technical *archerror.TechnicalError
	if errors.As(err, &technical) {
		t.Errorf("the failure is a TechnicalError (%v), want the user's own mistake reported as the user's", err)
	}
}

// writeDiagramFile writes a diagram into a folder of this test's own and hands back its path.
func writeDiagramFile(t *testing.T, name, text string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("writing the fixture diagram %s failed with %v", path, err)
	}
	return path
}
