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
	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestProjectGraphDrawsOneNodePerFileOfTheProject(t *testing.T) {
	// The wiring, end to end and through the public grammar: locate the fixture project, extract it, project
	// it, count it. Nothing here is configured, so this is the default report.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"internal/db/query.go",
		"main.go",
	}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the report's nodes are %v, want the project's five files %v", labels, wantNodes)
	}
	want := projection.Summary{Nodes: 5, Edges: 4, Dependencies: 4}
	if summary := snapshot.Summary(); summary != want {
		t.Errorf("the summary is %+v, want %+v", summary, want)
	}
}

func TestProjectGraphReadsNothingUntilTheTerminal(t *testing.T) {
	// A report is a value, not an action. Every modifier here would have to fail on a locator naming no project
	// if it read anything, and none of them does.
	nowhere := &extraction.ProjectLocator{Directory: t.TempDir()}

	report := fluentapi.ProjectGraph(nowhere).
		IncludingExternalDependencies().
		IncludingSelfDependencies().
		FocusOn("internal/**", 2).
		ReachableFrom("main.go").
		DependentsOf("internal/db/**").
		CollapseToFolderDepth(2).
		CollapseByPattern("api", "internal/api/**").
		Titled("built and never run").
		WithCheckOptions(&kernel.CheckOptions{IncludeTestFiles: true})

	if !strings.HasPrefix(report.String(), "project graph") {
		t.Errorf("String() = %q, want the sentence the user typed", report.String())
	}
	if strings.Contains(report.String(), "rejected") {
		t.Errorf("String() = %q, want nothing rejected: every modifier was given something valid", report.String())
	}
}

func TestDependencyGraphIsTheSameEntryPointAsProjectGraph(t *testing.T) {
	// One entry point under the two names the family gives it, so a chain can read as a noun phrase.
	if got, want := fluentapi.DependencyGraph(nil).String(), fluentapi.ProjectGraph(nil).String(); got != want {
		t.Errorf("DependencyGraph renders as %q, want the same report as %q", got, want)
	}
}

func TestProjectGraphCopiesTheLocatorItWasGiven(t *testing.T) {
	// A builder that is immutable in every stage but the argument it started from would be the more surprising
	// of the two contracts, and a caller building one report per directory reuses one struct.
	root := writeFixtureProject(t)
	locator := fixtureLocator(t, root)
	report := fluentapi.ProjectGraph(locator)
	locator.Directory = t.TempDir()

	if _, err := report.Snapshot(); err != nil {
		t.Errorf("Snapshot failed after the caller's locator was edited: %v", err)
	}
}

func TestAGraphBuilderIsImmutable(t *testing.T) {
	// The point of a value: a query is the expensive half of a report to write, and asking two questions about
	// one part of a system should not mean typing it twice.
	base := fluentapi.ProjectGraph(nil).FocusOn("internal/api/**", 1)

	inside := base.CollapseToFolderDepth(3)
	outside := base.IncludingExternalDependencies()

	if strings.Contains(base.String(), "collapse") || strings.Contains(base.String(), "including") {
		t.Errorf("the stored report reads %q, want it unchanged by what was chained onto it", base)
	}
	if !strings.Contains(inside.String(), "collapse to folder depth 3") {
		t.Errorf("the branch reads %q, want the collapse it was given", inside)
	}
	if strings.Contains(inside.String(), "including external dependencies") {
		t.Errorf("the branch reads %q, want nothing from the other branch in it", inside)
	}
	if strings.Contains(outside.String(), "collapse to folder depth") {
		t.Errorf("the other branch reads %q, want nothing from the first in it", outside)
	}
}

func TestAGraphBuilderDoesNotShareItsQueryWithTheReportItWasBranchedFrom(t *testing.T) {
	// The failure a struct copy alone would leave: the two branches append into one backing array, and the
	// second modifier silently overwrites the first branch's. The base carries three of the modifier, because
	// that is the length at which append leaves spare capacity behind for the second branch to land in — and
	// both branches are built before either is read, which is what makes the overwrite visible.
	t.Run("collapse by pattern", func(t *testing.T) {
		base := fluentapi.ProjectGraph(nil).
			CollapseByPattern("one", "a/**").
			CollapseByPattern("two", "b/**").
			CollapseByPattern("three", "c/**")

		api := base.CollapseByPattern("api", "internal/api/**")
		db := base.CollapseByPattern("db", "internal/db/**")

		if !strings.Contains(api.String(), `"api"`) || strings.Contains(api.String(), `"db"`) {
			t.Errorf("one branch reads %q, want the group it was given and none from the other branch", api)
		}
		if !strings.Contains(db.String(), `"db"`) || strings.Contains(db.String(), `"api"`) {
			t.Errorf("the other branch reads %q, want the group it was given and none from the first", db)
		}
		if rendered := base.String(); strings.Contains(rendered, `"api"`) || strings.Contains(rendered, `"db"`) {
			t.Errorf("the stored report reads %q, want only the three groups it was described with", rendered)
		}
	})

	t.Run("focus on", func(t *testing.T) {
		base := fluentapi.ProjectGraph(nil).
			FocusOn("a/**", 0).
			FocusOn("b/**", 0).
			FocusOn("c/**", 0)

		api := base.FocusOn("internal/api/**", 1)
		db := base.FocusOn("internal/db/**", 1)

		if !strings.Contains(api.String(), "internal/api/**") || strings.Contains(api.String(), "internal/db/**") {
			t.Errorf("one branch reads %q, want the focus it was given and none from the other branch", api)
		}
		if !strings.Contains(db.String(), "internal/db/**") || strings.Contains(db.String(), "internal/api/**") {
			t.Errorf("the other branch reads %q, want the focus it was given and none from the first", db)
		}
		if rendered := base.String(); strings.Contains(rendered, "internal/") {
			t.Errorf("the stored report reads %q, want only the three focuses it was described with", rendered)
		}
	})
}

func TestAGraphBuilderRendersItsModifiersInOneOrderWhicheverOrderTheyWereTypedIn(t *testing.T) {
	// The modifiers are order-independent, so two chains describing the same report should read the same — a
	// reader comparing two reports should not have to notice that one of them said `titled` first.
	first := fluentapi.ProjectGraph(nil).Titled("the api layer").IncludingExternalDependencies().FocusOn("internal/api/**", 1)
	second := fluentapi.ProjectGraph(nil).FocusOn("internal/api/**", 1).IncludingExternalDependencies().Titled("the api layer")

	want := `project graph, including external dependencies, focus on path matches "internal/api/**" within 1 hop, titled "the api layer"`
	if got := first.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := second.String(); got != want {
		t.Errorf("the same modifiers in another order render as %q, want %q", got, want)
	}
}

func TestTitledNamesTheReportTheRendererPrints(t *testing.T) {
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).Titled("the modules of this project").Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if snapshot.Title() != "the modules of this project" {
		t.Errorf("the report is titled %q, want what the chain said", snapshot.Title())
	}
}

func TestTitledTwiceKeepsTheLastTitle(t *testing.T) {
	// A title is one string, not a list.
	report := fluentapi.ProjectGraph(nil).Titled("first").Titled("second")

	if got, want := report.String(), `project graph, titled "second"`; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestWithCheckOptionsSaysHowTheProjectIsRead(t *testing.T) {
	// The bag is a modifier rather than an argument to the terminal, because a snapshot, a rendered diagram and
	// a file written to disk would each have to take the same one. This proves it is honored: the fixture has a
	// test file, and it is a node of the report only when the options ask for it.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		WithCheckOptions(&kernel.CheckOptions{IncludeTestFiles: true}).
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !slices.Contains(nodeLabels(snapshot.Nodes()), "internal/api/handler_test.go") {
		t.Errorf("the report's nodes are %v, want the test file among them", nodeLabels(snapshot.Nodes()))
	}
	if !strings.Contains(fluentapi.ProjectGraph(nil).WithCheckOptions(nil).String(), "with check options") {
		t.Error("the modifier is not in the sentence the report renders as, want a reader able to see it")
	}
}

func TestWithCheckOptionsCopiesTheBagItWasGiven(t *testing.T) {
	// A report already described must not change when the bag it was built from is edited afterwards, for the
	// reason the locator is copied at the entry point.
	root := writeFixtureProject(t)
	options := &kernel.CheckOptions{IncludeTestFiles: true}
	report := fluentapi.ProjectGraph(fixtureLocator(t, root)).WithCheckOptions(options)
	options.IncludeTestFiles = false

	snapshot, err := report.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !slices.Contains(nodeLabels(snapshot.Nodes()), "internal/api/handler_test.go") {
		t.Errorf("the report's nodes are %v, want the test file the options asked for when the report was described", nodeLabels(snapshot.Nodes()))
	}
}

func TestAGraphReportRejectsAPatternItCannotCompile(t *testing.T) {
	// A modifier has nowhere to put an error, so the terminal reports it — naming the modifier the user has to
	// go and fix and quoting the pattern as they typed it.
	tests := []struct {
		modifier string
		report   fluentapi.GraphBuilder
	}{
		{modifier: "focus on", report: fluentapi.ProjectGraph(nil).FocusOn("[unclosed", 1)},
		{modifier: "reachable from", report: fluentapi.ProjectGraph(nil).ReachableFrom("[unclosed")},
		{modifier: "dependents of", report: fluentapi.ProjectGraph(nil).DependentsOf("[unclosed")},
		{modifier: "collapse by pattern", report: fluentapi.ProjectGraph(nil).CollapseByPattern("api", "[unclosed")},
	}

	for _, test := range tests {
		t.Run(test.modifier, func(t *testing.T) {
			snapshot, err := test.report.Snapshot()

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Snapshot error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.modifier {
				t.Errorf("UserError.Operation = %q, want the modifier %q", user.Operation, test.modifier)
			}
			if user.Subject != "[unclosed" {
				t.Errorf("UserError.Subject = %q, want the pattern as the user typed it", user.Subject)
			}
			if !errors.Is(err, matching.ErrInvalidPattern) {
				t.Errorf("Snapshot error = %v, want it to wrap ErrInvalidPattern", err)
			}
			if !snapshot.Empty() {
				t.Errorf("Snapshot reports %v beside the error, want nothing said about the project", snapshot)
			}
			if rendered := test.report.String(); !strings.Contains(rendered, "rejected") {
				t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
			}
		})
	}
}

func TestAGraphReportReportsTheFirstRejectedModifierOfTheWholeChain(t *testing.T) {
	// The first is the one a user has to fix, and a chain reporting the last would point at the wrong line.
	report := fluentapi.ProjectGraph(nil).FocusOn("[unclosed", 1).CollapseToFolderDepth(0).ReachableFrom("[alsounclosed")

	_, err := report.Snapshot()

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Snapshot error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "focus on" || user.Subject != "[unclosed" {
		t.Errorf("UserError = %v, want the first rejected modifier of the chain", user)
	}
}

// fixtureLocator points at a project written for one test, and clears the graph cache around it. Extraction
// caches per directory, and a fixture is written fresh into a temporary directory that a later test may be
// handed again, so a cached graph would answer for the wrong source.
func fixtureLocator(t *testing.T, root string) *extraction.ProjectLocator {
	t.Helper()

	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	return &extraction.ProjectLocator{Directory: root}
}

// writeFixtureProject writes the project every report in this package is built from: five files in three
// folders, one test file, and two dependencies leaving the project — one of them a blank import, so that the
// import kinds an aggregated edge unions are visible.
//
//	main.go                 -> internal/api    (both files of the package)
//	internal/api/handler.go -> internal/db     (both files of the package), net/http
//	internal/db/conn.go     -> database/sql    (blank)
//
// So the project has four dependencies of its own and two that leave it, which is what the counts in these
// tests are.
func writeFixtureProject(t *testing.T) string {
	t.Helper()

	return writeProject(t, map[string]string{
		"go.mod":                       "module example.com/report\n\ngo 1.26\n",
		"main.go":                      "package main\n\nimport \"example.com/report/internal/api\"\n\nfunc main() { api.Handle() }\n",
		"internal/api/handler.go":      "package api\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/report/internal/db\"\n)\n\nfunc Handle() *http.Client { db.Connect(); return nil }\n",
		"internal/api/router.go":       "package api\n\nfunc Route() {}\n",
		"internal/api/handler_test.go": "package api\n\nimport \"testing\"\n\nfunc TestHandle(*testing.T) { Handle() }\n",
		"internal/db/conn.go":          "package db\n\nimport _ \"database/sql\"\n\nfunc Connect() {}\n",
		"internal/db/query.go":         "package db\n\nfunc Query() {}\n",
	})
}

// writeProject writes these files into a temporary directory and returns its root, which is what a fixture
// project is: a go.mod and the source the extractor is to read.
func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
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

// nodeLabels are the labels a report's nodes are drawn under, in order, for a message about what came out.
func nodeLabels(nodes []projection.Node) []string {
	labels := make([]string, 0, len(nodes))
	for _, node := range nodes {
		labels = append(labels, node.Label())
	}
	return labels
}

// edgeDescriptions are a report's dependencies as they render, in order.
func edgeDescriptions(edges []projection.Edge) []string {
	descriptions := make([]string, 0, len(edges))
	for _, edge := range edges {
		descriptions = append(descriptions, edge.String())
	}
	return descriptions
}
