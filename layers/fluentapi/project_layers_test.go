package fluentapi_test

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
)

func TestProjectLayersAndLayersAreOneEntryPoint(t *testing.T) {
	// The family gives every entry point two names, one with the verb and one without, and they build the same
	// policy — so a suite may write whichever reads better and a reader learns one grammar.
	//
	// Resolved against a fixture project rather than against nil: the locator is an argument of both entry
	// points, so naming a project that is not the one this test runs in and reading the layer's files back is
	// the only way to see that either of them keeps it. Dropping it would analyze this repository instead.
	root := writeLayeredFixtureProject(t)
	verbose := fluentapi.ProjectLayers(fixtureLocator(t, root)).Layer("api").DefinedByFolder("internal/api/**")
	terse := fluentapi.Layers(fixtureLocator(t, root)).Layer("api").DefinedByFolder("internal/api/**")

	if verbose.String() != terse.String() {
		t.Errorf("Layers builds %q and ProjectLayers %q, want one entry point under two names", terse, verbose)
	}
	entryPoints := []struct {
		name   string
		policy fluentapi.LayersBuilder
	}{
		{name: "project layers", policy: verbose},
		{name: "layers", policy: terse},
	}
	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	for _, entry := range entryPoints {
		membership, err := entry.policy.SelectLayerFiles(nil)
		if err != nil {
			t.Fatalf("`%s` failed to resolve against the project its locator names: %v", entry.name, err)
		}
		if !slices.Equal(membership["api"], want) {
			t.Errorf(`%s came to the layer "api" of %v, want the files of the project the locator names, %v`,
				entry.name, membership["api"], want)
		}
	}
}

func TestAPolicyRendersAsTheSentenceTheUserTyped(t *testing.T) {
	// A reader needs both halves: which files each layer is, spelled as the part of an identifier each pattern
	// was matched against, and what was said about them.
	policy := fluentapi.ProjectLayers(nil).
		Layer("api").DefinedByFolder("internal/api/**").
		Layer("db").DefinedBy("internal/db/*.go").
		WhereLayer("api").MayNotDependOnLayers("db")

	want := `project layers, ` +
		`layer "api" defined by path without filename matches "internal/api/**", ` +
		`layer "db" defined by path matches "internal/db/*.go", ` +
		`where layer "api", may not depend on layers "db"`
	if rendered := policy.String(); rendered != want {
		t.Errorf("the policy reads\n%q, want\n%q", rendered, want)
	}
}

func TestAPolicyCanBeBranchedFromAfterItsLayersAreDeclared(t *testing.T) {
	// Declaring a project's layers is the expensive half of writing a policy, so a builder is immutable and
	// branching from one is the point: two policies over one set of layers, and neither reaching into the other.
	layers := fluentapi.ProjectLayers(nil).
		Layer("api").DefinedByFolder("internal/api/**").
		Layer("db").DefinedByFolder("internal/db/**")

	strict := layers.WhereLayer("db").MayOnlyDependOnLayers()
	loose := layers.WhereLayer("db").MayNotDependOnLayers("api")

	if strict.String() == loose.String() {
		t.Errorf("both policies read %q, want the two different clauses they were written with", strict)
	}
	if declared := layers.String(); declared != `project layers, `+
		`layer "api" defined by path without filename matches "internal/api/**", `+
		`layer "db" defined by path without filename matches "internal/db/**"` {
		t.Errorf("the declarations now read %q, want the clauses to have left the builder they branched from alone", declared)
	}
}

func TestTwoClausesBranchedFromAPolicyThatHasSpareRoomForOneDoNotOverwriteEachOther(t *testing.T) {
	// Immutability of the clause list has to hold when the parent has room for another clause too, which is the
	// case a branch-and-append gets wrong silently rather than loudly: a policy grown to three clauses has spare
	// capacity, so two children each appending their fourth clause without copying first would write into the
	// same slot and the one built first would be judged by the other's sentence.
	root := writeLayeredFixtureProject(t)
	base := fixturePolicy(t, root).
		WhereLayer("api").MayOnlyDependOnLayers("domain", "db").
		WhereLayer("db").MayOnlyDependOnLayers().
		WhereLayer("domain").MayOnlyDependOnLayers("db")

	first := base.WhereLayer("api").MayNotDependOnLayers("db")
	second := base.WhereLayer("domain").MayNotDependOnLayers("db")

	if want := `, where layer "api", may not depend on layers "db"`; !strings.HasSuffix(first.String(), want) {
		t.Errorf("the first branch reads %q, want it to end in %q: its clause was written over", first, want)
	}
	if want := `, where layer "domain", may not depend on layers "db"`; !strings.HasSuffix(second.String(), want) {
		t.Errorf("the second branch reads %q, want it to end in %q", second, want)
	}
	brokenFirst, err := first.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", first, err)
	}
	brokenSecond, err := second.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", second, err)
	}
	// The three shared clauses allow every dependency the fixture has, so each branch is broken by its own
	// fourth clause and by nothing else — which is what makes the two results tell the branches apart.
	if pairs := offendingPairs(t, brokenFirst); !slices.Equal(pairs, []string{"api -> db"}) {
		t.Errorf("the first branch reported %v, want the dependency its own blocklist forbids", pairs)
	}
	if pairs := offendingPairs(t, brokenSecond); !slices.Equal(pairs, []string{"domain -> db"}) {
		t.Errorf("the second branch reported %v, want the dependency its own blocklist forbids", pairs)
	}
}

func TestWideningALayerOnABranchLeavesThePolicyItBranchedFromAlone(t *testing.T) {
	// A second declaration of a name widens that layer, and it widens it on the branch alone: merging into the
	// declaration already recorded is the one place a stage writes to an element rather than appending one, so a
	// stored policy would otherwise gain a folder nobody declared on it.
	root := writeLayeredFixtureProject(t)
	base := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("core").DefinedByFolder("internal/domain/**")

	widened := base.Layer("core").DefinedByFolder("internal/db/**")

	narrow, err := base.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}
	broad, err := widened.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}

	if want := []string{"internal/domain/order.go"}; !slices.Equal(narrow["core"], want) {
		t.Errorf(`the policy that was branched from came to the layer "core" of %v, want %v: the branch widened it too`,
			narrow["core"], want)
	}
	if want := []string{"internal/db/conn.go", "internal/domain/order.go"}; !slices.Equal(broad["core"], want) {
		t.Errorf(`the branch came to the layer "core" of %v, want %v: a name declared twice is both declarations`,
			broad["core"], want)
	}
}

func TestTwoLayersBranchedFromAPolicyThatHasSpareRoomForOneDoNotOverwriteEachOther(t *testing.T) {
	// The declaration list has the same hazard as the clause list, and for the same reason: three declared layers
	// leave room for a fourth, so two branches each declaring their own fourth layer without copying first would
	// hand both policies whichever was declared last.
	root := writeLayeredFixtureProject(t)
	base := fixturePolicy(t, root)

	entry := base.Layer("entry").DefinedBy("main.go")
	rest := base.Layer("rest").DefinedBy("main.go")

	entered, err := entry.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}
	remaining, err := rest.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}

	if want := []string{"main.go"}; !slices.Equal(entered["entry"], want) {
		t.Errorf(`the first branch came to the layer "entry" of %v (%v), want %v: its declaration was written over`,
			entered["entry"], slices.Sorted(maps.Keys(entered)), want)
	}
	if _, declared := remaining["entry"]; declared {
		t.Errorf(`the second branch declared the layer "entry" as well (%v), want only the one it named itself`,
			slices.Sorted(maps.Keys(remaining)))
	}
	if want := []string{"main.go"}; !slices.Equal(remaining["rest"], want) {
		t.Errorf(`the second branch came to the layer "rest" of %v, want %v`, remaining["rest"], want)
	}
}

func TestALayerDeclaredTwiceIsOneLayerOfBothDeclarations(t *testing.T) {
	// The one reading of a repeated name that is not a silent mistake, and how a layer living in two folders is
	// spelled: one layer, one key, the union of its declarations.
	root := writeLayeredFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("core").DefinedByFolder("internal/domain/**").
		Layer("core").DefinedByFolder("internal/db/**")

	membership, err := policy.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}

	want := map[string][]string{"core": {"internal/db/conn.go", "internal/domain/order.go"}}
	if !maps.EqualFunc(membership, want, slices.Equal) {
		t.Errorf("the layers are %v, want %v: a name declared twice is one layer of both declarations", membership, want)
	}
}

func TestSelectLayerFilesResolvesTheDeclarationsAgainstTheProject(t *testing.T) {
	// What a half-built policy is talking about, before anything is judged: the files of each layer, sorted,
	// under the name the clauses use.
	root := writeLayeredFixtureProject(t)

	membership, err := fixturePolicy(t, root).SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}

	want := map[string][]string{
		"api":    {"internal/api/handler.go", "internal/api/router.go"},
		"domain": {"internal/domain/order.go"},
		"db":     {"internal/db/conn.go"},
	}
	if !maps.EqualFunc(membership, want, slices.Equal) {
		t.Errorf("the layers of the fixture are %v, want %v", membership, want)
	}
}

func TestSelectLayerFilesKeepsALayerWhoseFolderHasBeenRenamed(t *testing.T) {
	// The stale glob is an empty layer here rather than an error or a missing key: whether it is a failure is
	// the terminal's question, and the guard is what asks it.
	root := writeLayeredFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("transport").DefinedByFolder("internal/transport/**")

	membership, err := policy.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}

	files, declared := membership["transport"]
	if !declared || len(files) != 0 {
		t.Errorf(`layer "transport" resolved to %v (declared: %t), want an empty layer`, files, declared)
	}
}

func TestAPatternTheLibraryCannotCompileIsReportedByTheTerminal(t *testing.T) {
	// A fluent method has nowhere to put an error, so the rejection travels to the terminal — where it is a
	// UserError naming the verb at fault and quoting the pattern as the user wrote it, before the project is
	// read at all. The chain still renders, with the rejection visible.
	policy := fluentapi.ProjectLayers(nil).
		Layer("api").DefinedByFolder("internal/[").
		WhereLayer("api").MayNotDependOnLayers("api")

	violations, err := policy.Check(nil)

	if len(violations) != 0 {
		t.Errorf("the rejected policy reported %v, want nothing: it was never a runnable rule", violations)
	}
	var userError *archerror.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Check returned %v, want a UserError naming the verb at fault", err)
	}
	if userError.Operation != "defined by folder" || userError.Subject != "internal/[" {
		t.Errorf("the error blames `%s %q`, want the verb and the pattern the user typed", userError.Operation, userError.Subject)
	}
}

func TestOnlyTheFirstRejectionOfAPolicyIsReported(t *testing.T) {
	// The first typo is the one to fix, and a chain reporting the last would point at the wrong line.
	policy := fluentapi.ProjectLayers(nil).
		Layer("api").DefinedByFolder("internal/[").
		Layer("db").DefinedBy("internal/db/*.(").
		WhereLayer("api").MayNotDependOnLayers("db")

	_, err := policy.Check(nil)

	var userError *archerror.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Check returned %v, want a UserError", err)
	}
	if userError.Subject != "internal/[" {
		t.Errorf("the error blames %q, want the first pattern the user has to fix", userError.Subject)
	}
}

func TestALayerWithoutANameIsRejected(t *testing.T) {
	// A layer is a name for a set of files, and a policy cannot talk about one that has none — at either end
	// of the chain, since a clause names a layer too.
	declared := fluentapi.ProjectLayers(nil).
		Layer("").DefinedByFolder("internal/api/**").
		Layer("db").DefinedByFolder("internal/db/**").
		WhereLayer("db").MayNotDependOnLayers("db")
	clausing := fluentapi.ProjectLayers(nil).
		Layer("db").DefinedByFolder("internal/db/**").
		WhereLayer("").MayNotDependOnLayers("db")

	for name, policy := range map[string]kernel.Checkable{"declared": declared, "named by a clause": clausing} {
		_, err := policy.Check(nil)
		if !errors.Is(err, fluentapi.ErrUnnamedLayer) {
			t.Errorf("a layer with no name (%s) failed with %v, want ErrUnnamedLayer", name, err)
		}
	}
}

func TestAClauseNamingALayerThePolicyNeverDeclaredIsRejected(t *testing.T) {
	// The typo the sibling libraries hit most. An undeclared layer has no files, so it is at neither end of any
	// projected dependency and a clause about it would judge nothing and pass forever — which is why this is a
	// UserError, quoting the name back at the user, rather than a silent green.
	rejections := map[string]struct {
		policy    kernel.Checkable
		operation string
		subject   string
	}{
		"as the layer a clause is about": {
			policy: fluentapi.ProjectLayers(nil).
				Layer("db").DefinedByFolder("internal/db/**").
				WhereLayer("api").MayNotDependOnLayers("db"),
			operation: "where layer",
			subject:   "api",
		},
		"as a layer a clause names": {
			policy: fluentapi.ProjectLayers(nil).
				Layer("db").DefinedByFolder("internal/db/**").
				WhereLayer("db").MayOnlyDependOnLayers("domain"),
			operation: "may only depend on layers",
			subject:   "domain",
		},
	}

	for name, rejection := range rejections {
		_, err := rejection.policy.Check(nil)
		if !errors.Is(err, fluentapi.ErrUndeclaredLayer) {
			t.Errorf("an undeclared layer %s failed with %v, want ErrUndeclaredLayer", name, err)
			continue
		}
		var userError *archerror.UserError
		if !errors.As(err, &userError) {
			t.Fatalf("the error is a %T, want a UserError naming the step at fault", err)
		}
		if userError.Operation != rejection.operation || userError.Subject != rejection.subject {
			t.Errorf("the error blames `%s %q`, want `%s %q`",
				userError.Operation, userError.Subject, rejection.operation, rejection.subject)
		}
	}
}

func TestTheLocatorAPolicyWasBuiltWithIsTheOneItMeans(t *testing.T) {
	// The locator is copied rather than kept, so a caller may reuse one struct to build a policy per directory
	// and each policy still means the directory it was built with.
	//
	// The directory it is repointed at is one with no go.mod at or above it, which is what makes a shared
	// pointer observable at all: anywhere inside the fixture would walk up to the same module file and resolve
	// the same layers either way, so the sharing would go unnoticed.
	root := writeLayeredFixtureProject(t)
	locator := &extraction.ProjectLocator{Directory: root}
	policy := fluentapi.ProjectLayers(locator).Layer("api").DefinedByFolder("internal/api/**")

	locator.Directory = t.TempDir()
	membership, err := policy.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers after the locator it was given was written to failed: %v", err)
	}

	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if !slices.Equal(membership["api"], want) {
		t.Errorf(`layer "api" resolved to %v, want %v: the locator was shared rather than copied`, membership["api"], want)
	}
}

func TestSelectLayerFilesResolvesTheLayersUnderTheCheckOptionsItIsGiven(t *testing.T) {
	// AllowEmptyTests travels its own way to the guard, so every other knob on the bag reaches a layer policy
	// through the extraction — and layers resolved against a differently-extracted project are layers the user
	// did not ask about. IncludeTestFiles is the cheapest to observe: the fixture's test file is a node only
	// when it is on, and it sits in the folder the db layer is defined by.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root)

	byDefault, err := policy.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving the layers failed: %v", err)
	}
	withTests, err := policy.SelectLayerFiles(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("resolving the layers with IncludeTestFiles failed: %v", err)
	}

	if want := []string{"internal/db/conn.go"}; !slices.Equal(byDefault["db"], want) {
		t.Errorf(`layer "db" came to %v by default, want %v: a policy is about the production code`, byDefault["db"], want)
	}
	want := []string{"internal/db/conn.go", "internal/db/conn_test.go"}
	if !slices.Equal(withTests["db"], want) {
		t.Errorf(`layer "db" came to %v with IncludeTestFiles, want %v — the test file among them`, withTests["db"], want)
	}
}

func TestAPolicyThatCannotLocateAProjectFailsWithAnError(t *testing.T) {
	// A technical failure is an error and never a violation: there was no project to judge, so the policy says
	// nothing about anybody's architecture.
	policy := fluentapi.ProjectLayers(&extraction.ProjectLocator{Directory: filepath.Join(t.TempDir(), "nowhere")}).
		Layer("api").DefinedByFolder("internal/api/**").
		WhereLayer("api").MayOnlyDependOnLayers()

	violations, err := policy.Check(nil)

	if err == nil {
		t.Fatalf("Check found %v in a directory that is not a project, want an error", violations)
	}
	if len(violations) != 0 {
		t.Errorf("Check reported %v beside the error, want nothing", violations)
	}
}

// fixturePolicy is the three declared layers of the fixture project, with no clause yet: the api folder, the
// domain folder and the db folder, which is what the tests of this package write their clauses over.
func fixturePolicy(t *testing.T, root string) fluentapi.LayersBuilder {
	t.Helper()

	return fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").
		Layer("domain").DefinedByFolder("internal/domain/**").
		Layer("db").DefinedByFolder("internal/db/**")
}

// fixtureLocator points a policy at a fixture project and clears the graph cache around it, because the cache
// is keyed by the project and a temporary directory of one test is not the one of the next.
func fixtureLocator(t *testing.T, root string) *extraction.ProjectLocator {
	t.Helper()

	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	return &extraction.ProjectLocator{Directory: root}
}

// writeLayeredFixtureProject writes a small layered project: an api of two files that reaches both other
// layers, a domain of one file that reaches the database, and a database whose production code reaches
// nothing and whose test file reaches back up to the api.
//
// The domain's dependency on the database is the offense every failing test in this package is written about —
// it is the edge a sealed domain, and a domain forbidden the database, both break — while the api's two
// dependencies are the ones an allowlist over them permits. main.go is in no declared layer, so it is also
// the fixture's answer to "edges where either end belongs to no declared layer are ignored".
//
// The database's test file is the fixture's answer to the check options: it is a node, and so the one
// dependency of the db layer on the api layer, only when CheckOptions.IncludeTestFiles asks for it. It is an
// external test package because the api it imports imports the database back, which is a cycle no in-package
// test file could be.
func writeLayeredFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":  "module example.com/layered\n\ngo 1.26\n",
		"main.go": "package main\n\nimport \"example.com/layered/internal/api\"\n\nfunc main() { api.Handle() }\n",
		"internal/api/handler.go": "package api\n\nimport (\n\t\"example.com/layered/internal/db\"\n\t" +
			"\"example.com/layered/internal/domain\"\n)\n\nfunc Handle() { db.Save(domain.Order{}) }\n",
		"internal/api/router.go": "package api\n\nfunc Route() {}\n",
		"internal/domain/order.go": "package domain\n\nimport \"example.com/layered/internal/db\"\n\n" +
			"type Order struct{}\n\nfunc Count() int { return db.Rows() }\n",
		"internal/db/conn.go": "package db\n\nfunc Save(any) {}\n\nfunc Rows() int { return 0 }\n",
		"internal/db/conn_test.go": "package db_test\n\nimport (\n\t\"testing\"\n\n\t" +
			"\"example.com/layered/internal/api\"\n)\n\nfunc TestHandle(*testing.T) { api.Handle() }\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating the folder of %q failed: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %q failed: %v", name, err)
		}
	}
	return root
}
