package extraction

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// fixtureProjectFiles is a project with one of everything the enumeration has to have an opinion
// about: production code, test files, vendored dependencies, build output, version-control and cache
// directories, a fixture project of its own, files the toolchain ignores, and a file that is not Go.
//
// `a-z.go` and `a/z.go` are there for the ordering: the walk reaches the directory `a` before the file
// `a-z.go`, while the identifier `a-z.go` sorts before `a/z.go`.
func fixtureProjectFiles() []string {
	return []string{
		"main.go",
		"main_test.go",
		"a-z.go",
		"a/z.go",
		"internal/api/handler.go",
		"internal/api/handler_test.go",
		"internal/api/.hidden.go",
		"internal/api/_ignored.go",
		"internal/db/repo.go",
		"internal/db/schema.sql",
		"docs/architecture.md",
		"vendor/github.com/lib/pq/conn.go",
		"node_modules/left-pad/index.go",
		"bin/tool.go",
		"dist/bundled.go",
		".git/hooks/commit.go",
		".cache/analysis/stale.go",
		"_scratch/notes.go",
		"testdata/project/go.mod",
		"testdata/project/broken.go",
	}
}

// identifiers is what a rule sees of an enumeration: the nodes, in the order they were handed back.
func identifiers(files []FileInfo) []string {
	found := make([]string, 0, len(files))
	for _, file := range files {
		found = append(found, file.Identifier)
	}
	return found
}

func TestExtractSourceFilesEnumeratesTheProjectsOwnGoFiles(t *testing.T) {
	root := writeProject(t, fixtureProjectFiles()...)

	files, err := ExtractSourceFiles(root, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	// Everything the defaults leave out is in the fixture: vendored code, build output, the toolchain's
	// invisible folders, a nested fixture project, dot- and underscore-prefixed files, test files and a
	// file that is not Go at all.
	want := []string{
		"a-z.go",
		"a/z.go",
		"internal/api/handler.go",
		"internal/db/repo.go",
		"main.go",
	}
	if got := identifiers(files); !slices.Equal(got, want) {
		t.Errorf("identifiers = %v, want %v", got, want)
	}
}

func TestExtractSourceFilesOrdersIdentifiersReproducibly(t *testing.T) {
	root := writeProject(t, fixtureProjectFiles()...)

	first, err := ExtractSourceFiles(root, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}
	second, err := ExtractSourceFiles(root, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	if !slices.IsSortedFunc(first, compareFilesByIdentifier) {
		t.Errorf("identifiers = %v, want them sorted rather than in walk order", identifiers(first))
	}
	if !slices.Equal(identifiers(first), identifiers(second)) {
		t.Errorf("two enumerations of one project disagree: %v and %v", identifiers(first), identifiers(second))
	}
}

func TestExtractSourceFilesIncludesTestFilesWhenAsked(t *testing.T) {
	root := writeProject(t, fixtureProjectFiles()...)

	files, err := ExtractSourceFiles(root, &SourceOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	want := []string{
		"a-z.go",
		"a/z.go",
		"internal/api/handler.go",
		"internal/api/handler_test.go",
		"internal/db/repo.go",
		"main.go",
		"main_test.go",
	}
	if got := identifiers(files); !slices.Equal(got, want) {
		t.Errorf("identifiers = %v, want %v", got, want)
	}
	for _, file := range files {
		if got := strings.HasSuffix(file.Identifier, "_test.go"); got != file.IsTest {
			t.Errorf("%q has IsTest = %v", file.Identifier, file.IsTest)
		}
	}
}

func TestExtractSourceFilesExcludedFoldersReplaceTheDefaults(t *testing.T) {
	root := writeProject(t, fixtureProjectFiles()...)

	files, err := ExtractSourceFiles(root, &SourceOptions{ExcludedFolders: []string{"internal", "a"}})
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	// The caller's list is the whole list, so vendored code and build output are back in — and the
	// folders the Go toolchain itself ignores are still out, because no caller decides that.
	want := []string{
		"a-z.go",
		"bin/tool.go",
		"dist/bundled.go",
		"main.go",
		"node_modules/left-pad/index.go",
		"vendor/github.com/lib/pq/conn.go",
	}
	if got := identifiers(files); !slices.Equal(got, want) {
		t.Errorf("identifiers = %v, want %v", got, want)
	}
}

func TestExtractSourceFilesWithoutExclusionsStillSkipsWhatTheToolchainIgnores(t *testing.T) {
	root := writeProject(t, fixtureProjectFiles()...)

	// A non-nil empty list is how a caller says "exclude nothing", as opposed to nil, which means the
	// defaults.
	files, err := ExtractSourceFiles(root, &SourceOptions{ExcludedFolders: []string{}})
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	found := identifiers(files)
	if !slices.Contains(found, "vendor/github.com/lib/pq/conn.go") {
		t.Errorf("identifiers = %v, want the vendored file the caller stopped excluding", found)
	}
	for _, identifier := range found {
		for _, ignored := range []string{".git/", ".cache/", "_scratch/", "testdata/"} {
			if strings.HasPrefix(identifier, ignored) {
				t.Errorf("identifiers = %v, want nothing under %q", found, ignored)
			}
		}
	}
}

func TestExtractSourceFilesReportsWhereEachFileIs(t *testing.T) {
	root := writeProject(t, fixtureProjectFiles()...)

	files, err := ExtractSourceFiles(root, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	for _, file := range files {
		// Path is for whatever opens the file, so it has to be openable and absolute.
		if !filepath.IsAbs(file.Path) {
			t.Errorf("%q has Path %q, want an absolute path", file.Identifier, file.Path)
		}
		if _, err := os.ReadFile(file.Path); err != nil {
			t.Errorf("%q has Path %q, which cannot be read: %v", file.Identifier, file.Path, err)
		}
		// Identifier is what a user pattern is matched against, so it is the canonical, project-relative
		// form of that same path and never a host path.
		if strings.Contains(file.Identifier, `\`) || filepath.IsAbs(file.Identifier) {
			t.Errorf("Identifier = %q, want a project-relative, forward-slashed identifier", file.Identifier)
		}
		relative, inside := RelativeIdentifier(root, file.Path)
		if !inside || relative != file.Identifier {
			t.Errorf("Identifier = %q, want %q, the path made relative to the project root", file.Identifier, relative)
		}
	}
}

func TestExtractSourceFilesResolvesARelativeRoot(t *testing.T) {
	root := writeProject(t, "internal/api/handler.go")
	t.Chdir(root)

	files, err := ExtractSourceFiles(".", nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	if got := identifiers(files); !slices.Equal(got, []string{"internal/api/handler.go"}) {
		t.Fatalf("identifiers = %v, want the one file in the project", got)
	}
	if !filepath.IsAbs(files[0].Path) {
		t.Errorf("Path = %q, want an absolute path even from a relative root", files[0].Path)
	}
}

func TestExtractSourceFilesEnumeratesAProjectInsideTestdata(t *testing.T) {
	// The rule that testdata is invisible applies to what the walk finds, not to where it starts: a
	// fixture project kept in testdata is a project in its own right, and later issues analyze one.
	outer := writeProject(t, "testdata/project/go.mod", "testdata/project/main.go")
	fixture := filepath.Join(outer, "testdata", "project")

	files, err := ExtractSourceFiles(fixture, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	if got := identifiers(files); !slices.Equal(got, []string{"main.go"}) {
		t.Errorf("identifiers = %v, want the fixture project's own file", got)
	}
}

func TestExtractSourceFilesDoesNotExcludeTheRootItself(t *testing.T) {
	// Neither half of the exclusion policy applies to the directory the walk starts at: a project may
	// live in one the walk would refuse to enter if it met it further down — the toolchain's own
	// `testdata`, or a `build` from the configurable list.
	root := writeProject(t, "testdata/main.go", "build/tool.go")

	insideTestdata, err := ExtractSourceFiles(filepath.Join(root, testDataFolder), nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}
	if got := identifiers(insideTestdata); !slices.Equal(got, []string{"main.go"}) {
		t.Errorf("identifiers = %v, want the file in the testdata directory the walk started at", got)
	}

	insideBuild, err := ExtractSourceFiles(filepath.Join(root, "build"), nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}
	if got := identifiers(insideBuild); !slices.Equal(got, []string{"tool.go"}) {
		t.Errorf("identifiers = %v, want the file in the build directory the walk started at", got)
	}
}

func TestExtractSourceFilesEnumeratesAProjectReachedThroughASymlink(t *testing.T) {
	// LocateProject leaves symlinks alone, so the root it hands back may end in one. filepath.WalkDir
	// lstats what it is pointed at, so without resolving that link first the root arrives as a
	// non-directory entry and a whole project enumerates as zero files.
	root := writeProject(t, "main.go", "internal/api/handler.go")
	link := filepath.Join(t.TempDir(), "linked-project")
	if err := os.Symlink(root, link); err != nil {
		t.Fatalf("linking %q to %q failed: %v", link, root, err)
	}

	files, err := ExtractSourceFiles(link, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	want := []string{"internal/api/handler.go", "main.go"}
	if got := identifiers(files); !slices.Equal(got, want) {
		t.Errorf("identifiers = %v, want %v", got, want)
	}
	for _, file := range files {
		if _, err := os.ReadFile(file.Path); err != nil {
			t.Errorf("%q has Path %q, which cannot be read: %v", file.Identifier, file.Path, err)
		}
	}
}

func TestExtractSourceFilesFindsNothingRatherThanFailingInAnEmptyProject(t *testing.T) {
	root := writeProject(t)

	files, err := ExtractSourceFiles(root, nil)
	// Whether an empty selection is a problem is a rule's question, answered by the empty-test guard.
	// Enumeration has no opinion, and turning it into an error here would take the choice away.
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("files = %v, want none", files)
	}
}

func TestExtractSourceFilesRejectsARootThatIsNotADirectory(t *testing.T) {
	root := writeProject(t, "main.go")

	_, err := ExtractSourceFiles(filepath.Join(root, "main.go"), nil)

	if err == nil {
		t.Fatal("ExtractSourceFiles walked a file as if it were a project")
	}
	// The root normally comes from LocateProject, so a root that is not a directory is the library or
	// its environment failing, not the rule being wrong.
	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExtractSourceFiles error = %v, want a *archerror.TechnicalError", err)
	}
	if !errors.Is(err, ErrNotADirectory) {
		t.Errorf("ExtractSourceFiles error = %v, want it to wrap ErrNotADirectory", err)
	}
}

func TestExtractSourceFilesRejectsARootThatDoesNotExist(t *testing.T) {
	_, err := ExtractSourceFiles(filepath.Join(t.TempDir(), "no", "such", "project"), nil)

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExtractSourceFiles error = %v, want a *archerror.TechnicalError", err)
	}
}

func TestExtractSourceFilesEnumeratesThisRepository(t *testing.T) {
	// The level above the unit tests: a real project, located and enumerated the way a check will do it,
	// with nothing hand-built about either step.
	root, err := LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}

	files, err := ExtractSourceFiles(root, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}

	found := identifiers(files)
	for _, wanted := range []string{"common/extraction/extract_source_files.go", "common/matching/filter.go"} {
		if !slices.Contains(found, wanted) {
			t.Errorf("identifiers = %v, want them to include %q", found, wanted)
		}
	}
	for _, file := range files {
		if file.IsTest {
			t.Errorf("%q is a test file, which the defaults leave out", file.Identifier)
		}
		if strings.HasPrefix(file.Identifier, ".") {
			t.Errorf("%q is under a folder the Go toolchain ignores", file.Identifier)
		}
	}
}
