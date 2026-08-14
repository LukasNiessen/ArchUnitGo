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
	_ archunit.Violation     = archunit.FileAdherenceViolation{}
	_ archunit.Circuit       = archunit.FileCycleViolation{}.Cycle
	_ archunit.ViolationKind = archunit.KindFileCycle
	_ archunit.ViolationKind = archunit.KindFileNaming
	_ archunit.ViolationKind = archunit.KindFileDependency
	_ archunit.ViolationKind = archunit.KindFileAdherence
	_ archunit.Mood          = archunit.FileNamingViolation{}.Mood
	_ archunit.Mood          = archunit.FileDependencyViolation{}.Mood
	_ archunit.Mood          = archunit.FileAdherenceViolation{}.Mood
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

func selectFiles(t *testing.T, rule archunit.FilesBuilder) []string {
	t.Helper()

	selected, err := rule.SelectFiles(nil)
	if err != nil {
		t.Fatalf("SelectFiles failed: %v", err)
	}
	return selected
}
