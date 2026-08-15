package extraction

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

func TestDescribeSourcesCountsTheLinesThatCarryCode(t *testing.T) {
	// The size of a file as a compiler would judge it: white space and comments are not code, and a comment
	// sharing a line with code costs that line nothing.
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{name: "one line", source: "package api\n", want: 1},
		{name: "no trailing newline", source: "package api", want: 1},
		{name: "blank lines do not count", source: "package api\n\n\nfunc Handle() {}\n", want: 2},
		{name: "a line of white space is blank", source: "package api\n   \t\nfunc Handle() {}\n", want: 2},
		{name: "a comment line does not count", source: "package api\n\n// Handle serves.\nfunc Handle() {}\n", want: 2},
		{name: "a trailing comment does not cost the line", source: "package api\n\nfunc Handle() {} // serves\n", want: 2},
		{name: "a block comment blanks the lines it spans", source: "package api\n\n/*\nnot code\nnot code either\n*/\nfunc Handle() {}\n", want: 2},
		{name: "code after a block comment still counts", source: "package api\n\n/* why */ func Handle() {}\n", want: 2},
		// The line structure has to survive the blanking: masking the newline inside this comment as well would
		// merge the two lines of code into one and cost the file a line it wrote.
		{name: "a block comment between code on both its lines", source: "package api\n\nfunc Handle() { /* why\nand why not */ }\n", want: 3},
		{name: "a build directive is a comment", source: "//go:build linux\n\npackage api\n", want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := describeOne(t, test.source)

			if file.LinesOfCode != test.want {
				t.Errorf("LinesOfCode = %d, want %d for %q", file.LinesOfCode, test.want, test.source)
			}
		})
	}
}

func TestDescribeSourcesCountsEveryStatementAtEveryDepth(t *testing.T) {
	// How much a file does, rather than how long it is: a nested statement is a statement, and the braces,
	// labels and case clauses holding them are not.
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{name: "no function bodies at all", source: "package api\n\nvar Global = 1\n\ntype Handler struct{}\n", want: 0},
		{name: "an empty body", source: "package api\n\nfunc Handle() {}\n", want: 0},
		{name: "a declaration inside a function is a statement", source: "package api\n\nfunc Handle() {\n\tvar local = 1\n\t_ = local\n}\n", want: 2},
		{name: "a label names the statement after it", source: "package api\n\nfunc Handle() {\nloop:\n\tfor {\n\t\tbreak loop\n\t}\n}\n", want: 2},
		{name: "a stray semicolon is not a statement", source: "package api\n\nfunc Handle() {\n\t;\n}\n", want: 0},
		{name: "a select clause is not a statement", source: "package api\n\nfunc Handle(c chan int) {\n\tselect {\n\tcase v := <-c:\n\t\t_ = v\n\t}\n}\n", want: 3},
		{name: "every depth of a body", source: statementFixture, want: 9},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := describeOne(t, test.source)

			if file.StatementCount != test.want {
				t.Errorf("StatementCount = %d, want %d for %q", file.StatementCount, test.want, test.source)
			}
		})
	}
}

func TestDescribeSourcesCountsImportSpecsRatherThanDeclarations(t *testing.T) {
	// An import is a dependency the file wrote down, whatever it does with it: a blank import counts, an
	// aliased one counts, and how the paths were bracketed changes nothing.
	source := "package api\n\nimport (\n\t\"fmt\"\n\t_ \"embed\"\n\ttext \"strings\"\n)\n\nimport \"os\"\n"

	file := describeOne(t, source)

	if file.ImportCount != 4 {
		t.Errorf("ImportCount = %d, want the 4 specs the two declarations hold", file.ImportCount)
	}
}

func TestDescribeSourcesCountsFunctionsWithoutTheirMethods(t *testing.T) {
	// A method belongs to the type it is declared on, where MethodCount counts it, and a function literal is
	// part of the statement holding it — so the three never count one declaration twice.
	source := "package api\n\ntype Handler struct{}\n\nfunc New() Handler { return Handler{} }\n\n" +
		"func Serve() {}\n\nfunc (h Handler) Handle() {}\n\nfunc (h *Handler) Close() error { return nil }\n\n" +
		"var handle = func() {}\n"

	file := describeOne(t, source)

	if file.FunctionCount != 2 {
		t.Errorf("FunctionCount = %d, want the 2 package-level functions without a receiver", file.FunctionCount)
	}
	if len(file.Classes) != 1 || file.Classes[0].MethodCount != 2 {
		t.Errorf("classes = %+v, want Handler carrying the two methods instead", file.Classes)
	}
}

func TestDescribeSourcesReportsASourceItCannotParse(t *testing.T) {
	// A file the graph says is part of the project and that is not Go is the environment disagreeing with the
	// library, and a metric over the files that happened to parse would be a number nobody could reproduce.
	_, err := describeSources([]source{{identifier: "internal/api/handler.go", text: "package api\n\nfunc {\n"}})

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("describeSources error = %v, want a *archerror.TechnicalError", err)
	}
	if technical.Subject != "internal/api/handler.go" {
		t.Errorf("TechnicalError.Subject = %q, want the file that could not be parsed", technical.Subject)
	}
}

func TestExtractFileInfoReadsEachIdentifierInOrder(t *testing.T) {
	// The gathering step a rule about numbers runs after its scope is resolved: the identifiers the selection
	// produced, turned back into files on the host, read and counted.
	root := writeMeasuredProject(t)

	files, err := ExtractFileInfo(root, []string{"main.go", "internal/api/handler.go"})
	if err != nil {
		t.Fatalf("ExtractFileInfo failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("ExtractFileInfo described %d files, want the 2 it was given", len(files))
	}
	if files[0].Path != "main.go" || files[1].Path != "internal/api/handler.go" {
		t.Errorf("ExtractFileInfo described %q then %q, want the order the identifiers arrived in", files[0].Path, files[1].Path)
	}
	if files[0].Directory != "." || files[1].Directory != "internal/api" {
		t.Errorf("directories are %q and %q, want the root's own identifier and the folder holding the other", files[0].Directory, files[1].Directory)
	}
	if files[1].LinesOfCode != 8 || files[1].ImportCount != 1 || files[1].FunctionCount != 1 {
		t.Errorf("internal/api/handler.go was described as %+v, want the counts it was written with", files[1])
	}
}

func TestExtractFileInfoTakesIdentifiersAndNotHostPaths(t *testing.T) {
	// An identifier is forward-slashed on every operating system, and joining it onto the root is what turns
	// it back into a path — so the same call measures the same file on Windows and on Linux.
	files, err := ExtractFileInfo(writeMeasuredProject(t), []string{"internal/db/conn.go"})
	if err != nil {
		t.Fatalf("ExtractFileInfo failed: %v", err)
	}

	if files[0].Path != "internal/db/conn.go" {
		t.Errorf("Path = %q, want the identifier it was asked for, whatever the host separator is", files[0].Path)
	}
}

func TestExtractFileInfoOfNoIdentifiersMeasuresNothing(t *testing.T) {
	// A rule that selected nothing is the empty-test guard's business, and it is asked before this, so an
	// empty selection is an ordinary answer here rather than an error.
	files, err := ExtractFileInfo(writeMeasuredProject(t), nil)
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
	_, err := ExtractFileInfo(writeMeasuredProject(t), []string{"main.go", "internal/db/gone.go"})

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExtractFileInfo error = %v, want a *archerror.TechnicalError", err)
	}
	if technical.Subject != "internal/db/gone.go" {
		t.Errorf("TechnicalError.Subject = %q, want the file that could not be read", technical.Subject)
	}
}

// statementFixture is a body holding one of most kinds of statement, nested: the assignment, the range, the
// if, the assignment and the branch inside it, the switch, the two assignments in its clauses and the return
// are nine, while the blocks and the case clauses are none.
const statementFixture = `package api

func Total(values []int) int {
	total := 0
	for _, value := range values {
		if value > 0 {
			total += value
			continue
		}
		switch value {
		case -1:
			total--
		default:
			total = 0
		}
	}
	return total
}
`

// describeOne counts one source, the way describeSources does for a whole selection.
func describeOne(t *testing.T, text string) FileInfo {
	t.Helper()

	files, err := describeSources([]source{{identifier: "internal/api/handler.go", text: text}})
	if err != nil {
		t.Fatalf("describeSources(%q) failed: %v", text, err)
	}
	return files[0]
}

// writeMeasuredProject writes the project the tests here read: a file at the root, and two below it.
func writeMeasuredProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":  "module example.com/fixture\n\ngo 1.26\n",
		"main.go": "package main\n\nfunc main() {}\n",
		// Eight lines of code, one import, one function and one class with two fields and one method.
		"internal/api/handler.go": "package api\n\nimport \"fmt\"\n\n// Handler serves.\ntype Handler struct {\n\tname string\n\tsize int\n}\n\nfunc New() Handler { return Handler{} }\n\nfunc (h Handler) Handle() { fmt.Println(h.name) }\n",
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
