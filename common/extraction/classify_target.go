package extraction

import (
	"path/filepath"
	"strings"
)

// targetIndex is what the Go toolchain said the project is made of, arranged for the one question the
// EXTRACT stage asks of every import declaration: does this dependency point at the project's own code,
// or at somebody else's? An Edge's External flag is that answer, and this is the only place it is
// decided.
//
// The toolchain's answer comes first and is authoritative: the project's own package paths are the ones
// `go list` reported for it, which is what makes vendoring, `replace` directives, build constraints and
// the standard library somebody else's problem instead of arithmetic on import strings. The module's own
// path is consulted only for an import path the toolchain reported nothing at all about — see
// ownsImportPath, which is the only arithmetic on an import string in the library.
type targetIndex struct {
	// root is the resolved project root as a host path, the directory holding the module's go.mod. It is
	// here for the nested-module check, which is the one question about an import path that the
	// filesystem, rather than the toolchain, has the answer to.
	root string
	// modulePath is the path of the module under analysis — `github.com/LukasNiessen/ArchUnitGo` — as the
	// toolchain reported it. Empty means it could not be determined, which switches ownsImportPath off
	// and leaves the toolchain's answer as the only one.
	modulePath string
	// packages maps each package path the project's own source may import to the identifiers of the nodes
	// that package is built from. A path present with no identifiers is a package of the project's whose
	// files are not nodes: it is not external, and it has nothing to point at.
	packages map[string][]string
}

// classify decides what one import path points at. There are three answers, and the two results tell
// them apart:
//
//   - one or more identifiers, not external: an import of the project's own code, resolved to every file
//     the imported package is built from;
//   - no identifiers, not external: still the project's own code, but with no node to point at — a
//     package the walk excluded, one whose every file a build constraint excluded, or one that does not
//     exist. It contributes no edge at all;
//   - no identifiers, external: the standard library, a dependency module, a vendored copy or a module
//     nested inside the project. The caller keeps the import path itself as the target.
func (i targetIndex) classify(importPath string) ([]string, bool) {
	if nodes, found := i.packages[importPath]; found {
		return nodes, false
	}
	if i.ownsImportPath(importPath) {
		return nil, false
	}
	return nil, true
}

// ownsImportPath reports whether an import path the toolchain reported no package for is nevertheless
// the project's own code. It exists because the toolchain has nothing to say about a package that is not
// there: an import of `<module>/internal/nope` — a package half-written, renamed or deleted — would
// otherwise be classified as an external module and fire every rule about third-party dependencies,
// naming the project's own path as the offender.
//
// Being under the module's path is not quite enough, because a module nested inside the project may
// declare a path under its parent's. Such a module is distributed, versioned and resolved on its own, so
// a dependency on it is external — and a go.mod on the way down is how the toolchain itself tells the
// two apart.
func (i targetIndex) ownsImportPath(importPath string) bool {
	if i.modulePath == "" {
		return false
	}
	folder, under := moduleRelativeFolder(i.modulePath, importPath)
	if !under {
		return false
	}
	return !insideNestedModule(i.root, folder)
}

// moduleRelativeFolder splits an import path into the project folder it names, as an identifier relative
// to the project root, and reports false for a path that is not the module's at all. The empty string is
// the root folder itself, which is what the module path imported on its own names.
//
// The separator is what makes this a containment test rather than a prefix test: `example.com/fixtures/api`
// is not part of `example.com/fixture`, and a bare prefix test would say that it was. Normalising what is
// left is what keeps a `..` in an import path — a string a file wrote, not a path a caller chose — from
// naming a folder outside the project.
func moduleRelativeFolder(modulePath, importPath string) (string, bool) {
	if importPath == modulePath {
		return "", true
	}
	suffix, under := strings.CutPrefix(importPath, modulePath+"/")
	if !under {
		return "", false
	}
	folder := NormalizeIdentifier(suffix)
	if folder == "" || folder == "." || strings.HasPrefix(folder, "/") || escapesUpwards(folder) {
		return "", false
	}
	return folder, true
}

// insideNestedModule reports whether the folder an import path names lies in a module of its own: it holds
// a go.mod, or one of the folders between it and the project root does. The project root's own go.mod is
// deliberately not looked at — that is the module the project *is*.
//
// folder is a normalised project-relative identifier, so the search is bounded by the import path rather
// than by the filesystem, and a folder that is not there simply holds no go.mod.
func insideNestedModule(root, folder string) bool {
	if folder == "" {
		return false
	}

	directory := root
	for _, element := range strings.Split(folder, "/") {
		directory = filepath.Join(directory, element)
		if holdsModuleFile(directory) {
			return true
		}
	}
	return false
}
