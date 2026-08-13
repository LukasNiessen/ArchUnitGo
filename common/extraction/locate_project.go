package extraction

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// moduleFileName is the file whose presence makes a directory a Go project root. Modules are how the
// toolchain itself decides where a project begins, so the library uses the same answer rather than a
// convention of its own.
const moduleFileName = "go.mod"

// locatorOperation names the call at fault in every error this file returns: the locator the user
// passed to an entry point, or the locator they did not pass and now need to.
const locatorOperation = "project locator"

// ErrModuleFileNotFound is the reason a project could not be located: neither the starting directory
// nor any of its parents holds a go.mod, so there is no module to analyze. The fix is to run the test
// from inside the project or to pass a ProjectLocator naming it.
var ErrModuleFileNotFound = errors.New("no go.mod file in this directory or any parent")

// ErrNotADirectory is the reason a path was rejected: a project is located from a directory, and this
// path is a file, a device or something else the walk cannot start at.
var ErrNotADirectory = errors.New("not a directory")

// ProjectLocator says where the project under analysis is. Every entry point takes one optionally —
// nil means auto-detect, which is what a test run inside the project it is about always wants:
//
//	thisProject := archunit.ProjectFiles(nil)
//	thatProject := archunit.ProjectFiles(&extraction.ProjectLocator{Directory: dir})
//
// It is an options bag rather than a bare string so that a second way of naming a project can be
// added without changing the signature of every entry point, and a *ProjectLocator is always allowed
// to be nil.
type ProjectLocator struct {
	// Directory is where the upward search for a go.mod starts. It may be the project root itself or
	// any directory inside it, absolute or relative to the working directory — the search walks up
	// either way, so a caller never has to know how far up the root is.
	//
	// Empty — the default — means the process's working directory, which under `go test` is the
	// directory of the test's own package and therefore inside the project the test is about.
	Directory string
}

// LocateProject finds the root of the project the rules are about: the nearest directory at or above
// the locator's starting point that holds a go.mod. That directory is the SOURCE stage of the
// pipeline in one value — every identifier in the extracted graph is relative to it.
//
// The result is an absolute path in the host's own form, cleaned but with symlinks left alone. It is
// a path and not an identifier: it is what the walk and the toolchain are pointed at, while the
// identifiers a rule matches against are the project-relative strings ExtractSourceFiles derives
// from it.
//
// The nearest go.mod wins, so a project holding a nested module — a tools module, a fixture project —
// resolves to the inner one when the search starts inside it. Not finding a go.mod at all is a
// UserError rather than a TechnicalError: nothing failed, the library was simply pointed at something
// that is not a Go project, and only the caller can say where the project really is.
func LocateProject(locator *ProjectLocator) (string, error) {
	start, subject, err := startingDirectory(locator)
	if err != nil {
		return "", err
	}

	directory := start
	for {
		if holdsModuleFile(directory) {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			// filepath.Dir is its own fixed point at the filesystem root, on every platform, which is
			// what ends the walk.
			return "", archerror.NewUserError(locatorOperation, subject, ErrModuleFileNotFound)
		}
		directory = parent
	}
}

// startingDirectory resolves where the upward search begins, and the subject an error should quote:
// the directory as the user typed it when they named one, so that they can find it in their own test,
// and the resolved working directory when they did not.
func startingDirectory(locator *ProjectLocator) (string, string, error) {
	if locator == nil || locator.Directory == "" {
		working, err := os.Getwd()
		if err != nil {
			return "", "", archerror.NewTechnicalError("find the working directory to look for a go.mod in", "", err)
		}
		return filepath.Clean(working), working, nil
	}

	absolute, err := filepath.Abs(locator.Directory)
	if err != nil {
		return "", "", archerror.NewUserError(locatorOperation, locator.Directory, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", "", archerror.NewUserError(locatorOperation, locator.Directory, err)
	}
	if !info.IsDir() {
		return "", "", archerror.NewUserError(locatorOperation, locator.Directory, ErrNotADirectory)
	}
	return absolute, locator.Directory, nil
}

// holdsModuleFile reports whether this directory is a module root. A directory named go.mod is not a
// module file, so the search walks past it rather than declaring a project where there is none.
func holdsModuleFile(directory string) bool {
	info, err := os.Stat(filepath.Join(directory, moduleFileName))
	return err == nil && !info.IsDir()
}
