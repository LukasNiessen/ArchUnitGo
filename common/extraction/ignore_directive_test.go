package extraction

import (
	"slices"
	"testing"
)

// directivePlacements is one file holding every place a directive can be written and every place one
// looks like it might be but is not. The import paths name what each line is testing, so a failure
// reads as the case that broke.
const directivePlacements = `package api

//archunit:ignore
import "example.com/above-a-lone-import"

import "example.com/trailing-a-lone-import" //archunit:ignore

//archunit:ignore
import (
	"example.com/first-in-the-block"
	"example.com/trailing" //archunit:ignore
	"example.com/after-a-trailing-directive"
	"example.com/ordinary" // an ordinary comment
	//archunit:ignore layers,slices
	"example.com/below-a-scoped-directive"
	// the driver registers itself with database/sql
	//archunit:ignore // and nothing in this file calls it
	"example.com/below-a-directive-with-a-reason"

	//archunit:ignore

	"example.com/across-a-blank-line"
)

import ( //archunit:ignore
	"example.com/beside-the-parenthesis"
)
`

func TestParseIgnoreDirective(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    IgnoreDirective
	}{
		{
			name:    "the canonical Go form",
			comment: "//archunit:ignore",
			want:    IgnoreDirective{Present: true},
		},
		{
			name:    "the sibling's spelling, transliterated",
			comment: "// archunit: ignore",
			want:    IgnoreDirective{Present: true},
		},
		{
			name:    "one scope",
			comment: "//archunit:ignore layers",
			want:    IgnoreDirective{Present: true, Scopes: "layers"},
		},
		{
			name:    "two, comma-separated as the family writes them",
			comment: "//archunit:ignore layers,slices",
			want:    IgnoreDirective{Present: true, Scopes: "layers,slices"},
		},
		{
			name:    "with the space a reader puts after a comma",
			comment: "//archunit:ignore layers, slices",
			want:    IgnoreDirective{Present: true, Scopes: "layers,slices"},
		},
		{
			name:    "separated by whitespace alone",
			comment: "//archunit:ignore  layers\tslices",
			want:    IgnoreDirective{Present: true, Scopes: "layers,slices"},
		},
		{
			name:    "a reason after the directive is prose, not a scope",
			comment: "//archunit:ignore // the driver registers itself",
			want:    IgnoreDirective{Present: true},
		},
		{
			name:    "and a reason after the scopes",
			comment: "//archunit:ignore layers // approved in ADR-7",
			want:    IgnoreDirective{Present: true, Scopes: "layers"},
		},
		{
			name:    "an ordinary comment",
			comment: "// the driver registers itself with database/sql",
		},
		{
			name:    "somebody else's directive",
			comment: "//nolint:gochecknoglobals // immutable lookup table",
		},
		{
			name:    "the toolchain's own",
			comment: "//go:build !windows",
		},
		{
			name:    "the tool with no verb",
			comment: "//archunit:",
		},
		{
			name:    "a verb this library does not read",
			comment: "//archunit:only files",
		},
		{
			name:    "a longer word beginning with the verb",
			comment: "//archunit:ignored",
		},
		{
			name:    "and one that merely starts the same way",
			comment: "//archunit:ignore-generated",
		},
		{
			name: "a block comment, which the language never writes a directive as",
			// It cannot be the last thing on the line an import is on, so reading one would accept a
			// spelling gofmt and every other Go tool disagree about.
			comment: "/*archunit:ignore*/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseIgnoreDirective(test.comment); got != test.want {
				t.Errorf("parseIgnoreDirective(%q) = %+v, want %+v", test.comment, got, test.want)
			}
		})
	}
}

func TestIgnoreDirectiveNames(t *testing.T) {
	tests := []struct {
		name      string
		directive IgnoreDirective
		want      []string
	}{
		{name: "no directive at all", directive: IgnoreDirective{}},
		{name: "a directive that named no scope", directive: IgnoreDirective{Present: true}},
		{
			name:      "one scope",
			directive: IgnoreDirective{Present: true, Scopes: "layers"},
			want:      []string{"layers"},
		},
		{
			name:      "the order the file wrote them in",
			directive: IgnoreDirective{Present: true, Scopes: "slices,layers"},
			want:      []string{"slices", "layers"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.directive.Names(); !slices.Equal(got, test.want) {
				t.Errorf("Names() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIgnoreDirectiveAppliesIn(t *testing.T) {
	tests := []struct {
		name      string
		directive IgnoreDirective
		scopes    []string
		want      bool
	}{
		{
			name:      "no directive is no reason to drop an import",
			directive: IgnoreDirective{},
			want:      false,
		},
		{
			name:      "an unscoped directive needs no configuration to be believed",
			directive: IgnoreDirective{Present: true},
			want:      true,
		},
		{
			name:      "and applies to an analysis that answers to scopes as well",
			directive: IgnoreDirective{Present: true},
			scopes:    []string{"layers"},
			want:      true,
		},
		{
			name:      "a scoped directive applies where its scope is answered to",
			directive: IgnoreDirective{Present: true, Scopes: "layers"},
			scopes:    []string{"layers"},
			want:      true,
		},
		{
			name:      "one of several is enough",
			directive: IgnoreDirective{Present: true, Scopes: "layers,slices"},
			scopes:    []string{"files", "slices"},
			want:      true,
		},
		{
			name:      "elsewhere the import is an ordinary dependency",
			directive: IgnoreDirective{Present: true, Scopes: "layers"},
			scopes:    []string{"files"},
			want:      false,
		},
		{
			name: "and so it is by default, which is what makes a misspelled scope visible",
			// The safe direction: a scope name nobody answers to leaves the dependency in the graph, so the
			// rule reports it. The other way round, the typo would silently suppress a real violation.
			directive: IgnoreDirective{Present: true, Scopes: "layer"},
			want:      false,
		},
		{
			name:      "a name is matched whole",
			directive: IgnoreDirective{Present: true, Scopes: "layers"},
			scopes:    []string{"layer"},
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.directive.AppliesIn(test.scopes); got != test.want {
				t.Errorf("%+v.AppliesIn(%v) = %v, want %v", test.directive, test.scopes, got, test.want)
			}
		})
	}
}

func TestExtractImportsReadsTheDirectiveWhereverAFileWroteIt(t *testing.T) {
	path := writeSourceFile(t, "handler.go", directivePlacements)

	imports, err := ExtractImports(path)
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	// The whole placement matrix, in source order. Every import here is plain, so the flavor says nothing
	// and the directive is the only thing under test.
	want := []ImportInfo{
		// A lone import declaration is its own line, so a comment above it is above that import.
		{Path: "example.com/above-a-lone-import", Ignore: IgnoreDirective{Present: true}},
		{Path: "example.com/trailing-a-lone-import", Ignore: IgnoreDirective{Present: true}},
		// A directive above a parenthesized block belongs to nothing: the line above this import holds the
		// `import (` that opened the block, not a comment. Leaving a whole file out of the graph is what a
		// folder exclusion is for.
		{Path: "example.com/first-in-the-block"},
		{Path: "example.com/trailing", Ignore: IgnoreDirective{Present: true}},
		// The line above holds the import before it, whose trailing directive is that import's own. This is
		// also the shape go/ast attaches no line comment for at all, which is why the index is built from
		// positions rather than from ast.ImportSpec.Comment.
		{Path: "example.com/after-a-trailing-directive"},
		{Path: "example.com/ordinary"},
		{Path: "example.com/below-a-scoped-directive", Ignore: IgnoreDirective{Present: true, Scopes: "layers,slices"}},
		// Reached across the explanation above it, which is how a directive is written in a comment block.
		{Path: "example.com/below-a-directive-with-a-reason", Ignore: IgnoreDirective{Present: true}},
		// A blank line ends the block above an import, so the directive is not this import's.
		{Path: "example.com/across-a-blank-line"},
		// Beside the opening parenthesis is the declaration's own line, not this import's.
		{Path: "example.com/beside-the-parenthesis"},
	}
	if len(imports) != len(want) {
		t.Fatalf("imports = %+v, want %+v", imports, want)
	}
	for index, imported := range imports {
		if imported != want[index] {
			t.Errorf("imports[%d] = %+v, want %+v", index, imported, want[index])
		}
	}
}

func TestExtractImportsReadsDirectivesByPhysicalLine(t *testing.T) {
	// Generated Go — goyacc output, committed cgo output — carries `//line` directives, and one in column 1
	// is legal inside an import block. The index is about physical adjacency, so it has to be built from
	// the lines the file really has: here the rewritten numbering puts `"strings"` on the same line as the
	// directive above `"fmt"`, and reading the adjusted line would hand `"strings"` a directive the file
	// never wrote on it.
	path := writeSourceFile(t, "generated.go", `package api

import (
	//archunit:ignore
	"fmt"
//line parser.y:4
	"strings"
)
`)

	imports, err := ExtractImports(path)
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	want := []ImportInfo{
		{Path: "fmt", Ignore: IgnoreDirective{Present: true}},
		{Path: "strings"},
	}
	if len(imports) != len(want) {
		t.Fatalf("imports = %+v, want %+v", imports, want)
	}
	for index, imported := range imports {
		if imported != want[index] {
			t.Errorf("imports[%d] = %+v, want %+v", index, imported, want[index])
		}
	}
}

func TestExtractImportsReadsNoDirectiveWhereAFileWroteNone(t *testing.T) {
	// The common case, and the one that must stay cheap and quiet: a file full of ordinary imports and
	// ordinary comments carries no directive anywhere.
	path := writeSourceFile(t, "handler.go", `package api

// Package api handles requests.

import (
	"fmt" // formatting
	// the standard library's own
	"strings"
)
`)

	imports, err := ExtractImports(path)
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	for _, imported := range imports {
		if imported.Ignore.Present {
			t.Errorf("import %+v carries a directive the file did not write", imported)
		}
	}
}
