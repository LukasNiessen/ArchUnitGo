package extraction

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"unicode"
)

const (
	// lineCommentPrefix is what a line comment begins with, and the only comment form a directive is
	// written in. It doubles as the separator between a directive and the reason after it.
	lineCommentPrefix = "//"
	// directiveTool names the tool the directive is addressed to, the way `//go:build` names the Go
	// toolchain.
	directiveTool = "archunit:"
	// ignoreVerb is the one directive this library reads. Nothing else under the `archunit:` prefix means
	// anything, and a comment that is not exactly this is somebody else's.
	ignoreVerb = "ignore"
	// scopeSeparator joins the scope names a directive listed into the canonical form Scopes holds, and
	// is the separator a file writes them with — as `//nolint:a,b` and `-tags=a,b` both do.
	scopeSeparator = ","
	// directiveSpace is the whitespace allowed inside a directive, before the tool and before the verb.
	directiveSpace = " \t"
)

// IgnoreDirective is the comment convention that keeps one import out of the dependency graph:
//
//	import (
//		"fmt"
//		_ "github.com/lib/pq" //archunit:ignore
//	)
//
// It is the Go spelling of ArchUnitPython's `# archunit: ignore`, written the way the language writes a
// machine-readable comment — `//go:build`, `//go:generate`, `//nolint` — so that gofmt leaves it where
// it was put. A space after the `//` and after the colon is accepted too, so that the sibling's
// spelling transliterates and still works for somebody arriving from another port.
//
// The directive belongs to the one import it is written on, either as the comment trailing that import
// or on a comment-only line directly above it. The second is where a directive carrying a reason
// usually goes, because everything after a second `//` is prose rather than scope names:
//
//	import (
//		// pq registers itself with database/sql
//		//archunit:ignore // and nothing in this file calls it
//		_ "github.com/lib/pq"
//	)
//
// A directive above the whole `import (` block, on the line of the `import` keyword, or separated from
// its import by a blank line, belongs to nothing: this is a per-import convention, and leaving a whole
// file or folder out of the graph is what SourceOptions.ExcludedFolders is for.
//
// The zero value is the absence of a directive, which is what all but a handful of imports carry.
type IgnoreDirective struct {
	// Present reports whether the import carries the directive at all. It is what tells the zero value
	// from a directive that named no scopes.
	Present bool
	// Scopes are the analysis scopes the directive named — `//archunit:ignore layers,slices` — as it
	// wrote them, separated by commas. Empty means it named none, and the import is left out of every
	// analysis.
	//
	// It is one comma-separated string rather than a []string for the reason ImportKindSet is a bit set
	// rather than a list: an ImportInfo carrying it stays comparable. Names reads it back as a list, and
	// AppliesIn is the question the extractor asks of it.
	Scopes string
}

// Names lists the scopes the directive named, in the order the file wrote them, and nothing at all for
// a directive that named none.
func (d IgnoreDirective) Names() []string {
	if d.Scopes == "" {
		return nil
	}
	return strings.Split(d.Scopes, scopeSeparator)
}

// AppliesIn reports whether this directive keeps its import out of an analysis answering to these scope
// names. A directive that named no scopes applies to every analysis, which is what a bare
// `//archunit:ignore` means; a scoped one applies only where one of its names is answered to, so an
// import ignored for `layers` is still a dependency everywhere else.
//
// Names are matched exactly. SourceOptions.IgnoreScopes is where the answered scopes come from and it
// is empty by default, so a directive naming a scope nobody answers to leaves its import in the graph —
// the rule then reports the dependency, rather than a misspelled scope silently passing.
func (d IgnoreDirective) AppliesIn(scopes []string) bool {
	if !d.Present {
		return false
	}
	named := d.Names()
	if len(named) == 0 {
		return true
	}
	for _, scope := range named {
		if slices.Contains(scopes, scope) {
			return true
		}
	}
	return false
}

// parseIgnoreDirective reads one comment as an ignore directive, returning the zero IgnoreDirective for
// a comment that is not one — which is almost every comment in a project.
//
// The canonical form is the Go one, `//archunit:ignore`, and the grammar accepted is deliberately a
// little wider: whitespace is allowed after the `//` and after the colon, so that ArchUnitPython's
// `# archunit: ignore` transliterates. Scope names follow the verb, separated by commas or whitespace,
// and a second `//` ends them so that a reason can be written after the directive.
//
// A `/* */` comment is never a directive. The language writes directives as line comments, and only a
// line comment can trail the import it is about.
func parseIgnoreDirective(comment string) IgnoreDirective {
	body, isLineComment := strings.CutPrefix(comment, lineCommentPrefix)
	if !isLineComment {
		return IgnoreDirective{}
	}
	body, addressed := strings.CutPrefix(strings.TrimLeft(body, directiveSpace), directiveTool)
	if !addressed {
		return IgnoreDirective{}
	}
	arguments, ignores := strings.CutPrefix(strings.TrimLeft(body, directiveSpace), ignoreVerb)
	if !ignores {
		return IgnoreDirective{}
	}
	// The verb has to end the word: `//archunit:ignored` and `//archunit:ignore-generated` are somebody
	// else's comment rather than a spelling of this one.
	if arguments != "" && strings.IndexFunc(arguments, isScopeSeparator) != 0 {
		return IgnoreDirective{}
	}
	// Everything from a second `//` on is a reason — `//archunit:ignore layers // approved in ADR-7`.
	// Without this the prose would be read as a scope list, and the directive would silently narrow
	// itself to scopes nobody answers to.
	arguments, _, _ = strings.Cut(arguments, lineCommentPrefix)
	return IgnoreDirective{
		Present: true,
		Scopes:  strings.Join(strings.FieldsFunc(arguments, isScopeSeparator), scopeSeparator),
	}
}

// isScopeSeparator reports whether a character separates two scope names, or the verb from the first of
// them. A comma is what the family writes; whitespace is accepted around one and instead of one,
// because a reader will put a space after a comma.
func isScopeSeparator(char rune) bool {
	return char == ',' || unicode.IsSpace(char)
}

// ignoreDirectives are the ignore directives of one parsed file, indexed by the line they were written
// on, alongside the lines its imports and its package clause occupy. Both halves are needed to tell an
// import's own directive from the trailing directive of the import above it: a comment on the line
// above an import is that import's only when the line holds nothing else.
//
// It is built from the file's comments and the positions of its declarations rather than from
// ast.ImportSpec's own Doc and Comment fields, because those are silent for exactly this question. Two
// shapes carry no line comment at all: an import whose declaration has no parentheses, and one whose
// trailing comment is followed by a comment on the next line. Both are covered in
// ignore_directive_test.go.
type ignoreDirectives struct {
	// comments maps every line a comment occupies to the directive that comment carries, or to the zero
	// IgnoreDirective when it carries none. A comment with nothing to say still gets an entry, because
	// the keys are what "the line above holds a comment" is read from.
	comments map[int]IgnoreDirective
	// code holds every line the package clause and the import declarations occupy — the keyword, the
	// parentheses and the import specifications themselves, and deliberately not the comment-only lines
	// between them.
	code map[int]struct{}
	// fileSet is what turns a position into the line it is on, and is the one the file was parsed with.
	fileSet *token.FileSet
}

// newIgnoreDirectives indexes what one parsed file wrote about its own imports. The file has to have
// been parsed with parser.ParseComments, or there are no comments to index.
func newIgnoreDirectives(fileSet *token.FileSet, file *ast.File) ignoreDirectives {
	found := ignoreDirectives{
		comments: make(map[int]IgnoreDirective, len(file.Comments)),
		code:     make(map[int]struct{}, len(file.Imports)+len(file.Decls)+1),
		fileSet:  fileSet,
	}

	for _, group := range file.Comments {
		for _, comment := range group.List {
			directive := parseIgnoreDirective(comment.Text)
			// Every line the comment spans, so that a block comment of several lines does not leave a gap
			// a directive above it cannot be reached across.
			found.occupyComment(comment.Pos(), comment.End(), directive)
		}
	}

	if file.Name != nil {
		// A file the parser gave up on may have no package clause. When it has one, its line is code, so
		// that a comment trailing it is not read as belonging to an import written on the next line.
		found.occupyCode(file.Name.Pos(), file.Name.End())
	}
	for _, declaration := range file.Decls {
		imports, ok := declaration.(*ast.GenDecl)
		if !ok || imports.Tok != token.IMPORT {
			// Anything that is not an import declaration is code from end to end, comments and all: nothing
			// inside a function or a var block is a directive about an import.
			found.occupyCode(declaration.Pos(), declaration.End())
			continue
		}
		// Not the whole declaration: a parenthesized import block spans the comment-only lines inside it,
		// and those are exactly the lines a directive is written on. So only the lines the keyword, the
		// parentheses and the specifications are on.
		found.occupyCode(imports.TokPos, imports.TokPos)
		found.occupyCode(imports.Lparen, imports.Lparen)
		found.occupyCode(imports.Rparen, imports.Rparen)
		for _, spec := range imports.Specs {
			found.occupyCode(spec.Pos(), spec.End())
		}
	}
	return found
}

// of returns the directive the file wrote about this import: the one trailing it on its own line, or
// else the nearest one on the comment-only lines directly above it.
func (d ignoreDirectives) of(spec *ast.ImportSpec) IgnoreDirective {
	if trailing := d.comments[d.line(spec.End())]; trailing.Present {
		return trailing
	}
	for line := d.line(spec.Pos()) - 1; line > 0; line-- {
		if _, occupied := d.code[line]; occupied {
			// The line above holds code, so a comment on it is that code's own — the trailing directive of
			// the import above this one, which belongs to that import and not to this one.
			break
		}
		directive, commented := d.comments[line]
		if !commented {
			// A blank line, or anything else that is not a comment: the block above this import has ended.
			break
		}
		if directive.Present {
			return directive
		}
	}
	return IgnoreDirective{}
}

// occupyComment records the directive on every line one comment occupies.
func (d ignoreDirectives) occupyComment(from, to token.Pos, directive IgnoreDirective) {
	for line := d.line(from); line <= d.line(to); line++ {
		d.comments[line] = directive
	}
}

// occupyCode records every line a piece of the file's own syntax occupies.
func (d ignoreDirectives) occupyCode(from, to token.Pos) {
	for line := d.line(from); line <= d.line(to); line++ {
		d.code[line] = struct{}{}
	}
}

// line is the physical line a position is on, and 0 for a position the file has none for — an omitted
// parenthesis, which is a line no lookup ever asks about.
//
// Unadjusted, so that a file carrying `//line` directives — goyacc output, committed cgo output, either
// of which may legally put one inside an import block — is indexed by the lines it really has. Every
// question this index answers is about physical adjacency: whether a comment trails an import, whether
// the line above holds code, whether a blank line ended the block. Rewritten line numbers can shift or
// collide, which would lose a directive or attach one to an import that never carried it.
func (d ignoreDirectives) line(position token.Pos) int {
	return d.fileSet.PositionFor(position, false).Line
}
