// Package architecture holds the architecture rules for this demo, written as
// ordinary Go unit tests against ArchUnitGo. Every test is one rule, named as
// the rule reads aloud, so a single rule can be run with:
//
//	go test ./architecture -run TestTheLayersFlowInwards -v
//
// Run them all with:
//
//	go test ./...
//
// A rule is a value, not an action: building one does no work, and only the
// terminal check reads the project. The chain reads left to right as an English
// sentence — "project files, in folder internal/domain/**, should not, depend on
// files" — which is the whole point of the library.
package architecture

import (
	"testing"

	archunit "github.com/LukasNiessen/ArchUnitGo"
)

// TestTheLayersFlowInwards is the headline rule: the whole hexagon in one
// sentence, expressed as a named-layer policy. The domain is sealed, the
// application may reach only the domain, and the adapters may reach only the
// application and the domain. Dependencies inside a layer are always allowed,
// and a dependency that points at code in no declared layer (cmd/) is ignored.
func TestTheLayersFlowInwards(t *testing.T) {
	rule := archunit.ProjectLayers(nil).
		Layer("domain").DefinedByFolder("internal/domain/**").
		Layer("application").DefinedByFolder("internal/application/**").
		Layer("adapters").DefinedByFolder("internal/adapters/**").
		WhereLayer("domain").MayOnlyDependOnLayers().
		WhereLayer("application").MayOnlyDependOnLayers("domain").
		WhereLayer("adapters").MayOnlyDependOnLayers("application", "domain")

	archunit.AssertPasses(t, rule, nil)
}

// TestTheDomainDependsOnNoOtherFile is the sealed core written as a file rule:
// no file under internal/domain may import any other file of the project. The
// domain is allowed the standard library and nothing else.
func TestTheDomainDependsOnNoOtherFile(t *testing.T) {
	rule := archunit.ProjectFiles(nil).
		InFolder("internal/domain/**").
		ShouldNot().
		DependOnFiles()

	archunit.AssertPasses(t, rule, nil)
}

// TestTheDomainDoesNotUseThirdPartyLibraries holds the same boundary on the
// other side of it: the domain may not reach any module whose import path starts
// with a domain name ("*.*/**" is the idiom for "third-party, not the standard
// library").
func TestTheDomainDoesNotUseThirdPartyLibraries(t *testing.T) {
	rule := archunit.ProjectFiles(nil).
		InFolder("internal/domain/**").
		ShouldNot().
		DependOnExternalModules().
		Matching("*.*/**")

	archunit.AssertPasses(t, rule, nil)
}

// TestPrimaryAdaptersDoNotDependOnSecondaryAdapters keeps the two sides of the
// hexagon apart: the driving adapters (HTTP) and the driven adapters (storage)
// are both "adapters", but neither side may reach the other directly. They meet
// only in the application layer.
func TestPrimaryAdaptersDoNotDependOnSecondaryAdapters(t *testing.T) {
	rule := archunit.ProjectFiles(nil).
		InFolder("internal/adapters/primary/**").
		ShouldNot().
		DependOnFiles().
		InFolder("internal/adapters/secondary/**")

	archunit.AssertPasses(t, rule, nil)
}

// TestNoCircularDependencies forbids a dependency cycle anywhere under internal/.
func TestNoCircularDependencies(t *testing.T) {
	rule := archunit.ProjectFiles(nil).
		InFolder("internal/**").
		Should().
		HaveNoCycles()

	archunit.AssertPasses(t, rule, nil)
}

// TestSecondaryAdaptersAreNamedAsRepositories holds a naming convention: every
// file under internal/adapters/secondary/** must be named *_repository.go.
func TestSecondaryAdaptersAreNamedAsRepositories(t *testing.T) {
	rule := archunit.ProjectFiles(nil).
		InFolder("internal/adapters/secondary/**").
		Should().
		HaveName("*_repository.go")

	archunit.AssertPasses(t, rule, nil)
}

// TestTheDomainStaysSmall is a metrics rule: every file in the domain is at most
// a hundred lines long. The metrics family has no mood stage — each threshold
// predicate spells its own.
func TestTheDomainStaysSmall(t *testing.T) {
	rule := archunit.Metrics(nil).
		InFolder("internal/domain/**").
		Count().
		LinesOfCode().
		ShouldBeBelow(100)

	archunit.AssertPasses(t, rule, nil)
}

// TestTheDomainSliceDoesNotDependOnTheAdaptersSlice is the same boundary said in
// the slices vocabulary. A slice is a name cut out of a file's identifier: the
// pattern internal/(**)/** makes the folders under internal/ the slices.
func TestTheDomainSliceDoesNotDependOnTheAdaptersSlice(t *testing.T) {
	rule := archunit.ProjectSlices(nil).
		DefinedBy("internal/(**)/**").
		ShouldNot().
		ContainDependency("domain", "adapters")

	archunit.AssertPasses(t, rule, nil)
}

// TestTheAdaptersSliceReachesTheApplicationSlice is the positive half of the
// same idea: a dependency the architecture relies on. The adapters must reach
// the application — an adapter that bypassed the use-case layer would break
// this rule.
func TestTheAdaptersSliceReachesTheApplicationSlice(t *testing.T) {
	rule := archunit.ProjectSlices(nil).
		DefinedBy("internal/(**)/**").
		Should().
		ContainDependency("adapters", "application")

	archunit.AssertPasses(t, rule, nil)
}
