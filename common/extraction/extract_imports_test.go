package extraction

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// writeSourceFile writes one Go file with real content and hands back its host path, which is what
// ExtractImports takes.
func writeSourceFile(t *testing.T, name, content string) string {
	t.Helper()

	root := t.TempDir()
	writeProjectFile(t, root, name, content)
	return filepath.Join(root, filepath.FromSlash(name))
}

func TestExtractImportsReadsEveryFlavorOfImport(t *testing.T) {
	path := writeSourceFile(t, "handler.go", `package api

import (
	"fmt"
	aliased "strings"
	_ "example.com/fixture/internal/db"
	. "errors"
)

import "sort"

var _ = []any{fmt.Sprint, aliased.TrimSpace, New, sort.Strings}
`)

	imports, err := ExtractImports(path)
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	// In source order, and a second import declaration is as much part of the file as the first.
	want := []ImportInfo{
		{Path: "fmt", Kind: ImportKindPlain},
		{Path: "strings", Kind: ImportKindAliased},
		{Path: "example.com/fixture/internal/db", Kind: ImportKindBlank},
		{Path: "errors", Kind: ImportKindDot},
		{Path: "sort", Kind: ImportKindPlain},
	}
	if len(imports) != len(want) {
		t.Fatalf("imports = %v, want %v", imports, want)
	}
	for index, imported := range imports {
		if imported != want[index] {
			t.Errorf("imports[%d] = %v, want %v", index, imported, want[index])
		}
	}
}

func TestExtractImportsFindsNoneInAFileThatImportsNothing(t *testing.T) {
	path := writeSourceFile(t, "router.go", "package api\n\nfunc Route() {}\n")

	imports, err := ExtractImports(path)
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}
	if len(imports) != 0 {
		t.Errorf("imports = %v, want none", imports)
	}
}

func TestExtractImportsIgnoresWhatTheFileDoesAfterItsImports(t *testing.T) {
	// Only the imports are parsed, so a body the compiler would reject — this one is mid-refactor —
	// still yields the dependencies the file declares.
	path := writeSourceFile(t, "handler.go", `package api

import "fmt"

func Handle() { this is not Go at all ((( }
`)

	imports, err := ExtractImports(path)
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}
	if len(imports) != 1 || imports[0].Path != "fmt" {
		t.Errorf("imports = %v, want just fmt", imports)
	}
}

func TestExtractImportsKeepsWhatItReachedInAFileThatDoesNotParse(t *testing.T) {
	// The failure is in the import block itself, which is the one place ExtractImports cannot read past.
	// It is still not fatal: the imports above the break are returned with the error, so a graph built
	// from this file is short of one edge rather than missing altogether.
	path := writeSourceFile(t, "broken.go", `package api

import (
	"fmt"
	"strings
)
`)

	imports, err := ExtractImports(path)

	if err == nil {
		t.Fatal("ExtractImports read a file that does not parse without saying so")
	}
	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExtractImports error = %v, want a *archerror.TechnicalError", err)
	}
	if len(imports) != 1 || imports[0].Path != "fmt" {
		t.Errorf("imports = %v, want the fmt import the parser reached before it gave up", imports)
	}
}

func TestExtractImportsRejectsAFileItCannotRead(t *testing.T) {
	_, err := ExtractImports(filepath.Join(t.TempDir(), "no", "such", "file.go"))

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExtractImports error = %v, want a *archerror.TechnicalError", err)
	}
}

func TestExtractImportsReadsThisFilesOwnFile(t *testing.T) {
	// The level above the hand-written fixtures: a real file of this repository, whose imports are
	// visible a few lines up.
	root, err := LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}

	imports, err := ExtractImports(filepath.Join(root, "common", "extraction", "extract_graph.go"))
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	want := []ImportInfo{
		{Path: "slices", Kind: ImportKindPlain},
		{Path: "strings", Kind: ImportKindPlain},
		{Path: "golang.org/x/tools/go/packages", Kind: ImportKindPlain},
		{Path: "github.com/LukasNiessen/ArchUnitGo/common/archerror", Kind: ImportKindPlain},
	}
	if len(imports) != len(want) {
		t.Fatalf("imports = %v, want %v", imports, want)
	}
	for index, imported := range imports {
		if imported != want[index] {
			t.Errorf("imports[%d] = %v, want %v", index, imported, want[index])
		}
	}
}
