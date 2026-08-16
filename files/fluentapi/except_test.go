package fluentapi_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

func TestExceptTakesAFolderBackOutOfTheScope(t *testing.T) {
	// The sentence the verb exists for: everything under `app/`, but not the generated folder, as one clause
	// rather than a rule about `**/generated` that says the opposite of what the team means.
	scope := fluentapi.ProjectFiles(fixtureLocator(t, writeExclusionFixtureProject(t))).InFolder("app/**")

	excepted := scope.Except("**/generated")

	want := []string{"app/api/handler.go", "app/api/handler_gen.go", "app/api/reader.go", "app/web/router.go"}
	if selected := selectedFiles(t, excepted); !slices.Equal(selected, want) {
		t.Errorf("%s selects %v, want %v", excepted, selected, want)
	}
	if selected := selectedFiles(t, scope); !slices.Contains(selected, "app/generated/schema.go") {
		t.Errorf("%s selects %v, want the generated file the exclusion is about: the fixture proves nothing", scope, selected)
	}
}

func TestAPlainExclusionOfTheScopeIsReadAgainstTheVerbItFollows(t *testing.T) {
	// A bare pattern is a second pattern of the same clause, so it looks at whatever part of an identifier the
	// verb it qualifies looks at: a folder after `in folder`, a name after `with name`.
	locator := fixtureLocator(t, writeExclusionFixtureProject(t))

	folder := fluentapi.ProjectFiles(locator).InFolder("app/**").Except("**/generated")
	name := fluentapi.ProjectFiles(locator).WithName("*.go").Except("*_gen.go")

	if selected := selectedFiles(t, folder); slices.Contains(selected, "app/generated/schema.go") {
		t.Errorf("%s selects %v, want the exclusion read as the folder its verb is about", folder, selected)
	}
	if selected := selectedFiles(t, name); slices.Contains(selected, "app/api/handler_gen.go") {
		t.Errorf("%s selects %v, want the exclusion read as the filename its verb is about", name, selected)
	}
}

func TestTheTargetedExclusionsOfTheScopeNameTheirOwnTarget(t *testing.T) {
	// The other half of the issue: an exclusion that is not about what its selector is about. Each of these
	// takes a different part of the identifier back out of a verb that reads another one.
	locator := fixtureLocator(t, writeExclusionFixtureProject(t))

	tests := []struct {
		name  string
		scope fluentapi.FilesBuilder
		want  []string
	}{
		{
			name:  "a folder verb excepting a filename",
			scope: fluentapi.ProjectFiles(locator).InFolder("app/**").ExceptWithName("*_gen.go"),
			want:  []string{"app/api/handler.go", "app/api/reader.go", "app/generated/schema.go", "app/web/router.go"},
		},
		{
			name:  "a filename verb excepting a folder",
			scope: fluentapi.ProjectFiles(locator).WithName("*.go").ExceptInFolder("app/**"),
			want:  []string{"internal/db/conn.go", "internal/db/dto/types.go", "main.go"},
		},
		{
			name:  "a folder verb excepting a whole path",
			scope: fluentapi.ProjectFiles(locator).InFolder("app/**").ExceptInPath("app/api/*_gen.go"),
			want:  []string{"app/api/handler.go", "app/api/reader.go", "app/generated/schema.go", "app/web/router.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if selected := selectedFiles(t, test.scope); !slices.Equal(selected, test.want) {
				t.Errorf("%s selects %v, want %v", test.scope, selected, test.want)
			}
		})
	}
}

func TestExclusionsOfTheScopeAccumulate(t *testing.T) {
	// Several patterns in one call and several calls are the same thing, because each exclusion vetoes on its
	// own: nothing here depends on the order they were typed in.
	locator := fixtureLocator(t, writeExclusionFixtureProject(t))

	together := fluentapi.ProjectFiles(locator).InFolder("app/**").Except("**/generated", "**/web")
	apart := fluentapi.ProjectFiles(locator).InFolder("app/**").Except("**/web").Except("**/generated")

	want := []string{"app/api/handler.go", "app/api/handler_gen.go", "app/api/reader.go"}
	if selected := selectedFiles(t, together); !slices.Equal(selected, want) {
		t.Errorf("%s selects %v, want %v", together, selected, want)
	}
	if selected := selectedFiles(t, apart); !slices.Equal(selected, want) {
		t.Errorf("%s selects %v, want %v", apart, selected, want)
	}
}

func TestAnExclusionQualifiesTheVerbItFollowsAndNotTheWholeScope(t *testing.T) {
	// A scope narrowed twice and then excepted once still means what it reads as: the exclusion belongs to the
	// clause it was written in, and the verbs before it are untouched.
	scope := fluentapi.ProjectFiles(nil).
		InFolder("app/**").
		WithName("*.go").
		Except("*_gen.go")

	selectors := scope.Selectors()

	if len(selectors) != 2 {
		t.Fatalf("Selectors() = %v, want the two verbs that were typed", selectors)
	}
	if rendered := selectors[0].String(); rendered != `path without filename matches "app/**"` {
		t.Errorf("the first verb reads %q, want it untouched by the exclusion", rendered)
	}
	if rendered := selectors[1].String(); rendered != `filename matches "*.go", excluding "*_gen.go"` {
		t.Errorf("the second verb reads %q, want the exclusion on the verb it followed", rendered)
	}
}

func TestAnExcludedScopeCanBeBranchedFrom(t *testing.T) {
	// A builder is a value and every verb hands back a new one, so two rules derived from one stored scope
	// cannot see each other's exclusions — the hazard being an exclusion slice with spare capacity behind both.
	scope := fluentapi.ProjectFiles(fixtureLocator(t, writeExclusionFixtureProject(t))).InFolder("app/**")

	generated := scope.Except("**/generated")
	web := scope.Except("**/web")

	if selected := selectedFiles(t, generated); !slices.Contains(selected, "app/web/router.go") {
		t.Errorf("%s selects %v, want the folder only its sibling excluded", generated, selected)
	}
	if selected := selectedFiles(t, web); !slices.Contains(selected, "app/generated/schema.go") {
		t.Errorf("%s selects %v, want the folder only its sibling excluded", web, selected)
	}
	if selected := selectedFiles(t, scope); len(selected) != 5 {
		t.Errorf("%s selects %v, want the five files it selected before either exclusion was derived", scope, selected)
	}
}

func TestExceptChangesWhoTheRuleIsAbout(t *testing.T) {
	// The whole pipeline with an exclusion in it: the generated folder is out of the scope, so the boundary it
	// crosses is not reported, while every file the exclusion did not name is judged exactly as before.
	locator := fixtureLocator(t, writeExclusionFixtureProject(t))
	boundary := func(scope fluentapi.FilesBuilder) fluentapi.FilesDependencyCondition {
		return scope.ShouldNot().DependOnFiles().InFolder("internal/db/**")
	}

	whole := boundary(fluentapi.ProjectFiles(locator).InFolder("app/**"))
	excepted := boundary(fluentapi.ProjectFiles(locator).InFolder("app/**").Except("**/generated"))

	violations, err := whole.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", whole, err)
	}
	want := []string{"app/api/handler.go", "app/api/handler_gen.go", "app/api/reader.go", "app/generated/schema.go"}
	if offenders := dependencyOffenders(t, whole, violations); !slices.Equal(offenders, want) {
		t.Fatalf("%s reports %v, want %v: the rule the exclusion is subtracted from", whole, offenders, want)
	}

	violations, err = excepted.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", excepted, err)
	}
	if offenders := dependencyOffenders(t, excepted, violations); !slices.Equal(offenders, want[:3]) {
		t.Errorf("%s reports %v, want %v", excepted, offenders, want[:3])
	}
}

func TestExceptOnTheObjectCarvesAHoleInABoundary(t *testing.T) {
	// The other half of the sentence: the api folder may not reach the database, except the data-transfer types
	// the team decided it may see. One rule with a documented hole, rather than two rules.
	locator := fixtureLocator(t, writeExclusionFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).
		InFolder("app/api/**").
		ShouldNot().
		DependOnFiles().
		InFolder("internal/db/**").
		Except("**/db/dto")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	want := []string{"app/api/handler.go", "app/api/handler_gen.go"}
	if offenders := dependencyOffenders(t, rule, violations); !slices.Equal(offenders, want) {
		t.Fatalf("%s reports %v, want %v: the file that only reaches the carve-out holds the rule", rule, offenders, want)
	}
	dependency, ok := violations[0].(filesassertion.DependencyViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a DependencyViolation", rule, violations[0])
	}
	if len(dependency.Required) != 1 || !strings.Contains(dependency.Required[0].String(), `excluding "**/db/dto"`) {
		t.Errorf("the violation carries %v, want the object as the user typed it, exclusion included", dependency.Required)
	}
}

func TestTheTargetedExclusionsOfTheObjectNameTheirOwnTarget(t *testing.T) {
	// The object's targeted variants, held to the same rule as the scope's: an exclusion may look at a part of
	// an identifier its own verb does not. Each of these carves the same hole out of the boundary a different way.
	locator := fixtureLocator(t, writeExclusionFixtureProject(t))
	api := func() fluentapi.FilesDependencyCondition {
		return fluentapi.ProjectFiles(locator).InFolder("app/api/**").ShouldNot().DependOnFiles()
	}

	tests := []struct {
		name string
		rule fluentapi.FilesDependencyCondition
	}{
		{name: "a folder verb excepting a filename", rule: api().InFolder("internal/db/**").ExceptWithName("types.go")},
		{name: "a filename verb excepting a folder", rule: api().WithName("*.go").ExceptInFolder("internal/db/dto")},
		{name: "a folder verb excepting a whole path", rule: api().InFolder("internal/db/**").ExceptInPath("internal/db/dto/*.go")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations, err := test.rule.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", test.rule, err)
			}
			want := []string{"app/api/handler.go", "app/api/handler_gen.go"}
			if offenders := dependencyOffenders(t, test.rule, violations); !slices.Equal(offenders, want) {
				t.Errorf("%s reports %v, want %v", test.rule, offenders, want)
			}
		})
	}
}

func TestExceptOnAThirdPartyRuleCarvesOutOneModule(t *testing.T) {
	// The third-party policy with a carve-out: no module of the outside world, except the one this project has
	// decided is part of its vocabulary.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).
		InFolder("internal/**").
		ShouldNot().
		DependOnExternalModules().
		Matching("**").
		Except("database/sql")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	want := []string{"internal/api/handler.go"}
	if offenders := externalOffenders(t, rule, violations); !slices.Equal(offenders, want) {
		t.Errorf("%s reports %v, want %v: the file whose only external module is the excluded one holds the rule", rule, offenders, want)
	}
}

func TestAnExclusionOfOneAlternativeLeavesTheOthersAlone(t *testing.T) {
	// `matching` is the one verb whose repetitions are combined with OR, and an exclusion qualifies the
	// alternative it follows rather than the list: the second `matching` still names what the first excluded.
	locator := fixtureLocator(t, writeExternalFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).
		InFolder("internal/db/**").
		ShouldNot().
		DependOnExternalModules().
		Matching("database/**").
		Except("database/sql").
		Matching("database/sql")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	want := []string{"internal/db/conn.go"}
	if offenders := externalOffenders(t, rule, violations); !slices.Equal(offenders, want) {
		t.Errorf("%s reports %v, want %v: the alternative typed after the exclusion is not qualified by it", rule, offenders, want)
	}
}

func TestARejectedExclusionOfTheScopeIsAUserErrorNamingTheExceptVerb(t *testing.T) {
	// Every way an exclusion can be typed wrongly: a pattern this library cannot understand, an exclusion with
	// nothing to qualify, and an exclusion with no pattern at all — a verb the user typed that narrows nothing
	// is reported rather than quietly kept, for the reason an empty test is.
	tests := []struct {
		name    string
		scope   fluentapi.FilesBuilder
		verb    string
		subject string
		cause   error
	}{
		{
			name:    "a pattern that will not compile",
			scope:   fluentapi.ProjectFiles(nil).InFolder("app/**").Except("[unclosed"),
			verb:    "except",
			subject: "[unclosed",
			cause:   matching.ErrInvalidPattern,
		},
		{
			name:    "a targeted pattern that will not compile",
			scope:   fluentapi.ProjectFiles(nil).InFolder("app/**").ExceptWithName("*.go", "[unclosed"),
			verb:    "except with name",
			subject: "*.go, [unclosed",
			cause:   matching.ErrInvalidPattern,
		},
		{
			name:    "an exclusion with nothing to qualify",
			scope:   fluentapi.ProjectFiles(nil).Except("**/generated"),
			verb:    "except",
			subject: "**/generated",
			cause:   matching.ErrExclusionWithoutSelector,
		},
		{
			name:    "an exclusion with no pattern",
			scope:   fluentapi.ProjectFiles(nil).InFolder("app/**").ExceptInPath(),
			verb:    "except in path",
			subject: "",
			cause:   matching.ErrExclusionWithoutPattern,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.scope.SelectFiles(nil)

			assertRejectedExclusion(t, test.scope, err, test.verb, test.subject, test.cause)
		})
	}
}

func TestARejectedExclusionOfAnObjectIsAUserErrorNamingTheExceptVerb(t *testing.T) {
	// The same three mistakes one stage later, on both object stages: the rejection joins the scope, so the
	// terminal reports it in the words of the verb the user typed rather than in the scope's.
	tests := []struct {
		name    string
		rule    reportingRule
		verb    string
		subject string
		cause   error
	}{
		{
			name:    "a pattern of an object verb that will not compile",
			rule:    fluentapi.ProjectFiles(nil).ShouldNot().DependOnFiles().InFolder("internal/db/**").ExceptInFolder("[unclosed"),
			verb:    "except in folder",
			subject: "[unclosed",
			cause:   matching.ErrInvalidPattern,
		},
		{
			name:    "an exclusion of an object verb that was never typed",
			rule:    fluentapi.ProjectFiles(nil).ShouldNot().DependOnFiles().Except("**/dto"),
			verb:    "except",
			subject: "**/dto",
			cause:   matching.ErrExclusionWithoutSelector,
		},
		{
			name:    "an exclusion of an object verb with no pattern",
			rule:    fluentapi.ProjectFiles(nil).Should().DependOnFiles().InPath("internal/**").Except(),
			verb:    "except",
			subject: "",
			cause:   matching.ErrExclusionWithoutPattern,
		},
		{
			name:    "a pattern of a module verb that will not compile",
			rule:    fluentapi.ProjectFiles(nil).ShouldNot().DependOnExternalModules().Matching("*.*/**").Except("[unclosed"),
			verb:    "except",
			subject: "[unclosed",
			cause:   matching.ErrInvalidPattern,
		},
		{
			name:    "an exclusion of a module verb that was never typed",
			rule:    fluentapi.ProjectFiles(nil).ShouldNot().DependOnExternalModules().Except("gorm.io/**"),
			verb:    "except",
			subject: "gorm.io/**",
			cause:   matching.ErrExclusionWithoutSelector,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations, err := test.rule.Check(nil)

			assertRejectedExclusion(t, test.rule, err, test.verb, test.subject, test.cause)
			if violations != nil {
				t.Errorf("Check reports %v beside the error, want nothing said about the rule", violations)
			}
		})
	}
}

func TestARejectedExclusionDoesNotWidenTheRule(t *testing.T) {
	// The rejection is deferred, so the chain goes on being typed: the verb the exclusion qualified must not
	// silently become a rule about a selection nobody asked for, and the pattern the user has to fix first is
	// the one reported.
	scope := fluentapi.ProjectFiles(nil).InFolder("app/**").Except("[unclosed").WithName("*.go")

	selectors := scope.Selectors()

	if len(selectors) != 2 {
		t.Fatalf("Selectors() = %v, want the two verbs that compiled and no exclusion at all", selectors)
	}
	if rendered := selectors[0].String(); strings.Contains(rendered, "excluding") {
		t.Errorf("the verb the rejected exclusion qualified reads %q, want it unchanged", rendered)
	}

	_, err := scope.SelectFiles(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) || user.Operation != "except" {
		t.Errorf("SelectFiles error = %v, want the UserError of the exclusion that was rejected", err)
	}
}

func TestAnExclusionIsNotAMood(t *testing.T) {
	// An exclusion always takes files out of the selection, so it reads the same in both moods and widens
	// neither: what it removed is judged by neither of them.
	locator := fixtureLocator(t, writeExclusionFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("app/**").Except("**/generated")

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
		t.Errorf("%s reports %v and %s reports %v, want the %d files the exclusion left split between them",
			positive, held, negated, broken, len(selected))
	}
	judged := slices.Concat(dependencyOffenders(t, positive, held), dependencyOffenders(t, negated, broken))
	if slices.Contains(judged, "app/generated/schema.go") {
		t.Errorf("%s judged %v, and the file the exclusion removed is among them", scope, judged)
	}
}

func TestAnExclusionRendersInTheSentence(t *testing.T) {
	// A rule prints as the sentence the user typed, exclusions included, because that string is what a failing
	// test shows and a reader has to be able to see the carve-out in it.
	rule := fluentapi.ProjectFiles(nil).
		InFolder("app/**").
		Except("**/generated").
		ExceptWithName("*_gen.go").
		ShouldNot().
		DependOnFiles().
		InFolder("internal/db/**").
		Except("**/db/dto")

	want := `project files, path without filename matches "app/**", excluding "**/generated", ` +
		`excluding filename matches "*_gen.go", should not, depend on files, ` +
		`path without filename matches "internal/db/**", excluding "**/db/dto"`
	if rendered := rule.String(); rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
}

// reportingRule is a terminal that also renders itself, which is what a table of rejected exclusions needs of
// the two object stages: the assertions are the same for both, and neither is reached through the other.
type reportingRule interface {
	kernel.Checkable
	String() string
}

// assertRejectedExclusion is the one shape a rejected exclusion has, at either level: a UserError naming the
// `except` verb the user typed, quoting the patterns as they were written, wrapping the reason the exclusion
// was refused, and visible in the sentence the rule renders as.
func assertRejectedExclusion(t *testing.T, rule fmt.Stringer, err error, verb, subject string, cause error) {
	t.Helper()

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("the terminal's error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != verb {
		t.Errorf("UserError.Operation = %q, want the verb %q", user.Operation, verb)
	}
	if user.Subject != subject {
		t.Errorf("UserError.Subject = %q, want the patterns as the user typed them, %q", user.Subject, subject)
	}
	if !errors.Is(err, cause) {
		t.Errorf("the terminal's error = %v, want it to wrap %v", err, cause)
	}
	if rendered := rule.String(); !strings.Contains(rendered, "rejected") {
		t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
	}
}

// writeExclusionFixtureProject writes the project the `except` companion is about: a folder a rule is written
// over, `app/`, holding a generated folder and a generated file that a rule about the folder would otherwise
// judge, and a database folder holding the data-transfer package an exclusion on the object carves out.
//
//	app/api/handler.go      -> internal/db          the boundary crossing a rule is meant to report
//	app/api/handler_gen.go  -> internal/db          the same crossing, in a generated file
//	app/api/reader.go       -> internal/db/dto      the crossing an exclusion on the object allows
//	app/generated/schema.go -> internal/db          the crossing an exclusion on the scope removes
//	app/web/router.go                               a selected file that reaches nothing
func writeExclusionFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                   "module example.com/exclusion\n\ngo 1.26\n",
		"main.go":                  "package main\n\nimport \"example.com/exclusion/app/api\"\n\nfunc main() { api.Handle() }\n",
		"app/api/handler.go":       "package api\n\nimport \"example.com/exclusion/internal/db\"\n\nfunc Handle() { db.Connect() }\n",
		"app/api/handler_gen.go":   "package api\n\nimport \"example.com/exclusion/internal/db\"\n\nfunc Generated() { db.Connect() }\n",
		"app/api/reader.go":        "package api\n\nimport \"example.com/exclusion/internal/db/dto\"\n\nfunc Read() dto.Row { return dto.Row{} }\n",
		"app/generated/schema.go":  "package generated\n\nimport \"example.com/exclusion/internal/db\"\n\nfunc Schema() { db.Connect() }\n",
		"app/web/router.go":        "package web\n\nfunc Route() {}\n",
		"internal/db/conn.go":      "package db\n\nfunc Connect() {}\n",
		"internal/db/dto/types.go": "package dto\n\ntype Row struct{}\n",
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
