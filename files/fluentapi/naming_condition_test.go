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
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

// A built rule is a Checkable and nothing else has to be known about it, so the interface is asserted at
// compile time rather than in a test that could be deleted.
var _ kernel.Checkable = fluentapi.FilesNamingCondition{}

func TestANamingRuleReportsAScopeThatSelectedNoFile(t *testing.T) {
	// Every file of an empty selection is named and placed exactly as any rule requires, in either mood, so a
	// rule with no subject would pass forever. The empty-test guard answers instead of the requirement, and
	// AllowEmptyTests is how a user who means an empty selection opts out.
	locator := fixtureLocator(t, writeFixtureProject(t))
	rules := []fluentapi.FilesNamingCondition{
		fluentapi.ProjectFiles(locator).InFolder("internal/apis/**").Should().HaveName("*.go"),
		fluentapi.ProjectFiles(locator).InFolder("internal/apis/**").ShouldNot().BeInFolder("internal/**"),
		fluentapi.ProjectFiles(locator).WithName("*.java").Should().BeInPath("**/*.java"),
	}

	for _, rule := range rules {
		t.Run(rule.String(), func(t *testing.T) {
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
				t.Errorf("the violation carries %v, want the scope verb that selected nothing", empty.Selectors)
			}

			allowed, err := rule.Check(&kernel.CheckOptions{AllowEmptyTests: true})
			if err != nil {
				t.Fatalf("%s failed with AllowEmptyTests: %v", rule, err)
			}
			if len(allowed) != 0 {
				t.Errorf("%s reports %v with AllowEmptyTests, want the pass", rule, allowed)
			}
		})
	}
}

func TestANamingRuleThreadsTheCheckOptionsIntoTheExtraction(t *testing.T) {
	// AllowEmptyTests travels its own way to the guard, so every other knob on the bag reaches the rule
	// through the extraction — and a terminal judging a differently-extracted project would hold the rule
	// over files the user did not ask about, silently. IncludeTestFiles is the cheapest to observe: the
	// fixture's test file is a node only when it is on, so the offenders of one requirement over one folder
	// grow by exactly that file when it is set.
	locator := fixtureLocator(t, writeFixtureProject(t))
	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api").Should().HaveName("handler.go")

	byDefault, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	withTests, err := rule.Check(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("%s failed with IncludeTestFiles: %v", rule, err)
	}

	// By default the folder holds two files, one of which is the handler the requirement asks for.
	if offenders := namedOffenders(t, rule, byDefault); !slices.Equal(offenders, []string{"internal/api/router.go"}) {
		t.Errorf("%s reports %v by default, want only the file of that folder that is not the handler", rule, offenders)
	}
	// With the test files included the same folder holds a third file, and it breaks the requirement too — so
	// a terminal that extracted with the default options instead of the user's would miss it here.
	want := []string{"internal/api/handler_test.go", "internal/api/router.go"}
	if offenders := namedOffenders(t, rule, withTests); !slices.Equal(offenders, want) {
		t.Errorf("%s reports %v with IncludeTestFiles, want %v — the test file among the selection", rule, offenders, want)
	}
}

func TestANamingRuleReportsAPatternThePredicateRejected(t *testing.T) {
	// A pattern this library cannot understand is the user's error, deferred to the terminal because a fluent
	// method has nowhere to put one — and it is an error rather than a violation, so a suite cannot read it as
	// a rule that holds. It names the predicate the user has to go and fix, exactly as a scope verb's does.
	tests := []struct {
		verb string
		rule fluentapi.FilesNamingCondition
	}{
		{verb: "have name", rule: fluentapi.ProjectFiles(nil).Should().HaveName("[unclosed")},
		{verb: "be in folder", rule: fluentapi.ProjectFiles(nil).Should().BeInFolder("[unclosed")},
		{verb: "be in path", rule: fluentapi.ProjectFiles(nil).ShouldNot().BeInPath("[unclosed")},
	}

	for _, test := range tests {
		t.Run(test.verb, func(t *testing.T) {
			violations, err := test.rule.Check(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Check error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the predicate %q", user.Operation, test.verb)
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

func TestANamingRuleReportsTheFirstRejectedPatternOfTheWholeChain(t *testing.T) {
	// The rejection joins the scope, so the rule the user has to fix first is the one reported however many
	// stages later the second typo sits. A predicate that rejected its pattern also selects nothing extra:
	// the requirement is left unset and no check reaches it.
	rule := fluentapi.ProjectFiles(nil).InFolder("[unclosed").Should().HaveName("[alsounclosed")

	_, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the first rejected verb, `in folder`", user.Operation)
	}
	if user.Subject != "[unclosed" {
		t.Errorf("UserError.Subject = %q, want the first rejected pattern", user.Subject)
	}
}

func TestANamingRuleRejectsALocatorThatIsNotAProject(t *testing.T) {
	// Nothing is read while a rule is built, so a locator naming no Go project is the terminal's error and
	// not the entry point's.
	locator := &extraction.ProjectLocator{Directory: t.TempDir()}
	rule := fluentapi.ProjectFiles(locator).Should().HaveName("*.go")

	violations, err := rule.Check(nil)

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Fatalf("Check error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
	if violations != nil {
		t.Errorf("Check reports %v beside the error, want nothing said about the rule", violations)
	}
}

func TestAFilesNamingConditionCanBeStoredAndCheckedTwice(t *testing.T) {
	// A rule is a value: built once from a stored scope, checked as often as it is useful, and the same
	// answer every time.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**")

	rule := scope.Should().HaveName("handler*.go")

	first, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	second, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed on the second check: %v", rule, err)
	}
	if !slices.Equal(namedOffenders(t, rule, first), namedOffenders(t, rule, second)) {
		t.Errorf("%s reports %v and then %v, want the same answer both times", rule, first, second)
	}
	// The scope it was built from is untouched, so another rule can still be branched from it.
	if selectors := scope.Selectors(); len(selectors) != 1 {
		t.Errorf("the stored scope's Selectors() = %v, want the one verb it was built with", selectors)
	}
}

func TestANamingRuleRendersTheWholeSentence(t *testing.T) {
	// The scope verbs render as their filters, because a reader needs to see which part of an identifier a
	// pattern selected on; the predicate renders as the words the user typed, because after the mood the
	// sentence has to read as English.
	tests := []struct {
		rule fluentapi.FilesNamingCondition
		want string
	}{
		{
			rule: fluentapi.ProjectFiles(nil).InFolder("internal/**").Should().HaveName("*.go"),
			want: `project files, path without filename matches "internal/**", should, have name "*.go"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).WithName("*_handler.go").Should().BeInFolder("internal/api/**"),
			want: `project files, filename matches "*_handler.go", should, be in folder "internal/api/**"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).ShouldNot().BeInPath("**/legacy/**"),
			want: `project files, should not, be in path "**/legacy/**"`,
		},
		{
			rule: fluentapi.ProjectFiles(nil).ShouldNot().HaveName("*_deprecated.go"),
			want: `project files, should not, have name "*_deprecated.go"`,
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

func TestTheThreeNamingPredicatesAreOnBothMoodsAndSpelledOneWay(t *testing.T) {
	// Each of the three has a meaningful negation — a name or a place can be required or forbidden — so each
	// is on both moods, and neither mood has a second spelling of any of them. Checked on the method set,
	// because a reviewer's memory is not a check.
	predicates := []string{"HaveName", "BeInFolder", "BeInPath"}
	stages := []struct {
		name    string
		methods []string
	}{
		{name: "`should`", methods: methodsOf(fluentapi.ProjectFiles(nil).Should())},
		{name: "`should not`", methods: methodsOf(fluentapi.ProjectFiles(nil).ShouldNot())},
	}
	// A synonym is how a fluent API grows two ways to say one thing, and these are the words that would be
	// reached for: the vocabulary is `have name`, `be in folder`, `be in path` and nothing else.
	synonyms := []string{
		"HaveNameMatching", "BeNamed", "HaveFilename", "MatchName", "WithName",
		"BeInDirectory", "LiveInFolder", "ResideInFolder", "BeInFolderMatching", "InFolder",
		"HavePath", "MatchPath", "BeInPathMatching", "InPath",
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			for _, predicate := range predicates {
				if !slices.Contains(stage.methods, predicate) {
					t.Errorf("%s offers %v, want %s among them", stage.name, stage.methods, predicate)
				}
			}
			for _, method := range stage.methods {
				if slices.Contains(synonyms, method) {
					t.Errorf("%s offers %s, want the three predicates spelled one way each", stage.name, method)
				}
			}
		})
	}
}
