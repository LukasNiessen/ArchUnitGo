package fluentapi_test

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/slices/fluentapi"
)

func TestProjectSlicesCutsItsSlicesOutOfTheProjectItsLocatorNames(t *testing.T) {
	// Resolved against a project on disk, not against a fixture graph: the locator is an argument nothing
	// else observes, and the whole point of a slicing is that nobody declared the names in it.
	root := writeSlicedFixtureProject(t)

	membership, err := fixtureSlicing(t, root).SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("`project slices` failed to resolve against the project its locator names: %v", err)
	}

	want := map[string][]string{
		"api":    {"internal/api/handler.go", "internal/api/router.go"},
		"db":     {"internal/db/conn.go"},
		"domain": {"internal/domain/order.go"},
	}
	if !maps.EqualFunc(membership, want, slices.Equal) {
		t.Errorf("the project's slices are %v, want %v", membership, want)
	}
}

func TestAFileTheSlicingDoesNotMatchIsInNoSlice(t *testing.T) {
	// main.go is outside internal/, so the slicing has no name for it. It is in no slice rather than in a
	// slice of its own, which is what makes a slicing a projection of part of a project.
	root := writeSlicedFixtureProject(t)

	membership, err := fixtureSlicing(t, root).SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("resolving the slicing failed: %v", err)
	}

	for slice, files := range membership {
		if slices.Contains(files, "main.go") {
			t.Errorf("main.go is in the slice %q, want it in none: the slicing does not match it", slice)
		}
	}
}

func TestASlicingRendersAsTheSentenceTheUserTyped(t *testing.T) {
	// A reader needs both halves: which files are sliced and how they are named, spelled as the part of an
	// identifier the pattern was matched against, and what was said about them.
	rule := fluentapi.ProjectSlices(nil).
		DefinedBy("internal/(**)/**").
		ShouldNot().
		ContainDependency("api", "db")

	want := `project slices, path matches "internal/(**)/**", should not, contain dependency "api" -> "db"`
	if rendered := rule.String(); rendered != want {
		t.Errorf("the rule reads\n%q, want\n%q", rendered, want)
	}
}

func TestASlicingCanBeBranchedFromOnceItIsWritten(t *testing.T) {
	// Saying what a project's slices are is the half of a rule worth typing once, so a builder is immutable
	// and branching from one is the point: two rules over one slicing, and neither reaching into the other.
	slicing := fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**")

	forbidden := slicing.ShouldNot().ContainDependency("api", "db")
	required := slicing.Should().ContainDependency("api", "domain")

	if forbidden.String() == required.String() {
		t.Errorf("both rules read %q, want the two different sentences they were written with", forbidden)
	}
	if written := slicing.String(); written != `project slices, path matches "internal/(**)/**"` {
		t.Errorf("the slicing now reads %q, want the moods to have left the builder they branched from alone", written)
	}
}

func TestTheLocatorProjectSlicesWasGivenCannotBeChangedAfterwards(t *testing.T) {
	// The other half of immutability, and the half a value receiver cannot give: the builder holds a
	// *ProjectLocator, so a caller reusing one struct to build a rule per directory would leave every stored
	// rule pointing at the last of them.
	root := writeSlicedFixtureProject(t)
	locator := &extraction.ProjectLocator{Directory: root}
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()

	slicing := fluentapi.ProjectSlices(locator).DefinedBy("internal/(**)/**")
	locator.Directory = t.TempDir()

	membership, err := slicing.SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("the slicing failed after the locator it was given was written to: %v", err)
	}
	if want := []string{"internal/db/conn.go"}; !slices.Equal(membership["db"], want) {
		t.Errorf(`the slice "db" is %v, want the project the rule was built with, %v`, membership["db"], want)
	}
}

func TestSelectSliceFilesResolvesTheSlicesUnderTheCheckOptionsItIsGiven(t *testing.T) {
	// AllowEmptyTests travels its own way to the guard, so every other knob on the bag reaches a slicing through
	// the extraction — and slices cut out of a differently-extracted project are slices the user did not ask
	// about. IncludeTestFiles is the cheapest to observe: the fixture's test file is a node only when it is on,
	// and it sits in the folder the slicing names the db slice after.
	root := writeSlicedFixtureProject(t)
	slicing := fixtureSlicing(t, root)

	byDefault, err := slicing.SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("resolving the slicing failed: %v", err)
	}
	withTests, err := slicing.SelectSliceFiles(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("resolving the slicing with IncludeTestFiles failed: %v", err)
	}

	if want := []string{"internal/db/conn.go"}; !slices.Equal(byDefault["db"], want) {
		t.Errorf(`the slice "db" came to %v by default, want %v: a rule is about the production code`, byDefault["db"], want)
	}
	want := []string{"internal/db/conn.go", "internal/db/conn_test.go"}
	if !slices.Equal(withTests["db"], want) {
		t.Errorf(`the slice "db" came to %v with IncludeTestFiles, want %v — the test file among them`, withTests["db"], want)
	}
}

func TestASlicingThatCannotLocateAProjectFailsWithAnError(t *testing.T) {
	// A technical failure is an error and never a violation: there was no project to cut into slices, so the rule
	// says nothing about anybody's architecture — and it must not quietly answer with the project this test runs
	// in either, which is what a locator dropped on the way to the extraction would look like.
	rule := fluentapi.ProjectSlices(&extraction.ProjectLocator{Directory: filepath.Join(t.TempDir(), "nowhere")}).
		DefinedBy("internal/(**)/**").
		ShouldNot().
		ContainDependency("api", "db")

	violations, err := rule.Check(nil)

	if err == nil {
		t.Fatalf("checking %s found %v in a directory that is not a project, want an error", rule, violations)
	}
	if len(violations) != 0 {
		t.Errorf("checking %s reported %v beside the error, want nothing", rule, violations)
	}
}

func TestAChainWithNoSlicingIsAUserErrorNamingTheEntryPoint(t *testing.T) {
	// `project slices` on its own is not a rule about anything: nobody said what the slices of the project
	// are, so there is no vocabulary to write one in. The type system cannot ask for the slicing — the mood
	// is a method on the entry point itself — so it is a rejection, and it names the step that is missing one.
	unsliced := fluentapi.ProjectSlices(&extraction.ProjectLocator{Directory: writeSlicedFixtureProject(t)})
	rule := unsliced.ShouldNot().ContainDependency("api", "db")

	if _, err := unsliced.SelectSliceFiles(nil); !errors.Is(err, fluentapi.ErrNoSlicing) {
		t.Errorf("resolving an unsliced chain returned %v, want ErrNoSlicing", err)
	}
	_, err := rule.Check(nil)
	if !errors.Is(err, fluentapi.ErrNoSlicing) {
		t.Fatalf("checking %s returned %v, want ErrNoSlicing", rule, err)
	}
	if operation := userError(t, err).Operation; operation != "project slices" {
		t.Errorf("UserError.Operation = %q, want the step the slicing is missing from, %q", operation, "project slices")
	}
}

func TestAChainWithNoSlicingRendersAsTheRuleItIsNot(t *testing.T) {
	// A rule caught in a log line has to say why it cannot run, and the missing slicing is visible as the
	// rejection rather than as a slicing the user never typed.
	rule := fluentapi.ProjectSlices(nil).Should().ContainDependency("api", "db")

	want := `project slices, should, contain dependency "api" -> "db" ` +
		`(rejected: archunit: project slices: no slicing)`
	if rendered := rule.String(); rendered != want {
		t.Errorf("the rule reads\n%q, want\n%q", rendered, want)
	}
}

func TestTheMissingSlicingIsTheFirstMistakeReported(t *testing.T) {
	// Two mistakes in one chain, and the one to report is the one a reader has to fix first: without a
	// slicing there are no slice names, so complaining about the names would send them to the wrong line.
	rule := fluentapi.ProjectSlices(nil).ShouldNot().ContainDependency("api", "api")

	_, err := rule.Check(nil)

	if !errors.Is(err, fluentapi.ErrNoSlicing) {
		t.Errorf("checking %s returned %v, want the missing slicing rather than the self-dependency", rule, err)
	}
}

// fixtureSlicing is the fixture project sliced by the folder its files live in under internal/ — the slicing
// nearly every rule about a Go project is written with, and the one the fixture below was built for.
func fixtureSlicing(t *testing.T, root string) fluentapi.SlicesBuilder {
	t.Helper()

	return fluentapi.ProjectSlices(fixtureLocator(t, root)).DefinedBy("internal/(**)/**")
}

// fixtureLocator points a rule at a fixture project and clears the graph cache around it, because the cache is
// keyed by the project and a temporary directory of one test is not the one of the next.
func fixtureLocator(t *testing.T, root string) *extraction.ProjectLocator {
	t.Helper()

	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	return &extraction.ProjectLocator{Directory: root}
}

// userError is this error as the misuse it should be, failing the test when a rule returned anything else.
func userError(t *testing.T, err error) *archerror.UserError {
	t.Helper()

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("the error is a %T, want a *archerror.UserError: %v", err, err)
	}
	return user
}

// writeSlicedFixtureProject writes a small project of three folders under internal/, which are its three
// slices: an api of two files that reaches both the others, a domain of one file that reaches nothing, and a
// database whose production code reaches nothing either and whose test file reaches back up to the api.
//
// The api's dependency on the database is the offense every failing rule in this package is written about, and
// its dependency on the domain is the one a required dependency is satisfied by. main.go is outside internal/,
// so it is also the fixture's answer to "a file the slicing does not match is in no slice": every dependency
// of it and on it is dropped by the projection rather than landing in a slice of its own.
//
// The database's test file is the fixture's answer to the check options: it is a file of the db slice, and so
// the one dependency of the db slice on the api slice, only when CheckOptions.IncludeTestFiles asks for it. It
// is an external test package because the api it imports imports the database back, which is a cycle no
// in-package test file could be.
func writeSlicedFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFiles(t, root, map[string]string{
		"go.mod":  "module example.com/sliced\n\ngo 1.26\n",
		"main.go": "package main\n\nimport \"example.com/sliced/internal/api\"\n\nfunc main() { api.Handle() }\n",
		"internal/api/handler.go": "package api\n\nimport (\n\t\"example.com/sliced/internal/db\"\n\t" +
			"\"example.com/sliced/internal/domain\"\n)\n\nfunc Handle() { db.Save(domain.Order{}) }\n",
		"internal/api/router.go":   "package api\n\nfunc Route() {}\n",
		"internal/domain/order.go": "package domain\n\ntype Order struct{}\n",
		"internal/db/conn.go":      "package db\n\nfunc Save(any) {}\n",
		"internal/db/conn_test.go": "package db_test\n\nimport (\n\t\"testing\"\n\n\t" +
			"\"example.com/sliced/internal/api\"\n)\n\nfunc TestHandle(*testing.T) { api.Handle() }\n",
	})
	return root
}

// writeFixtureFiles writes files given by their project-relative slash-separated names into a fixture project,
// creating the folders they are in. It is how a test that needs a slice the shared fixture has not got adds one,
// so that a folder written by hand is written the same way the fixture's own are.
func writeFixtureFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating the folder of %q failed: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %q failed: %v", name, err)
		}
	}
}
