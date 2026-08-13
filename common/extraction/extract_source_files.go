package extraction

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// goFileExtension is what makes a file Go source. Only .go files become nodes: a cgo `.c` or an
// assembly `.s` file carries no import declaration for the graph to be built out of.
const goFileExtension = ".go"

// testFileSuffix is how the Go toolchain spells a test file, and the only thing IncludeTestFiles has
// to recognize.
const testFileSuffix = "_test" + goFileExtension

// FileInfo is one source file the walk found: the node it becomes in the graph, and where to read it.
//
// The two strings are deliberately different things, and mixing them up is the mistake this type
// exists to prevent. Identifier is canonical, project-relative and the only one a user pattern is ever
// matched against; Path is a host path that means nothing to a rule and everything to whatever opens
// the file.
type FileInfo struct {
	// Identifier is the node this file appears as in the graph: project-relative, forward-slashed and
	// lexically clean, as identifier.go describes — `internal/api/handler.go`.
	Identifier string
	// Path is the file on the host filesystem, absolute and in the host's own form, for whatever reads
	// or parses it. It is never matched against a user pattern.
	Path string
	// IsTest marks a Go test file, one whose name ends in _test.go. Only files a rule asked for are
	// enumerated at all, so this is descriptive rather than a filter: it is here because the walk
	// already decided it, and a caller re-deriving it from the name would be asking the same question
	// twice.
	IsTest bool
}

// ExtractSourceFiles walks a located project and lists the Go source files a rule can be about. It is
// the enumeration half of the SOURCE stage: LocateProject says where the project is, this says what is
// in it, and the graph extractor turns each file into edges.
//
// root is a host path, normally the one LocateProject returned; a relative one is resolved against the
// working directory so that every Path in the result is absolute. A root that is itself a symbolic link
// to a directory is resolved before the walk starts — LocateProject may well hand one back, since it
// leaves links alone — but no link found during the walk is followed, which is what keeps a link
// pointing at a parent directory from turning the walk into a loop.
//
// What is left out, and none of it is an error:
//
//   - anything that is not a .go file;
//   - anything the Go toolchain itself ignores — a name beginning with `.` or `_`, or anything under a
//     dot-prefixed, underscore-prefixed or testdata directory;
//   - the folders SourceOptions.ExcludedFolders names, vendored dependencies and build output by
//     default;
//   - _test.go files, unless SourceOptions.IncludeTestFiles asks for them.
//
// A project with no source files at all yields an empty list and no error. Whether that emptiness is a
// problem is a rule's question, and the empty-test guard is where it is asked.
//
// The result is ordered by identifier, so that a report built from it is reproducible. That is not the
// order the walk visits files in: a directory `a` is walked before a sibling file `a-z.go`, while the
// identifier `a-z.go` sorts before `a/z.go`.
func ExtractSourceFiles(root string, options *SourceOptions) ([]FileInfo, error) {
	resolved := options.WithDefaults()

	walkRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, archerror.NewTechnicalError("resolve the project root", root, err)
	}
	info, err := os.Stat(walkRoot)
	if err != nil {
		return nil, archerror.NewTechnicalError("read the project root", root, err)
	}
	if !info.IsDir() {
		return nil, archerror.NewTechnicalError("read the project root", root, ErrNotADirectory)
	}
	// os.Stat above followed a link at the end of root; filepath.WalkDir instead lstats what it is
	// pointed at, so a linked root would arrive at the callback as a non-directory entry and the walk
	// would visit nothing at all. Resolving it is conditional on there being a link to resolve:
	// resolving unconditionally would rewrite every Path — on macOS /var is itself a link to
	// /private/var — and with it the project-relative form of every identifier.
	if link, err := os.Lstat(walkRoot); err == nil && link.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := filepath.EvalSymlinks(walkRoot)
		if err != nil {
			return nil, archerror.NewTechnicalError("read the project root", root, err)
		}
		walkRoot = linkTarget
	}

	var files []FileInfo
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// The root itself is never excluded: a project may perfectly well live in a directory named
			// `build`, or inside the testdata of the project whose fixture it is.
			if path != walkRoot && resolved.ExcludesFolder(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !isSourceFile(entry.Name(), resolved.IncludeTestFiles) {
			return nil
		}
		identifier, inside := RelativeIdentifier(walkRoot, path)
		if !inside {
			return nil
		}
		files = append(files, FileInfo{
			Identifier: identifier,
			Path:       path,
			IsTest:     isTestFile(entry.Name()),
		})
		return nil
	}
	if err := filepath.WalkDir(walkRoot, walk); err != nil {
		return nil, archerror.NewTechnicalError("walk the project's source files", root, err)
	}

	slices.SortFunc(files, compareFilesByIdentifier)
	return files, nil
}

// isSourceFile reports whether a file of this name is Go source the enumeration should collect.
func isSourceFile(name string, includeTestFiles bool) bool {
	if !strings.HasSuffix(name, goFileExtension) || ignoredByToolchain(name) {
		return false
	}
	return includeTestFiles || !isTestFile(name)
}

// isTestFile reports whether a file of this name is a Go test file.
func isTestFile(name string) bool {
	return strings.HasSuffix(name, testFileSuffix)
}

// compareFilesByIdentifier orders the enumeration. A filesystem cannot hold two files with the same
// project-relative path, so this is a total order and the result is reproducible.
func compareFilesByIdentifier(left, right FileInfo) int {
	return strings.Compare(left.Identifier, right.Identifier)
}
