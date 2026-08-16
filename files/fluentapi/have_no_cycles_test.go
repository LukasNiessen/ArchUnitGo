package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

// A built rule is a Checkable and nothing else has to be known about it, so the interface is asserted at
// compile time rather than in a test that could be deleted.
var _ kernel.Checkable = fluentapi.FilesCyclesCondition{}

func TestHaveNoCyclesPassesForAProjectWithoutCycles(t *testing.T) {
	// The whole pipeline through the public surface: a project on disk, located, extracted, projected and
	// judged. An empty result is the pass.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).Should().HaveNoCycles()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) != 0 {
		t.Errorf("%s reports %v, want the pass: no file of that project depends on itself in a circle", rule, violations)
	}
}

func TestHaveNoCyclesReportsTheCycleAsAReadablePath(t *testing.T) {
	// The violation the issue asks for, over a project whose two packages import each other. Go rejects an
	// import cycle at type-check time, and extraction deliberately never type-checks: a project mid-refactor
	// still has a shape, and this is the shape a rule about cycles exists for.
	locator := fixtureLocator(t, writeCyclicFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).Should().HaveNoCycles()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) != 1 {
		t.Fatalf("%s reports %v, want the one cycle the project has", rule, violations)
	}
	if kind := violations[0].Kind(); kind != filesassertion.KindFileCycle {
		t.Errorf("the violation is of kind %q, want %q", kind, filesassertion.KindFileCycle)
	}
	cycle, ok := violations[0].(filesassertion.CycleViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a CycleViolation", rule, violations[0])
	}
	want := "internal/api/handler.go -> internal/db/conn.go -> internal/api/handler.go"
	if rendered := cycle.String(); rendered != want {
		t.Errorf("the violation reads %q, want the readable path %q", rendered, want)
	}
	if files := cycle.Files(); !slices.Equal(files, []string{"internal/api/handler.go", "internal/db/conn.go"}) {
		t.Errorf("the violation names %v, want both files of the cycle", files)
	}
	// The dependency behind each step survives, which is what a report needs in order to name the import
	// rather than only the file.
	for _, edge := range cycle.Cycle.Edges() {
		if raw := edge.CumulatedEdges(); len(raw) == 0 {
			t.Errorf("%s carries no raw edge, want the import that made the dependency", edge)
		}
	}
}

func TestHaveNoCyclesIsAboutTheDependenciesBetweenTheSelectedFilesOnly(t *testing.T) {
	// The scope is read as "the dependencies between these files", so a cycle running out of the scope and
	// back is not this rule's cycle — widening it is what makes the same cycle visible.
	locator := fixtureLocator(t, writeCyclicFixtureProject(t))
	narrowed := fluentapi.ProjectFiles(locator).InFolder("internal/api").Should().HaveNoCycles()
	widened := fluentapi.ProjectFiles(locator).InFolder("internal/**").Should().HaveNoCycles()

	inside, err := narrowed.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", narrowed, err)
	}
	across, err := widened.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", widened, err)
	}

	if len(inside) != 0 {
		t.Errorf("%s reports %v, want none: the cycle leaves the folder it selected", narrowed, inside)
	}
	if len(across) != 1 {
		t.Errorf("%s reports %v, want the one cycle between the files it selected", widened, across)
	}
}

func TestHaveNoCyclesReportsAScopeThatSelectedNoFile(t *testing.T) {
	// A rule with no subject would pass forever — no file means no dependency between two of them — so the
	// empty-test guard answers instead of the cycle detection, and AllowEmptyTests is how a user opts out.
	locator := fixtureLocator(t, writeFixtureProject(t))
	rule := fluentapi.ProjectFiles(locator).InFolder("internal/apis/**").Should().HaveNoCycles()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("%s reports %v, want the one empty-test violation of a stale pattern", rule, violations)
	}
	if kind := violations[0].Kind(); kind != assertion.KindEmptyTest {
		t.Errorf("the violation is of kind %q, want %q", kind, assertion.KindEmptyTest)
	}
	empty, ok := violations[0].(assertion.EmptyTestViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an EmptyTestViolation", rule, violations[0])
	}
	if empty.Subject != "files" {
		t.Errorf("the violation says the rule selected no %q, want `files`", empty.Subject)
	}
	if len(empty.Selectors) != 1 {
		t.Errorf("the violation carries %v, want the pattern that selected nothing", empty.Selectors)
	}

	allowed, err := rule.Check(&kernel.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("%s failed with AllowEmptyTests: %v", rule, err)
	}
	if len(allowed) != 0 {
		t.Errorf("%s reports %v with AllowEmptyTests, want the pass", rule, allowed)
	}
}

func TestHaveNoCyclesThreadsTheCheckOptionsIntoTheExtraction(t *testing.T) {
	// AllowEmptyTests travels its own way to the guard, so every other knob on the bag reaches the rule
	// through the extraction — and a terminal that resolved its scope against a differently-extracted
	// project would judge the rule against files the user did not ask about, silently. IncludeTestFiles is
	// the cheapest of those knobs to observe: the fixture's test file is a node only when it is on, so the
	// same rule has no subject by default and exactly one afterwards.
	locator := fixtureLocator(t, writeFixtureProject(t))
	rule := fluentapi.ProjectFiles(locator).InFile("internal/api/handler_test.go").Should().HaveNoCycles()

	byDefault, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	withTests, err := rule.Check(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("%s failed with IncludeTestFiles: %v", rule, err)
	}

	if len(byDefault) != 1 || byDefault[0].Kind() != assertion.KindEmptyTest {
		t.Fatalf("%s reports %v by default, want the one empty-test violation: a test file is not a node", rule, byDefault)
	}
	if len(withTests) != 0 {
		t.Errorf("%s reports %v with IncludeTestFiles, want the pass: the file it named is a node now", rule, withTests)
	}
}

func TestHaveNoCyclesReportsAPatternTheScopeRejected(t *testing.T) {
	// A pattern this library cannot understand is the user's error, deferred to the terminal because a
	// fluent method has nowhere to put one — and it is an error rather than a violation, so a suite cannot
	// read it as a rule that holds.
	rule := fluentapi.ProjectFiles(nil).InFolder("[unclosed").Should().HaveNoCycles()

	violations, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the scope verb `in folder`", user.Operation)
	}
	if violations != nil {
		t.Errorf("Check reports %v beside the error, want nothing said about the rule", violations)
	}
	if rendered := rule.String(); !strings.Contains(rendered, "rejected") {
		t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
	}
}

func TestHaveNoCyclesRejectsALocatorThatIsNotAProject(t *testing.T) {
	// Nothing is read while a rule is built, so a locator naming no Go project is the terminal's error and
	// not the entry point's.
	locator := &extraction.ProjectLocator{Directory: t.TempDir()}
	rule := fluentapi.ProjectFiles(locator).Should().HaveNoCycles()

	violations, err := rule.Check(nil)

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Fatalf("Check error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
	if violations != nil {
		t.Errorf("Check reports %v beside the error, want nothing said about the rule", violations)
	}
}

func TestAFilesCyclesConditionCanBeStoredAndCheckedTwice(t *testing.T) {
	// A rule is a value: built once from a stored scope, checked as often as it is useful, and the same
	// answer every time.
	locator := fixtureLocator(t, writeCyclicFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**")

	rule := scope.Should().HaveNoCycles()

	first, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	second, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed on the second check: %v", rule, err)
	}
	if len(first) != 1 || len(second) != len(first) {
		t.Errorf("%s reports %v and then %v, want the same one cycle both times", rule, first, second)
	}
	// The scope it was built from is untouched, so another rule can still be branched from it.
	if selectors := scope.Selectors(); len(selectors) != 1 {
		t.Errorf("the stored scope's Selectors() = %v, want the one verb it was built with", selectors)
	}
}

func TestHaveNoCyclesRendersTheWholeSentence(t *testing.T) {
	rule := fluentapi.ProjectFiles(nil).InFolder("internal/**").Should().HaveNoCycles()

	rendered := rule.String()

	want := `project files, path without filename matches "internal/**", should, have no cycles`
	if rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
	if bare := fluentapi.ProjectFiles(nil).Should().HaveNoCycles().String(); bare != "project files, should, have no cycles" {
		t.Errorf("String() = %q, want the sentence without a scope verb", bare)
	}
}

func TestHaveNoCyclesExistsOnThePositiveMoodAlone(t *testing.T) {
	// `should not have no cycles` demands that the files be cyclic and fails with nothing to report but the
	// absence of a cycle, so the type system refuses the sentence instead of the library carrying a
	// violation it cannot fill. Checked on the method set, because a reviewer's memory is not a check.
	positive := methodsOf(fluentapi.ProjectFiles(nil).Should())
	negated := methodsOf(fluentapi.ProjectFiles(nil).ShouldNot())

	if !slices.Contains(positive, "HaveNoCycles") {
		t.Errorf("`should` offers %v, want HaveNoCycles among them", positive)
	}
	if slices.Contains(negated, "HaveNoCycles") {
		t.Errorf("`should not` offers %v, want no HaveNoCycles", negated)
	}
	// And no second spelling of the predicate on either mood, which is how a fluent API grows two ways to
	// say one thing.
	for _, methods := range [][]string{positive, negated} {
		for _, method := range methods {
			if strings.Contains(method, "Cycle") && method != "HaveNoCycles" {
				t.Errorf("a mood offers %s, want `have no cycles` spelled one way", method)
			}
		}
	}
}

// fixtureLocator points a rule at a project written for one test, with the graph cache cleared around it:
// the cache is keyed by project, and a fixture written under a fresh directory per test is a project the
// memo has never seen — clearing it is what keeps that true however the fixtures are reused.
func fixtureLocator(t *testing.T, root string) *extraction.ProjectLocator {
	t.Helper()

	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	return &extraction.ProjectLocator{Directory: root}
}

// writeCyclicFixtureProject writes a project whose two packages import each other, which is the only way
// a file-level cycle exists: the compiler rejects an import cycle, so no project that builds has one, and
// extraction reads the shape of the source rather than type-checking it.
//
// The cycle is internal/api/handler.go -> internal/db/conn.go -> internal/api/handler.go. main.go depends
// on the cyclic region without being part of it, and internal/db/query.go is in the imported package
// without importing anything, so neither is in the cycle a rule must report.
func writeCyclicFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                  "module example.com/cyclic\n\ngo 1.26\n",
		"main.go":                 "package main\n\nimport \"example.com/cyclic/internal/api\"\n\nfunc main() { api.Handle() }\n",
		"internal/api/handler.go": "package api\n\nimport \"example.com/cyclic/internal/db\"\n\nfunc Handle() { db.Connect() }\n",
		"internal/db/conn.go":     "package db\n\nimport \"example.com/cyclic/internal/api\"\n\nfunc Connect() { api.Handle() }\n",
		"internal/db/query.go":    "package db\n\nfunc Query() {}\n",
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
