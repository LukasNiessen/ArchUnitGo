// Package archunit is the public surface of ArchUnitGo: architecture rules written as ordinary Go unit
// tests.
//
//	func TestTheApiDoesNotTouchTheDatabase(t *testing.T) {
//		files, err := archunit.ProjectFiles(nil).InFolder("internal/api/**").SelectFiles(nil)
//		...
//	}
//
// It re-exports and does nothing else. Every name here is defined in a package under common/ or in a
// domain module, and this package exists so that a user imports one path and never has to know the
// layout — which is also why nothing inside the library is allowed to depend on it.
//
// A rule is a value, not an action: building one does no work, and only a terminal reads the project.
// The chain starts at an entry point — ProjectFiles today, one per rule family as they land — and every
// entry point takes an optional *ProjectLocator, where nil means the project the test itself is in.
package archunit

import (
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	filesapi "github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

// ProjectLocator says where the project under analysis is. A nil *ProjectLocator means auto-detect,
// which walks up from the working directory to the nearest go.mod.
type ProjectLocator = extraction.ProjectLocator

// CheckOptions is the one options bag a terminal takes: everything about how a rule is run, as opposed
// to what it says. A nil *CheckOptions means the defaults.
type CheckOptions = fluentapi.CheckOptions

// FilesBuilder is the scope stage of a rule about files, which ProjectFiles and Files return and every
// scope verb hands back a new one of. It is named here so that a half-built rule can be stored in a
// struct field or passed to a helper.
type FilesBuilder = filesapi.FilesBuilder

// ProjectFiles is the entry point of every rule about files: `project files`. The locator is optional
// and nil means auto-detect.
func ProjectFiles(locator *ProjectLocator) FilesBuilder {
	return filesapi.ProjectFiles(locator)
}

// Files is ProjectFiles under the shorter name the family also gives it. The two are one entry point.
func Files(locator *ProjectLocator) FilesBuilder {
	return filesapi.Files(locator)
}

// ClearGraphCache forgets every dependency graph a check extracted earlier in this process, so the next
// rule reads the source again.
//
// A test suite asks about one project many times and extraction is the expensive half of a check, so the
// graph is memoised by default. Call this when the source has changed underneath the library — a test
// that writes a fixture project, or generated code produced between two checks — or set ClearCache on
// the check options, which is the same thing scoped to one rule.
func ClearGraphCache() {
	extraction.ClearGraphCache()
}
