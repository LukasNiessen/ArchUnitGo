package extraction_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/files/extraction"
)

func TestNewFileInfoDerivesTheNameTheExtensionAndTheFolder(t *testing.T) {
	// What a user predicate is handed about where a file is and what it is called, all of it derived from the
	// one identifier the rule's own patterns were matched against.
	tests := []struct {
		identifier string
		path       string
		name       string
		extension  string
		directory  string
	}{
		{identifier: "internal/api/handler.go", path: "internal/api/handler.go", name: "handler", extension: ".go", directory: "internal/api"},
		{identifier: "main.go", path: "main.go", name: "main", extension: ".go", directory: "."},
		{identifier: "internal/api/handler_test.go", path: "internal/api/handler_test.go", name: "handler_test", extension: ".go", directory: "internal/api"},
		{identifier: "docs/README", path: "docs/README", name: "README", extension: "", directory: "docs"},
		// The last extension wins, which is what path.Ext means by one: a name with two of them is a name
		// carrying a dot, not a file of a compound type.
		{identifier: "web/app.min.js", path: "web/app.min.js", name: "app.min", extension: ".js", directory: "web"},
		// An identifier written the way a host spells a path describes the same file: it is normalised on the
		// way in, so a hand-built FileInfo and one the graph produced agree.
		{identifier: `internal\db\conn.go`, path: "internal/db/conn.go", name: "conn", extension: ".go", directory: "internal/db"},
		{identifier: "./internal/db/../db/conn.go", path: "internal/db/conn.go", name: "conn", extension: ".go", directory: "internal/db"},
	}

	for _, test := range tests {
		t.Run(test.identifier, func(t *testing.T) {
			file := extraction.NewFileInfo(test.identifier, "package api\n")

			if file.Path != test.path {
				t.Errorf("Path = %q, want the normalised identifier %q", file.Path, test.path)
			}
			if file.Name != test.name {
				t.Errorf("Name = %q, want the name without its extension, %q", file.Name, test.name)
			}
			if file.Extension != test.extension {
				t.Errorf("Extension = %q, want %q", file.Extension, test.extension)
			}
			if file.Directory != test.directory {
				t.Errorf("Directory = %q, want the folder holding it, %q", file.Directory, test.directory)
			}
		})
	}
}

func TestNewFileInfoKeepsTheSourceExactlyAsItWasGiven(t *testing.T) {
	// A predicate about what a file contains reads the text itself, so nothing may be stripped from it: a
	// user's own regexp is written against the file as it is on disk.
	source := "package api\r\n\r\n\timport \"fmt\"\n\n// trailing comment"

	file := extraction.NewFileInfo("internal/api/handler.go", source)

	if file.Source != source {
		t.Errorf("Source = %q, want the text unchanged: %q", file.Source, source)
	}
}

func TestNewFileInfoCountsOnlyTheLinesThatCarrySomething(t *testing.T) {
	// The size of a file as a reader would judge it: blank lines do not count, a line of white space is
	// blank, and whether the file ends in a newline changes nothing.
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{name: "empty file", source: "", want: 0},
		{name: "one line without a newline", source: "package api", want: 1},
		{name: "one line with a newline", source: "package api\n", want: 1},
		{name: "blank lines between", source: "package api\n\n\nfunc Handle() {}\n", want: 2},
		{name: "white space only", source: "   \n\t\n \t \n", want: 0},
		{name: "indented code counts", source: "func Handle() {\n\tprintln()\n}\n", want: 3},
		{name: "windows line endings", source: "package api\r\n\r\nfunc Handle() {}\r\n", want: 2},
		{name: "trailing blank lines", source: "package api\n\n\n\n", want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := extraction.NewFileInfo("internal/api/handler.go", test.source)

			if file.NonBlankLineCount != test.want {
				t.Errorf("NonBlankLineCount = %d, want %d for %q", file.NonBlankLineCount, test.want, test.source)
			}
		})
	}
}

func TestNewFileInfoOfNoIdentifierDerivesNoNameAndNoFolder(t *testing.T) {
	// The empty identifier is the absence of a file, not a file at the project root, so there is nothing to
	// derive from it — and inventing `.` as its name and its folder would describe the root itself.
	file := extraction.NewFileInfo("", "package api\n")

	if file.Path != "" || file.Name != "" || file.Extension != "" || file.Directory != "" {
		t.Errorf("NewFileInfo(\"\", ...) = %+v, want nothing derived from an identifier that names no file", file)
	}
	if file.NonBlankLineCount != 1 {
		t.Errorf("NonBlankLineCount = %d, want the source still described: 1", file.NonBlankLineCount)
	}
}

func TestExtractFileInfoReadsTheSourceOfEachIdentifierInOrder(t *testing.T) {
	// The gathering step a rule about a file's contents runs after its scope is resolved: the identifiers the
	// selection produced, turned back into files on the host, read and described.
	root := writeSourceProject(t)

	files, err := extraction.ExtractFileInfo(root, []string{"main.go", "internal/db/conn.go"})
	if err != nil {
		t.Fatalf("ExtractFileInfo failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("ExtractFileInfo described %d files, want the 2 it was given", len(files))
	}
	if files[0].Path != "main.go" || files[1].Path != "internal/db/conn.go" {
		t.Errorf("ExtractFileInfo described %q then %q, want the order the identifiers arrived in", files[0].Path, files[1].Path)
	}
	if !strings.Contains(files[0].Source, "func main()") {
		t.Errorf("the source of main.go is %q, want the text of that file", files[0].Source)
	}
	if files[1].Name != "conn" || files[1].Directory != "internal/db" {
		t.Errorf("internal/db/conn.go was described as %+v, want its name and folder derived from the identifier", files[1])
	}
	if files[1].NonBlankLineCount != 2 {
		t.Errorf("internal/db/conn.go has %d non-blank lines, want the 2 it was written with", files[1].NonBlankLineCount)
	}
}

func TestExtractFileInfoTakesIdentifiersAndNotHostPaths(t *testing.T) {
	// An identifier is forward-slashed on every operating system, and joining it onto the root is what turns
	// it back into a path — so the same call describes the same file on Windows and on Linux.
	root := writeSourceProject(t)

	files, err := extraction.ExtractFileInfo(root, []string{"internal/api/handler.go"})
	if err != nil {
		t.Fatalf("ExtractFileInfo failed: %v", err)
	}

	if files[0].Path != "internal/api/handler.go" {
		t.Errorf("Path = %q, want the identifier it was asked for, whatever the host separator is", files[0].Path)
	}
}

func TestExtractFileInfoOfNoIdentifiersDescribesNothing(t *testing.T) {
	// A rule that selected nothing is the empty-test guard's business, and the terminal asks it first, so an
	// empty selection is an ordinary answer here rather than an error.
	files, err := extraction.ExtractFileInfo(writeSourceProject(t), nil)
	if err != nil {
		t.Fatalf("ExtractFileInfo failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("ExtractFileInfo described %v, want nothing", files)
	}
}

func TestExtractFileInfoReportsAFileItCannotRead(t *testing.T) {
	// The graph says the file is part of the project, so failing to open it is the environment disagreeing
	// with the library — a TechnicalError naming the file — and never a violation.
	root := writeSourceProject(t)

	_, err := extraction.ExtractFileInfo(root, []string{"internal/db/conn.go", "internal/db/gone.go"})

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExtractFileInfo error = %v, want a *archerror.TechnicalError", err)
	}
	if technical.Subject != "internal/db/gone.go" {
		t.Errorf("TechnicalError.Subject = %q, want the file that could not be read", technical.Subject)
	}
}

// writeSourceProject writes the project the tests here read: a file at the root, and two below it.
func writeSourceProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                  "module example.com/fixture\n\ngo 1.26\n",
		"main.go":                 "package main\n\nfunc main() {}\n",
		"internal/api/handler.go": "package api\n\nfunc Handle() {}\n",
		"internal/db/conn.go":     "package db\n\nfunc Connect() {}\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating the folder of %q failed: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %q failed: %v", name, err)
		}
	}
	return root
}
