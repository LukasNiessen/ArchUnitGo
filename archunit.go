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
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	filesapi "github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

// ProjectLocator says where the project under analysis is. A nil *ProjectLocator means auto-detect,
// which walks up from the working directory to the nearest go.mod.
type ProjectLocator = extraction.ProjectLocator

// CheckOptions is the one options bag a terminal takes: everything about how a rule is run, as opposed
// to what it says. A nil *CheckOptions means the defaults.
type CheckOptions = fluentapi.CheckOptions

// Mood is which of the two moods a rule was written in, `should` or `should not`. It is what the mood
// stage of a chain reports, and the flag the library's assertions take so that a rule and its negation
// are one piece of logic.
type Mood = assertion.Mood

const (
	// Should is the positive mood: the rule holds where its predicate is satisfied.
	Should = assertion.Should
	// ShouldNot is the negated mood: the rule holds where its predicate is not satisfied.
	ShouldNot = assertion.ShouldNot
)

// Checkable is a rule that can be run: what every chain ends in, and the one thing a helper that loops
// over a list of rules has to know about them. Check returns one Violation per place the code disagrees
// with the rule, and an empty result is the pass.
type Checkable = fluentapi.Checkable

// Violation is one disagreement between the code and a rule: the atom of every rule's result. It carries
// the thing that disagreed rather than a sentence about it, and Kind is what says which family it belongs
// to.
type Violation = assertion.Violation

// ViolationKind identifies a family of violations — `empty-test`, `file-cycle` — spelled the same way in
// every ArchUnit port. It is what a report groups and phrases by, without asserting on a concrete type
// first.
type ViolationKind = assertion.ViolationKind

// EmptyTestViolation says a rule selected nothing at all: its scope matched no file, so there was nothing
// to judge and a pass would have meant nothing. Every terminal reports it, and CheckOptions.AllowEmptyTests
// is how a user who really means an empty selection opts out.
type EmptyTestViolation = assertion.EmptyTestViolation

// FileCycleViolation says that some of the files a rule selected depend on each other in a circle. It is
// what `should have no cycles` reports, one per cycle, and it carries the cycle itself — printable as the
// readable path `a.go -> b.go -> a.go`.
type FileCycleViolation = filesassertion.CycleViolation

// Circuit is one cycle: the chain of dependencies that leaves a node, returns to it and touches nothing
// twice on the way. It is what a FileCycleViolation carries, and it renders itself as a readable path.
type Circuit = cycles.Circuit

// FileNamingViolation says that one file is not named, or not placed, the way a rule requires. It is what
// `should have name`, `should be in folder` and `should be in path` report, and their negations, one per
// offending file — carrying the file, the requirement as a compiled pattern with the part of the identifier
// it was matched against, and the mood the rule was written in.
type FileNamingViolation = filesassertion.NamingViolation

const (
	// KindEmptyTest is the kind of EmptyTestViolation.
	KindEmptyTest = assertion.KindEmptyTest
	// KindFileCycle is the kind of FileCycleViolation.
	KindFileCycle = filesassertion.KindFileCycle
	// KindFileNaming is the kind of FileNamingViolation.
	KindFileNaming = filesassertion.KindFileNaming
)

// FilesBuilder is the scope stage of a rule about files, which ProjectFiles and Files return and every
// scope verb hands back a new one of. It is named here so that a half-built rule can be stored in a
// struct field or passed to a helper.
type FilesBuilder = filesapi.FilesBuilder

// FilesShouldBuilder is the positive mood of a rule about files, which FilesBuilder.Should returns:
// `project files, in folder "internal/api/**", should`.
type FilesShouldBuilder = filesapi.FilesShouldBuilder

// FilesShouldNotBuilder is the negated mood of a rule about files, which FilesBuilder.ShouldNot
// returns: `project files, in folder "internal/api/**", should not`. It is the positive builder's twin —
// same scope, same terminals, every predicate with a meaningful negation, one flag apart — with
// HaveNoCycles the one predicate offered on the positive mood alone, because its negation would have
// nothing to report.
type FilesShouldNotBuilder = filesapi.FilesShouldNotBuilder

// FilesCyclesCondition is the terminal of `project files, ..., should, have no cycles`, which
// FilesShouldBuilder.HaveNoCycles returns. It is a Checkable, so a built rule can be stored in a struct
// field, passed to a helper or kept in a list of the suite's rules.
type FilesCyclesCondition = filesapi.FilesCyclesCondition

// FilesNamingCondition is the terminal of the three self-contained rules about how a project's files are
// named and where they live — `have name`, `be in folder`, `be in path` — which HaveName, BeInFolder and
// BeInPath return on either mood. It is a Checkable, so a built rule can be stored, passed to a helper or
// kept in a list of the suite's rules.
type FilesNamingCondition = filesapi.FilesNamingCondition

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
