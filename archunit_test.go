package archunit_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	archunit "github.com/LukasNiessen/ArchUnitGo"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// The public surface names both mood stages by the type the chain actually returns, so that a
// half-built rule of either mood can be stored in a struct field or passed to a helper. Checked at
// compile time, because a type alias that named the wrong thing would fail nowhere else.
var (
	_ archunit.FilesShouldBuilder    = archunit.ProjectFiles(nil).Should()
	_ archunit.FilesShouldNotBuilder = archunit.ProjectFiles(nil).ShouldNot()
)

// A built rule is a Checkable, which is the one thing a helper looping over a suite's rules has to know
// about it, and its own type is named too so that it can be stored. Both at compile time, for the same
// reason as above.
var (
	_ archunit.Checkable            = archunit.ProjectFiles(nil).Should().HaveNoCycles()
	_ archunit.FilesCyclesCondition = archunit.ProjectFiles(nil).Should().HaveNoCycles()
)

// The three self-contained naming and location rules, in both moods, are one terminal type — and they are
// Checkables like every other built rule, so a suite can keep them in one list.
var (
	_ archunit.Checkable            = archunit.ProjectFiles(nil).Should().HaveName("*.go")
	_ archunit.FilesNamingCondition = archunit.ProjectFiles(nil).ShouldNot().HaveName("*.go")
	_ archunit.FilesNamingCondition = archunit.ProjectFiles(nil).Should().BeInFolder("common/**")
	_ archunit.FilesNamingCondition = archunit.ProjectFiles(nil).ShouldNot().BeInFolder("common/**")
	_ archunit.FilesNamingCondition = archunit.ProjectFiles(nil).Should().BeInPath("common/**/*.go")
	_ archunit.FilesNamingCondition = archunit.ProjectFiles(nil).ShouldNot().BeInPath("common/**/*.go")
)

// The relational rule, in both moods, is one type that is both the object stage and the terminal — so a
// half-built rule whose object is still being narrowed can be stored too.
var (
	_ archunit.Checkable                = archunit.ProjectFiles(nil).ShouldNot().DependOnFiles().InFolder("files/**")
	_ archunit.FilesDependencyCondition = archunit.ProjectFiles(nil).ShouldNot().DependOnFiles()
	_ archunit.FilesDependencyCondition = archunit.ProjectFiles(nil).Should().DependOnFiles().WithName("*.go")
	_ archunit.FilesDependencyCondition = archunit.ProjectFiles(nil).Should().DependOnFiles().InPath("common/**/*.go")
)

// The third-party rule, in both moods, is one type that is both the object stage and the terminal — and its
// object verb is repeatable, so a policy naming several modules is still one storable value.
var (
	_ archunit.Checkable = archunit.ProjectFiles(nil).ShouldNot().
		DependOnExternalModules().Matching("*.*/**")
	_ archunit.FilesExternalDependencyCondition = archunit.ProjectFiles(nil).ShouldNot().DependOnExternalModules()
	_ archunit.FilesExternalDependencyCondition = archunit.ProjectFiles(nil).Should().
		DependOnExternalModules().Matching("golang.org/x/tools/**")
	_ archunit.FilesExternalDependencyCondition = archunit.ProjectFiles(nil).ShouldNot().
		DependOnExternalModules().Matching("github.com/deprecated/**").Matching("gopkg.in/**")
)

// The rule whose predicate the user writes themselves, in both moods, is one terminal type — and the function
// it takes and the file that function is handed are named on the surface too, so that a predicate can be
// declared as a variable, stored beside the rules it is used by or shared between two of them.
var (
	_ archunit.Checkable               = archunit.ProjectFiles(nil).Should().AdhereTo(isGoFile, "be a Go file")
	_ archunit.FilesAdherenceCondition = archunit.ProjectFiles(nil).ShouldNot().AdhereTo(isGoFile, "be a Go file")
	_ archunit.FilePredicate           = isGoFile
)

// isGoFile is a user predicate written the way a user would write one: a question about one archunit.FileInfo,
// answered from the fields the public surface names on it.
func isGoFile(file archunit.FileInfo) bool {
	return file.Extension == ".go" && file.NonBlankLineCount > 0
}

// The layer policy's three stages and its terminal, each named by the type the chain actually returns —
// including the declaration stage, because declaring a project's layers is the expensive half of writing a
// policy and a suite should be able to do it once in a helper and branch every policy off the result. There is
// no mood stage to name: the two predicates are the two moods.
var (
	_ archunit.LayersBuilder      = archunit.ProjectLayers(nil).Layer("kernel").DefinedByFolder("common/**")
	_ archunit.LayerBuilder       = archunit.ProjectLayers(nil).Layer("kernel")
	_ archunit.LayerPolicyBuilder = archunit.ProjectLayers(nil).
		Layer("kernel").DefinedByFolder("common/**").WhereLayer("kernel")
	_ archunit.Checkable = archunit.ProjectLayers(nil).
		Layer("kernel").DefinedByFolder("common/**").
		WhereLayer("kernel").MayOnlyDependOnLayers()
	_ archunit.LayersPolicyCondition = archunit.Layers(nil).
		Layer("kernel").DefinedBy("common/**").
		Layer("files").DefinedBy("files/**").
		WhereLayer("files").MayNotDependOnLayers("kernel")
)

// The slices family: the slicing, both moods and the terminal, each named by the type the chain actually returns
// — including the slicing, because cutting a project into slices is the half of the rule a suite writes once and
// branches every rule off.
var (
	_ archunit.SlicesBuilder          = archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**")
	_ archunit.SlicesBuilder          = archunit.ProjectSlices(nil).DefinedByRegex(`internal/([^/]+)/.*`)
	_ archunit.SlicesShouldBuilder    = archunit.ProjectSlices(nil).DefinedBy("(*)/**").Should()
	_ archunit.SlicesShouldNotBuilder = archunit.ProjectSlices(nil).DefinedBy("(*)/**").ShouldNot()
	_ archunit.Checkable              = archunit.ProjectSlices(nil).
		DefinedBy("(*)/**").Should().ContainDependency("files", "common")
	_ archunit.SlicesDependencyCondition = archunit.ProjectSlices(nil).
		DefinedBy("(*)/**").ShouldNot().ContainDependency("common", "files")
)

// And the four slicing projections, for a caller who wants the mapper rather than a rule: three of them cut a
// name out of an identifier and the fourth relabels nothing, which is what a report of a project's own files
// speaks. The two that take a pattern can fail, so their names are asserted in the shape they actually have.
var (
	_ func(string) (archunit.MapFunction, error) = archunit.SliceByPattern
	_ func(string) (archunit.MapFunction, error) = archunit.SliceByRegex
	_ func() archunit.MapFunction                = archunit.SliceByFileSuffix
	_ func() archunit.MapFunction                = archunit.Identity
)

// The metrics family's three stages, each named by the type the chain actually returns, and the eight count
// verbs as method values, because which numbers this library can take of a project is part of the surface. There
// is no mood stage to name yet — the six threshold predicates land with the rules that judge a number — so what
// is named as a terminal is the resolution door, and a Measurement with it, because a suite that wants the
// numbers of its project rather than a pass or a fail reads them.
var (
	_ archunit.MetricsBuilder = archunit.Metrics(nil).
		WithName("*.go").
		InFolder("common/**").
		InPath("common/**/*.go").
		ForClassesMatching("*Builder")
	_ archunit.MetricsCountBuilder = archunit.Metrics(nil).Count()
	_ archunit.MetricBuilder       = archunit.Metrics(nil).Count().LinesOfCode()
	_ archunit.Measurement         = archunit.Measurement{Metric: "lines of code", Subject: "archunit.go", Value: 1}

	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().LinesOfCode
	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().Statements
	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().Imports
	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().Functions
	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().Classes
	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().Interfaces
	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().MethodCount
	_ func() archunit.MetricBuilder = archunit.Metrics(nil).Count().FieldCount

	_ func(*archunit.CheckOptions) ([]archunit.Measurement, error) = archunit.Metrics(nil).Count().Imports().Measure
)

// The report family, whose chain is one stage and whose terminals are not Checkables, because a report is not a
// rule: the builder is named so that a described query can be stored and branched into as many focuses and
// output formats as a suite wants, and the snapshot's four parts are named so that a caller can write a
// renderer of their own over them beside the six the library ships. The two slices are asserted as slices because that is the only compile-time
// way to say that these aliases name what a snapshot actually hands back.
var (
	_ archunit.GraphBuilder = archunit.ProjectGraph(nil).
		IncludingExternalDependencies().
		IncludingSelfDependencies().
		FocusOn("common/**", 1).
		ReachableFrom("archunit.go").
		DependentsOf("common/extraction/**").
		CollapseToFolderDepth(2).
		CollapseByPattern("kernel", "common/**").
		Titled("the modules of this library").
		WithCheckOptions(&archunit.CheckOptions{IncludeTestFiles: true})
	_ archunit.GraphBuilder = archunit.DependencyGraph(nil)
	_ archunit.GraphSummary = archunit.GraphSnapshot{}.Summary()
	_ []archunit.GraphNode  = archunit.GraphSnapshot{}.Nodes()
	_ []archunit.GraphEdge  = archunit.GraphSnapshot{}.Edges()
)

// The report's other twelve terminals — six output formats, each in the string form and the file form — as
// method values rather than calls, because what they render is the graph module's own business and reading the
// project is not this file's. What is said here is that all twelve are on the public surface, under these
// names and in these two shapes.
var (
	_ func() (string, error) = archunit.ProjectGraph(nil).ToDot
	_ func() (string, error) = archunit.ProjectGraph(nil).ToMermaid
	_ func() (string, error) = archunit.ProjectGraph(nil).ToD2
	_ func() (string, error) = archunit.ProjectGraph(nil).ToCSV
	_ func() (string, error) = archunit.ProjectGraph(nil).ToJSON
	_ func() (string, error) = archunit.ProjectGraph(nil).ToHTML
	_ func(string) error     = archunit.ProjectGraph(nil).ExportAsDot
	_ func(string) error     = archunit.ProjectGraph(nil).ExportAsMermaid
	_ func(string) error     = archunit.ProjectGraph(nil).ExportAsD2
	_ func(string) error     = archunit.ProjectGraph(nil).ExportAsCSV
	_ func(string) error     = archunit.ProjectGraph(nil).ExportAsJSON
	_ func(string) error     = archunit.ProjectGraph(nil).ExportAsHTML
)

// The violation types a rule reports are on the surface too, because a user who wants more than a pass or
// a fail reads the violation rather than its message.
var (
	_ archunit.Violation     = archunit.FileCycleViolation{}
	_ archunit.Violation     = archunit.EmptyTestViolation{}
	_ archunit.Violation     = archunit.FileNamingViolation{}
	_ archunit.Violation     = archunit.FileDependencyViolation{}
	_ archunit.Violation     = archunit.FileExternalDependencyViolation{}
	_ archunit.Violation     = archunit.FileAdherenceViolation{}
	_ archunit.Violation     = archunit.LayerDependencyViolation{}
	_ archunit.Violation     = archunit.SliceDependencyViolation{}
	_ archunit.Circuit       = archunit.FileCycleViolation{}.Cycle
	_ archunit.ViolationKind = archunit.KindFileCycle
	_ archunit.ViolationKind = archunit.KindFileNaming
	_ archunit.ViolationKind = archunit.KindFileDependency
	_ archunit.ViolationKind = archunit.KindFileExternalDependency
	_ archunit.ViolationKind = archunit.KindFileAdherence
	_ archunit.ViolationKind = archunit.KindLayerDependency
	_ archunit.ViolationKind = archunit.KindSliceDependency
	_ archunit.Mood          = archunit.FileNamingViolation{}.Mood
	_ archunit.Mood          = archunit.FileDependencyViolation{}.Mood
	_ archunit.Mood          = archunit.FileExternalDependencyViolation{}.Mood
	_ archunit.Mood          = archunit.FileAdherenceViolation{}.Mood
	_ archunit.Mood          = archunit.LayerDependencyViolation{}.Mood
	_ archunit.Mood          = archunit.SliceDependencyViolation{}.Mood
)

// The report layer is on the surface too, because a user who has a rule's violations still needs the message
// they read as — and a palette can be filled in without reaching for the package it comes from.
var (
	_ archunit.Result           = archunit.NewResultFactory(nil).Result(nil)
	_ archunit.ResultFactory    = archunit.NewResultFactory(&archunit.MessageOptions{MaxViolations: 10})
	_ archunit.ViolationFactory = archunit.NewViolationFactory(nil)
	_ archunit.Palette          = archunit.DefaultPalette()
	_ archunit.Palette          = archunit.Palette{Subject: archunit.ColorCyan, Requirement: archunit.ColorYellow}
	_ archunit.Color            = archunit.ColorNone
)

// And the assert helpers' own three names, because the handle a framework hands a test is what a user passes to
// them and the bag is what a suite fills in: the stdlib's handle satisfies both interfaces with no adapter, one
// written by hand satisfies the smaller of the two, and the suite form asks for the subtest only the stdlib's
// has.
var (
	_ archunit.TestingT      = (*testing.T)(nil)
	_ archunit.TestingT      = (*recorder)(nil)
	_ archunit.TestingRunner = (*testing.T)(nil)
	_ archunit.TestingRunner = (*runner)(nil)
	_ archunit.AssertOptions = archunit.AssertOptions{
		Check:   archunit.CheckOptions{AllowEmptyTests: true},
		Message: archunit.MessageOptions{MaxViolations: 10},
	}
)

func TestProjectFilesSelectsTheFilesOfThisRepository(t *testing.T) {
	// The whole chain through the public surface, dogfooding on this library: no locator, so the project
	// is the one this test is in, and the scope is a folder of it.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/matching")

	selected, err := rule.SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}

	for _, wanted := range []string{
		"common/matching/filter.go",
		"common/matching/match_target.go",
		"common/matching/regex_factory.go",
	} {
		if !slices.Contains(selected, wanted) {
			t.Errorf("%s selects %v, want %q among them", rule, selected, wanted)
		}
	}
	for _, file := range selected {
		if !strings.HasPrefix(file, "common/matching/") || strings.Count(file, "/") != 2 {
			t.Errorf("%s selects %q, want only the files of that one folder", rule, file)
		}
		if strings.HasSuffix(file, "_test.go") {
			t.Errorf("%s selects %q, want the test files left out by default", rule, file)
		}
	}
}

func TestFilesIsTheShortAliasOfProjectFilesOnThePublicSurface(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	verbose, err := archunit.ProjectFiles(nil).InFolder("common/**").WithName("regex_factory.go").SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}
	short, err := archunit.Files(nil).InFolder("common/**").WithName("regex_factory.go").SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}

	want := []string{"common/matching/regex_factory.go"}
	if !slices.Equal(verbose, want) {
		t.Errorf("`project files, in folder, with name` selects %v, want %v", verbose, want)
	}
	if !slices.Equal(short, verbose) {
		t.Errorf("`files, ...` selects %v, want what `project files, ...` selects, %v", short, verbose)
	}
}

func TestAStoredRuleCanBeBranchedFromOnThePublicSurface(t *testing.T) {
	// The example from AGENTS.md, on the surface a user types: one half-built rule, two branches, and the
	// base unchanged by either.
	t.Cleanup(archunit.ClearGraphCache)

	base := archunit.ProjectFiles(nil).InFolder("common/projection/**")
	cycles := base.InFolder("common/projection/cycles")
	tarjan := base.WithName("tarjan_scc.go")

	selectedBase := selectFiles(t, base)
	selectedCycles := selectFiles(t, cycles)
	selectedTarjan := selectFiles(t, tarjan)

	if !slices.Contains(selectedBase, "common/projection/project_edges.go") {
		t.Errorf("the stored rule selects %v, want the files of the folder itself among them", selectedBase)
	}
	if !slices.Contains(selectedBase, "common/projection/cycles/tarjan_scc.go") {
		t.Errorf("the stored rule selects %v, want the files below the folder among them", selectedBase)
	}
	if slices.Contains(selectedCycles, "common/projection/project_edges.go") {
		t.Errorf("the first branch selects %v, want only the subfolder it narrowed to", selectedCycles)
	}
	if want := []string{"common/projection/cycles/tarjan_scc.go"}; !slices.Equal(selectedTarjan, want) {
		t.Errorf("the second branch selects %v, want %v", selectedTarjan, want)
	}
}

func TestTheCheckOptionsReachTheSelection(t *testing.T) {
	// A nil bag is the defaults, and the one knob a scope can see today is whether the test files are
	// nodes of the graph at all.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/matching")

	production := selectFiles(t, rule)
	withTests, err := rule.SelectFiles(&archunit.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}

	if slices.Contains(production, "common/matching/regex_factory_test.go") {
		t.Errorf("%s selects %v by default, want the test files left out", rule, production)
	}
	if !slices.Contains(withTests, "common/matching/regex_factory_test.go") {
		t.Errorf("%s selects %v with IncludeTestFiles, want the test files among them", rule, withTests)
	}
}

func TestTheLocatorReachesTheProjectThroughEitherEntryPoint(t *testing.T) {
	// Both wrappers thread the locator through to the extraction, so a rule pointed at a directory that
	// holds no Go project says so — rather than quietly analyzing the repository this test runs in, which
	// is what dropping the argument would look like.
	t.Cleanup(archunit.ClearGraphCache)

	entryPoints := []struct {
		name string
		rule archunit.FilesBuilder
	}{
		{name: "project files", rule: archunit.ProjectFiles(&archunit.ProjectLocator{Directory: t.TempDir()})},
		{name: "files", rule: archunit.Files(&archunit.ProjectLocator{Directory: t.TempDir()})},
	}

	for _, entry := range entryPoints {
		t.Run(entry.name, func(t *testing.T) {
			selected, err := entry.rule.SelectFiles(nil)

			if err == nil {
				t.Errorf("`%s` selected %v against a directory that is no project, want an error naming it", entry.name, selected)
			}
			if len(selected) != 0 {
				t.Errorf("`%s` selects %v, want nothing when the project cannot be located", entry.name, selected)
			}
		})
	}
}

func TestARuleThatNamesAFolderNoFileIsInSelectsNothing(t *testing.T) {
	// A stale glob selects nothing rather than everything, and it is not an error: reporting it is the
	// empty-test guard's job, at the terminal that judges the rule.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/renamed/**")

	if selected := selectFiles(t, rule); len(selected) != 0 {
		t.Errorf("%s selects %v, want nothing", rule, selected)
	}
}

func TestBothMoodsAreReachableThroughThePublicSurface(t *testing.T) {
	// The mood stage on the surface a user types, dogfooding on this library: one scope, both moods, and
	// the two of them differing by the flag and by nothing else.
	t.Cleanup(archunit.ClearGraphCache)

	base := archunit.ProjectFiles(nil).InFolder("common/matching")
	positive := base.Should()
	negated := base.ShouldNot()

	if mood := positive.Mood(); mood != archunit.Should {
		t.Errorf("`should`.Mood() = %s, want %s", mood, archunit.Should)
	}
	if mood := negated.Mood(); mood != archunit.ShouldNot {
		t.Errorf("`should not`.Mood() = %s, want %s", mood, archunit.ShouldNot)
	}
	if !negated.Mood().Negated() {
		t.Error("`should not` is not the negated mood, want the flag the assertions read")
	}
	if positive.Mood().Negated() {
		t.Error("`should` is the negated mood, want the positive one")
	}

	// Which files the rule is about is the scope's answer, whichever mood was taken.
	selected, err := negated.SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}
	if want := selectFiles(t, base); !slices.Equal(selected, want) {
		t.Errorf("`%s` is about %v, want the files its scope is about, %v", negated, selected, want)
	}
	if !slices.Contains(selected, "common/matching/filter.go") {
		t.Errorf("`%s` is about %v, want the files of that folder among them", negated, selected)
	}
	if rendered := negated.String(); !strings.HasSuffix(rendered, ", should not") {
		t.Errorf("String() = %q, want the sentence to end in the mood", rendered)
	}
}

func TestAMoodIsTakenFromAStoredRuleWithoutChangingIt(t *testing.T) {
	// The AGENTS.md example one stage further on: the base rule is stored, one branch takes each mood,
	// and the base is still the scope it was — so a suite can build a rule per mood from one scope.
	t.Cleanup(archunit.ClearGraphCache)

	base := archunit.ProjectFiles(nil).InFolder("common/projection/**")
	cycles := base.InFolder("common/projection/cycles").ShouldNot()

	if rendered := base.String(); strings.Contains(rendered, "should") {
		t.Errorf("the stored scope renders as %q, want a scope with no mood on it", rendered)
	}
	selected, err := cycles.SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}
	if !slices.Contains(selected, "common/projection/cycles/tarjan_scc.go") {
		t.Errorf("`%s` is about %v, want the branch's own folder", cycles, selected)
	}
	if slices.Contains(selected, "common/projection/project_edges.go") {
		t.Errorf("`%s` is about %v, want only the folder the branch narrowed to", cycles, selected)
	}
	if base := selectFiles(t, base); !slices.Contains(base, "common/projection/project_edges.go") {
		t.Errorf("the stored scope is about %v, want it unchanged by the branch that took a mood", base)
	}
}

func TestThisRepositoryHasNoCyclesBetweenItsFiles(t *testing.T) {
	// A whole rule through the public surface, dogfooding on this library: no locator, no scope verb, so
	// every file of this repository is held to it. It is also the rule AGENTS.md asks of the layout — one
	// concept per file, dependencies pointing one way — and a green run here means nothing has to be read
	// in a circle.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).Should().HaveNoCycles()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	for _, violation := range violations {
		if violation.Kind() != archunit.KindFileCycle {
			t.Errorf("%s reports a %s violation, want a cycle", rule, violation.Kind())
			continue
		}
		cycle, ok := violation.(archunit.FileCycleViolation)
		if !ok {
			t.Errorf("%s reports a %T, want a FileCycleViolation", rule, violation)
			continue
		}
		// The readable path the report is for, printed here because a cycle is what has to be broken and
		// the files alone would not say where.
		t.Errorf("%s: the files %v depend on each other in a circle: %s", rule, cycle.Files(), cycle)
	}
}

func TestARuleThatSelectedNothingFailsAsAnEmptyTest(t *testing.T) {
	// The stale glob one stage further on than the scope test above: a terminal is what turns selecting
	// nothing into a failure, because `have no cycles` over no file at all would be green forever.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/renamed/**").Should().HaveNoCycles()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("%s reports %v, want the one empty-test violation", rule, violations)
	}
	if kind := violations[0].Kind(); kind != archunit.KindEmptyTest {
		t.Errorf("the violation is of kind %q, want %q", kind, archunit.KindEmptyTest)
	}
	if _, ok := violations[0].(archunit.EmptyTestViolation); !ok {
		t.Errorf("%s reports a %T, want an EmptyTestViolation", rule, violations[0])
	}
	// And the opt-out for a suite that really means an empty selection.
	allowed, err := rule.Check(&archunit.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("%s failed with AllowEmptyTests: %v", rule, err)
	}
	if len(allowed) != 0 {
		t.Errorf("%s reports %v with AllowEmptyTests, want the pass", rule, allowed)
	}
}

func TestEveryTerminalOfThisLibraryWiresInTheEmptyTestGuard(t *testing.T) {
	// The guard held to the word `every`, across the whole library rather than one module: a file that
	// declares a terminal's Check method has to ask the guard in it. Each module tests its own terminals
	// through the grammar — files/fluentapi does it by walking its method sets — and this is the rule that
	// notices a module nobody wrote such a test for, because it reads the source of every package.
	//
	// It is written with `adhere to` because that is what the predicate needs to be: which methods a file
	// declares is not something a pattern over identifiers can say. A rule that passes today and fails the
	// day a terminal in layers/, slices/, metrics/ or graph/ forgets the guard is the point of it.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).Should().AdhereTo(func(file archunit.FileInfo) bool {
		// A file that declares no terminal has nothing to guard. The method declaration is what is looked
		// for and not the word `Check`, so the interface in common/fluentapi — which declares the contract
		// rather than implementing it — is not asked to satisfy its own requirement.
		if !strings.Contains(file.Source, ") Check(") {
			return true
		}
		return strings.Contains(file.Source, "GatherEmptyTestViolations(")
	}, "ask the empty-test guard, if it declares a terminal's Check method")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	for _, violation := range violations {
		t.Errorf("%s: %s", rule, violation)
	}
}

func TestEveryFileOfThisRepositoryIsNamedTheWayTheLayoutAsksFor(t *testing.T) {
	// The three predicates dogfooded on this library, as rules a user would write: the file names of this
	// repository are lower-case with underscores, the packages of common/ live where AGENTS.md puts them, and
	// a named file is where it is expected to be. Real conventions, so a green run is worth something.
	t.Cleanup(archunit.ClearGraphCache)

	rules := []archunit.FilesNamingCondition{
		// Snake case, the convention the linter also holds this repository to, spelled as a rule here so
		// that the library says it about itself.
		archunit.ProjectFiles(nil).Should().HaveName("[_a-z][_a-z0-9]*.go"),
		// Nothing in the shared kernel sits outside it, whichever folder of it a file is in.
		archunit.ProjectFiles(nil).InFolder("common/**").Should().BeInFolder("common/**"),
		// A rule about one file: the cycle detection lives with the other cycle code.
		archunit.ProjectFiles(nil).WithName("tarjan_scc.go").Should().BeInPath("common/projection/cycles/*.go"),
		// And the negations of the same three, which is the half of the API most rules are written in.
		archunit.ProjectFiles(nil).ShouldNot().HaveName("*.java"),
		archunit.ProjectFiles(nil).InFolder("common/**").ShouldNot().BeInFolder("files/**"),
		archunit.ProjectFiles(nil).ShouldNot().BeInPath("**/legacy/**"),
	}

	for _, rule := range rules {
		t.Run(rule.String(), func(t *testing.T) {
			violations, err := rule.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", rule, err)
			}
			for _, violation := range violations {
				t.Errorf("%s: %s", rule, violation)
			}
		})
	}
}

func TestANamingRuleThisRepositoryBreaksReportsTheOffendingFiles(t *testing.T) {
	// The failing half, because a rule that cannot fail says nothing: one folder of this repository held to
	// the name of one of its own files. The violation carries the file, the requirement and the mood, which
	// is what a report needs in order to phrase the failure itself.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/matching").Should().HaveName("regex_factory.go")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != archunit.KindFileNaming {
			t.Errorf("%s reports a %s violation, want a naming one", rule, kind)
			continue
		}
		naming, ok := violation.(archunit.FileNamingViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a FileNamingViolation", rule, violation)
		}
		if naming.Mood != archunit.Should {
			t.Errorf("the violation of %s was judged in mood %s, want %s", naming.File, naming.Mood, archunit.Should)
		}
		if source := naming.Required.Pattern().Source(); source != "regex_factory.go" {
			t.Errorf("the violation quotes %q, want the pattern the rule was written with", source)
		}
		offenders = append(offenders, naming.File)
	}

	want := []string{"common/matching/filter.go", "common/matching/match_target.go"}
	if !slices.Equal(offenders, want) {
		t.Errorf("%s reports %v, want the folder's other files, %v", rule, offenders, want)
	}
}

func TestTheTwoMoodsOfANamingRuleAreComplementaryOnThePublicSurface(t *testing.T) {
	// One scope, one pattern, both moods: every selected file offends exactly one of the two rules, which is
	// what it means for the negation to be a flag rather than a second implementation.
	t.Cleanup(archunit.ClearGraphCache)

	base := archunit.ProjectFiles(nil).InFolder("common/**")
	required := base.Should().BeInFolder("common/matching")
	forbidden := base.ShouldNot().BeInFolder("common/matching")

	elsewhere, err := required.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", required, err)
	}
	inside, err := forbidden.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", forbidden, err)
	}

	selected := selectFiles(t, base)
	if len(elsewhere)+len(inside) != len(selected) {
		t.Errorf("%s reports %d files and %s reports %d, want the %d selected files split between them",
			required, len(elsewhere), forbidden, len(inside), len(selected))
	}
	if len(inside) == 0 || len(elsewhere) == 0 {
		t.Errorf("%s reports %d and %s reports %d, want both moods to have something to say about this repository",
			required, len(elsewhere), forbidden, len(inside))
	}
}

func TestThisRepositoryObeysItsOwnDependencyRules(t *testing.T) {
	// The four dependency rules AGENTS.md states about this library, written in the library's own words and
	// held against the library itself. This is the feature the whole project exists for, so a green run here
	// is the one that means the most: the layering these rules describe is real, and any commit that broke it
	// would fail this test rather than a reviewer's memory.
	t.Cleanup(archunit.ClearGraphCache)

	rules := []archunit.FilesDependencyCondition{
		// The shared kernel is shared, so it cannot know about a domain module — which would make the module
		// impossible to remove and the kernel impossible to reuse.
		archunit.ProjectFiles(nil).InFolder("common/**").ShouldNot().DependOnFiles().InFolder("files/**"),
		// The kernel does not know about the report layer either, for the same reason and the other way round:
		// a violation carries data, and the words for it are added afterwards by something the kernel cannot see.
		archunit.ProjectFiles(nil).InFolder("common/**").ShouldNot().DependOnFiles().InFolder("archtest"),
		// And the report layer reads what a rule reported and nothing else: a dependency on a module's fluent
		// API or on its projection would mean it was phrasing something other than a violation.
		archunit.ProjectFiles(nil).InFolder("archtest").ShouldNot().DependOnFiles().InFolder("files/fluentapi"),
		archunit.ProjectFiles(nil).InFolder("archtest").ShouldNot().DependOnFiles().InFolder("files/projection"),
		// Nothing inside the library depends on the public surface, because that package is re-exports and a
		// dependency on it would be a cycle through the root of the module.
		archunit.ProjectFiles(nil).ShouldNot().DependOnFiles().WithName("archunit.go"),
		// The pure halves of a domain module are pure: they take data and return data, so they cannot reach
		// back into the fluent API that calls them.
		archunit.ProjectFiles(nil).InFolder("files/assertion").ShouldNot().DependOnFiles().InFolder("files/fluentapi"),
		archunit.ProjectFiles(nil).InFolder("files/projection").ShouldNot().DependOnFiles().InFolder("files/fluentapi"),
		// And the assertion half of a module and its projection half do not know about each other either: the
		// fluent API is what puts a projection's answer into an assertion.
		archunit.ProjectFiles(nil).InFolder("files/assertion").ShouldNot().DependOnFiles().InFolder("files/projection"),
	}

	for _, rule := range rules {
		t.Run(rule.String(), func(t *testing.T) {
			violations, err := rule.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", rule, err)
			}
			for _, violation := range violations {
				t.Errorf("%s: %s", rule, violation)
			}
		})
	}
}

func TestADependencyRuleThisRepositoryBreaksReportsTheOffendingFiles(t *testing.T) {
	// The failing half, because a rule that cannot fail says nothing: the fluent API of the files module does
	// depend on the shared pattern-matching package, and forbidding that reports the files that do it, each
	// carrying the dependency it was broken by. That data is what a report phrases a failure from.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("files/fluentapi").ShouldNot().DependOnFiles().InFolder("common/matching")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != archunit.KindFileDependency {
			t.Errorf("%s reports a %s violation, want a dependency one", rule, kind)
			continue
		}
		dependency, ok := violation.(archunit.FileDependencyViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a FileDependencyViolation", rule, violation)
		}
		if dependency.Mood != archunit.ShouldNot {
			t.Errorf("the violation of %s was judged in mood %s, want %s", dependency.File, dependency.Mood, archunit.ShouldNot)
		}
		if len(dependency.Required) != 1 || dependency.Required[0].Pattern().Source() != "common/matching" {
			t.Errorf("the violation of %s quotes %v, want the object the rule was written with", dependency.File, dependency.Required)
		}
		if len(dependency.Dependencies) == 0 {
			t.Errorf("the violation of %s carries no dependency, want the ones it was broken by", dependency.File)
		}
		for _, found := range dependency.Dependencies {
			if !strings.HasPrefix(found, "common/matching/") {
				t.Errorf("the violation of %s carries %q, want only files the object named", dependency.File, found)
			}
		}
		offenders = append(offenders, dependency.File)
	}

	if !slices.Contains(offenders, "files/fluentapi/project_files.go") {
		t.Errorf("%s reports %v, want the file that compiles the scope's patterns among them", rule, offenders)
	}
	if !slices.Contains(offenders, "files/fluentapi/depend_on_files.go") {
		t.Errorf("%s reports %v, want the file that compiles the object's patterns among them", rule, offenders)
	}
}

func TestThisRepositoryObeysItsOwnThirdPartyDependencyPolicy(t *testing.T) {
	// The third-party policy this library was built to keep, written in the library's own words and held
	// against the library itself: one dependency outside the standard library, reached from one package. Every
	// rule here names third-party modules with `*.*/**` — a first segment with a dot in it is a domain — so the
	// standard library, which is external too, is deliberately outside what they forbid.
	t.Cleanup(archunit.ClearGraphCache)

	rules := []archunit.FilesExternalDependencyCondition{
		// The one third-party module this repository has is reached from the one package whose job is to know
		// what Go source means. Every other package is stdlib-only, so a new dependency anywhere else fails
		// here rather than in a reviewer's reading of go.mod.
		archunit.ProjectFiles(nil).InFolder("files/**").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("layers/**").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("slices/**").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("metrics/**").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("graph/**").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("archtest").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("common/assertion").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("common/projection/**").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("common/fluentapi").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("common/matching").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		archunit.ProjectFiles(nil).InFolder("common/archerror").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		// And the positive mood of the same predicate over the one file that is allowed to know the loader:
		// extraction is the only layer that knows Go, so this is where the dependency belongs.
		archunit.ProjectFiles(nil).InFile("common/extraction/extract_graph.go").Should().
			DependOnExternalModules().Matching("golang.org/x/tools/**"),
	}

	for _, rule := range rules {
		t.Run(rule.String(), func(t *testing.T) {
			violations, err := rule.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", rule, err)
			}
			for _, violation := range violations {
				t.Errorf("%s: %s", rule, violation)
			}
		})
	}
}

func TestAThirdPartyRuleThisRepositoryBreaksReportsTheOffendingFiles(t *testing.T) {
	// The failing half, because a rule that cannot fail says nothing: this repository does depend on
	// golang.org/x/tools, from the extractor, and forbidding that reports the file that does it — carrying the
	// import path as the file wrote it, which is the package rather than the module it was published as. That
	// data is what a report phrases a failure from.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/extraction").ShouldNot().
		DependOnExternalModules().Matching("golang.org/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != archunit.KindFileExternalDependency {
			t.Errorf("%s reports a %s violation, want an external dependency one", rule, kind)
			continue
		}
		dependency, ok := violation.(archunit.FileExternalDependencyViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a FileExternalDependencyViolation", rule, violation)
		}
		if dependency.Mood != archunit.ShouldNot {
			t.Errorf("the violation of %s was judged in mood %s, want %s", dependency.File, dependency.Mood, archunit.ShouldNot)
		}
		if len(dependency.Required) != 1 || dependency.Required[0].Pattern().Source() != "golang.org/**" {
			t.Errorf("the violation of %s quotes %v, want the object the rule was written with", dependency.File, dependency.Required)
		}
		if !slices.Contains(dependency.Modules, "golang.org/x/tools/go/packages") {
			t.Errorf("the violation of %s carries %v, want the package it imports rather than the module",
				dependency.File, dependency.Modules)
		}
		offenders = append(offenders, dependency.File)
	}

	if !slices.Equal(offenders, []string{"common/extraction/extract_graph.go"}) {
		t.Errorf("%s reports %v, want the one file that loads a Go project", rule, offenders)
	}
}

func TestThisRepositoryAdheresToTheConventionsNoGlobExpresses(t *testing.T) {
	// `adhere to` dogfooded on this library: three conventions about the contents of a file, which is exactly
	// what no pattern over identifiers can say. The predicate is a Go function, so these read as the rules a
	// user would write for their own project — and the message beside each one is what a failure would say.
	t.Cleanup(archunit.ClearGraphCache)

	rules := []archunit.FilesAdherenceCondition{
		// Every file is gofmt'ed, and the part of that a rule can see from the text is the final newline.
		archunit.ProjectFiles(nil).Should().AdhereTo(func(file archunit.FileInfo) bool {
			return strings.HasSuffix(file.Source, "\n")
		}, "end with a newline"),
		// Every file of this repository is Go source with something in it.
		archunit.ProjectFiles(nil).Should().AdhereTo(isGoFile, "be a Go file with something in it"),
		// AGENTS.md: the assertion half of a module is pure, so it does not reach for the filesystem. The
		// linter holds this repository to it through depguard; here the library says it about itself.
		archunit.ProjectFiles(nil).InFolder("files/assertion").ShouldNot().AdhereTo(func(file archunit.FileInfo) bool {
			return strings.Contains(file.Source, `"os"`) || strings.Contains(file.Source, `"path/filepath"`)
		}, "import the filesystem"),
	}

	for _, rule := range rules {
		t.Run(rule.String(), func(t *testing.T) {
			violations, err := rule.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", rule, err)
			}
			for _, violation := range violations {
				t.Errorf("%s: %s", rule, violation)
			}
		})
	}
}

func TestAnAdherenceRuleThisRepositoryBreaksReportsTheOffendingFiles(t *testing.T) {
	// The failing half, because a rule that cannot fail says nothing: a size limit this repository does not
	// keep, over one folder of it. The violation carries the file, the words the rule was written with and the
	// mood — all a report can have, because the rule itself was a function.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("files/assertion").Should().AdhereTo(func(file archunit.FileInfo) bool {
		return file.NonBlankLineCount <= 20
	}, "be at most 20 lines long")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(violations) == 0 {
		t.Fatalf("%s reports nothing, want the files of that folder that are longer than that", rule)
	}
	for _, violation := range violations {
		if kind := violation.Kind(); kind != archunit.KindFileAdherence {
			t.Errorf("%s reports a %s violation, want an adherence one", rule, kind)
			continue
		}
		adherence, ok := violation.(archunit.FileAdherenceViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a FileAdherenceViolation", rule, violation)
		}
		if adherence.Requirement != "be at most 20 lines long" {
			t.Errorf("the violation of %s says %q, want the message the rule was written with", adherence.File, adherence.Requirement)
		}
		if adherence.Mood != archunit.Should {
			t.Errorf("the violation of %s was judged in mood %s, want %s", adherence.File, adherence.Mood, archunit.Should)
		}
		if !strings.HasPrefix(adherence.File, "files/assertion/") {
			t.Errorf("%s reports %q, want only files the scope selected", rule, adherence.File)
		}
	}
}

// theLayersOfThisRepository is the layout AGENTS.md describes, declared as a named-layer policy declares it:
// the shared kernel, the five domain modules written so far and the report layer, one folder of this repository
// each. It is a function rather than seven repeated stages because that is the point of the declaration stage
// being a value — a project's layers are typed once and every policy below branches off them.
func theLayersOfThisRepository() archunit.LayersBuilder {
	return archunit.ProjectLayers(nil).
		Layer("kernel").DefinedByFolder("common/**").
		Layer("files").DefinedByFolder("files/**").
		Layer("layers").DefinedByFolder("layers/**").
		Layer("slices").DefinedByFolder("slices/**").
		Layer("metrics").DefinedByFolder("metrics/**").
		Layer("graph").DefinedByFolder("graph/**").
		Layer("report").DefinedByFolder("archtest/**")
}

func TestProjectLayersSelectsTheFilesOfEachLayerOfThisRepository(t *testing.T) {
	// The declaration half of a policy through the public surface, dogfooding on this library: no locator, so
	// the project is the one this test is in, and every layer is a folder of it. This is what a user reaches
	// for to see what a policy is talking about before asking whether it holds.
	t.Cleanup(archunit.ClearGraphCache)

	policy := theLayersOfThisRepository()

	membership, err := policy.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("SelectLayerFiles failed: %v", err)
	}

	if len(membership) != 7 {
		t.Errorf("%s came to %d layers, want one key per declared layer", policy, len(membership))
	}
	for _, wanted := range []struct {
		layer string
		file  string
	}{
		{layer: "kernel", file: "common/matching/filter.go"},
		{layer: "files", file: "files/fluentapi/project_files.go"},
		{layer: "layers", file: "layers/fluentapi/project_layers.go"},
		{layer: "slices", file: "slices/fluentapi/project_slices.go"},
		{layer: "metrics", file: "metrics/fluentapi/metrics.go"},
		{layer: "graph", file: "graph/fluentapi/project_graph.go"},
		{layer: "report", file: "archtest/violation_factory.go"},
	} {
		if !slices.Contains(membership[wanted.layer], wanted.file) {
			t.Errorf("the layer %q came to %v, want %q among them", wanted.layer, membership[wanted.layer], wanted.file)
		}
	}
	// The public surface is in no declared layer, deliberately: a file an edge ends in no layer at is ignored
	// by every clause, which is what lets a policy describe part of a project instead of all of it.
	for layer, files := range membership {
		if slices.Contains(files, "archunit.go") {
			t.Errorf("the layer %q came to %q, want a file no pattern describes left out of every layer", layer, "archunit.go")
		}
	}
}

func TestLayersIsTheShortAliasOfProjectLayersOnThePublicSurface(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	verbose, err := archunit.ProjectLayers(nil).Layer("kernel").DefinedByFolder("common/matching").SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("SelectLayerFiles failed: %v", err)
	}
	short, err := archunit.Layers(nil).Layer("kernel").DefinedByFolder("common/matching").SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("SelectLayerFiles failed: %v", err)
	}

	if !slices.Contains(verbose["kernel"], "common/matching/filter.go") {
		t.Errorf("`project layers, layer defined by folder` came to %v, want the files of that folder", verbose["kernel"])
	}
	if !slices.Equal(short["kernel"], verbose["kernel"]) {
		t.Errorf("`layers, ...` came to %v, want what `project layers, ...` came to, %v", short["kernel"], verbose["kernel"])
	}
}

func TestTheLocatorReachesTheProjectThroughEitherLayersEntryPoint(t *testing.T) {
	// Both wrappers thread the locator through to the extraction, so a policy pointed at a directory that holds
	// no Go project says so — rather than quietly analyzing the repository this test runs in, which is what
	// dropping the argument would look like, and which no comparison of two nil-located policies would notice.
	t.Cleanup(archunit.ClearGraphCache)

	entryPoints := []struct {
		name   string
		policy archunit.LayersBuilder
	}{
		{
			name: "project layers",
			policy: archunit.ProjectLayers(&archunit.ProjectLocator{Directory: t.TempDir()}).
				Layer("kernel").DefinedByFolder("common/**"),
		},
		{
			name: "layers",
			policy: archunit.Layers(&archunit.ProjectLocator{Directory: t.TempDir()}).
				Layer("kernel").DefinedByFolder("common/**"),
		},
	}

	for _, entry := range entryPoints {
		t.Run(entry.name, func(t *testing.T) {
			membership, err := entry.policy.SelectLayerFiles(nil)

			if err == nil {
				t.Errorf("`%s` came to %v against a directory that is no project, want an error naming it", entry.name, membership)
			}
			if len(membership) != 0 {
				t.Errorf("`%s` came to %v, want nothing when the project cannot be located", entry.name, membership)
			}
		})
	}
}

func TestALayerIsDefinedByAFolderOrByAWholePath(t *testing.T) {
	// The two ways a layer is described, through the public surface: `defined by folder` is the one almost every
	// policy wants, and `defined by` takes the whole path — which is how a layer of a single file, or one named
	// by the file names rather than by the folder, is declared.
	t.Cleanup(archunit.ClearGraphCache)

	byFolder, err := archunit.ProjectLayers(nil).
		Layer("cycles").DefinedByFolder("common/projection/cycles").SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("SelectLayerFiles failed: %v", err)
	}
	byPath, err := archunit.ProjectLayers(nil).
		Layer("cycles").DefinedBy("common/projection/cycles/*.go").SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("SelectLayerFiles failed: %v", err)
	}
	oneFile, err := archunit.ProjectLayers(nil).
		Layer("tarjan").DefinedBy("common/projection/cycles/tarjan_scc.go").SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("SelectLayerFiles failed: %v", err)
	}

	if !slices.Contains(byFolder["cycles"], "common/projection/cycles/tarjan_scc.go") {
		t.Errorf("the layer defined by folder came to %v, want the files of that folder", byFolder["cycles"])
	}
	if !slices.Equal(byPath["cycles"], byFolder["cycles"]) {
		t.Errorf("the layer defined by path came to %v, want the same files as the folder, %v", byPath["cycles"], byFolder["cycles"])
	}
	if want := []string{"common/projection/cycles/tarjan_scc.go"}; !slices.Equal(oneFile["tarjan"], want) {
		t.Errorf("the layer defined by one path came to %v, want %v", oneFile["tarjan"], want)
	}
}

func TestThisRepositoryObeysItsOwnLayerPolicy(t *testing.T) {
	// The four dependency rules of AGENTS.md again — the ones written as pairwise file rules above — this time as
	// the one policy they actually are. That is the whole reason this module exists: the same statement is four
	// clauses here and a rule per ordered pair of layers there, and it reads as the architecture rather than as a
	// list of globs. A green run means the layering is real, and the sealed kernel is the clause that keeps it so.
	t.Cleanup(archunit.ClearGraphCache)

	policy := theLayersOfThisRepository().
		// The shared kernel is shared, so it knows about no module and no report layer: sealed.
		WhereLayer("kernel").MayOnlyDependOnLayers().
		// A domain module knows the kernel and nothing else — not the report layer, and not a sibling module,
		// which is what makes a module removable.
		WhereLayer("files").MayOnlyDependOnLayers("kernel").
		WhereLayer("layers").MayOnlyDependOnLayers("kernel").
		WhereLayer("slices").MayOnlyDependOnLayers("kernel").
		WhereLayer("metrics").MayOnlyDependOnLayers("kernel").
		WhereLayer("graph").MayOnlyDependOnLayers("kernel").
		// And the report layer reads what a rule reported: the kernel and the three modules that report violations,
		// whose pure assertion halves are the only part of them it is allowed to reach — which the file rules above
		// say the rest of. The graph and metrics modules report none, so they are left out and this clause forbids
		// the dependency.
		WhereLayer("report").MayOnlyDependOnLayers("kernel", "files", "layers", "slices").
		// The same thing the other way round, as the blocklist a team tightening one edge would write.
		WhereLayer("files").MayNotDependOnLayers("layers")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", policy, err)
	}
	for _, violation := range violations {
		t.Errorf("%s: %s", policy, archunit.NewViolationFactory(nil).Message(violation))
	}
}

func TestALayerPolicyThisRepositoryBreaksReportsTheOffendingLayers(t *testing.T) {
	// The failing half, because a rule that cannot fail says nothing: the report layer does depend on a module,
	// and the files module does depend on the kernel. One violation per offending pair of layers rather than one
	// per import, each carrying the two layers, the clause it broke and the concrete file dependencies — which is
	// what makes a layer report short and still actionable.
	t.Cleanup(archunit.ClearGraphCache)

	policy := theLayersOfThisRepository().
		WhereLayer("report").MayNotDependOnLayers("files").
		WhereLayer("files").MayOnlyDependOnLayers()

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", policy, err)
	}

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != archunit.KindLayerDependency {
			t.Errorf("%s reports a %s violation, want a layer dependency one", policy, kind)
			continue
		}
		dependency, ok := violation.(archunit.LayerDependencyViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a LayerDependencyViolation", policy, violation)
		}
		if len(dependency.Dependencies) == 0 {
			t.Errorf("the violation of %q carries no file dependency, want the ones the layers are connected by",
				dependency.Layer)
		}
		offenders = append(offenders, dependency.Layer+" -> "+dependency.DependsOn)
	}

	want := []string{"files -> kernel", "report -> files"}
	if !slices.Equal(offenders, want) {
		t.Fatalf("%s reports %v, want %v", policy, offenders, want)
	}

	// The blocklist violation in full, because the two clauses are judged differently and the data is what a
	// report is phrased from: the layers the clause named, the mood it was written in and the files.
	blocked, ok := violations[1].(archunit.LayerDependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a LayerDependencyViolation", policy, violations[1])
	}
	if blocked.Mood != archunit.ShouldNot || !slices.Equal(blocked.Named, []string{"files"}) {
		t.Errorf("the violation blames `%s %v`, want the blocklist that forbade the pair", blocked.Mood, blocked.Named)
	}
	for _, edge := range blocked.Dependencies {
		if !strings.HasPrefix(edge.Source, "archtest/") || !strings.HasPrefix(edge.Target, "files/") {
			t.Errorf("the violation carries %s -> %s, want a dependency between the two layers it is about",
				edge.Source, edge.Target)
		}
	}
	// And the sealed layer, whose clause named nothing at all.
	sealed, ok := violations[0].(archunit.LayerDependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a LayerDependencyViolation", policy, violations[0])
	}
	if sealed.Mood != archunit.Should || len(sealed.Named) != 0 {
		t.Errorf("the violation blames `%s %v`, want the sealed layer's clause", sealed.Mood, sealed.Named)
	}

	// And the report a test failure would print, through the same factory every other rule's violations go to.
	message := archunit.NewViolationFactory(nil).Message(blocked)
	if !strings.HasPrefix(message, `layer "report": may not depend on layers "files"; it depends on files through archtest/`) {
		t.Errorf("the violation reads %q, want the pair of layers first and the files after it", message)
	}
	if sealed := archunit.NewViolationFactory(nil).Message(sealed); !strings.HasPrefix(sealed,
		`layer "files": may only depend on no layers; it depends on kernel through files/`) {
		t.Errorf("the sealed layer's violation reads %q, want its clause read as `no layers`", sealed)
	}
}

func TestALayerNoFileIsInFailsAsAnEmptyTestThroughThePublicSurface(t *testing.T) {
	// The empty-test guard on this family's terminal, through the public surface: a policy has one population per
	// declared layer, because a layer nobody is in makes every clause about it vacuous and the whole policy green
	// forever. The guard names the layer, so a reader knows which of a policy's patterns went stale.
	t.Cleanup(archunit.ClearGraphCache)

	// The layer nobody is in is `common/util`, because AGENTS.md says in as many words that there is no such
	// folder and there will not be one — so this test keeps failing for the reason it was written for, rather
	// than going green the day the folder it named got filled in.
	policy := archunit.ProjectLayers(nil).
		Layer("kernel").DefinedByFolder("common/**").
		Layer("util").DefinedByFolder("common/util/**").
		WhereLayer("util").MayOnlyDependOnLayers("kernel")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", policy, err)
	}

	if len(violations) != 1 {
		t.Fatalf("%s reports %v, want the one empty layer it has", policy, violations)
	}
	if kind := violations[0].Kind(); kind != archunit.KindEmptyTest {
		t.Errorf("the violation is of kind %q, want %q", kind, archunit.KindEmptyTest)
	}
	empty, ok := violations[0].(archunit.EmptyTestViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an EmptyTestViolation", policy, violations[0])
	}
	if empty.Subject != `files in layer "util"` {
		t.Errorf("the guard reports %q, want the layer nobody is in named", empty.Subject)
	}
	// And the opt-out, which is the same knob on the same bag every other terminal threads into the guard.
	allowed, err := policy.Check(&archunit.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("%s failed with AllowEmptyTests: %v", policy, err)
	}
	if len(allowed) != 0 {
		t.Errorf("%s reports %v with AllowEmptyTests, want the pass", policy, allowed)
	}
}

func TestProjectSlicesCutsThisRepositoryIntoItsModulesThroughThePublicSurface(t *testing.T) {
	// The slicing half of a rule through the public surface, dogfooding on this library: no locator, so the
	// project is the one this test is in, and the capture is the top-level folder every file of it lives in.
	// Nothing declares the slices — this repository's own directories are the answer.
	t.Cleanup(archunit.ClearGraphCache)

	slicing := archunit.ProjectSlices(nil).DefinedBy("(*)/**")

	membership, err := slicing.SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("SelectSliceFiles failed: %v", err)
	}

	for _, wanted := range []struct {
		slice string
		file  string
	}{
		{slice: "common", file: "common/matching/filter.go"},
		{slice: "files", file: "files/fluentapi/project_files.go"},
		{slice: "layers", file: "layers/fluentapi/project_layers.go"},
		{slice: "slices", file: "slices/fluentapi/project_slices.go"},
		{slice: "metrics", file: "metrics/fluentapi/metrics.go"},
		{slice: "graph", file: "graph/fluentapi/project_graph.go"},
		{slice: "archtest", file: "archtest/violation_factory.go"},
	} {
		if !slices.Contains(membership[wanted.slice], wanted.file) {
			t.Errorf("the slice %q came to %v, want %q among them", wanted.slice, membership[wanted.slice], wanted.file)
		}
	}
	// The public surface is its own slice, because `(*)/**` reads a file at the root of the project as the whole
	// of its own name. A slicing describes what it describes; there is no list of slices to leave it out of.
	if want := []string{"archunit.go"}; !slices.Equal(membership["archunit.go"], want) {
		t.Errorf("the slice %q came to %v, want %v", "archunit.go", membership["archunit.go"], want)
	}
}

func TestTheLocatorReachesTheProjectThroughTheSlicesEntryPoint(t *testing.T) {
	// The wrapper threads the locator through to the extraction, so a slicing pointed at a directory that holds no
	// Go project says so — rather than quietly cutting up the repository this test runs in, which is what dropping
	// the argument would look like, and which no comparison of two nil-located slicings would notice.
	t.Cleanup(archunit.ClearGraphCache)

	slicing := archunit.ProjectSlices(&archunit.ProjectLocator{Directory: t.TempDir()}).DefinedBy("(*)/**")

	membership, err := slicing.SelectSliceFiles(nil)

	if err == nil {
		t.Errorf("`project slices` came to %v against a directory that is no project, want an error naming it", membership)
	}
	if len(membership) != 0 {
		t.Errorf("`project slices` came to %v, want nothing when the project cannot be located", membership)
	}
}

func TestThisRepositoryObeysItsOwnSliceRule(t *testing.T) {
	// AGENTS.md rule 1 as a rule about slices rather than about layers: the kernel is written against the
	// standard library and the analysis toolchain, so no file of common/ imports a domain module. The two moods
	// of the same sentence, on the same slicing.
	t.Cleanup(archunit.ClearGraphCache)

	slicing := archunit.ProjectSlices(nil).DefinedBy("(*)/**")
	for _, rule := range []archunit.Checkable{
		slicing.ShouldNot().ContainDependency("common", "files"),
		slicing.ShouldNot().ContainDependency("common", "layers"),
		slicing.ShouldNot().ContainDependency("common", "slices"),
		slicing.Should().ContainDependency("files", "common"),
	} {
		violations, err := rule.Check(nil)
		if err != nil {
			t.Fatalf("%s failed: %v", rule, err)
		}
		if len(violations) != 0 {
			t.Errorf("%s reports %v, want the pass", rule, violations)
		}
	}
}

func TestASliceRuleThisRepositoryBreaksReportsTheFilesThatBrokeIt(t *testing.T) {
	// The converse of the rule above, which this repository breaks on purpose: a domain module is written
	// against the kernel, so `files` does depend on `common`. What comes back is the pair of slices and the
	// imports that connect them, and the message a failing test would print.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectSlices(nil).DefinedBy("(*)/**").ShouldNot().ContainDependency("files", "common")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("%s reports %v, want the one dependency it forbids", rule, violations)
	}
	if kind := violations[0].Kind(); kind != archunit.KindSliceDependency {
		t.Errorf("the violation is of kind %q, want %q", kind, archunit.KindSliceDependency)
	}
	broken, ok := violations[0].(archunit.SliceDependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a SliceDependencyViolation", rule, violations[0])
	}
	if broken.Slice != "files" || broken.DependsOn != "common" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", broken.Slice, broken.DependsOn, "files", "common")
	}
	if broken.Mood != archunit.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want %s", broken.Mood, archunit.ShouldNot)
	}
	if len(broken.Dependencies) == 0 {
		t.Errorf("the violation carries no files, want the imports that connect the two slices")
	}

	// And the report a test failure would print, through the same factory every other rule's violations go to.
	message := archunit.NewViolationFactory(nil).Message(broken)
	if !strings.HasPrefix(message, `slice "files": should not, contain dependency "common"; it depends on common through files/`) {
		t.Errorf("the violation reads %q, want the pair of slices first and the files after it", message)
	}
}

func TestASliceNobodyIsInFailsAsAnEmptyTestThroughThePublicSurface(t *testing.T) {
	// The empty-test guard on this family's terminal, through the public surface: a rule about a slice the
	// slicing did not produce is vacuous in both moods, so it is a violation rather than a pass, and the guard
	// names the slice so a reader knows which half of the sentence went stale.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectSlices(nil).DefinedBy("(*)/**").ShouldNot().ContainDependency("files", "util")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("%s reports %v, want the one slice nobody is in", rule, violations)
	}
	empty, ok := violations[0].(archunit.EmptyTestViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an EmptyTestViolation", rule, violations[0])
	}
	if empty.Subject != `files in slice "util"` {
		t.Errorf("the guard reports %q, want the slice nobody is in named", empty.Subject)
	}
	// And the opt-out, which is the same knob on the same bag every other terminal threads into the guard.
	allowed, err := rule.Check(&archunit.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("%s failed with AllowEmptyTests: %v", rule, err)
	}
	if len(allowed) != 0 {
		t.Errorf("%s reports %v with AllowEmptyTests, want the pass", rule, allowed)
	}
}

func TestTheSlicingsThemselvesAreOnThePublicSurfaceForDirectUse(t *testing.T) {
	// The four projections `defined by` is written over, for a caller who wants the mapper rather than a rule —
	// a projection of their own over common/projection, or a report in the vocabulary of slices. Each of them
	// is run over one hand-built edge here, because what the surface owes a caller is the projection the doc
	// comment names and not merely a function: the four differ only in what they label and what they drop, so
	// a re-export wired to the wrong one type-checks. The two that take a pattern also reject one that does
	// not name exactly one slice.
	byPattern, err := archunit.SliceByPattern("internal/(**)/**")
	if err != nil {
		t.Fatalf("SliceByPattern failed: %v", err)
	}
	byRegex, err := archunit.SliceByRegex(`internal/([^/]+)/.*`)
	if err != nil {
		t.Fatalf("SliceByRegex failed: %v", err)
	}

	// Both pattern syntaxes slice by the folder under internal, so the dependency between two of the project's
	// files is the dependency between the two slices they live in.
	dependency := extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain)
	for name, mapper := range map[string]archunit.MapFunction{"SliceByPattern": byPattern, "SliceByRegex": byRegex} {
		mapped, sliced := mapper(dependency)
		if !sliced || mapped.SourceLabel != "api" || mapped.TargetLabel != "db" {
			t.Errorf("%s maps %v to %+v (sliced %t), want the dependency of slice \"api\" on slice \"db\"",
				name, dependency, mapped, sliced)
		}
	}

	// The suffix slicing names a file by what kind of file it is, and it keeps the self-edge that says a file
	// exists at all — which is what the membership of a slice can be read off.
	handler := extraction.SelfEdge("internal/api/order_handler.go")
	if mapped, sliced := archunit.SliceByFileSuffix()(handler); !sliced || mapped.SourceLabel != "handler" ||
		mapped.TargetLabel != "handler" {
		t.Errorf("SliceByFileSuffix maps %v to %+v (sliced %t), want both ends in slice \"handler\"",
			handler, mapped, sliced)
	}

	// Identity relabels nothing and drops nothing: the self-edge and the dependency that leaves the project
	// both come back under the identifiers they already carried.
	external := extraction.NewEdge("internal/api/handler.go", "gorm.io/gorm", true, extraction.ImportKindPlain)
	for _, edge := range []extraction.Edge{handler, external} {
		mapped, kept := archunit.Identity()(edge)
		if !kept || mapped.SourceLabel != edge.Source || mapped.TargetLabel != edge.Target {
			t.Errorf("Identity maps %v to %+v (kept %t), want the edge under its own identifiers", edge, mapped, kept)
		}
	}

	if _, err := archunit.SliceByPattern("internal/**"); err == nil {
		t.Error("SliceByPattern accepted a glob that captures nothing, want the pattern refused")
	}
	if _, err := archunit.SliceByRegex(`(\w+)/(\w+)/.*`); err == nil {
		t.Error("SliceByRegex accepted an expression with two captures, want the pattern refused")
	}
}

func TestMetricsCountsTheFilesOfThisRepositoryThroughThePublicSurface(t *testing.T) {
	// The metrics family end to end through the public surface, dogfooding on this library: no locator, so the
	// project is the one this test is in, the scope is a folder of it, and what comes back is one number per
	// file of that folder. This is what a user reaches for to see the numbers before writing a rule about them.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.Metrics(nil).InFolder("common/matching").WithName("*.go")

	measurements, err := rule.Count().LinesOfCode().Measure(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	subjects := make([]string, 0, len(measurements))
	for _, measurement := range measurements {
		if measurement.Metric != "lines of code" {
			t.Errorf("%s reports %q, want the metric it was asked for", rule, measurement.Metric)
		}
		if measurement.Value <= 0 {
			t.Errorf("%s came to %s, want a file of this repository to carry code", rule, measurement)
		}
		if !strings.HasPrefix(measurement.Subject, "common/matching/") {
			t.Errorf("%s measures %q, want only files the scope selected", rule, measurement.Subject)
		}
		if strings.HasSuffix(measurement.Subject, "_test.go") {
			t.Errorf("%s measures %q, want the test files left out by default", rule, measurement.Subject)
		}
		subjects = append(subjects, measurement.Subject)
	}
	if !slices.Contains(subjects, "common/matching/filter.go") {
		t.Errorf("%s measures %v, want the files of that folder among them", rule, subjects)
	}
	// The same knob every other terminal threads through, on the read a metric is taken of: with the test files
	// in, the siblings are subjects too.
	withTests, err := rule.Count().LinesOfCode().Measure(&archunit.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("%s failed with IncludeTestFiles: %v", rule, err)
	}
	if len(withTests) <= len(measurements) {
		t.Errorf("%s measures %d files with IncludeTestFiles and %d without, want the test files among them",
			rule, len(withTests), len(measurements))
	}
}

func TestEveryCountOfThisLibraryReadsItsOwnPopulationThroughThePublicSurface(t *testing.T) {
	// The eight numbers the family names, each taken of one folder of this repository through the public
	// surface: the six about a file are reported per file identifier and the two about a class per class
	// identifier, which is what `for classes matching` selects and what a rule about a class would name.
	//
	// What is asserted is the subject and the metric rather than the number, because the counts of this
	// library's own source change with every commit — the numbers themselves are pinned in the metrics
	// module's tests, against fixtures a commit cannot move.
	t.Cleanup(archunit.ClearGraphCache)

	group := archunit.Metrics(nil).InFolder("common/matching").Count()
	tests := []struct {
		metric  string
		rule    archunit.MetricBuilder
		subject string
	}{
		{metric: "lines of code", rule: group.LinesOfCode(), subject: "common/matching/filter.go"},
		{metric: "statements", rule: group.Statements(), subject: "common/matching/filter.go"},
		{metric: "imports", rule: group.Imports(), subject: "common/matching/filter.go"},
		{metric: "functions", rule: group.Functions(), subject: "common/matching/filter.go"},
		{metric: "classes", rule: group.Classes(), subject: "common/matching/filter.go"},
		{metric: "interfaces", rule: group.Interfaces(), subject: "common/matching/filter.go"},
		{metric: "method count", rule: group.MethodCount(), subject: "common/matching.Filter"},
		{metric: "field count", rule: group.FieldCount(), subject: "common/matching.Filter"},
	}

	for _, test := range tests {
		t.Run(test.metric, func(t *testing.T) {
			measurements, err := test.rule.Measure(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", test.rule, err)
			}

			subjects := make([]string, 0, len(measurements))
			for _, measurement := range measurements {
				if measurement.Metric != test.metric {
					t.Errorf("%s reports %q, want the metric it was asked for", test.rule, measurement.Metric)
				}
				if measurement.Value < 0 {
					t.Errorf("%s came to %s, want a count", test.rule, measurement)
				}
				subjects = append(subjects, measurement.Subject)
			}
			if !slices.Contains(subjects, test.subject) {
				t.Errorf("%s measures %v, want %q among them", test.rule, subjects, test.subject)
			}
		})
	}
}

func TestAMetricAboutAClassIsMeasuredOverTheClassesTheScopeNamesThroughThePublicSurface(t *testing.T) {
	// `for classes matching` dogfooded on this library, which is full of builders: the pattern is matched
	// against the declared name, so it says nothing about the package, while every subject it selects still
	// carries the folder its type was declared in — and a class of this library has methods.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.Metrics(nil).InFolder("metrics/fluentapi").ForClassesMatching("Metrics*Builder").Count().MethodCount()

	measurements, err := rule.Measure(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	subjects := make([]string, 0, len(measurements))
	for _, measurement := range measurements {
		if measurement.Value <= 0 {
			t.Errorf("%s came to %s, want a builder of this library to have methods", rule, measurement)
		}
		subjects = append(subjects, measurement.Subject)
	}
	want := []string{"metrics/fluentapi.MetricsBuilder", "metrics/fluentapi.MetricsCountBuilder"}
	for _, class := range want {
		if !slices.Contains(subjects, class) {
			t.Errorf("%s measures %v, want %q among them", rule, subjects, class)
		}
	}
	if slices.Contains(subjects, "metrics/fluentapi.MetricBuilder") {
		t.Errorf("%s measures %v, want a class the pattern does not describe left out", rule, subjects)
	}
}

func TestMeasuringNothingIsNoErrorThroughThePublicSurface(t *testing.T) {
	// A scope no file of the project is in: measuring nothing is an ordinary answer at this door, because
	// whether that is a failure is a question only a rule that judges a number can ask — and the six threshold
	// predicates are where the empty-test guard will be wired in.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.Metrics(nil).InFolder("no/such/folder/**").Count().LinesOfCode()

	measurements, err := rule.Measure(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(measurements) != 0 {
		t.Errorf("%s measures %v, want nothing", rule, measurements)
	}
	if selectors := rule.Selectors(); len(selectors) != 1 {
		t.Errorf("%s reports %v as its selectors, want the verb that selected nothing", rule, selectors)
	}
}

func TestTheLocatorReachesTheProjectThroughTheMetricsEntryPoint(t *testing.T) {
	// The wrapper threads the locator through to the extraction, so a rule pointed at a directory that holds
	// no Go project says so — rather than quietly measuring the repository this test runs in, which is what
	// dropping the argument would look like, and which no comparison of two nil-located rules would notice.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.Metrics(&archunit.ProjectLocator{Directory: t.TempDir()}).Count().LinesOfCode()

	measurements, err := rule.Measure(nil)

	if err == nil {
		t.Errorf("`metrics` measured %v against a directory that is no project, want an error naming it", measurements)
	}
	if len(measurements) != 0 {
		t.Errorf("`metrics` measures %v, want nothing when the project cannot be located", measurements)
	}
}

func TestProjectGraphDrawsThisRepositoryAsItsPackagesThroughThePublicSurface(t *testing.T) {
	// The report family end to end through the public surface, dogfooding on this library: no locator, so the
	// project is the one this test is in, and the modifiers are the two a diagram of a Go project is almost
	// always drawn with — collapse the four hundred files onto the folders they live in, and leave somebody
	// else's code out of it. A report is a value, so nothing about this is a rule and nothing is judged; what
	// comes back is the data all six output formats are written over.
	t.Cleanup(archunit.ClearGraphCache)

	report := archunit.ProjectGraph(nil).
		CollapseToFolderDepth(2).
		Titled("the packages of ArchUnitGo")

	snapshot, err := report.Snapshot()
	if err != nil {
		t.Fatalf("%s failed: %v", report, err)
	}

	if snapshot.Title() != "the packages of ArchUnitGo" {
		t.Errorf("%s is titled %q, want what the chain said", report, snapshot.Title())
	}
	labels := make([]string, 0, len(snapshot.Nodes()))
	for _, node := range snapshot.Nodes() {
		labels = append(labels, node.Label())
		if node.IsExternal() {
			t.Errorf("%s draws %v, want this repository's own packages only by default", report, node)
		}
	}
	// The public surface is at the project root, so it is drawn as `.` — which is the root's own identifier.
	for _, wanted := range []string{".", "common/matching", "common/projection", "graph/fluentapi", "graph/projection", "archtest"} {
		if !slices.Contains(labels, wanted) {
			t.Errorf("%s draws %v, want %q among them", report, labels, wanted)
		}
	}
	if slices.Contains(labels, "archunit.go") {
		t.Errorf("%s draws %v, want every file collapsed onto its folder", report, labels)
	}
	// One arrow this library's layering guarantees, and the count that keeps a collapsed diagram honest: the
	// module's chain is written over its own projection, by more than one file.
	found := false
	for _, edge := range snapshot.Edges() {
		if edge.SourceLabel() != "graph/fluentapi" || edge.TargetLabel() != "graph/projection" {
			continue
		}
		found = true
		if edge.Count() < 2 {
			t.Errorf("%s draws %v, want every dependency behind the arrow counted", report, edge)
		}
		if edge.IsExternal() {
			t.Errorf("%s draws %v as leaving the project, want it internal", report, edge)
		}
	}
	if !found {
		t.Errorf("%s draws no arrow from graph/fluentapi to graph/projection, want the one this module is built as", report)
	}
	summary := snapshot.Summary()
	if summary.Nodes != len(snapshot.Nodes()) || summary.Edges != len(snapshot.Edges()) {
		t.Errorf("%s summarizes itself as %v, want the diagram it is printed over", report, summary)
	}
	if summary.Dependencies < summary.Edges {
		t.Errorf("%s summarizes itself as %v, want at least one dependency behind every arrow", report, summary)
	}
	if summary.ExternalNodes != 0 || summary.ExternalEdges != 0 {
		t.Errorf("%s summarizes itself as %v, want nothing external in the default report", report, summary)
	}
}

func TestIncludingExternalDependenciesDrawsWhatThisRepositoryDependsOn(t *testing.T) {
	// The other report a diagram of a Go project is drawn for: not how the code is arranged but what it pulls
	// in. This library depends on one module of its own accord, and the standard library everywhere, so the
	// arrow asserted here is the one AGENTS.md allows and every other rule in this file is about keeping.
	t.Cleanup(archunit.ClearGraphCache)

	report := archunit.ProjectGraph(nil).IncludingExternalDependencies().CollapseToFolderDepth(2)

	snapshot, err := report.Snapshot()
	if err != nil {
		t.Fatalf("%s failed: %v", report, err)
	}

	toolchain := "golang.org/x/tools/go/packages"
	drawn := false
	for _, node := range snapshot.Nodes() {
		if node.Label() != toolchain {
			continue
		}
		drawn = true
		if !node.IsExternal() {
			t.Errorf("%s draws %v as this repository's own code, want it external", report, node)
		}
	}
	if !drawn {
		t.Errorf("%s draws no node for %q, want the one module this library depends on", report, toolchain)
	}
	want := "common/extraction -> " + toolchain + " [1 dependency] (external) [plain]"
	descriptions := make([]string, 0, len(snapshot.Edges()))
	for _, edge := range snapshot.Edges() {
		descriptions = append(descriptions, edge.String())
	}
	if !slices.Contains(descriptions, want) {
		t.Errorf("%s draws %v, want %q among them: the toolchain is reached from one package of one folder",
			report, descriptions, want)
	}
	if summary := snapshot.Summary(); summary.ExternalNodes == 0 || summary.ExternalEdges == 0 {
		t.Errorf("%s summarizes itself as %v, want the dependencies that leave the project counted", report, summary)
	}
}

func TestDependencyGraphIsTheOtherNameOfProjectGraphOnThePublicSurface(t *testing.T) {
	t.Cleanup(archunit.ClearGraphCache)

	verbose, err := archunit.ProjectGraph(nil).CollapseByPattern("kernel", "common/**").Snapshot()
	if err != nil {
		t.Fatalf("`project graph, collapse by pattern` failed: %v", err)
	}
	other, err := archunit.DependencyGraph(nil).CollapseByPattern("kernel", "common/**").Snapshot()
	if err != nil {
		t.Fatalf("`dependency graph, collapse by pattern` failed: %v", err)
	}

	if !slices.Contains(nodeLabelsOf(verbose), "kernel") {
		t.Errorf("`project graph, collapse by pattern` drew %v, want the group it named", nodeLabelsOf(verbose))
	}
	if other.String() != verbose.String() {
		t.Errorf("`dependency graph, ...` drew\n%s\nwant what `project graph, ...` drew\n%s", other, verbose)
	}
}

func TestTheLocatorReachesTheProjectThroughEitherGraphEntryPoint(t *testing.T) {
	// Both wrappers thread the locator through to the extraction, so a report pointed at a directory that holds
	// no Go project says so — rather than quietly drawing the repository this test runs in, which is what
	// dropping the argument would look like and what no comparison of two nil-located reports would notice.
	t.Cleanup(archunit.ClearGraphCache)

	entryPoints := []struct {
		name   string
		report archunit.GraphBuilder
	}{
		{name: "project graph", report: archunit.ProjectGraph(&archunit.ProjectLocator{Directory: t.TempDir()})},
		{name: "dependency graph", report: archunit.DependencyGraph(&archunit.ProjectLocator{Directory: t.TempDir()})},
	}

	for _, entry := range entryPoints {
		t.Run(entry.name, func(t *testing.T) {
			snapshot, err := entry.report.Snapshot()

			if err == nil {
				t.Errorf("`%s` drew %v against a directory that is no project, want an error naming it", entry.name, snapshot)
			}
			if !snapshot.Empty() {
				t.Errorf("`%s` drew %v, want nothing when the project cannot be located", entry.name, snapshot)
			}
		})
	}
}

func TestTheSixOutputFormatsRenderAndExportThisRepositoryThroughThePublicSurface(t *testing.T) {
	// The REPORT stage end to end through the public surface, dogfooding on this library: one described report,
	// rendered as all six formats and written to disk as all six, and the file is the string byte for byte. That
	// last part is what a user relies on when a diagram is committed beside the code — a test asserts on the
	// string form, and the file a build exports is the same document.
	t.Cleanup(archunit.ClearGraphCache)

	report := archunit.ProjectGraph(nil).
		CollapseToFolderDepth(2).
		Titled("the packages of ArchUnitGo")
	folder := t.TempDir()

	formats := []struct {
		name     string
		file     string
		rendered func() (string, error)
		exported func(string) error
		want     string
	}{
		{
			name: "dot", file: "architecture.dot", rendered: report.ToDot, exported: report.ExportAsDot,
			want: `digraph "the packages of ArchUnitGo" {`,
		},
		{
			name: "mermaid", file: "architecture.mmd", rendered: report.ToMermaid, exported: report.ExportAsMermaid,
			want: "flowchart LR",
		},
		{
			name: "d2", file: "architecture.d2", rendered: report.ToD2, exported: report.ExportAsD2,
			want: "direction: right",
		},
		{
			name: "csv", file: "dependencies.csv", rendered: report.ToCSV, exported: report.ExportAsCSV,
			want: "kind,source,target,dependencies,external,import kinds",
		},
		{
			name: "json", file: "dependencies.json", rendered: report.ToJSON, exported: report.ExportAsJSON,
			want: `"title": "the packages of ArchUnitGo"`,
		},
		{
			name: "html", file: "architecture.html", rendered: report.ToHTML, exported: report.ExportAsHTML,
			want: "<h1>the packages of ArchUnitGo</h1>",
		},
	}

	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			document, err := format.rendered()
			if err != nil {
				t.Fatalf("rendering %s as %s failed: %v", report, format.name, err)
			}
			if !strings.Contains(document, format.want) {
				t.Errorf("%s rendered as %s does not hold %q:\n%s", report, format.name, format.want, document)
			}
			// Every format names this library's own packages, whatever else it says about them.
			for _, label := range []string{"graph/fluentapi", "graph/rendering"} {
				if !strings.Contains(document, label) {
					t.Errorf("%s rendered as %s does not draw %q:\n%s", report, format.name, label, document)
				}
			}

			path := filepath.Join(folder, "reports", format.file)
			if err := format.exported(path); err != nil {
				t.Fatalf("exporting %s as %s failed: %v", report, format.name, err)
			}
			exported, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading the exported %s report failed: %v", format.name, err)
			}
			if string(exported) != document {
				t.Errorf("the exported %s report holds\n%s\nwant the document its string form renders\n%s",
					format.name, exported, document)
			}
		})
	}
}

// nodeLabelsOf are the labels a report's nodes are drawn under, in order, for a message about what came out.
func nodeLabelsOf(snapshot archunit.GraphSnapshot) []string {
	labels := make([]string, 0, len(snapshot.Nodes()))
	for _, node := range snapshot.Nodes() {
		labels = append(labels, node.Label())
	}
	return labels
}

func TestTheReportOfARuleThisRepositoryBreaksNamesEveryOffender(t *testing.T) {
	// The last stage of the pipeline through the public surface: a rule this repository breaks, its
	// violations shaped into the report a test failure prints. The rule is the naming one above, so the
	// violations are known to be there; what is tested here is that the report counts them, numbers them and
	// names each offending file — and that it is plain text, because a report is read from a CI log as often
	// as from a terminal.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/matching").Should().HaveName("regex_factory.go")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) == 0 {
		t.Fatalf("%s reports nothing, want the files of that folder that are named otherwise", rule)
	}

	result := archunit.NewResultFactory(nil).Result(violations)

	if result.Passed {
		t.Errorf("the report of %s passed, want the failure its %d violations are", rule, len(violations))
	}
	lines := strings.Split(result.Message, "\n")
	if want := strconv.Itoa(len(violations)) + " violations:"; lines[0] != want {
		t.Errorf("the report begins %q, want %q", lines[0], want)
	}
	if len(lines) != len(violations)+1 {
		t.Errorf("the report has %d lines, want the count and one per violation:\n%s", len(lines), result.Message)
	}
	for number, violation := range violations {
		naming, ok := violation.(archunit.FileNamingViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a FileNamingViolation", rule, violation)
		}
		want := "  " + strconv.Itoa(number+1) + ". " + naming.File +
			`: should, filename matches "regex_factory.go"; it does not`
		if lines[number+1] != want {
			t.Errorf("the report reads\n\t%s\nwant\n\t%s", lines[number+1], want)
		}
	}
	if strings.Contains(result.Message, "\x1b") {
		t.Errorf("the report carries an escape sequence by default, want plain text:\n%q", result.Message)
	}

	// And the same report painted, which is the one thing the options bag is for.
	colored := archunit.NewResultFactory(&archunit.MessageOptions{Palette: archunit.DefaultPalette()}).Result(violations)
	if !strings.HasPrefix(colored.Message, "\x1b[31m") {
		t.Errorf("the colored report begins %q, want the count painted in the failure color", colored.Message)
	}
	if colored.Passed != result.Passed {
		t.Error("the colored report and the plain one disagree about the pass, want color to be decoration only")
	}

	// And one of the same violations phrased on its own, through the other re-exported factory, which is
	// what a caller writing a report of its own shape goes to. The options reach it or the sentence comes
	// out plain.
	offender, ok := violations[0].(archunit.FileNamingViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a FileNamingViolation", rule, violations[0])
	}
	painted := archunit.NewViolationFactory(&archunit.MessageOptions{Palette: archunit.DefaultPalette()}).
		Message(violations[0])
	want := "\x1b[36m" + offender.File + "\x1b[0m: \x1b[33mshould, filename matches \"regex_factory.go\"\x1b[0m; " +
		"\x1b[31mit does not\x1b[0m"
	if painted != want {
		t.Errorf("the painted violation reads\n\t%s\nwant\n\t%s", painted, want)
	}
}

func TestTheReportOfARuleThisRepositoryKeepsIsThePass(t *testing.T) {
	// The passing direction, which is the report a green suite produces and therefore the one nobody reads:
	// one line, no violations, and the pass flag read off the empty list rather than tracked beside it.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).Should().HaveName("[_a-z][_a-z0-9]*.go")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	for _, violation := range violations {
		t.Errorf("%s: %s", rule, violation)
	}

	result := archunit.NewResultFactory(nil).Result(violations)

	if !result.Passed {
		t.Errorf("the report of %s failed, want the pass", rule)
	}
	if result.Message != "no violations" {
		t.Errorf("the report of %s reads %q, want %q", rule, result.Message, "no violations")
	}
}

func TestTheReportOfAnEmptyRuleExplainsWhyThatIsAFailure(t *testing.T) {
	// The failure a reader is most likely to think is a bug in the library: a stale glob selected nothing, so
	// the rule had nothing to judge. The report names the pattern that matched nothing and the knob that
	// makes it a pass, because those are the two things they are about to go looking for.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/renamed/**").Should().HaveNoCycles()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	result := archunit.NewResultFactory(nil).Result(violations)

	if result.Passed {
		t.Errorf("the report of %s passed, want the empty rule reported as the failure it is", rule)
	}
	want := "1 violation:\n" +
		`  1. no files matched: path without filename matches "common/renamed/**"; ` +
		"an empty rule would hold forever, so selecting nothing is a violation rather than a pass " +
		"(AllowEmptyTests opts out)"
	if result.Message != want {
		t.Errorf("the report of %s reads\n%s\nwant\n%s", rule, result.Message, want)
	}
}

func TestARuleThisRepositoryKeepsPassesThroughTheAssertHelper(t *testing.T) {
	// The library used exactly as its documentation says to use it, on itself: a rule, the assert helper, the
	// real *testing.T. Nothing is inspected here on purpose — a rule that holds reports nothing at all, so a
	// green run is the assertion, and the day this repository breaks its own naming convention this test says
	// so in the words a user would be shown.
	t.Cleanup(archunit.ClearGraphCache)

	archunit.AssertPasses(t, archunit.ProjectFiles(nil).Should().HaveName("[_a-z][_a-z0-9]*.go"), nil)
	archunit.AssertPasses(t, archunit.ProjectFiles(nil).InFolder("common/**").ShouldNot().BeInFolder("files/**"), nil)
	archunit.AssertPasses(t, archunit.ProjectFiles(nil).Should().HaveNoCycles(), nil)
}

func TestTheAssertHelperFailsTheTestWithTheReportOfARuleThisRepositoryBreaks(t *testing.T) {
	// The failing direction, which cannot be tested with the real handle without failing this test: a rule this
	// repository does break, asserted against a handle that records instead of failing. What a user would see
	// is one failure — the rule as they wrote it, the count, and their offending files numbered under it.
	t.Cleanup(archunit.ClearGraphCache)

	framework := &recorder{}
	rule := archunit.ProjectFiles(nil).InFolder("common/matching").Should().HaveName("regex_factory.go")

	archunit.AssertPasses(framework, rule, nil)

	if len(framework.failures) != 1 {
		t.Fatalf("%s reported %d failures, want the one report:\n%v", rule, len(framework.failures), framework.failures)
	}
	lines := strings.Split(framework.failures[0], "\n")
	if lines[0] != rule.String() {
		t.Errorf("the failure begins %q, want the rule as it was written, %q", lines[0], rule.String())
	}
	if lines[1] != "2 violations:" {
		t.Errorf("the failure counts them as %q, want %q", lines[1], "2 violations:")
	}
	for number, offender := range []string{"common/matching/filter.go", "common/matching/match_target.go"} {
		want := "  " + strconv.Itoa(number+1) + ". " + offender +
			`: should, filename matches "regex_factory.go"; it does not`
		if lines[number+2] != want {
			t.Errorf("the failure reads\n\t%s\nwant\n\t%s", lines[number+2], want)
		}
	}
}

func TestBothHalvesOfTheAssertOptionsReachTheirOwnHalfOfTheAssertion(t *testing.T) {
	// The options bag through the public surface: the check half decides what the rule reports, the message half
	// decides how it reads. The rule is the stale glob, because AllowEmptyTests is the one knob that changes a
	// rule's answer — and a suite that really means an empty selection is the only reason it exists.
	t.Cleanup(archunit.ClearGraphCache)

	rule := archunit.ProjectFiles(nil).InFolder("common/renamed/**").Should().HaveNoCycles()
	strict, allowed, painted := &recorder{}, &recorder{}, &recorder{}

	archunit.AssertPasses(strict, rule, nil)
	archunit.AssertPasses(allowed, rule, &archunit.AssertOptions{
		Check: archunit.CheckOptions{AllowEmptyTests: true},
	})
	archunit.AssertPasses(painted, rule, &archunit.AssertOptions{
		Message: archunit.MessageOptions{Palette: archunit.DefaultPalette()},
	})

	if len(allowed.failures) != 0 {
		t.Errorf("%s with AllowEmptyTests reported %v, want the pass", rule, allowed.failures)
	}
	if len(strict.failures) != 1 {
		t.Fatalf("%s reported %d failures, want the empty rule reported as the failure it is", rule, len(strict.failures))
	}
	want := rule.String() + "\n1 violation:\n" +
		`  1. no files matched: path without filename matches "common/renamed/**"; ` +
		"an empty rule would hold forever, so selecting nothing is a violation rather than a pass " +
		"(AllowEmptyTests opts out)"
	if strict.failures[0] != want {
		t.Errorf("the failure reads\n%s\nwant\n%s", strict.failures[0], want)
	}
	if len(painted.failures) != 1 || !strings.Contains(painted.failures[0], "\x1b[") {
		t.Errorf("the painted failure reads %q, want the same report with its parts colored", painted.failures)
	}
	if strings.Contains(strict.failures[0], "\x1b[") {
		t.Errorf("the default failure reads %q, want plain text", strict.failures[0])
	}
}

func TestEveryFrameBetweenTheUserAndTheReportMarksItselfAsAHelper(t *testing.T) {
	// A framework blames the first frame on the stack that has not marked itself, so a re-export that forwarded
	// without marking would attribute every failure to archunit.go instead of to the user's own assertion line —
	// on exactly the call form the documentation prescribes. Two marks: this file's wrapper and the helper it
	// delegates to. It is counted whether or not the rule holds, because a passing assertion leaves frames behind
	// for the next one just the same.
	t.Cleanup(archunit.ClearGraphCache)

	holds := archunit.ProjectFiles(nil).Should().HaveName("[_a-z][_a-z0-9]*.go")
	broken := archunit.ProjectFiles(nil).InFolder("common/matching").Should().HaveName("regex_factory.go")

	for _, rule := range []archunit.Checkable{holds, broken} {
		framework := &recorder{}

		archunit.AssertPasses(framework, rule, nil)

		if framework.helpers != 2 {
			t.Errorf("%s marked %d frames as helpers, want the re-export and the helper it delegates to",
				rule, framework.helpers)
		}
	}
}

func TestASuiteOfRulesThisRepositoryKeepsPassesAsNamedSubtests(t *testing.T) {
	// The library used the way a real suite uses it, on itself: a map of named rules, the suite helper, the real
	// *testing.T. Each rule runs as a subtest of this one — `go test -run` picks any of them out by name — and
	// nothing is inspected on purpose, because a rule that holds reports nothing at all and a green run is the
	// assertion. The day this repository breaks one of these, the subtest named after it is what says so.
	t.Cleanup(archunit.ClearGraphCache)

	archunit.AssertAllPass(t, map[string]archunit.Checkable{
		"the kernel does not depend on a domain module": archunit.ProjectFiles(nil).
			InFolder("common/**").ShouldNot().DependOnFiles().InFolder("files/**"),
		"the report layer does not depend on a module's fluent api": archunit.ProjectFiles(nil).
			InFolder("archtest").ShouldNot().DependOnFiles().InFolder("files/fluentapi"),
		"every file is named the way the layout asks for": archunit.ProjectFiles(nil).
			Should().HaveName("[_a-z][_a-z0-9]*.go"),
		"no file depends on another in a circle": archunit.ProjectFiles(nil).Should().HaveNoCycles(),
		// A whole N-layer policy is one Checkable, so it sits in a suite beside the single-file rules as one
		// entry rather than as the rule per pair of layers it would otherwise be.
		"the layers of this library only depend inwards": theLayersOfThisRepository().
			WhereLayer("kernel").MayOnlyDependOnLayers().
			WhereLayer("files").MayOnlyDependOnLayers("kernel").
			WhereLayer("layers").MayOnlyDependOnLayers("kernel").
			WhereLayer("slices").MayOnlyDependOnLayers("kernel").
			WhereLayer("graph").MayOnlyDependOnLayers("kernel").
			WhereLayer("report").MayOnlyDependOnLayers("kernel", "files", "layers", "slices"),
	}, nil)
}

func TestTheSuiteHelperRunsOneSubtestPerRuleUnderTheNameItWasGiven(t *testing.T) {
	// What the real handle above cannot show, because a subtest that ran is indistinguishable from a suite that
	// quietly skipped it: the names, and that there is one per rule. They come back sorted rather than in map
	// order, so that a suite's output is the same on every run and two runs of it can be diffed.
	t.Cleanup(archunit.ClearGraphCache)

	framework := &runner{}
	rules := map[string]archunit.Checkable{
		"the report layer phrases violations and nothing else": archunit.ProjectFiles(nil).
			InFolder("archtest").ShouldNot().DependOnFiles().InFolder("files/projection"),
		"a pure assertion cannot reach back into the fluent api": archunit.ProjectFiles(nil).
			InFolder("files/assertion").ShouldNot().DependOnFiles().InFolder("files/fluentapi"),
		"nothing inside the library depends on the public surface": archunit.ProjectFiles(nil).
			ShouldNot().DependOnFiles().WithName("archunit.go"),
	}

	archunit.AssertAllPass(framework, rules, nil)

	want := slices.Sorted(maps.Keys(rules))
	if names := framework.names(); !slices.Equal(names, want) {
		t.Errorf("the suite ran the subtests %v, want one per rule, sorted by name: %v", names, want)
	}
	if len(framework.failures) != 0 {
		t.Errorf("the suite reported %v against the test itself, want every failure inside its own subtest", framework.failures)
	}
	for _, ran := range framework.subtests {
		if ran.failed {
			t.Errorf("the subtest %q failed, want the pass a rule this repository keeps reports", ran.name)
		}
	}
}

func TestTheOptionsOfASuiteReachItsRulesThroughThePublicSurface(t *testing.T) {
	// The suite's options bag through the public surface, read off the outcome of a rule the knob decides: the
	// stale glob selects nothing, so it fails under the defaults and holds under AllowEmptyTests. A re-export
	// that forwarded a nil bag would pass the second half's rule through the defaults instead, and the first
	// half is what says the outcome is one a subtest that really ran reported.
	t.Cleanup(archunit.ClearGraphCache)

	rules := map[string]archunit.Checkable{
		"a folder this repository no longer has": archunit.ProjectFiles(nil).
			InFolder("common/renamed/**").Should().HaveNoCycles(),
	}
	strict, allowed := &runner{}, &runner{}

	archunit.AssertAllPass(strict, rules, nil)
	archunit.AssertAllPass(allowed, rules, &archunit.AssertOptions{
		Check: archunit.CheckOptions{AllowEmptyTests: true},
	})

	for _, framework := range []struct {
		bag    string
		ran    *runner
		failed bool
	}{
		{bag: "the default options", ran: strict, failed: true},
		{bag: "AllowEmptyTests", ran: allowed, failed: false},
	} {
		if len(framework.ran.subtests) != 1 {
			t.Fatalf("the suite under %s ran %d subtests, want the one its rule is named by: %v",
				framework.bag, len(framework.ran.subtests), framework.ran.names())
		}
		if framework.ran.subtests[0].failed != framework.failed {
			t.Errorf("the subtest of the suite under %s failed: %t, want %t",
				framework.bag, framework.ran.subtests[0].failed, framework.failed)
		}
	}
}

func TestASuiteWithNoRulesInItFailsThroughThePublicSurface(t *testing.T) {
	// A suite is a policy, so a policy with nothing in it is the empty test one level up — a map an editor
	// emptied, or one filled in by a loop over a list that turned out to be empty. It is reported in its own
	// words rather than passed over, and both frames between the user and the report step aside so that the
	// failure lands on the AssertAllPass line the user wrote instead of on a line of this library.
	framework := &runner{}

	archunit.AssertAllPass(framework, nil, nil)

	if len(framework.failures) != 1 {
		t.Fatalf("a suite with no rules reported %d failures, want the one:\n%v", len(framework.failures), framework.failures)
	}
	if want := "there are no rules to check: AssertAllPass was given no rules"; framework.failures[0] != want {
		t.Errorf("the failure reads %q, want %q", framework.failures[0], want)
	}
	if len(framework.subtests) != 0 {
		t.Errorf("a suite with no rules ran the subtests %v, want none", framework.names())
	}
	if framework.helpers != 2 {
		t.Errorf("the suite marked %d frames as helpers, want the re-export and the helper it delegates to",
			framework.helpers)
	}
}

// recorder is a test framework's handle that records what it was told instead of failing, which is the only way
// to test what a user is shown when a rule does not hold. It is also the whole of what archunit.TestingT asks
// of a framework — one method — so it doubles as the proof that a framework other than the stdlib's needs no
// adapter and no registration. Helper() is counted rather than acted on: it is the optional half of a handle,
// and counting is how a test sees that the frames between the user and the report all step aside.
type recorder struct {
	failures []string
	helpers  int
}

func (r *recorder) Error(args ...any) {
	r.failures = append(r.failures, fmt.Sprint(args...))
}

func (r *recorder) Helper() {
	r.helpers++
}

// runner is the standard library's handle as the suite helper sees one: the recorder above, plus Run. It runs
// each subtest's body against a fresh *testing.T of its own and records the name the suite gave it together
// with whether the rule asserted inside it held — which is how a test reads a suite's per-rule outcomes
// without failing this repository's own suite with them. A recorder cannot stand in for that handle, because
// Run's argument is a *testing.T.
type runner struct {
	recorder
	subtests []subtest
}

// subtest is one rule's outcome as a framework reports it: the name the suite ran it under, and whether the
// rule asserted inside it held.
type subtest struct {
	name   string
	failed bool
}

func (r *runner) Run(name string, f func(t *testing.T)) bool {
	// A fresh *testing.T with no parent: the assert helper reports through Error, and Failed is what a
	// framework reads that back off, so running the body here is what lets a test see a rule's own outcome.
	handle := &testing.T{}
	f(handle)
	r.subtests = append(r.subtests, subtest{name: name, failed: handle.Failed()})
	return !handle.Failed()
}

// names are the subtests the suite ran, in the order it ran them.
func (r *runner) names() []string {
	names := make([]string, 0, len(r.subtests))
	for _, subtest := range r.subtests {
		names = append(names, subtest.name)
	}
	return names
}

func selectFiles(t *testing.T, rule archunit.FilesBuilder) []string {
	t.Helper()

	selected, err := rule.SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}
	return selected
}
