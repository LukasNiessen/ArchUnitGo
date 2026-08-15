// Package extraction is the metrics module's own half of the SOURCE stage: it reads and parses the files
// a rule selected, so that a rule can count what is *in* them.
//
// The dependency graph says nothing about the size or the shape of a file — it is edges between
// identifiers — so a rule about a number needs a second gathering step, the way the files module needs
// one for a user's own predicate. FileInfo and the ClassInfo, FieldInfo and MethodInfo under it are what
// it produces, ExtractFileInfo is the one door to it, and metrics/calculation is what turns those facts
// into a measurement.
//
// This is the one impure package of the module, as common/extraction is of the kernel: it opens files and
// it parses Go. Everything a metric could ever want to know about a file is read here, once, while the
// syntax tree is in hand — how many of its lines carry code, how many statements they are, its imports,
// its functions, the types it declares, and which fields each of a type's methods reaches — so that the
// numeric half stays pure and can be tested against a hand-built FileInfo.
//
// Nothing downstream of this package knows what a statement or a receiver is, which is the same promise
// common/extraction makes about imports: the vocabulary of Go stops here.
package extraction

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// FileInfo is one of the project's files as a metric sees it: where it is, and the counts that were read
// off its syntax tree.
//
// It is the subject of every metric about a file — `lines of code`, `statements`, `imports`,
// `functions`, `classes`, `interfaces` — one per file the rule's scope selected, and it carries the
// classes that file declares so that a metric about a class has subjects too.
//
// Every field is a fact about the file rather than a formula over it: which of the numbers a rule is
// about is metrics/calculation's business, and a count that two metrics share is counted once. That is
// also what lets the numeric half be tested with a FileInfo literal and no project on disk.
//
// It is deliberately neither common/extraction.FileInfo, the enumeration record the walk produces, nor
// files/extraction.FileInfo, the file a user's own predicate is handed: this one carries no host path and
// no source text, because a measurement is a number about an identifier and nothing else has to survive
// the read.
type FileInfo struct {
	// Path is the file's identifier, exactly as the graph and the rule's own patterns spell it:
	// project-relative, forward-slashed and lexically clean — `internal/api/handler.go`.
	Path string
	// Directory is the folder holding the file, as an identifier — `internal/api`. For a file at the
	// project root it is `.`, which is the root's own identifier.
	//
	// It is what a declared type is qualified by, and what says which files are a package's: methods
	// travel with the folder they were declared in rather than with the file, so this is the key
	// ExtractFileInfo attributes them by.
	Directory string
	// LinesOfCode is how many of the file's lines carry code: a line holding nothing but white space
	// does not count, and neither does one holding nothing but a comment.
	//
	// Comments are masked out rather than the lines dropped, so a line that holds code *and* a trailing
	// comment still counts, and a comment ending in the middle of a line of code does not take the code
	// with it. That makes the number the size of the file as a compiler would judge it, which is what a
	// rule about how much there is in one file means; files/extraction.FileInfo.NonBlankLineCount is the
	// other reading, the size as a reader would judge it, and it is the one a user's own predicate gets.
	LinesOfCode int
	// StatementCount is how many statements the file's function bodies are made of, counted one per
	// statement however deeply nested: an `if` and each statement inside it are three, not one.
	//
	// A block, a bare label, a `case` and a `select` case are structure rather than statements of their
	// own and are not counted, so the number does not change when a body gains braces. A declaration at
	// package level is not a statement either — a file of nothing but `var` blocks has none — while
	// `var` inside a function is one.
	StatementCount int
	// ImportCount is how many import specs the file's import declarations hold, blank and dot imports
	// included: an import is a dependency this file wrote down, whatever it does with it, and a rule
	// about how much one file reaches for should not depend on the spelling.
	//
	// It counts specs rather than declarations, so one grouped `import (...)` block of five paths is
	// five.
	ImportCount int
	// FunctionCount is how many functions the file declares at package level, without a receiver.
	//
	// A method is not one of them — it belongs to the type it is declared on, where MethodCount counts
	// it — and neither is a function literal, which is part of the statement that holds it. Those two
	// exclusions are what make this number and ClassInfo.MethodCount two questions rather than one asked
	// twice.
	FunctionCount int
	// Classes are the types this file declares, in the order they were declared. Go has no classes; the
	// vocabulary is the family's, and here it means a declared type, exactly as
	// matching.TargetClassname says. It is empty for a file that declares none.
	Classes []ClassInfo
}

// ExtractFileInfo reads the source of each of these files, parses it and counts what a metric can be
// measured over, in the order the identifiers arrived — which for a rule is the order
// metrics/projection.SelectFiles sorted its selection into, so a report built from the result is
// reproducible.
//
// root is the project root as a host path, the one common/extraction.LocateProject returned, and the
// identifiers are project-relative ones from the graph extracted under it. That pairing is the whole
// contract: an identifier is turned back into a host path by joining it onto the root, because the
// identifier was minted by making the path relative to that same root in the first place.
//
// The files are described together rather than one at a time for one reason: a method is declared beside
// the type it belongs to rather than inside it, so how many methods a type has — and which of its fields
// each of them reaches — is a question about a whole package. The methods of every file handed in are
// attributed to the classes of the same folder,
// which has a consequence worth knowing — a scope that selects some files of a package measures the
// methods it selected, the way a scope that selects some files of a project measures the dependencies
// between them. Widening the scope is what makes the rest visible.
//
// No identifiers at all is an empty result and no error. Whether a rule that selected nothing is a
// failure is the empty-test guard's question, and a terminal asks it before it gets here.
//
// A file that cannot be read, or cannot be parsed, is a TechnicalError naming it rather than a violation:
// the graph says the file is part of the project, so a file that is not there or is not Go is the
// environment disagreeing with the library rather than the code disagreeing with a rule. That is
// deliberately stricter than the graph extractor, which skips a file it cannot parse and carries on —
// there, a file with a syntax error still has a node and the rest of the project is still worth judging;
// here, it is the very thing the rule wanted to measure, and a metric quietly counted over the files that
// happened to compile would be a number nobody could reproduce. The usual cause is a project that changed
// underneath a cached graph, which CheckOptions.ClearCache exists for.
func ExtractFileInfo(root string, identifiers []string) ([]FileInfo, error) {
	sources := make([]source, 0, len(identifiers))
	for _, identifier := range identifiers {
		text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(identifier)))
		if err != nil {
			return nil, archerror.NewTechnicalError("read the source of a project file", identifier, err)
		}
		sources = append(sources, source{identifier: identifier, text: string(text)})
	}
	return describeSources(sources)
}

// source is one file as the metrics extractor reads it: its identifier, and its text. It is what makes
// the counting half of this package a function of strings alone, so that every count below can be tested
// without a project on disk.
type source struct {
	identifier string
	text       string
}

// describeSources counts what a metric needs over each of these sources, and then attributes the methods
// each file declared to the classes of its folder. It is ExtractFileInfo with the filesystem taken out.
//
// The two passes are why this exists at all: the first can only count what one file says, and which type
// a method belongs to is only known once every file of the folder has been read.
func describeSources(sources []source) ([]FileInfo, error) {
	files := make([]FileInfo, 0, len(sources))
	methods := make(map[receiver][]declaredMethod, len(sources))
	for _, read := range sources {
		file, err := describeSource(read, methods)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	attributeMethods(files, methods)
	return files, nil
}

// describeSource parses one file and reads everything a metric can ask about it, adding the methods it
// declares to methods on the way — the tally describeSources hands to attributeMethods afterwards.
func describeSource(read source, methods map[receiver][]declaredMethod) (FileInfo, error) {
	fileSet := token.NewFileSet()
	// Comments are wanted because they are what LinesOfCode has to leave out, and object resolution is
	// not: nothing here follows an identifier to its declaration.
	parsed, err := parser.ParseFile(fileSet, read.identifier, read.text, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return FileInfo{}, archerror.NewTechnicalError("parse the source of a project file", read.identifier, err)
	}

	identifier := extraction.NormalizeIdentifier(read.identifier)
	directory := path.Dir(identifier)
	collectReceiverMethods(methods, directory, parsed)
	return FileInfo{
		Path:           identifier,
		Directory:      directory,
		LinesOfCode:    countLinesOfCode(fileSet, parsed, read.text),
		StatementCount: countStatements(parsed),
		ImportCount:    len(parsed.Imports),
		FunctionCount:  countFunctions(parsed),
		Classes:        extractClassInfo(identifier, directory, parsed),
	}, nil
}

// countLinesOfCode counts the lines of this file that carry code, by blanking every byte the comments
// occupy and then counting what is left.
//
// Masking is what keeps the answer honest at both ends of a line. Dropping whole comment lines instead
// would lose the code before a trailing `// comment`, and counting lines the parser reported as comments
// would keep a line whose code ended inside a `/* ... */`. Newlines are never masked, so the file's own
// line structure survives the blanking and a block comment spanning ten lines blanks ten of them.
func countLinesOfCode(fileSet *token.FileSet, file *ast.File, text string) int {
	masked := []byte(text)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			start := max(fileSet.PositionFor(comment.Pos(), false).Offset, 0)
			end := min(fileSet.PositionFor(comment.End(), false).Offset, len(masked))
			for index := start; index < end; index++ {
				if masked[index] != '\n' {
					masked[index] = ' '
				}
			}
		}
	}
	return countNonBlankLines(string(masked))
}

// countNonBlankLines counts the lines of this text that hold anything but white space. Applied to the
// masked text above, that is the count of lines holding code — which is why this counts white space
// rather than comments and knows nothing about them.
func countNonBlankLines(text string) int {
	count := 0
	for line := range strings.Lines(text) {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// countStatements counts every statement of the file, at every depth.
//
// Five node kinds are deliberately not statements here. A block is punctuation — counting it would make
// a body's number depend on how it is bracketed — a label names the statement that follows it, an empty
// statement is a stray semicolon, and a `case` or `select` clause is the shape of the switch it belongs
// to rather than something the program does.
func countStatements(file *ast.File) int {
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.BlockStmt, *ast.EmptyStmt, *ast.LabeledStmt, *ast.CaseClause, *ast.CommClause:
			return true
		case ast.Stmt:
			count++
		}
		return true
	})
	return count
}

// countFunctions counts the functions this file declares at package level.
//
// A declaration with a receiver is a method and is left out: it is counted on the type it belongs to, by
// collectReceiverMethods, so that a package's functions and its types' methods add up to its declarations
// instead of overlapping. A function literal is not a declaration at all and is part of the statement
// holding it.
func countFunctions(file *ast.File) int {
	count := 0
	for _, declaration := range file.Decls {
		if function, isFunction := declaration.(*ast.FuncDecl); isFunction && function.Recv == nil {
			count++
		}
	}
	return count
}
