package fluentapi_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

// A built rule is a Checkable and nothing else has to be known about it, so the interface is asserted at
// compile time rather than in a test that could be deleted.
var _ kernel.Checkable = fluentapi.FilesDependencyCondition{}

func TestDependOnFilesInTheNegatedMoodReportsTheFilesThatCrossTheBoundary(t *testing.T) {
	// The sentence AGENTS.md opens with, through the whole pipeline: a project on disk, located, extracted,
	// both halves of the rule resolved against it, and one violation per file that reaches the forbidden folder.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api/**").ShouldNot().DependOnFiles().InFolder("internal/db/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := dependencyOffenders(t, rule, violations); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Fatalf("%s reports %v, want the one file of that folder that reaches the database", rule, offenders)
	}
	dependency, ok := violations[0].(filesassertion.DependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a DependencyViolation", rule, violations[0])
	}
	if !slices.Equal(dependency.Dependencies, []string{"internal/db/conn.go"}) {
		t.Errorf("the violation carries %v, want the file it was broken by", dependency.Dependencies)
	}
	if len(dependency.Required) != 1 || dependency.Required[0].Pattern().Source() != "internal/db/**" {
		t.Errorf("the violation carries %v, want the object as the user typed it", dependency.Required)
	}
	if dependency.Required[0].Target() != matching.TargetPathWithoutFilename {
		t.Errorf("the object was matched against the %s, want the folder `in folder` reads", dependency.Required[0].Target())
	}
	if dependency.Mood != assertion.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want the rule's own %s", dependency.Mood, assertion.ShouldNot)
	}
	want := `internal/api/handler.go: should not, depend on files, path without filename matches "internal/db/**" -> internal/db/conn.go`
	if rendered := dependency.String(); rendered != want {
		t.Errorf("the violation reads %q, want %q", rendered, want)
	}
}

func TestDependOnFilesInThePositiveMoodRequiresEachSelectedFileToReachTheObject(t *testing.T) {
	// `should depend on files` is satisfied per file, existentially: the handler reaches the database and holds
	// the rule, and the file of the same folder that reaches nothing there is the offender — carrying no
	// dependency, because the absence is the offense.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api/**").Should().DependOnFiles().InFolder("internal/db/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := dependencyOffenders(t, rule, violations); !slices.Equal(offenders, []string{"internal/api/router.go"}) {
		t.Fatalf("%s reports %v, want the one file of that folder that reaches nothing there", rule, offenders)
	}
	dependency, ok := violations[0].(filesassertion.DependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a DependencyViolation", rule, violations[0])
	}
	if len(dependency.Dependencies) != 0 {
		t.Errorf("the violation carries %v, want none: it was reported for reaching nothing", dependency.Dependencies)
	}
	want := `internal/api/router.go: should, depend on files, path without filename matches "internal/db/**" -> nothing`
	if rendered := dependency.String(); rendered != want {
		t.Errorf("the violation reads %q, want %q", rendered, want)
	}
}

func TestDependOnFilesInTheTwoMoodsPartitionsTheSelection(t *testing.T) {
	// One rule and its negation over one stored scope: every selected file offends exactly one of them, which is
	// the property that lets the two moods share a single assertion.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**")

	positive := scope.Should().DependOnFiles().InFolder("internal/db/**")
	negated := scope.ShouldNot().DependOnFiles().InFolder("internal/db/**")

	held, err := positive.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", positive, err)
	}
	broken, err := negated.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", negated, err)
	}
	if selected := selectedFiles(t, scope); len(held)+len(broken) != len(selected) {
		t.Errorf("%s reports %v and %s reports %v, want the %d selected files split between them",
			positive, held, negated, broken, len(selected))
	}
	if len(held) == 0 || len(broken) == 0 {
		t.Errorf("%s reports %d and %s reports %d, want both moods to have something to say about that project",
			positive, len(held), negated, len(broken))
	}
}

func TestDependOnFilesIsDirected(t *testing.T) {
	// The rule reads one way: the api folder depends on the db folder, so `db should not depend on api` holds
	// while `api should not depend on db` does not. A predicate that read the two halves symmetrically would
	// report the same violation for both sentences, and a layering rule would be unwritable.
	locator := fixtureLocator(t, writeFixtureProject(t))

	forwards := fluentapi.ProjectFiles(locator).InFolder("internal/api/**").ShouldNot().DependOnFiles().InFolder("internal/db/**")
	backwards := fluentapi.ProjectFiles(locator).InFolder("internal/db/**").ShouldNot().DependOnFiles().InFolder("internal/api/**")

	broken, err := forwards.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", forwards, err)
	}
	held, err := backwards.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", backwards, err)
	}
	if len(broken) == 0 {
		t.Errorf("%s reports nothing, want the dependency the fixture has", forwards)
	}
	if len(held) != 0 {
		t.Errorf("%s reports %v, want the pass: the database depends on nothing", backwards, held)
	}
}

func TestTheObjectVerbsAreChainedWithAnd(t *testing.T) {
	// The object is a selection like the scope is, so its verbs narrow it and their order cannot matter. Narrowed
	// down to the one file the dependency actually points at, the rule still fires; narrowed past it, it passes.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/api/**")

	narrowed := scope.ShouldNot().DependOnFiles().InFolder("internal/**").WithName("conn.go")
	reversed := scope.ShouldNot().DependOnFiles().WithName("conn.go").InFolder("internal/**")
	past := scope.ShouldNot().DependOnFiles().InFolder("internal/**").WithName("router.go")

	broken, err := narrowed.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", narrowed, err)
	}
	if offenders := dependencyOffenders(t, narrowed, broken); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Errorf("%s reports %v, want the file that depends on that one file", narrowed, offenders)
	}
	other, err := reversed.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", reversed, err)
	}
	if !slices.Equal(dependencyOffenders(t, reversed, other), dependencyOffenders(t, narrowed, broken)) {
		t.Errorf("%s reports %v and %s reports %v, want the order of the object verbs not to matter",
			reversed, other, narrowed, broken)
	}
	held, err := past.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", past, err)
	}
	if len(held) != 0 {
		t.Errorf("%s reports %v, want the pass: no file of that folder depends on that file", past, held)
	}
}

func TestDependOnFilesWithNoObjectVerbIsEveryFileOfTheProject(t *testing.T) {
	// `should not depend on files` with nothing chained onto it is a file that must import nothing of its own
	// project — the leaf of the dependency graph — and it is deliberately the loud reading: a chain whose object
	// the user forgot to type fails, rather than passing because it forbade an empty set.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api").ShouldNot().DependOnFiles()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := dependencyOffenders(t, rule, violations); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Fatalf("%s reports %v, want the file of that folder that imports something of the project", rule, offenders)
	}
	if rendered := rule.String(); rendered != `project files, path without filename matches "internal/api", should not, depend on files` {
		t.Errorf("String() = %q, want the predicate with no object clause after it", rendered)
	}
	// And the positive mood of the same object is the file that imports nothing of its own project at all.
	leaf := fluentapi.ProjectFiles(locator).InFolder("internal/api").Should().DependOnFiles()
	reported, err := leaf.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", leaf, err)
	}
	if offenders := dependencyOffenders(t, leaf, reported); !slices.Equal(offenders, []string{"internal/api/router.go"}) {
		t.Errorf("%s reports %v, want the file of that folder that imports nothing of the project", leaf, offenders)
	}
}

func TestADependencyRuleReportsAScopeThatSelectedNoFile(t *testing.T) {
	// The subject half of the empty-test guard: a stale scope has no file to judge, so one mood of the rule
	// would pass forever and the other would be red for nothing a reader could go and look at.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/apis/**").ShouldNot().DependOnFiles().InFolder("internal/db/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	empty := emptyTestViolations(t, rule, violations)
	if len(empty) != 1 {
		t.Fatalf("%s reports %v, want the one empty-test violation of a stale scope", rule, violations)
	}
	if empty[0].Subject != "files" {
		t.Errorf("the violation says the rule selected no %q, want `files`", empty[0].Subject)
	}
	if len(empty[0].Selectors) != 1 || empty[0].Selectors[0].Pattern().Source() != "internal/apis/**" {
		t.Errorf("the violation carries %v, want the scope verb that selected nothing", empty[0].Selectors)
	}
	assertAllowEmptyTestsPasses(t, rule)
}

func TestADependencyRuleReportsAnObjectThatNamedNoFile(t *testing.T) {
	// The half a relational rule adds, and the most valuable one it has: an object naming a folder that has been
	// renamed forbids nothing, so `should not depend on files in folder "internal/database/**"` would be green
	// forever. The report names the object rather than the scope, because that is the half to go and fix.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api/**").ShouldNot().DependOnFiles().InFolder("internal/database/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	empty := emptyTestViolations(t, rule, violations)
	if len(empty) != 1 {
		t.Fatalf("%s reports %v, want the one empty-test violation of a stale object", rule, violations)
	}
	if empty[0].Subject != "files to depend on" {
		t.Errorf("the violation says the rule named no %q, want the object's own vocabulary", empty[0].Subject)
	}
	if len(empty[0].Selectors) != 1 || empty[0].Selectors[0].Pattern().Source() != "internal/database/**" {
		t.Errorf("the violation carries %v, want the object verb that named nothing", empty[0].Selectors)
	}
	assertAllowEmptyTestsPasses(t, rule)
}

func TestADependencyRuleReportsBothHalvesOfAnEmptySentence(t *testing.T) {
	// Both patterns are wrong, so both are reported: a reader who fixed the scope alone would come straight back
	// for the object.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/apis/**").Should().DependOnFiles().InFolder("internal/database/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	empty := emptyTestViolations(t, rule, violations)
	if len(empty) != 2 {
		t.Fatalf("%s reports %v, want one empty-test violation per half of the sentence", rule, violations)
	}
	subjects := []string{empty[0].Subject, empty[1].Subject}
	if want := []string{"files", "files to depend on"}; !slices.Equal(subjects, want) {
		t.Errorf("the violations say the rule selected no %v, want %v — the scope first, as the user typed it", subjects, want)
	}
	assertAllowEmptyTestsPasses(t, rule)
}

func TestADependencyRuleThreadsTheCheckOptionsIntoTheExtraction(t *testing.T) {
	// Every knob but AllowEmptyTests reaches the rule through the extraction, so a terminal judging a
	// differently-extracted project would hold the rule over files the user did not ask about, silently.
	// IncludeTestFiles is the cheapest to observe: the fixture's test file imports nothing of its own project,
	// so it is an offender of the positive mood exactly when it is a node at all.
	locator := fixtureLocator(t, writeFixtureProject(t))
	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api").Should().DependOnFiles().InFolder("internal/db/**")

	byDefault, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	withTests, err := rule.Check(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("%s failed with IncludeTestFiles: %v", rule, err)
	}

	if offenders := dependencyOffenders(t, rule, byDefault); !slices.Equal(offenders, []string{"internal/api/router.go"}) {
		t.Errorf("%s reports %v by default, want the production file that reaches nothing there", rule, offenders)
	}
	want := []string{"internal/api/handler_test.go", "internal/api/router.go"}
	if offenders := dependencyOffenders(t, rule, withTests); !slices.Equal(offenders, want) {
		t.Errorf("%s reports %v with IncludeTestFiles, want %v — the test file among the selection", rule, offenders, want)
	}
}

func TestADependencyRuleReportsAPatternAnObjectVerbRejected(t *testing.T) {
	// A pattern this library cannot understand is the user's error, deferred to the terminal because a fluent
	// method has nowhere to put one — and it names the object verb the user has to go and fix, exactly as a scope
	// verb's does.
	tests := []struct {
		verb string
		rule fluentapi.FilesDependencyCondition
	}{
		{verb: "with name", rule: fluentapi.ProjectFiles(nil).ShouldNot().DependOnFiles().WithName("[unclosed")},
		{verb: "in folder", rule: fluentapi.ProjectFiles(nil).ShouldNot().DependOnFiles().InFolder("[unclosed")},
		{verb: "in path", rule: fluentapi.ProjectFiles(nil).Should().DependOnFiles().InPath("[unclosed")},
	}

	for _, test := range tests {
		t.Run(test.verb, func(t *testing.T) {
			violations, err := test.rule.Check(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Check error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the object verb %q", user.Operation, test.verb)
			}
			if user.Subject != "[unclosed" {
				t.Errorf("UserError.Subject = %q, want the pattern as the user typed it", user.Subject)
			}
			if !errors.Is(err, matching.ErrInvalidPattern) {
				t.Errorf("Check error = %v, want it to wrap ErrInvalidPattern", err)
			}
			if violations != nil {
				t.Errorf("Check reports %v beside the error, want nothing said about the rule", violations)
			}
			if rendered := test.rule.String(); !strings.Contains(rendered, "rejected") {
				t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
			}
		})
	}
}

func TestADependencyRuleReportsTheFirstRejectedPatternOfTheWholeChain(t *testing.T) {
	// The rejection joins the scope, so the pattern the user has to fix first is the one reported however many
	// stages later the second typo sits — and a rejected object verb narrows nothing, so the rule does not
	// quietly become one about an object nobody typed.
	rule := fluentapi.ProjectFiles(nil).InFolder("[unclosed").ShouldNot().DependOnFiles().InFolder("[alsounclosed")

	_, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" || user.Subject != "[unclosed" {
		t.Errorf("UserError = %v, want the first rejected pattern of the chain", user)
	}
}

func TestADependencyRuleRejectsALocatorThatIsNotAProject(t *testing.T) {
	// Nothing is read while a rule is built, so a locator naming no Go project is the terminal's error and not
	// the entry point's.
	locator := &extraction.ProjectLocator{Directory: t.TempDir()}
	rule := fluentapi.ProjectFiles(locator).ShouldNot().DependOnFiles().InFolder("internal/db/**")

	violations, err := rule.Check(nil)

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Fatalf("Check error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
	if violations != nil {
		t.Errorf("Check reports %v beside the error, want nothing said about the rule", violations)
	}
}

func TestADependencyConditionIsImmutableAndCanBeBranchedFrom(t *testing.T) {
	// The object stage is a stage like every other one, so a half-built rule stored in a variable can be
	// branched into two objects and is unchanged by either. The base carries three object verbs, because that
	// is the length at which append leaves spare capacity behind — a shared array the second branch would
	// write the first branch's object out of, if the verbs were not cloned.
	locator := fixtureLocator(t, writeFixtureProject(t))
	base := fluentapi.ProjectFiles(locator).InFolder("internal/**").ShouldNot().DependOnFiles().
		InFolder("internal/**").InPath("internal/**").WithName("*.go")

	conn := base.WithName("conn.go")
	router := base.WithName("router.go")

	broken, err := conn.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", conn, err)
	}
	if offenders := dependencyOffenders(t, conn, broken); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Errorf("the first branch reports %v, want the file that depends on conn.go", offenders)
	}
	held, err := router.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", router, err)
	}
	if len(held) != 0 {
		t.Errorf("the second branch reports %v, want the pass: nothing under internal/ depends on router.go", held)
	}
	// And the rule the two were branched from is still the object it was.
	if rendered := base.String(); strings.Contains(rendered, "conn.go") || strings.Contains(rendered, "router.go") {
		t.Errorf("the stored rule renders as %q, want it unchanged by either branch", rendered)
	}
	first, err := base.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", base, err)
	}
	second, err := base.Check(nil)
	if err != nil {
		t.Fatalf("%s failed on the second check: %v", base, err)
	}
	if !slices.Equal(dependencyOffenders(t, base, first), dependencyOffenders(t, base, second)) {
		t.Errorf("%s reports %v and then %v, want the same answer both times", base, first, second)
	}
}

func TestADependencyRuleRendersTheWholeSentence(t *testing.T) {
	// The whole grammar in one line — entry, scope, mood, predicate, object — with the scope and object verbs
	// rendering as their filters, because a reader needs to see which part of an identifier each pattern was
	// matched against, and the predicate as the words the user typed.
	tests := []struct {
		rule fluentapi.FilesDependencyCondition
		want string
	}{
		{
			rule: fluentapi.ProjectFiles(nil).InFolder("internal/api/**").ShouldNot().DependOnFiles().InFolder("internal/db/**"),
			want: `project files, path without filename matches "internal/api/**", should not, depend on files, path without filename matches "internal/db/**"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).WithName("*_handler.go").Should().DependOnFiles().WithName("*_service.go"),
			want: `project files, filename matches "*_handler.go", should, depend on files, filename matches "*_service.go"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).ShouldNot().DependOnFiles().InFolder("internal/**").InPath("**/legacy/*.go"),
			want: `project files, should not, depend on files, path without filename matches "internal/**", path matches "**/legacy/*.go"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).InFile("main.go").Should().DependOnFiles(),
			want: `project files, path matches "main.go", should, depend on files`,
		},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if rendered := test.rule.String(); rendered != test.want {
				t.Errorf("String() = %q, want %q", rendered, test.want)
			}
		})
	}
}

func TestDependOnFilesIsOnBothMoodsAndSpelledOneWay(t *testing.T) {
	// The relational predicate has a meaningful negation — a dependency can be required or forbidden — so it is
	// on both moods, and neither mood has a second spelling of it. Its object verbs are the scope's own three
	// words, spelled once each. Checked on the method set, because a reviewer's memory is not a check.
	moods := []struct {
		name    string
		methods []string
	}{
		{name: "`should`", methods: methodsOf(fluentapi.ProjectFiles(nil).Should())},
		{name: "`should not`", methods: methodsOf(fluentapi.ProjectFiles(nil).ShouldNot())},
	}
	// A synonym is how a fluent API grows two ways to say one thing, and these are the words that would be
	// reached for: the predicate is `depend on files` and nothing else.
	synonyms := []string{
		"DependOn", "DependsOnFiles", "DependOnFile", "HaveDependencyOnFiles", "AccessFiles",
		"ImportFiles", "UseFiles", "ReferenceFiles", "DependOnFilesThat", "OnlyDependOnFiles",
	}

	for _, mood := range moods {
		t.Run(mood.name, func(t *testing.T) {
			if !slices.Contains(mood.methods, "DependOnFiles") {
				t.Errorf("%s offers %v, want DependOnFiles among them", mood.name, mood.methods)
			}
			for _, method := range mood.methods {
				if slices.Contains(synonyms, method) {
					t.Errorf("%s offers %s, want the predicate spelled one way", mood.name, method)
				}
			}
		})
	}

	object := methodsOf(fluentapi.ProjectFiles(nil).ShouldNot().DependOnFiles())
	for _, verb := range []string{"WithName", "InFolder", "InPath", "Check"} {
		if !slices.Contains(object, verb) {
			t.Errorf("the object stage offers %v, want %s among them", object, verb)
		}
	}
	// The object verbs are the scope's words, so the sentence reads the same on either side of the predicate.
	for _, method := range object {
		if slices.Contains([]string{"InDirectory", "WithFilename", "MatchingName", "InPathMatching", "AndInFolder"}, method) {
			t.Errorf("the object stage offers %s, want the three verbs spelled as the scope spells them", method)
		}
	}
}

// dependencyOffenders names the files a dependency rule reported, in the order it reported them, checking on
// the way that every violation is a DependencyViolation of the file-dependency kind.
func dependencyOffenders(t *testing.T, rule fluentapi.FilesDependencyCondition, violations []assertion.Violation) []string {
	t.Helper()

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != filesassertion.KindFileDependency {
			t.Errorf("%s reports a violation of kind %q, want %q", rule, kind, filesassertion.KindFileDependency)
		}
		dependency, ok := violation.(filesassertion.DependencyViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a DependencyViolation", rule, violation)
		}
		offenders = append(offenders, dependency.File)
	}
	return offenders
}

// emptyTestViolations are the empty-test violations a rule reported, in the order it reported them, checking
// on the way that nothing else is among them.
func emptyTestViolations(t *testing.T, rule fluentapi.FilesDependencyCondition, violations []assertion.Violation) []assertion.EmptyTestViolation {
	t.Helper()

	reported := make([]assertion.EmptyTestViolation, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != assertion.KindEmptyTest {
			t.Errorf("%s reports a violation of kind %q, want %q", rule, kind, assertion.KindEmptyTest)
			continue
		}
		empty, ok := violation.(assertion.EmptyTestViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want an EmptyTestViolation", rule, violation)
		}
		reported = append(reported, empty)
	}
	return reported
}

// assertAllowEmptyTestsPasses checks the opt-out every empty-test guard has to honor: a user who really means
// a rule with an empty half says so on the check options, and the rule passes instead of reporting it.
func assertAllowEmptyTestsPasses(t *testing.T, rule fluentapi.FilesDependencyCondition) {
	t.Helper()

	allowed, err := rule.Check(&kernel.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("%s failed with AllowEmptyTests: %v", rule, err)
	}
	if len(allowed) != 0 {
		t.Errorf("%s reports %v with AllowEmptyTests, want the pass", rule, allowed)
	}
}
