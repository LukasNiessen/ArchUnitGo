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
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

// A built rule is a Checkable and nothing else has to be known about it, so the interface is asserted at
// compile time rather than in a test that could be deleted.
var _ kernel.Checkable = fluentapi.FilesExternalDependencyCondition{}

func TestDependOnExternalModulesInTheNegatedMoodReportsTheFilesThatLeaveTheProject(t *testing.T) {
	// The third-party policy through the whole pipeline: a project on disk, located, extracted, the modules it
	// depends on discovered from the graph, and one violation per selected file that reaches a forbidden one.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api/**").ShouldNot().DependOnExternalModules().Matching("net/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := externalOffenders(t, rule, violations); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Fatalf("%s reports %v, want the one file of that folder that reaches such a module", rule, offenders)
	}
	dependency, ok := violations[0].(filesassertion.ExternalDependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an ExternalDependencyViolation", rule, violations[0])
	}
	if !slices.Equal(dependency.Modules, []string{"net/http"}) {
		t.Errorf("the violation carries %v, want the import path it was broken by", dependency.Modules)
	}
	if len(dependency.Required) != 1 || dependency.Required[0].Pattern().Source() != "net/**" {
		t.Errorf("the violation carries %v, want the object as the user typed it", dependency.Required)
	}
	if dependency.Required[0].Target() != matching.TargetPath {
		t.Errorf("the object was matched against the %s, want the whole import path", dependency.Required[0].Target())
	}
	if dependency.Mood != assertion.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want the rule's own %s", dependency.Mood, assertion.ShouldNot)
	}
	want := `internal/api/handler.go: should not, depend on external modules, path matches "net/**" -> net/http`
	if rendered := dependency.String(); rendered != want {
		t.Errorf("the violation reads %q, want %q", rendered, want)
	}
}

func TestDependOnExternalModulesInThePositiveMoodRequiresEachSelectedFileToReachOne(t *testing.T) {
	// `should depend on external modules` is satisfied per file, existentially: the handler imports net/http and
	// holds the rule, and the file of the same folder that imports nothing outside the project is the offender —
	// carrying no module, because the absence is the offense.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api/**").Should().DependOnExternalModules().Matching("net/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := externalOffenders(t, rule, violations); !slices.Equal(offenders, []string{"internal/api/router.go"}) {
		t.Fatalf("%s reports %v, want the one file of that folder that reaches no such module", rule, offenders)
	}
	dependency, ok := violations[0].(filesassertion.ExternalDependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an ExternalDependencyViolation", rule, violations[0])
	}
	if len(dependency.Modules) != 0 {
		t.Errorf("the violation carries %v, want none: it was reported for reaching nothing", dependency.Modules)
	}
	want := `internal/api/router.go: should, depend on external modules, path matches "net/**" -> nothing`
	if rendered := dependency.String(); rendered != want {
		t.Errorf("the violation reads %q, want %q", rendered, want)
	}
}

func TestDependOnExternalModulesInTheTwoMoodsPartitionsTheSelection(t *testing.T) {
	// One rule and its negation over one stored scope: every selected file offends exactly one of them, which is
	// the property that lets the two moods share a single assertion.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**")

	positive := scope.Should().DependOnExternalModules().Matching("net/**")
	negated := scope.ShouldNot().DependOnExternalModules().Matching("net/**")

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

func TestTheObjectVerbsOfAThirdPartyRuleAreChainedWithOr(t *testing.T) {
	// The one place in this library where chaining widens: a policy is a list of alternatives, so each `matching`
	// adds a module the rule is about and the order of the calls cannot matter. A module cannot be two modules at
	// once, which is why ANDing them would name the empty set instead.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**")

	either := scope.ShouldNot().DependOnExternalModules().Matching("net/**").Matching("database/**")
	reversed := scope.ShouldNot().DependOnExternalModules().Matching("database/**").Matching("net/**")
	one := scope.ShouldNot().DependOnExternalModules().Matching("net/**")

	widened, err := either.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", either, err)
	}
	want := []string{"internal/api/handler.go", "internal/db/conn.go"}
	if offenders := externalOffenders(t, either, widened); !slices.Equal(offenders, want) {
		t.Errorf("%s reports %v, want the file that reaches either module: %v", either, offenders, want)
	}
	other, err := reversed.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", reversed, err)
	}
	if !slices.Equal(externalOffenders(t, reversed, other), externalOffenders(t, either, widened)) {
		t.Errorf("%s reports %v and %s reports %v, want the order of the object verbs not to matter",
			reversed, other, either, widened)
	}
	// And one verb alone is the narrower rule, which is what makes the second one a widening rather than a filter.
	narrower, err := one.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", one, err)
	}
	if offenders := externalOffenders(t, one, narrower); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Errorf("%s reports %v, want only the file that reaches that one module", one, offenders)
	}
}

func TestDependOnExternalModulesWithNoObjectVerbIsEveryModuleTheProjectDependsOn(t *testing.T) {
	// `should not depend on external modules` with nothing chained onto it is a file that must import nothing but
	// its own project — the strictest reading, and a meaningful one for a domain package. The standard library is
	// among the modules, because what external means was settled in extraction and is not re-decided here.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api").ShouldNot().DependOnExternalModules()

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := externalOffenders(t, rule, violations); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Fatalf("%s reports %v, want the file of that folder that imports something outside the project", rule, offenders)
	}
	if rendered := rule.String(); rendered != `project files, path without filename matches "internal/api", should not, depend on external modules` {
		t.Errorf("String() = %q, want the predicate with no object clause after it", rendered)
	}
	// And the positive mood of the same object is the file that imports nothing outside the project at all.
	leaf := fluentapi.ProjectFiles(locator).InFolder("internal/api").Should().DependOnExternalModules()
	reported, err := leaf.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", leaf, err)
	}
	if offenders := externalOffenders(t, leaf, reported); !slices.Equal(offenders, []string{"internal/api/router.go"}) {
		t.Errorf("%s reports %v, want the file of that folder that imports nothing outside the project", leaf, offenders)
	}
}

func TestAThirdPartyRulePassesWhenTheProjectDependsOnNoneOfTheModulesItNames(t *testing.T) {
	// The one place this family departs from `depend on files`: its object is not put through the empty-test
	// guard. A pattern that matched no module and a project that imports no such module are one statement here,
	// and for the negated mood that statement is exactly the pass — a guard would fail every project that obeys
	// its own third-party policy, which is the opposite of what the guard is for.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))

	tests := []struct {
		name string
		rule fluentapi.FilesExternalDependencyCondition
	}{
		{
			name: "a module the project does not depend on",
			rule: fluentapi.ProjectFiles(locator).InFolder("internal/**").ShouldNot().
				DependOnExternalModules().Matching("github.com/deprecated/**"),
		},
		{
			// The documented idiom for "third-party, but not the standard library": a first segment with a dot in
			// it is a domain. This fixture has no third-party dependency at all, so the whole policy holds.
			name: "every third-party module, of which this project has none",
			rule: fluentapi.ProjectFiles(locator).ShouldNot().DependOnExternalModules().Matching("*.*/**"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations, err := test.rule.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", test.rule, err)
			}
			if len(violations) != 0 {
				t.Errorf("%s reports %v, want the pass: nothing of the project reaches such a module", test.rule, violations)
			}
		})
	}
}

func TestAThirdPartyRuleOfThePositiveMoodIsLoudWhenItNamesAModuleNobodyImports(t *testing.T) {
	// The other half of that decision, and the reason it costs nothing: `should depend on external modules
	// matching "github.com/approved/**"` over a project that imports no such module reports every selected file,
	// so the mood in which an unmatched object could hide a mistake is the mood that shouts about it anyway.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/api/**")

	rule := scope.Should().DependOnExternalModules().Matching("github.com/approved/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := externalOffenders(t, rule, violations); !slices.Equal(offenders, selectedFiles(t, scope)) {
		t.Errorf("%s reports %v, want every selected file: %v", rule, offenders, selectedFiles(t, scope))
	}
}

func TestAThirdPartyRuleReportsAScopeThatSelectedNoFile(t *testing.T) {
	// The half of the empty-test guard that is real for this family: a stale scope has no file to judge, so one
	// mood of the rule would pass forever and the other would be red for nothing a reader could go and look at.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/apis/**").ShouldNot().DependOnExternalModules().Matching("net/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	empty := externalEmptyTestViolations(t, rule, violations)
	if len(empty) != 1 {
		t.Fatalf("%s reports %v, want the one empty-test violation of a stale scope", rule, violations)
	}
	if empty[0].Subject != "files" {
		t.Errorf("the violation says the rule selected no %q, want `files`", empty[0].Subject)
	}
	if len(empty[0].Selectors) != 1 || empty[0].Selectors[0].Pattern().Source() != "internal/apis/**" {
		t.Errorf("the violation carries %v, want the scope verb that selected nothing", empty[0].Selectors)
	}

	allowed, err := rule.Check(&kernel.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("%s failed with AllowEmptyTests: %v", rule, err)
	}
	if len(allowed) != 0 {
		t.Errorf("%s reports %v with AllowEmptyTests, want the pass", rule, allowed)
	}
}

func TestAThirdPartyRuleThreadsTheCheckOptionsIntoTheExtraction(t *testing.T) {
	// Every knob but AllowEmptyTests reaches the rule through the extraction, so a terminal judging a
	// differently-extracted project would hold the rule over files the user did not ask about, silently.
	// IncludeTestFiles is the cheapest to observe: the fixture's test file is the only one that imports
	// `testing`, so it is an offender of the negated mood exactly when it is a node at all.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))
	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api").ShouldNot().DependOnExternalModules().Matching("testing")

	byDefault, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	withTests, err := rule.Check(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("%s failed with IncludeTestFiles: %v", rule, err)
	}

	if len(byDefault) != 0 {
		t.Errorf("%s reports %v by default, want the pass: no production file of that folder imports it", rule, byDefault)
	}
	if offenders := externalOffenders(t, rule, withTests); !slices.Equal(offenders, []string{"internal/api/handler_test.go"}) {
		t.Errorf("%s reports %v with IncludeTestFiles, want the test file among the selection", rule, offenders)
	}
}

func TestAThirdPartyRuleReportsAPatternItsObjectVerbRejected(t *testing.T) {
	// A pattern this library cannot understand is the user's error, deferred to the terminal because a fluent
	// method has nowhere to put one — and it names the object verb the user has to go and fix, exactly as the
	// object verbs of `depend on files` do.
	rule := fluentapi.ProjectFiles(nil).ShouldNot().DependOnExternalModules().Matching("[unclosed")

	violations, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "matching" {
		t.Errorf("UserError.Operation = %q, want the object verb `matching`", user.Operation)
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
	if rendered := rule.String(); !strings.Contains(rendered, "rejected") {
		t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
	}
}

func TestAThirdPartyRuleReportsTheFirstRejectedPatternOfTheWholeChain(t *testing.T) {
	// The rejection joins the scope, so the pattern the user has to fix first is the one reported however many
	// stages later the second typo sits — and a rejected object verb widens nothing, so the rule does not
	// quietly become one about a module nobody typed.
	rule := fluentapi.ProjectFiles(nil).InFolder("[unclosed").ShouldNot().DependOnExternalModules().Matching("[alsounclosed")

	_, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" || user.Subject != "[unclosed" {
		t.Errorf("UserError = %v, want the first rejected pattern of the chain", user)
	}
}

func TestAThirdPartyRuleRejectsALocatorThatIsNotAProject(t *testing.T) {
	// Nothing is read while a rule is built, so a locator naming no Go project is the terminal's error and not
	// the entry point's.
	locator := &extraction.ProjectLocator{Directory: t.TempDir()}
	rule := fluentapi.ProjectFiles(locator).ShouldNot().DependOnExternalModules().Matching("*.*/**")

	violations, err := rule.Check(nil)

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Fatalf("Check error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
	if violations != nil {
		t.Errorf("Check reports %v beside the error, want nothing said about the rule", violations)
	}
}

func TestAThirdPartyConditionIsImmutableAndCanBeBranchedFrom(t *testing.T) {
	// The object stage is a stage like every other one, so a half-built rule stored in a variable can be branched
	// into two objects and is unchanged by either. The base carries three object verbs, because that is the length
	// at which append leaves spare capacity behind — a shared array the second branch would write the first
	// branch's object out of, if the verbs were not cloned.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))
	base := fluentapi.ProjectFiles(locator).InFolder("internal/**").ShouldNot().DependOnExternalModules().
		Matching("github.com/deprecated/**").Matching("gopkg.in/**").Matching("example.org/**")

	web := base.Matching("net/**")
	database := base.Matching("database/**")

	broken, err := web.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", web, err)
	}
	if offenders := externalOffenders(t, web, broken); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Errorf("the first branch reports %v, want the file that imports net/http", offenders)
	}
	other, err := database.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", database, err)
	}
	if offenders := externalOffenders(t, database, other); !slices.Equal(offenders, []string{"internal/db/conn.go"}) {
		t.Errorf("the second branch reports %v, want the file that imports database/sql", offenders)
	}
	// And the rule the two were branched from is still the object it was, which for this family is the pass:
	// none of the three modules it names is imported anywhere in the fixture.
	if rendered := base.String(); strings.Contains(rendered, "net/**") || strings.Contains(rendered, "database/**") {
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
	if len(first) != 0 || len(second) != 0 {
		t.Errorf("%s reports %v and then %v, want the pass both times", base, first, second)
	}
}

func TestAThirdPartyRuleRendersTheWholeSentence(t *testing.T) {
	// The whole grammar in one line — entry, scope, mood, predicate, object — with the scope verbs rendering as
	// their filters and the object's alternatives joined with `or`, because that join is the difference between
	// this family and `depend on files` and a sentence that hid it would read as a requirement no module could
	// ever meet.
	tests := []struct {
		rule fluentapi.FilesExternalDependencyCondition
		want string
	}{
		{
			rule: fluentapi.ProjectFiles(nil).InFolder("internal/domain/**").ShouldNot().DependOnExternalModules().Matching("*.*/**"),
			want: `project files, path without filename matches "internal/domain/**", should not, depend on external modules, path matches "*.*/**"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).InFolder("internal/adapter/**").Should().DependOnExternalModules().Matching("gorm.io/**"),
			want: `project files, path without filename matches "internal/adapter/**", should, depend on external modules, path matches "gorm.io/**"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).ShouldNot().DependOnExternalModules().
				Matching("github.com/deprecated/**").Matching("gopkg.in/**"),
			want: `project files, should not, depend on external modules, path matches "github.com/deprecated/**" or path matches "gopkg.in/**"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).InFile("main.go").Should().DependOnExternalModules(),
			want: `project files, path matches "main.go", should, depend on external modules`,
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

func TestDependOnExternalModulesIsOnBothMoodsAndSpelledOneWay(t *testing.T) {
	// The third-party predicate has a meaningful negation — a module can be required or forbidden — so it is on
	// both moods, and neither mood has a second spelling of it. Its object verb is one word, and `matching` is
	// that word in every port of the library. Checked on the method set, because a reviewer's memory is not a
	// check.
	moods := []struct {
		name    string
		methods []string
	}{
		{name: "`should`", methods: methodsOf(fluentapi.ProjectFiles(nil).Should())},
		{name: "`should not`", methods: methodsOf(fluentapi.ProjectFiles(nil).ShouldNot())},
	}
	// A synonym is how a fluent API grows two ways to say one thing, and these are the words that would be
	// reached for: the predicate is `depend on external modules` and nothing else.
	synonyms := []string{
		"DependOnExternalModule", "DependOnExternalPackages", "DependOnThirdPartyModules", "DependOnExternals",
		"DependsOnExternalModules", "ImportExternalModules", "UseExternalModules", "HaveExternalDependencies",
		"OnlyDependOnExternalModules", "DependOnModules",
	}

	for _, mood := range moods {
		t.Run(mood.name, func(t *testing.T) {
			if !slices.Contains(mood.methods, "DependOnExternalModules") {
				t.Errorf("%s offers %v, want DependOnExternalModules among them", mood.name, mood.methods)
			}
			for _, method := range mood.methods {
				if slices.Contains(synonyms, method) {
					t.Errorf("%s offers %s, want the predicate spelled one way", mood.name, method)
				}
			}
		})
	}

	object := methodsOf(fluentapi.ProjectFiles(nil).ShouldNot().DependOnExternalModules())
	for _, verb := range []string{"Matching", "Check"} {
		if !slices.Contains(object, verb) {
			t.Errorf("the object stage offers %v, want %s among them", object, verb)
		}
	}
	// The object of this family is a set of import paths, so the scope's own verbs are deliberately not on it: a
	// module has no folder and no filename, and `with name` here would read as a rule about a file.
	for _, method := range object {
		if slices.Contains([]string{"WithName", "InFolder", "InPath", "InFile", "Named", "OrMatching"}, method) {
			t.Errorf("the object stage offers %s, want `matching` as its one verb", method)
		}
	}
}

// writeExternalFixtureProject writes a project that depends on nobody else's code but the standard library,
// which is exactly what a rule about external modules needs and all it needs: `fmt`, `net/http`,
// `database/sql` and `testing` are dependencies this project does not own, so the extractor marks them
// external, and the fixture stays a directory of source files with no module to fetch.
//
// It is written here rather than shared with writeFixtureProject because the shape a third-party rule needs is
// its own: one file per module, one file that imports nothing outside the project — internal/api/router.go,
// the offender of the positive mood — and one import that is only in a test file, so IncludeTestFiles can be
// observed. No file imports a module with a dot in its first segment, which is what makes `matching "*.*/**"`
// the pass this family's object is not guarded for.
func writeExternalFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                       "module example.com/external\n\ngo 1.26\n",
		"main.go":                      "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/external/internal/api\"\n)\n\nfunc main() { fmt.Println(api.Handle()) }\n",
		"internal/api/handler.go":      "package api\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/external/internal/db\"\n)\n\nfunc Handle() int { db.Connect(); return http.StatusOK }\n",
		"internal/api/router.go":       "package api\n\nfunc Route() {}\n",
		"internal/api/handler_test.go": "package api\n\nimport \"testing\"\n\nfunc TestHandle(*testing.T) { Handle() }\n",
		"internal/db/conn.go":          "package db\n\nimport \"database/sql\"\n\nvar handle *sql.DB\n\nfunc Connect() {}\n",
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

// externalOffenders names the files a third-party dependency rule reported, in the order it reported them,
// checking on the way that every violation is an ExternalDependencyViolation of the file-external-dependency
// kind.
func externalOffenders(t *testing.T, rule fluentapi.FilesExternalDependencyCondition, violations []assertion.Violation) []string {
	t.Helper()

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != filesassertion.KindFileExternalDependency {
			t.Errorf("%s reports a violation of kind %q, want %q", rule, kind, filesassertion.KindFileExternalDependency)
		}
		dependency, ok := violation.(filesassertion.ExternalDependencyViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want an ExternalDependencyViolation", rule, violation)
		}
		offenders = append(offenders, dependency.File)
	}
	return offenders
}

// externalEmptyTestViolations are the empty-test violations a third-party rule reported, in the order it
// reported them, checking on the way that nothing else is among them.
func externalEmptyTestViolations(
	t *testing.T,
	rule fluentapi.FilesExternalDependencyCondition,
	violations []assertion.Violation,
) []assertion.EmptyTestViolation {
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
