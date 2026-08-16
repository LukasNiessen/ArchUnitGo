package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

func TestProjectFilesAndFilesAreOneEntryPoint(t *testing.T) {
	// Resolved against a project on disk, not against the fixture graph: the locator is an argument of
	// both entry points, so the only way to see that either of them keeps it is to name a project that is
	// not the one this test is running in and read the files back.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	locator := &extraction.ProjectLocator{Directory: writeFixtureProject(t)}

	verbose := fluentapi.ProjectFiles(locator).InFolder("internal/**").WithName("*.go")
	short := fluentapi.Files(locator).InFolder("internal/**").WithName("*.go")

	if verbose.String() != short.String() {
		t.Errorf("Files() built %s, want the rule ProjectFiles() builds, %s", short, verbose)
	}
	entryPoints := []struct {
		name string
		rule fluentapi.FilesBuilder
	}{
		{name: "project files", rule: verbose},
		{name: "files", rule: short},
	}
	want := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
	for _, entry := range entryPoints {
		selected, err := entry.rule.SelectFiles(nil)
		if err != nil {
			t.Fatalf("`%s` failed to select against the project its locator names: %v", entry.name, err)
		}
		if !slices.Equal(selected, want) {
			t.Errorf("`%s` selects %v, want the files of the project the locator names, %v", entry.name, selected, want)
		}
	}
}

func TestProjectFilesWithNoScopeVerbIsEveryFileOfTheProject(t *testing.T) {
	rule := fluentapi.ProjectFiles(nil)

	if selectors := rule.Selectors(); len(selectors) != 0 {
		t.Errorf("Selectors() = %v, want none before a scope verb is chained", selectors)
	}
	want := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"main.go",
	}
	if selected := selectFrom(t, rule); !slices.Equal(selected, want) {
		t.Errorf("`project files` selects %v, want every file %v", selected, want)
	}
}

func TestEachScopeVerbLooksAtThePartOfAnIdentifierItNames(t *testing.T) {
	tests := []struct {
		name string
		rule fluentapi.FilesBuilder
		want []string
	}{
		{
			name: "with name",
			rule: fluentapi.ProjectFiles(nil).WithName("*.go"),
			want: []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go", "main.go"},
		},
		{
			name: "with name, wherever the file lives",
			rule: fluentapi.ProjectFiles(nil).WithName("conn.go"),
			want: []string{"internal/db/conn.go"},
		},
		{
			name: "in folder",
			rule: fluentapi.ProjectFiles(nil).InFolder("internal/api"),
			want: []string{"internal/api/handler.go", "internal/api/router.go"},
		},
		{
			name: "in folder, and everything below it",
			rule: fluentapi.ProjectFiles(nil).InFolder("internal/**"),
			want: []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"},
		},
		{
			name: "in folder, the project root",
			rule: fluentapi.ProjectFiles(nil).InFolder("."),
			want: []string{"main.go"},
		},
		{
			name: "in path",
			rule: fluentapi.ProjectFiles(nil).InPath("internal/*/handler.go"),
			want: []string{"internal/api/handler.go"},
		},
		{
			name: "in file",
			rule: fluentapi.ProjectFiles(nil).InFile("internal/db/conn.go"),
			want: []string{"internal/db/conn.go"},
		},
		{
			name: "in file, taken literally rather than as a pattern",
			rule: fluentapi.ProjectFiles(nil).InFile("internal/*/conn.go"),
			want: []string{},
		},
		{
			name: "in file, a bare name is not an identifier",
			rule: fluentapi.ProjectFiles(nil).InFile("conn.go"),
			want: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if selected := selectFrom(t, test.rule); !slices.Equal(selected, test.want) {
				t.Errorf("%s selects %v, want %v", test.rule, selected, test.want)
			}
		})
	}
}

func TestTheScopeVerbsAreChainedWithAnd(t *testing.T) {
	narrowed := fluentapi.ProjectFiles(nil).InFolder("internal/**").WithName("*r.go")
	reversed := fluentapi.ProjectFiles(nil).WithName("*r.go").InFolder("internal/**")

	if selectors := narrowed.Selectors(); len(selectors) != 2 {
		t.Fatalf("Selectors() = %v, want one per chained verb", selectors)
	}
	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if selected := selectFrom(t, narrowed); !slices.Equal(selected, want) {
		t.Errorf("%s selects %v, want %v", narrowed, selected, want)
	}
	// Each verb narrows, so their order cannot change the rule.
	if selected := selectFrom(t, reversed); !slices.Equal(selected, want) {
		t.Errorf("%s selects %v, want %v", reversed, selected, want)
	}
}

func TestAFilesBuilderIsImmutableAndCanBeBranchedFrom(t *testing.T) {
	// The property the whole builder design exists for: a half-built rule stored in a variable, branched
	// from twice, and unchanged by either branch.
	base := fluentapi.ProjectFiles(nil).InFolder("internal/**")

	sources := base.WithName("*.go")
	one := base.InFile("internal/db/conn.go")

	if selectors := base.Selectors(); len(selectors) != 1 {
		t.Errorf("the stored rule's Selectors() = %v, want the one verb it was built with", selectors)
	}
	if selectors := sources.Selectors(); len(selectors) != 2 {
		t.Errorf("the branch's Selectors() = %v, want the base's verb and its own", selectors)
	}
	wantBase := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
	if selected := selectFrom(t, base); !slices.Equal(selected, wantBase) {
		t.Errorf("the stored rule selects %v, want %v", selected, wantBase)
	}
	if selected := selectFrom(t, sources); !slices.Equal(selected, wantBase) {
		t.Errorf("the first branch selects %v, want %v", selected, wantBase)
	}
	wantOne := []string{"internal/db/conn.go"}
	if selected := selectFrom(t, one); !slices.Equal(selected, wantOne) {
		t.Errorf("the second branch selects %v, want %v", selected, wantOne)
	}
}

func TestAFilesBuildersSelectorsAreTheCallersOwnCopy(t *testing.T) {
	rule := fluentapi.ProjectFiles(nil).InFolder("internal/**")

	rule.Selectors()[0] = matching.Filter{}

	if selectors := rule.Selectors(); selectors[0].Target() != matching.TargetPathWithoutFilename {
		t.Errorf("Selectors() = %v after a caller overwrote the result, want the rule unchanged", selectors)
	}
	want := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
	if selected := selectFrom(t, rule); !slices.Equal(selected, want) {
		t.Errorf("the rule selects %v, want %v", selected, want)
	}
}

func TestABranchDoesNotWriteIntoTheRuleItGrewFrom(t *testing.T) {
	// The trap a value receiver alone does not close: a struct copy shares the selectors' backing array,
	// so two branches appending to it would write over each other. Both branches are built before either
	// is read, which is what makes the overwrite visible — and the base carries three verbs, because that
	// is where append leaves spare capacity for the second branch to write into.
	base := fluentapi.ProjectFiles(nil).InPath("internal/**").WithName("*.go").InFolder("internal/**")

	api := base.InFolder("internal/api")
	db := base.InFolder("internal/db")

	wantAPI := []string{"internal/api/handler.go", "internal/api/router.go"}
	if selected := selectFrom(t, api); !slices.Equal(selected, wantAPI) {
		t.Errorf("the first branch selects %v, want %v", selected, wantAPI)
	}
	wantDB := []string{"internal/db/conn.go"}
	if selected := selectFrom(t, db); !slices.Equal(selected, wantDB) {
		t.Errorf("the second branch selects %v, want %v", selected, wantDB)
	}
}

func TestTheLocatorAnEntryPointWasGivenCannotBeChangedAfterwards(t *testing.T) {
	// The other half of immutability, and the half a value receiver cannot give: the builder holds a
	// *ProjectLocator, so a caller reusing one struct to build a rule per directory would leave every
	// stored rule pointing at the last of them. Resolved against a project on disk, because the locator is
	// an argument nothing else observes.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	root := writeFixtureProject(t)
	entryPoints := map[string]func(*extraction.ProjectLocator) fluentapi.FilesBuilder{
		"project files": fluentapi.ProjectFiles,
		"files":         fluentapi.Files,
	}

	for name, entryPoint := range entryPoints {
		t.Run(name, func(t *testing.T) {
			locator := &extraction.ProjectLocator{Directory: root}
			rule := entryPoint(locator).InFolder("internal/**").WithName("*.go")

			locator.Directory = t.TempDir()

			selected, err := rule.SelectFiles(nil)
			if err != nil {
				t.Fatalf("`%s` failed after the locator it was given was written to: %v", name, err)
			}
			want := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
			if !slices.Equal(selected, want) {
				t.Errorf("`%s` selects %v, want the project it was built with, %v", name, selected, want)
			}
		})
	}
}

func TestARejectedPatternIsAUserErrorNamingTheScopeVerb(t *testing.T) {
	tests := []struct {
		verb string
		rule fluentapi.FilesBuilder
	}{
		{verb: "with name", rule: fluentapi.ProjectFiles(nil).WithName("[unclosed")},
		{verb: "in folder", rule: fluentapi.ProjectFiles(nil).InFolder("[unclosed")},
		{verb: "in path", rule: fluentapi.ProjectFiles(nil).InPath("[unclosed")},
		{verb: "in file", rule: fluentapi.ProjectFiles(nil).InFile("")},
	}

	for _, test := range tests {
		t.Run(test.verb, func(t *testing.T) {
			_, err := test.rule.SelectFiles(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("SelectFiles error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the scope verb %q", user.Operation, test.verb)
			}
			if !errors.Is(err, matching.ErrInvalidPattern) {
				t.Errorf("SelectFiles error = %v, want it to wrap matching.ErrInvalidPattern", err)
			}
			if !strings.Contains(test.rule.String(), "rejected") {
				t.Errorf("%s renders without the rejection, want it visible in a test failure", test.rule)
			}
		})
	}
}

func TestARejectedPatternIsReportedBeforeTheProjectIsRead(t *testing.T) {
	// What the user typed is wrong whatever the project turns out to be, and reading the project first
	// would answer a typo with a complaint about the locator.
	rule := fluentapi.ProjectFiles(&extraction.ProjectLocator{Directory: t.TempDir()}).InFolder("[unclosed")

	_, err := rule.SelectFiles(nil)

	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("SelectFiles error = %v, want the rejected pattern rather than the missing project", err)
	}
	if errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("SelectFiles error = %v, want the project left unread", err)
	}
}

func TestARejectedPatternNarrowsNothingAndOnlyTheFirstIsReported(t *testing.T) {
	rule := fluentapi.ProjectFiles(nil).
		InFolder("internal/**").
		WithName("[unclosed").
		InPath("[also unclosed")

	if selectors := rule.Selectors(); len(selectors) != 1 {
		// A zero Filter matches nothing, so a rejected pattern joining the selectors would report every
		// file as unselected instead of reporting the typo.
		t.Errorf("Selectors() = %v, want only the verb that compiled", selectors)
	}

	_, err := rule.SelectFiles(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("SelectFiles error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "with name" {
		t.Errorf("UserError.Operation = %q, want the first rejected verb, `with name`", user.Operation)
	}
	if user.Subject != "[unclosed" {
		t.Errorf("UserError.Subject = %q, want the pattern as the user typed it", user.Subject)
	}
}

func TestSelectFilesSelectsTheFilesOfARealProject(t *testing.T) {
	// The level above the unit tests: a rule built through the fluent API, resolved against a project on
	// disk, with the locator and the check options threaded through it.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	root := writeFixtureProject(t)
	locator := &extraction.ProjectLocator{Directory: root}

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/**").WithName("*.go")

	selected, err := rule.SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}

	want := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
	if !slices.Equal(selected, want) {
		t.Errorf("%s selects %v, want %v", rule, selected, want)
	}

	// The check options reach the extraction, so a rule can be held to the test files too.
	withTests, err := rule.SelectFiles(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}
	if !slices.Contains(withTests, "internal/api/handler_test.go") {
		t.Errorf("%s selects %v with IncludeTestFiles, want the test file among them", rule, withTests)
	}
	if slices.Contains(selected, "internal/api/handler_test.go") {
		t.Errorf("%s selects %v by default, want the test file left out", rule, selected)
	}
}

func TestSelectFilesRejectsALocatorThatIsNotAProject(t *testing.T) {
	rule := fluentapi.ProjectFiles(&extraction.ProjectLocator{Directory: t.TempDir()}).InFolder("internal/**")

	_, err := rule.SelectFiles(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("SelectFiles error = %v, want a *archerror.UserError", err)
	}
	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("SelectFiles error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
}

func TestAFilesBuilderRendersTheScopeItDescribes(t *testing.T) {
	rule := fluentapi.ProjectFiles(nil).InFolder("internal/**").WithName("*.go")

	rendered := rule.String()

	want := `project files, path without filename matches "internal/**", filename matches "*.go"`
	if rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
	if entry := fluentapi.ProjectFiles(nil).String(); entry != "project files" {
		t.Errorf("String() = %q, want the entry point on its own", entry)
	}
}

// selectFrom resolves a rule's scope against the fixture graph, which is what a terminal will do with
// the graph it extracted. It goes through Selectors, so it is the same route the coming predicates take.
func selectFrom(t *testing.T, rule fluentapi.FilesBuilder) []string {
	t.Helper()

	return projection.SelectFiles(fixtureGraph(), rule.Selectors()...)
}

// fixtureGraph is the fixture project of writeFixtureProject, hand-built in the shape the extractor
// produces one: a self-edge per file, and the dependencies between them.
func fixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("main.go", "internal/api/router.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
	)
}

// writeFixtureProject writes the project fixtureGraph describes into a directory of this test's own: two
// packages under internal/, a main package at the root that depends on one of them, and a test file that
// is a node only when the check options ask for it.
func writeFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                       "module example.com/fixture\n\ngo 1.26\n",
		"main.go":                      "package main\n\nimport \"example.com/fixture/internal/api\"\n\nfunc main() { api.Handle() }\n",
		"internal/api/handler.go":      "package api\n\nimport \"example.com/fixture/internal/db\"\n\nfunc Handle() { db.Connect() }\n",
		"internal/api/router.go":       "package api\n\nfunc Route() {}\n",
		"internal/api/handler_test.go": "package api\n\nimport \"testing\"\n\nfunc TestHandle(*testing.T) { Handle() }\n",
		"internal/db/conn.go":          "package db\n\nfunc Connect() {}\n",
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
