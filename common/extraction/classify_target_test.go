package extraction

import (
	"slices"
	"testing"
)

// fixtureTargetIndex is the index loadProjectBuild builds for a project like fixtureSourceProject,
// written by hand so that the one decision every external rule hangs off can be tested without running
// the toolchain: the module is `example.com/fixture`, one of its packages is built from two files, and
// one is present with nothing to point at.
//
// root is the project root, which only the nested-module check reads. A test that has no nested module to
// find may pass anything.
func fixtureTargetIndex(root string) targetIndex {
	return targetIndex{
		root:       root,
		modulePath: "example.com/fixture",
		packages: map[string][]string{
			"example.com/fixture":              {"main.go"},
			"example.com/fixture/internal/api": {"internal/api/handler.go", "internal/api/router.go"},
			// A package of the project's whose every file the walk excluded: the toolchain reported it, so
			// the key is here, and there is no node for an edge to point at.
			"example.com/fixture/build/tool": nil,
		},
	}
}

func TestClassifyResolvesAnImportOfTheProjectsOwnPackageToItsFiles(t *testing.T) {
	index := fixtureTargetIndex(t.TempDir())

	// A package is compiled as a whole, so importing it is depending on every file it is built from.
	targets, external := index.classify("example.com/fixture/internal/api")

	if external {
		t.Error("classify reported the project's own package as external")
	}
	if want := []string{"internal/api/handler.go", "internal/api/router.go"}; !slices.Equal(targets, want) {
		t.Errorf("targets = %q, want %q", targets, want)
	}
}

func TestClassifyMarksAnImportOfSomebodyElsesCodeExternal(t *testing.T) {
	index := fixtureTargetIndex(t.TempDir())

	for _, importPath := range []string{
		"fmt",                                  // the standard library
		"net/http",                             // a standard library package with a separator in it
		"github.com/lib/pq",                    // a dependency module
		"golang.org/x/tools/go/packages",       // a package of a dependency module
		"C",                                    // the cgo directive, which is not a package at all
		"example.com/fixtures/api",             // a module whose path merely starts like the project's
		"example.com/fixture-tools/api",        // the same trap with the other separator
		"example.com/fixtur",                   // a prefix of the project's own module path
		"example.com/fixture/../outside/thing", // an import path that tries to climb out of the project
	} {
		targets, external := index.classify(importPath)

		if !external {
			t.Errorf("classify(%q) reported an external target as the project's own", importPath)
		}
		if len(targets) != 0 {
			t.Errorf("classify(%q) targets = %q, want none: an external edge points at the import path", importPath, targets)
		}
	}
}

func TestClassifyYieldsNoTargetForAPackageOfTheProjectsWithoutNodes(t *testing.T) {
	index := fixtureTargetIndex(t.TempDir())

	// `build` is excluded from the walk by default, so the package it holds is nobody's node. It is still
	// the project's own code, and calling it an external module would fire every rule about third-party
	// dependencies on the project's own build tooling.
	targets, external := index.classify("example.com/fixture/build/tool")

	if external {
		t.Error("classify reported a package of the project's whose files the walk excluded as external")
	}
	if len(targets) != 0 {
		t.Errorf("targets = %q, want none: there is no node to point at", targets)
	}
}

func TestClassifyDoesNotCallAMissingPackageOfTheProjectsOwnExternal(t *testing.T) {
	index := fixtureTargetIndex(t.TempDir())

	// The toolchain reports nothing about a package that is not there — half-written, renamed or deleted —
	// and the module's own path is the only thing that can tell it from a third-party module.
	for _, importPath := range []string{
		"example.com/fixture/internal/nope",
		"example.com/fixture/internal/api/deeper",
	} {
		targets, external := index.classify(importPath)

		if external {
			t.Errorf("classify(%q) reported the project's own code as an external module", importPath)
		}
		if len(targets) != 0 {
			t.Errorf("classify(%q) targets = %q, want none: there is no node to point at", importPath, targets)
		}
	}
}

func TestClassifyMarksAModuleNestedInTheProjectExternal(t *testing.T) {
	// A module inside the project that declares a path under its parent's is still a module of its own:
	// resolved through the module graph, versioned on its own, and so an external dependency. The go.mod
	// on the way down is what tells it from a package of the project's that is simply missing.
	root := t.TempDir()
	writeProjectFile(t, root, moduleFileName, "module example.com/fixture\n\ngo 1.26\n")
	writeProjectFile(t, root, "tools/"+moduleFileName, "module example.com/fixture/tools\n\ngo 1.26\n")

	index := fixtureTargetIndex(root)

	for _, importPath := range []string{
		"example.com/fixture/tools",
		"example.com/fixture/tools/internal/generate", // below the nested module, so the search walks down to it
	} {
		if _, external := index.classify(importPath); !external {
			t.Errorf("classify(%q) reported a nested module as the project's own code", importPath)
		}
	}
}

func TestClassifyTrustsTheToolchainAloneWithoutAModulePath(t *testing.T) {
	index := fixtureTargetIndex(t.TempDir())
	index.modulePath = ""

	// Nothing said what the module is, so the toolchain's answer is the only one there is: a reported
	// package is the project's, and everything else is external.
	if _, external := index.classify("example.com/fixture/internal/api"); external {
		t.Error("classify reported a package the toolchain reported as external")
	}
	if _, external := index.classify("example.com/fixture/internal/nope"); !external {
		t.Error("classify guessed at the project's own code with no module path to go on")
	}
}

func TestClassifyReadsTheProjectRootsOwnModuleFileAsTheProjectsOwn(t *testing.T) {
	// The root's go.mod is the module the project *is*, so finding it must not make the project external
	// to itself. Only the module path is imported here; no package of it was reported.
	root := t.TempDir()
	writeProjectFile(t, root, moduleFileName, "module example.com/other\n\ngo 1.26\n")

	index := targetIndex{root: root, modulePath: "example.com/other", packages: map[string][]string{}}

	if _, external := index.classify("example.com/other"); external {
		t.Error("classify reported the module under analysis as external to itself")
	}
}
