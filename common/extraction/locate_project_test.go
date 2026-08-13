package extraction

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// writeProject builds a fixture project on disk: a go.mod at the root, plus every listed file, created
// with its parent directories. The files are named by their project-relative identifier, which is what
// an enumeration of the project has to hand back.
func writeProject(t *testing.T, files ...string) string {
	t.Helper()

	root := t.TempDir()
	writeProjectFile(t, root, moduleFileName, "module example.com/fixture\n\ngo 1.26\n")
	for _, file := range files {
		writeProjectFile(t, root, file, "package fixture\n")
	}
	return root
}

func writeProjectFile(t *testing.T, root, identifier, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(identifier))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating the folder for %q failed: %v", identifier, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %q failed: %v", identifier, err)
	}
}

// resolvedPath is what a path looks like once its symlinks are gone. Comparing a located root with the
// temporary directory it was located from needs it: on macOS t.TempDir hands back a path under /var,
// which is a link to /private/var, and which of the two the walk started from depends on how the
// working directory was reached.
func resolvedPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolving %q failed: %v", path, err)
	}
	return resolved
}

func TestLocateProjectFindsTheDirectoryHoldingTheModuleFile(t *testing.T) {
	root := writeProject(t, "internal/api/handler.go")

	// The locator names a directory well inside the project: a user should not have to know how far up
	// the root is.
	located, err := LocateProject(&ProjectLocator{Directory: filepath.Join(root, "internal", "api")})
	if err != nil {
		t.Fatalf("LocateProject failed: %v", err)
	}

	if located != root {
		t.Errorf("LocateProject() = %q, want the directory holding go.mod %q", located, root)
	}
}

func TestLocateProjectAcceptsTheRootItself(t *testing.T) {
	root := writeProject(t)

	located, err := LocateProject(&ProjectLocator{Directory: root})
	if err != nil {
		t.Fatalf("LocateProject failed: %v", err)
	}

	if located != root {
		t.Errorf("LocateProject() = %q, want %q", located, root)
	}
}

func TestLocateProjectStopsAtTheNearestModuleFile(t *testing.T) {
	outer := writeProject(t, "main.go")
	inner := filepath.Join(outer, "tools")
	writeProjectFile(t, inner, moduleFileName, "module example.com/fixture/tools\n\ngo 1.26\n")
	writeProjectFile(t, inner, "cmd/generate/main.go", "package main\n")

	located, err := LocateProject(&ProjectLocator{Directory: filepath.Join(inner, "cmd", "generate")})
	if err != nil {
		t.Fatalf("LocateProject failed: %v", err)
	}

	// A nested module is a project of its own: a rule written inside it is about it, not about the
	// repository that happens to contain it.
	if located != inner {
		t.Errorf("LocateProject() = %q, want the nested module %q", located, inner)
	}
}

func TestLocateProjectAutoDetectsFromTheWorkingDirectory(t *testing.T) {
	root := writeProject(t, "internal/api/handler.go")
	t.Chdir(filepath.Join(root, "internal", "api"))

	// A nil locator is the whole point: auto-detection by default, an explicit locator never required.
	located, err := LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}

	if resolvedPath(t, located) != resolvedPath(t, root) {
		t.Errorf("LocateProject(nil) = %q, want the project above the working directory %q", located, root)
	}
}

func TestLocateProjectAcceptsARelativeStartingDirectory(t *testing.T) {
	root := writeProject(t, "internal/api/handler.go")
	t.Chdir(root)

	located, err := LocateProject(&ProjectLocator{Directory: filepath.Join("internal", "api")})
	if err != nil {
		t.Fatalf("LocateProject failed: %v", err)
	}

	if resolvedPath(t, located) != resolvedPath(t, root) {
		t.Errorf("LocateProject() = %q, want an absolute root %q", located, root)
	}
	if !filepath.IsAbs(located) {
		t.Errorf("LocateProject() = %q, want an absolute path", located)
	}
}

func TestLocateProjectRejectsADirectoryOutsideAnyModule(t *testing.T) {
	outside := t.TempDir()

	located, err := LocateProject(&ProjectLocator{Directory: outside})

	if located != "" {
		t.Errorf("LocateProject() = %q, want no root", located)
	}
	if !errors.Is(err, ErrModuleFileNotFound) {
		t.Fatalf("LocateProject() error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
	// Nothing failed — the library was pointed at something that is not a Go project — so the blame is
	// the caller's, and the message has to quote what they typed for them to find it.
	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("LocateProject() error = %v, want a *archerror.UserError", err)
	}
	if user.Subject != outside {
		t.Errorf("UserError.Subject = %q, want the directory the locator named %q", user.Subject, outside)
	}
}

func TestLocateProjectQuotesTheStartingDirectoryAsTheUserTypedIt(t *testing.T) {
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "sub"), 0o755); err != nil {
		t.Fatalf("creating the starting directory failed: %v", err)
	}
	t.Chdir(outside)

	_, err := LocateProject(&ProjectLocator{Directory: "sub"})

	if !errors.Is(err, ErrModuleFileNotFound) {
		t.Fatalf("LocateProject() error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("LocateProject() error = %v, want a *archerror.UserError", err)
	}
	// The search runs on the resolved absolute path, but the message quotes the relative one the user
	// wrote — an absolute path they never typed is not something they can find in their own test.
	if user.Subject != "sub" {
		t.Errorf("UserError.Subject = %q, want the directory the locator named %q", user.Subject, "sub")
	}
}

func TestLocateProjectWalksPastADirectoryNamedGoMod(t *testing.T) {
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, moduleFileName), 0o755); err != nil {
		t.Fatalf("creating a directory named go.mod failed: %v", err)
	}

	// A directory named go.mod is not a module file, and declaring a project root there would give
	// every identifier in the graph the wrong prefix.
	if _, err := LocateProject(&ProjectLocator{Directory: outside}); !errors.Is(err, ErrModuleFileNotFound) {
		t.Fatalf("LocateProject() error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
}

func TestLocateProjectRejectsAStartingPointThatIsNotADirectory(t *testing.T) {
	root := writeProject(t, "main.go")

	_, err := LocateProject(&ProjectLocator{Directory: filepath.Join(root, "main.go")})

	if !errors.Is(err, ErrNotADirectory) {
		t.Fatalf("LocateProject() error = %v, want it to wrap ErrNotADirectory", err)
	}
	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Errorf("LocateProject() error = %v, want a *archerror.UserError", err)
	}
}

func TestLocateProjectRejectsAStartingPointThatDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "there", "is", "no", "such", "folder")

	_, err := LocateProject(&ProjectLocator{Directory: missing})

	// The wrapped cause reaches the user intact, so a caller can still ask the ordinary question about
	// it rather than matching on prose.
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LocateProject() error = %v, want it to wrap fs.ErrNotExist", err)
	}
	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Errorf("LocateProject() error = %v, want a *archerror.UserError", err)
	}
}

func TestLocateProjectFindsThisRepository(t *testing.T) {
	// The test's working directory is this package's own folder, so auto-detection has to walk up out of
	// common/extraction and land on the repository root.
	located, err := LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}

	if !holdsModuleFile(located) {
		t.Fatalf("LocateProject(nil) = %q, which holds no go.mod", located)
	}
	// Named by its contents rather than by its folder name, which is whatever a clone happens to be
	// called: this file is where the walk started from.
	if _, err := os.Stat(filepath.Join(located, "common", "extraction", "locate_project.go")); err != nil {
		t.Errorf("LocateProject(nil) = %q, want this repository's root: %v", located, err)
	}
}
