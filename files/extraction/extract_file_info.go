// Package extraction is the files module's own half of the SOURCE stage: it reads the text of the files
// a rule selected, so that a rule can judge what is *in* a file rather than what it depends on.
//
// The dependency graph says nothing about a file's contents — it is edges between identifiers — so a
// predicate the user writes themselves, `project files, ..., should, adhere to`, needs a second and much
// smaller gathering step. FileInfo is what it gets, ExtractFileInfo is what produces one per selected
// file, and files/assertion.GatherAdherenceViolations is what judges them.
//
// This is the one impure package of the module, as common/extraction is of the kernel: it opens files.
// Everything derived from a file — its name, its folder, how many of its lines carry something — is
// derived here, once, by NewFileInfo, so that the judging half stays pure and can be tested against a
// hand-built FileInfo.
package extraction

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// FileInfo is one of the project's files as a user predicate sees it: where it is, what it is called and
// what is in it.
//
// It is what the function passed to `adhere to` is handed, one per file the rule's scope selected, and it
// is the whole of what such a function has to work with:
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/**").
//		Should().
//		AdhereTo(func(file archunit.FileInfo) bool {
//			return file.NonBlankLineCount <= 400
//		}, "be at most 400 lines long")
//
// Every field is derived from the identifier and the source text, by NewFileInfo, so a rule about a
// file's size, its name and its folder is one predicate rather than three, and a test can build a
// FileInfo by hand without a project on disk.
//
// It is deliberately not common/extraction.FileInfo, which is the enumeration record the walk produces:
// that one carries a host path and whether the file is a test, and is what points the toolchain at the
// project. This one carries no host path at all. A user predicate is handed identifiers, because those
// are the strings a rule's patterns matched and the only ones that mean the same thing on every machine.
type FileInfo struct {
	// Path is the file's identifier, exactly as the graph and the rule's own patterns spell it:
	// project-relative, forward-slashed and lexically clean — `internal/api/handler.go`. It is not a host
	// path, so it is the same string on every operating system and safe to compare against in a test.
	Path string
	// Name is the file's name with its extension removed — `handler` of `internal/api/handler.go`. It is
	// the name a Go convention is usually about: `*_test`, `*_gen`, `doc`.
	Name string
	// Extension is the last extension of the name, leading dot included — `.go`. It is empty for a name
	// that has none.
	Extension string
	// Directory is the folder holding the file, as an identifier — `internal/api`. For a file at the
	// project root it is `.`, which is the root's own identifier and what the scope verb `in folder`
	// matches such a file against.
	Directory string
	// Source is the file's full text, exactly as it is on disk: nothing stripped, no lines dropped, the
	// original line endings intact. A predicate about what a file contains reads it directly —
	// strings.Contains, a regexp of the user's own, a count of occurrences.
	Source string
	// NonBlankLineCount is how many of the file's lines carry something. A line holding nothing but white
	// space does not count, and neither does the empty last line of a file that ends in a newline, so
	// this is the size of the file as a reader would judge it rather than a count of `\n`.
	//
	// It is counted once, when the file is read, because a predicate that asks for it asks for it per
	// file and one pass over the text is cheaper than the read that produced it.
	NonBlankLineCount int
}

// NewFileInfo derives everything a user predicate sees about a file from the two things that are actually
// read: the file's identifier, and its source text.
//
// It is pure — no filesystem, whatever the identifier says — which is what lets a test of a rule's
// predicate build its own file:
//
//	file := extraction.NewFileInfo("internal/api/handler.go", "package api\n")
//
// The identifier is normalised on the way in, through common/extraction.NormalizeIdentifier, so a
// hand-written `internal\api\handler.go` describes the same file as the graph's own spelling of it and
// the derived name and folder do not depend on which the caller used.
//
// The empty identifier is the absence of a file rather than a file at the project root, which is what
// NormalizeIdentifier means by it, so there is no name, no extension and no folder to derive from it.
func NewFileInfo(identifier, source string) FileInfo {
	normalized := extraction.NormalizeIdentifier(identifier)
	info := FileInfo{
		Path:              normalized,
		Source:            source,
		NonBlankLineCount: countNonBlankLines(source),
	}
	if normalized == "" {
		return info
	}
	info.Extension = path.Ext(normalized)
	info.Name = strings.TrimSuffix(path.Base(normalized), info.Extension)
	info.Directory = path.Dir(normalized)
	return info
}

// ExtractFileInfo reads the source of each of these files and describes it, in the order the identifiers
// arrived — which for a rule is the order files/projection.SelectFiles sorted its selection into, so a
// report built from the result is reproducible.
//
// root is the project root as a host path, the one common/extraction.LocateProject returned, and the
// identifiers are project-relative ones from the graph extracted under it. That pairing is the whole
// contract: an identifier is turned back into a host path by joining it onto the root, because the
// identifier was minted by making the path relative to that same root in the first place.
//
// No identifiers at all is an empty result and no error. Whether a rule that selected nothing is a
// failure is the empty-test guard's question, and a terminal asks it before it gets here.
//
// A file that cannot be read is a TechnicalError naming it, not a violation: the graph says the file is
// part of the project, so failing to open it is the environment disagreeing with the library rather than
// the code disagreeing with a rule. The usual cause is a project that changed underneath a cached graph,
// which CheckOptions.ClearCache exists for.
func ExtractFileInfo(root string, identifiers []string) ([]FileInfo, error) {
	files := make([]FileInfo, 0, len(identifiers))
	for _, identifier := range identifiers {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(identifier)))
		if err != nil {
			return nil, archerror.NewTechnicalError("read the source of a project file", identifier, err)
		}
		files = append(files, NewFileInfo(identifier, string(source)))
	}
	return files, nil
}

// countNonBlankLines counts the lines of this text that hold anything but white space.
//
// Blank lines are left out because a predicate about a file's size is about how much there is to read:
// the alternative — counting separators — would make a file's measured length depend on how generously it
// was spaced and on whether it ends in a newline.
func countNonBlankLines(source string) int {
	count := 0
	for line := range strings.Lines(source) {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
