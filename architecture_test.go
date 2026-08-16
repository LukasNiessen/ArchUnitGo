package archunit_test

import (
	"path"
	"slices"
	"strings"
	"testing"

	archunit "github.com/LukasNiessen/ArchUnitGo"
)

// This file is ArchUnitGo held to its own architecture, by itself: the four dependency rules AGENTS.md
// states about this library, written as a suite of rules through the public surface and checked against
// this repository's own source. Every other test in this package proves that a feature works and uses
// this repository as the project to work on; this one is the other way round — the rules are the point,
// and the library is only how they are written.
//
// It is the deliberate dogfooding both shipped siblings do, for the reason they do it. A rule stated in
// prose decays the first time somebody is in a hurry: nothing fails when `files` reaches into `layers`
// for one helper, and the layout table in AGENTS.md quietly stops describing the code. A rule stated
// here fails the build in the commit that broke it, names the file that did it, and reads as the
// sentence a reviewer would have written in the comment they now do not have to leave. It is also the
// library's own best documentation — the whole product, on a project the reader has in front of them.
//
// Six tests in archunit_test.go already state parts of the same architecture, so three of the rule
// shapes below are stated twice — deliberately, with the emphasis the other way round in each place.
// Stated twice are: the acyclicity rule, verbatim, in TestThisRepositoryHasNoCyclesBetweenItsFiles; the
// four rules the loop at the bottom of this file writes, glob for glob, in
// TestADomainModuleOfThisRepositoryDependsOnTheKernelAndOnNoOtherModule; and the third-party half of
// rule 1, the same `*.*/**` with the same hole for the toolchain, in
// TestTheThirdPartyPolicyOfThisRepositoryIsOneRuleWithOneDocumentedHole and in
// TestThisRepositoryObeysItsOwnThirdPartyDependencyPolicy — the second of which also carries the
// positive `the extractor knows the analysis toolchain` rule as this file writes it. The two that state
// what no rule of one shape per module can are TestThisRepositoryObeysItsOwnDependencyRules, a
// hand-listed set of pairwise rules that goes on to the boundaries *inside* a module — a pure assertion
// half that may not reach back into its fluent API — and TestThisRepositoryObeysItsOwnLayerPolicy, the
// four rules as the one policy they actually are, which is the spelling that says which boundary broke
// rather than which file.
//
// All six stay where they are, because in each of them the rule is the vehicle and a feature is what is
// being proved — `have no cycles` with no scope verb, an exclusion on the object of a rule, the `except`
// companion end to end, a layer policy read as a policy — and this repository is only the project they
// are pointed at. Folding one of them in here would delete a family's integration test to save a
// duplicated sentence. The suite below is the other way round, and it is the only one of them that covers
// every module of the library by shape, so that the module landing next month is inside the rules the day
// its folder appears.
//
// Test files are outside every rule here, because extraction leaves them out by default: an
// architecture rule describes the shape of the production code, and this very file — a test that
// imports the public surface it forbids everything to depend on — is why that default is right.

// TestTheArchitectureOfThisRepositoryHolds is the suite: every rule of the architecture, each in its own
// named subtest, each selectable on its own with
// `go test -run 'TestTheArchitectureOfThisRepositoryHolds/nothing_depends_on_the_public_surface'`.
//
// It is one AssertAllPass call and nothing else, which is what a user's own architecture test should look
// like too: the rules are values built by a function, the assertion is the library's, and there is no
// loop, no comparison and no bookkeeping of this test's own for a reader to check before they can trust
// the result. A rule that does not hold prints the sentence it is and the files that broke it.
func TestTheArchitectureOfThisRepositoryHolds(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	archunit.AssertAllPass(t, architectureRulesOfThisRepository(), nil)
}

// TestTheArchitectureSuiteSaysSomethingAboutEveryFolderOfThisRepository is the guard on the suite above,
// and it exists because of the one thing a suite of rules cannot notice: a folder nobody wrote a rule
// about. The empty-test guard catches the opposite mistake — a rule whose folder was renamed selects
// nothing and fails — but a new module landing under a name the suite never mentions breaks nothing,
// and the suite stays green while saying nothing at all about the newest part of the library.
//
// So every top-level folder the extractor finds has to be one the suite knows: the shared kernel, the
// report layer, or a domain module named in domainModulesOfThisRepository. A file at the repository root
// is the public surface, which has a rule of its own rather than a folder.
//
// Naming a module in that list is only half of being covered, so the suite itself is asked for the two rules
// the loop over it is supposed to produce, by the names it files them under. A dropped assignment in that
// loop, or a key that stopped varying per module and so collapsed all four modules onto one entry, is a
// folder the suite says nothing about again — and nothing else in this file counts or names the rules the
// loop produced.
func TestTheArchitectureSuiteSaysSomethingAboutEveryFolderOfThisRepository(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	rules := architectureRulesOfThisRepository()
	for _, module := range domainModulesOfThisRepository() {
		for _, name := range []string{
			"the " + module + " module depends on the kernel and on no sibling module",
			"the " + module + " module depends on no third-party module",
		} {
			if _, stated := rules[name]; !stated {
				t.Errorf("the suite states no rule named %q, so it says nothing about the %s module: "+
					"the loop over domainModulesOfThisRepository has to file one rule per module under that name",
					name, module)
			}
		}
	}

	covered := append([]string{kernelFolder, reportFolder}, domainModulesOfThisRepository()...)

	for _, file := range selectFiles(t, archunit.ProjectFiles(nil)) {
		folder, _, nested := strings.Cut(file, "/")
		if !nested {
			continue
		}
		if !slices.Contains(covered, folder) {
			t.Errorf("%s is in the top-level folder %q, which the architecture suite says nothing about: "+
				"add the folder to domainModulesOfThisRepository, or give it a rule of its own", file, folder)
		}
	}
}

// TestAnArchitectureRuleOfThisRepositoryReportsTheFileThatBreaksIt is the failing half of the suite,
// because a rule that cannot fail says nothing. The suite is green, so nothing in it prints anything, and
// a boundary written with a pattern that quietly matched no file would look exactly the same.
//
// Half of that worry is answered by the library itself: the scope of every rule in the suite goes through the
// empty-test guard, and so does the object of the `depend on files` rules, so a folder renamed out from under
// one of those patterns fails the suite instead of emptying it. The object of a `depend on external modules`
// rule is unguarded by design — "no module matched" and "no selected file depends on such a module" are one
// statement there, and under the negated mood that statement is exactly the pass
// (files/fluentapi/depend_on_external_modules.go says so at its Check) — so for those the pattern is pinned
// instead: by the positive `the extractor knows the analysis toolchain` rule in the suite, and by
// TestAThirdPartyRuleOfThisRepositoryReportsTheFileThatBreaksIt below.
//
// What the guard cannot answer either way is whether an exclusion that legitimately leaves files behind still
// leaves the interesting ones — so the rule shape the suite states about the report layer is checked here with
// its one hole taken away, against a dependency this repository really has.
func TestAnArchitectureRuleOfThisRepositoryReportsTheFileThatBreaksIt(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	// The report layer's rule without `except "*/assertion"`: the layer does read the modules' violation
	// types, so forbidding that reports the files that do it, each carrying the dependencies it was broken
	// by. That data is what the report a failing suite prints is phrased from.
	rule := archunit.ProjectFiles(nil).
		InFolder(reportFolder).
		ShouldNot().
		DependOnFiles().
		InFolder(anyFolder).
		Except(kernelFolder + "/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		dependency, ok := violation.(archunit.FileDependencyViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a FileDependencyViolation", rule, violation)
		}
		if !strings.HasPrefix(dependency.File, reportFolder+"/") {
			t.Errorf("%s reports %q, want only files the scope selected", rule, dependency.File)
		}
		for _, found := range dependency.Dependencies {
			if !strings.HasSuffix(path.Dir(found), "/assertion") {
				t.Errorf("the violation of %s carries %q, want the violation types the suite excepts",
					dependency.File, found)
			}
		}
		offenders = append(offenders, dependency.File)
	}

	if !slices.Contains(offenders, reportFolder+"/violation_factory.go") {
		t.Errorf("%s reports %v, want the file that phrases every module's violations among them", rule, offenders)
	}
}

// TestAThirdPartyRuleOfThisRepositoryReportsTheFileThatBreaksIt is the failing half of the other rule shape in
// the suite, and it is the shape that needs one most: with the object of a `depend on external modules` rule
// outside the empty-test guard, `thirdPartyModules` carries seven negated rules and is held up by nothing else.
// Changed to a pattern no dependency of this repository matches, it would make all seven report nothing and
// leave the suite green — and the positive `the extractor knows the analysis toolchain` rule does not cover
// that, because it is written with the toolchain's own literal prefix rather than with this pattern.
//
// So the kernel's third-party rule is checked here with its `except` for the toolchain taken away, against the
// one third-party dependency this repository really has: the pattern has to reach that module from the file
// that loads a project through it, or every rule written with it is about nothing.
func TestAThirdPartyRuleOfThisRepositoryReportsTheFileThatBreaksIt(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).
		InFolder(kernelFolder + "/**").
		ShouldNot().
		DependOnExternalModules().
		Matching(thirdPartyModules)

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	reported := false
	for _, violation := range violations {
		dependency, ok := violation.(archunit.FileExternalDependencyViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a FileExternalDependencyViolation", rule, violation)
		}
		if dependency.File != extractorFile {
			continue
		}
		reported = true
		if !slices.Contains(dependency.Modules, analysisToolchainPackage) {
			t.Errorf("the violation of %s carries %v, want the toolchain package it loads a project with",
				dependency.File, dependency.Modules)
		}
	}

	if !reported {
		t.Errorf("%s reports nothing about %s, so the pattern %q names no module this repository depends on "+
			"and every rule in the suite written with it passes vacuously", rule, extractorFile, thirdPartyModules)
	}
}

// TestThePublicSurfaceIsTheOneFileTheSuiteForbidsDependingOn pins the object of the one rule in the suite
// whose population is a single file: `nothing depends on the public surface` is written as a pattern over
// the repository root rather than as the name `archunit.go`, so that a second file at the root is inside
// the rule the day it lands. A pattern that named the wrong thing would leave the rule green — not
// vacuous, because the empty-test guard would fire on an empty object, but about the wrong file.
func TestThePublicSurfaceIsTheOneFileTheSuiteForbidsDependingOn(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	surface := selectFiles(t, archunit.ProjectFiles(nil).InPath(publicSurfaceFiles))

	if !slices.Equal(surface, []string{"archunit.go"}) {
		t.Errorf("the pattern %q names %v, want the public surface and nothing else", publicSurfaceFiles, surface)
	}
}

// The parts of the layout the rules below are about and the patterns they are written with, spelled once
// each: the shared kernel, the report layer, the extractor and the toolchain it is allowed to know, and
// the three patterns whose reading is worth stating in words. The domain modules are the one part that is
// a list rather than a constant, in domainModulesOfThisRepository.
const (
	// kernelFolder holds everything shared — the kernel of AGENTS.md's layout table.
	kernelFolder = "common"
	// reportFolder holds the test-framework glue. It is `archtest` rather than the siblings' `testing`
	// because a package of that name would shadow the standard library's in the file that needs both.
	reportFolder = "archtest"
	// extractorFolder holds the only Go-specific code in the library, and so the only code with any
	// business knowing the toolchain that parses Go.
	extractorFolder = kernelFolder + "/extraction"
	// extractorFile is the one file that loads a project through the toolchain. Every other file of the
	// extractor works on what it returned.
	extractorFile = extractorFolder + "/extract_graph.go"
	// thirdPartyModules is the idiom for `a module somebody else published`: an import path whose first
	// segment holds a dot is a domain, and no package of the standard library has one. The standard
	// library is external too — `fmt` is code this project uses and does not own — so a rule that means
	// third-party alone has to say so with a pattern.
	thirdPartyModules = "*.*/**"
	// analysisToolchain is the one third-party dependency this repository has, as a pattern over the
	// import paths of its packages rather than over the module path go.mod names.
	analysisToolchain = "golang.org/x/tools/**"
	// analysisToolchainPackage is the one package of that toolchain this repository imports, spelled as the
	// extractor's own import writes it. It is what both patterns above have to reach to be about anything, and
	// TestAThirdPartyRuleOfThisRepositoryReportsTheFileThatBreaksIt is where thirdPartyModules is held to
	// reaching it.
	analysisToolchainPackage = "golang.org/x/tools/go/packages"
	// anyFolder is every folder of the project, at any depth, and the folder of a file at the root along
	// with them. It is what the object of a `depend on files` rule names when the rule is about where a
	// dependency may *not* point: `in folder "*/**", except "common/**"` is one sentence, where a rule
	// per forbidden folder would be a list that a new folder is missing from.
	anyFolder = "*/**"
	// publicSurfaceFiles are the Go files at the repository root, which is where the public surface is
	// and the only thing that is there. It is spelled as a pattern rather than as `archunit.go` so that
	// a second file at the root — a doc.go, a second façade — is inside the rule the day it lands.
	publicSurfaceFiles = "*.go"
)

// domainModulesOfThisRepository names the domain modules of AGENTS.md's layout table, one top-level
// folder of this repository each. Each of them gets the same two rules below, so a module that lands is
// covered by adding its name here — and TestTheArchitectureSuiteSaysSomethingAboutEveryFolderOfThisRepository
// fails until somebody does.
//
// It is a function rather than a package-level variable for the reason every list in this library is one:
// a caller may sort it, append to it or hand it on without reaching into a value the next test also reads.
func domainModulesOfThisRepository() []string {
	return []string{"files", "graph", "layers", "metrics", "slices"}
}

// architectureRulesOfThisRepository is the architecture of this library as rules: the four dependency
// rules of AGENTS.md, plus the acyclicity the layout table's one-way arrows assume.
//
// The suite is a map from what the rule means to the rule itself, which is the shape AssertAllPass takes:
// the name is what a failure is filed under, so it says the architecture in the words a person would use
// and leaves the globs to the sentence underneath it.
func architectureRulesOfThisRepository() map[string]archunit.Checkable {
	rules := map[string]archunit.Checkable{
		// Rule 1. The kernel is shared by every module, so a dependency of the kernel is a dependency of
		// the whole library — which is why the only ones it may have are the standard library's and the
		// toolchain that parses Go.
		"the kernel depends on nothing but the standard library and the analysis toolchain": archunit.
			ProjectFiles(nil).
			InFolder(kernelFolder + "/**").
			ShouldNot().
			DependOnExternalModules().
			Matching(thirdPartyModules).
			Except(analysisToolchain),
		// And the hole that exclusion is stays where the layout puts it: EXTRACT is the only stage that
		// knows Go, so it is the only folder of the kernel a third-party dependency may be reached from.
		"no package of the kernel but the extractor knows a third-party module": archunit.
			ProjectFiles(nil).
			InFolder(kernelFolder + "/**").
			ExceptInFolder(extractorFolder).
			ShouldNot().
			DependOnExternalModules().
			Matching(thirdPartyModules),
		// The positive mood of the same predicate, over the one file that loads a project: the rule above
		// permits a dependency, and this one is what says the permission is used and used there. Without
		// it, an extractor that stopped asking the toolchain — and went back to arithmetic on import
		// strings — would leave every third-party rule in this suite green.
		"the extractor knows the analysis toolchain": archunit.
			ProjectFiles(nil).
			InFile(extractorFile).
			Should().
			DependOnExternalModules().
			Matching(analysisToolchain),
		// The other half of rule 1, inside the project: the kernel knows no module, no report layer and no
		// public surface. A kernel that knew a module could not be reused and the module could not be
		// removed, which is the whole reason the shared code is a kernel and not a fifth module.
		"the kernel knows no domain module, no report layer and no public surface": archunit.
			ProjectFiles(nil).
			InFolder(kernelFolder + "/**").
			ShouldNot().
			DependOnFiles().
			InFolder(anyFolder).
			Except(kernelFolder + "/**"),
		// Rule 3. The report layer turns violations into a test failure, so what it reads is the kernel's
		// vocabulary and each module's violation types — the pure assertion half. A dependency on a
		// module's fluent API, projection or extraction would mean it was phrasing something other than
		// what a rule reported, and the one place that decides how a failure reads would then have a
		// second idea of what a failure is.
		"the report layer reads the kernel and the modules' violations, and nothing else": archunit.
			ProjectFiles(nil).
			InFolder(reportFolder).
			ShouldNot().
			DependOnFiles().
			InFolder(anyFolder).
			Except(kernelFolder+"/**", "*/assertion"),
		"the report layer depends on no third-party module": archunit.
			ProjectFiles(nil).
			InFolder(reportFolder).
			ShouldNot().
			DependOnExternalModules().
			Matching(thirdPartyModules),
		// Rule 4, the half that is a rule about everybody: the public surface re-exports the whole library,
		// so anything inside the library depending on it would be a cycle through the root of the module —
		// and would make the one package a user imports impossible to change without changing the code it
		// re-exports.
		"nothing depends on the public surface": archunit.
			ProjectFiles(nil).
			ShouldNot().
			DependOnFiles().
			InPath(publicSurfaceFiles),
		// And the acyclicity the layout table assumes rather than states: every arrow in it points one way,
		// which is only true if no two files of this repository have to be read in a circle.
		"no file of this repository depends on another in a circle": archunit.
			ProjectFiles(nil).
			Should().
			HaveNoCycles(),
	}

	for _, module := range domainModulesOfThisRepository() {
		// Rule 2, the one AGENTS.md says decays first: a module may reach the kernel and itself, and no
		// sibling. `files` reaching into `slices` for one helper is the classic failure, and the fix is
		// always the same — the helper belongs in the kernel — so this is the rule worth having written
		// down as N sentences of one shape rather than as a rule per ordered pair of modules.
		rules["the "+module+" module depends on the kernel and on no sibling module"] = archunit.
			ProjectFiles(nil).
			InFolder(module+"/**").
			ShouldNot().
			DependOnFiles().
			InFolder(anyFolder).
			Except(kernelFolder+"/**", module+"/**")
		// And the third-party half of the same thing: the toolchain is the extractor's, so a module that
		// picked up a dependency of its own would be a dependency of every user of the library.
		rules["the "+module+" module depends on no third-party module"] = archunit.
			ProjectFiles(nil).
			InFolder(module + "/**").
			ShouldNot().
			DependOnExternalModules().
			Matching(thirdPartyModules)
	}
	return rules
}
