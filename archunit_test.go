package archunit_test

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	archunit "github.com/LukasNiessen/ArchUnitGo"
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

// The violation types a rule reports are on the surface too, because a user who wants more than a pass or
// a fail reads the violation rather than its message.
var (
	_ archunit.Violation     = archunit.FileCycleViolation{}
	_ archunit.Violation     = archunit.EmptyTestViolation{}
	_ archunit.Violation     = archunit.FileNamingViolation{}
	_ archunit.Violation     = archunit.FileDependencyViolation{}
	_ archunit.Violation     = archunit.FileExternalDependencyViolation{}
	_ archunit.Violation     = archunit.FileAdherenceViolation{}
	_ archunit.Circuit       = archunit.FileCycleViolation{}.Cycle
	_ archunit.ViolationKind = archunit.KindFileCycle
	_ archunit.ViolationKind = archunit.KindFileNaming
	_ archunit.ViolationKind = archunit.KindFileDependency
	_ archunit.ViolationKind = archunit.KindFileExternalDependency
	_ archunit.ViolationKind = archunit.KindFileAdherence
	_ archunit.Mood          = archunit.FileNamingViolation{}.Mood
	_ archunit.Mood          = archunit.FileDependencyViolation{}.Mood
	_ archunit.Mood          = archunit.FileExternalDependencyViolation{}.Mood
	_ archunit.Mood          = archunit.FileAdherenceViolation{}.Mood
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
