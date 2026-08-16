package extraction

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// ImportInfo is one import declaration, as the file wrote it: where it points and how. It is the raw
// material of an Edge and not an Edge itself, because an import names a *package* while a node in the
// graph is a file — turning the one into the other is resolution, and it needs the whole project.
type ImportInfo struct {
	// Path is the import path exactly as the source quoted it, unquoted: `fmt`,
	// `github.com/LukasNiessen/ArchUnitGo/common/matching`. It is a package path and not yet an
	// identifier of anything in the graph.
	Path string
	// Kind is the flavor of the declaration — plain, aliased, blank or dot.
	Kind ImportKind
	// Ignore is the `//archunit:ignore` directive the file wrote on this import, or the zero
	// IgnoreDirective when it wrote none. Whether it is honored is the extractor's question, asked
	// through SourceOptions.IgnoresImport, because a scoped directive depends on what the analysis is.
	Ignore IgnoreDirective
}

// ExtractImports reads one Go file and lists the imports it declares, in the order they appear. It is
// the parsing half of the EXTRACT stage: what each import *points at* is decided by ExtractGraph,
// which is the only place that knows the whole project.
//
// path is a host path, normally a FileInfo.Path from ExtractSourceFiles.
//
// Only the file's import declarations are parsed, so a file whose body is broken, generated or vast
// costs no more than its header. Build constraints are deliberately not evaluated here: whether a file
// is in the build is the toolchain's answer, and ExtractGraph asks it there.
//
// The comments around those declarations are read too, for the one thing a file may say about its own
// dependencies: an `//archunit:ignore` directive lands on the ImportInfo it was written on. Reading it
// is all that happens here — honoring it depends on the analysis, and SourceOptions.IgnoresImport is
// where that is decided.
//
// A file that cannot be parsed is not fatal. The imports found before the parser gave up are returned
// alongside the error, because an import block sits at the top of a file and is normally intact even
// when the rest of it is not — a caller extracting a graph uses them and carries on, so that one
// unparsable file does not fail every rule in a suite. A file that cannot be read at all yields no
// imports and the error.
func ExtractImports(path string) ([]ImportInfo, error) {
	// ParseComments for the ignore directive, which is a comment and reachable no other way.
	// SkipObjectResolution: nothing here looks anything up by name, and resolving identifiers over a
	// file we deliberately stopped parsing early would be work thrown away.
	fileSet := token.NewFileSet()
	parsed, parseError := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly|parser.ParseComments|parser.SkipObjectResolution)
	if parsed == nil {
		return nil, archerror.NewTechnicalError("parse the imports of", path, parseError)
	}

	directives := newIgnoreDirectives(fileSet, parsed)
	imports := make([]ImportInfo, 0, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		imported, quoted := importInfo(spec, directives)
		if !quoted {
			continue
		}
		imports = append(imports, imported)
	}
	if parseError != nil {
		return imports, archerror.NewTechnicalError("parse the imports of", path, parseError)
	}
	return imports, nil
}

// importInfo reads one import specification. It reports false for a specification whose path is not a
// quoted string the way the language requires — the shape a half-parsed file leaves behind — because
// there is no package path in it to resolve.
func importInfo(spec *ast.ImportSpec, directives ignoreDirectives) (ImportInfo, bool) {
	if spec.Path == nil {
		return ImportInfo{}, false
	}
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil || path == "" {
		return ImportInfo{}, false
	}
	return ImportInfo{Path: path, Kind: importKind(spec), Ignore: directives.of(spec)}, true
}

// importKind names the flavor of an import declaration from the name the file gave it. An import with
// no name is plain; `_` and `.` are the two names the language gives a meaning of its own, and
// anything else is an alias.
func importKind(spec *ast.ImportSpec) ImportKind {
	if spec.Name == nil {
		return ImportKindPlain
	}
	switch spec.Name.Name {
	case "_":
		return ImportKindBlank
	case ".":
		return ImportKindDot
	default:
		return ImportKindAliased
	}
}
