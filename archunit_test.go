package archunit_test

import (
	"slices"
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

// The violation types a rule reports are on the surface too, because a user who wants more than a pass or
// a fail reads the violation rather than its message.
var (
	_ archunit.Violation     = archunit.FileCycleViolation{}
	_ archunit.Violation     = archunit.EmptyTestViolation{}
	_ archunit.Circuit       = archunit.FileCycleViolation{}.Cycle
	_ archunit.ViolationKind = archunit.KindFileCycle
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

func selectFiles(t *testing.T, rule archunit.FilesBuilder) []string {
	t.Helper()

	selected, err := rule.SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}
	return selected
}
