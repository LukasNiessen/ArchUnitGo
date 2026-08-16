package fluentapi_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	filesextraction "github.com/LukasNiessen/ArchUnitGo/files/extraction"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

func TestAdhereToPassesWhenEverySelectedFileSatisfiesThePredicate(t *testing.T) {
	// The whole pipeline through the public surface: a project on disk, located, extracted, selected, read and
	// judged by a function the user wrote. An empty result is the pass.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator)

	rule := scope.Should().AdhereTo(func(file filesextraction.FileInfo) bool {
		return strings.HasPrefix(file.Source, "package ")
	}, "begin with its package clause")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) != 0 {
		t.Errorf("%s reports %v, want the pass: every file of that project begins that way", rule, violations)
	}
	if selected := selectedFiles(t, scope); len(selected) == 0 {
		t.Fatalf("%s is about nothing, want the whole project — only a rule with a subject can pass at all", rule)
	}
}

func TestAdhereToReportsEachSelectedFileThePredicateSaysNoAbout(t *testing.T) {
	// The violation the issue asks for: a convention no glob expresses, held over a folder, one violation per
	// file that breaks it — carrying the file, the words the user wrote and the mood.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/**").Should().AdhereTo(func(file filesextraction.FileInfo) bool {
		return strings.Contains(file.Source, "import ")
	}, "import something")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	want := []string{"internal/api/router.go", "internal/db/conn.go"}
	if offenders := adheringOffenders(t, rule, violations); !slices.Equal(offenders, want) {
		t.Fatalf("%s reports %v, want the files that import nothing: %v", rule, offenders, want)
	}
	adherence, ok := violations[0].(filesassertion.AdherenceViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an AdherenceViolation", rule, violations[0])
	}
	if adherence.Requirement != "import something" {
		t.Errorf("the violation requires %q, want the message the user wrote", adherence.Requirement)
	}
	if adherence.Mood != assertion.Should {
		t.Errorf("the violation was judged in mood %s, want the rule's own %s", adherence.Mood, assertion.Should)
	}
	first := `internal/api/router.go: should, adhere to "import something"`
	if rendered := adherence.String(); rendered != first {
		t.Errorf("the violation reads %q, want %q", rendered, first)
	}
}

func TestAdhereToHandsThePredicateTheFileAsTheProjectSpellsIt(t *testing.T) {
	// What the user's function is given: the identifier the rule's own patterns were matched against, the name,
	// the extension, the folder, the file's own text and how many of its lines carry something.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/db")

	seen := map[string]filesextraction.FileInfo{}
	rule := scope.Should().AdhereTo(func(file filesextraction.FileInfo) bool {
		seen[file.Path] = file
		return true
	}, "be looked at")

	if _, err := rule.Check(nil); err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(seen) != 1 {
		t.Fatalf("the predicate saw %v, want the one file that folder holds", slices.Sorted(maps.Keys(seen)))
	}
	file, judged := seen["internal/db/conn.go"]
	if !judged {
		t.Fatalf("the predicate saw %v, want internal/db/conn.go as the graph spells it", slices.Sorted(maps.Keys(seen)))
	}
	if file.Name != "conn" || file.Extension != ".go" || file.Directory != "internal/db" {
		t.Errorf("the predicate saw %+v, want its name, extension and folder derived from the identifier", file)
	}
	if !strings.Contains(file.Source, "func Connect()") {
		t.Errorf("the predicate saw the source %q, want the text of that file", file.Source)
	}
	if file.NonBlankLineCount != 2 {
		t.Errorf("the predicate saw %d non-blank lines, want the 2 the fixture wrote", file.NonBlankLineCount)
	}
}

func TestAdhereToInTheNegatedMoodForbidsWhatThePredicateDescribes(t *testing.T) {
	// `should not adhere to` is the same walk with assertion.Mood flipped: the offender is the file the positive
	// mood would have been happy with, and the violation says which mood it was judged in.
	locator := fixtureLocator(t, writeFixtureProject(t))
	touchesTheDatabase := func(file filesextraction.FileInfo) bool {
		return strings.Contains(file.Source, "internal/db")
	}

	forbidden := fluentapi.ProjectFiles(locator).InFolder("internal/api").ShouldNot().AdhereTo(touchesTheDatabase, "import the database package")

	violations, err := forbidden.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", forbidden, err)
	}
	if offenders := adheringOffenders(t, forbidden, violations); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Fatalf("%s reports %v, want the one file that does it", forbidden, offenders)
	}
	adherence, ok := violations[0].(filesassertion.AdherenceViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an AdherenceViolation", forbidden, violations[0])
	}
	if adherence.Mood != assertion.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want the rule's own %s", adherence.Mood, assertion.ShouldNot)
	}
	want := `internal/api/handler.go: should not, adhere to "import the database package"`
	if rendered := adherence.String(); rendered != want {
		t.Errorf("the violation reads %q, want the requirement as the rule stated it", rendered)
	}
}

func TestAdhereToInTheTwoMoodsPartitionsTheSelection(t *testing.T) {
	// One rule and its negation over one stored scope: every selected file offends exactly one of them, which is
	// the property that lets the two moods share a single assertion.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator)
	isSmall := func(file filesextraction.FileInfo) bool { return file.NonBlankLineCount < 3 }

	positive := scope.Should().AdhereTo(isSmall, "be at most two lines long")
	negated := scope.ShouldNot().AdhereTo(isSmall, "be at most two lines long")

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

func TestAdhereToReachesTheFilesTheCheckOptionsAskFor(t *testing.T) {
	// The options thread through to the reading, because the selection they shape is what is read: with
	// IncludeTestFiles the project's test files are files the user's function is asked about too.
	locator := fixtureLocator(t, writeFixtureProject(t))
	var seen []string
	rule := fluentapi.ProjectFiles(locator).Should().AdhereTo(func(file filesextraction.FileInfo) bool {
		seen = append(seen, file.Path)
		return true
	}, "be looked at")

	if _, err := rule.Check(&kernel.CheckOptions{IncludeTestFiles: true, ClearCache: true}); err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if !slices.Contains(seen, "internal/api/handler_test.go") {
		t.Errorf("the predicate saw %v with IncludeTestFiles, want the test file among them", seen)
	}
}

func TestAdhereToOfAScopeThatSelectedNothingIsAnEmptyTest(t *testing.T) {
	// Zero matches is a violation and not a pass: every file of an empty selection satisfies every predicate, so
	// a stale glob would be green forever. And nothing is read or asked when there is nothing to judge.
	locator := fixtureLocator(t, writeFixtureProject(t))
	asked := 0
	rule := fluentapi.ProjectFiles(locator).InFolder("internal/legacy/**").Should().AdhereTo(func(filesextraction.FileInfo) bool {
		asked++
		return false
	}, "be at most 400 lines long")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("%s reports %v, want the one empty-test violation of a scope that named no file", rule, violations)
	}
	empty, ok := violations[0].(assertion.EmptyTestViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want an EmptyTestViolation", rule, violations[0])
	}
	if empty.Subject != "files" {
		t.Errorf("the violation says the rule selected no %q, want `files`", empty.Subject)
	}
	if asked != 0 {
		t.Errorf("the predicate was asked %d times, want none: there was nothing to judge", asked)
	}

	allowed, err := rule.Check(&kernel.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("%s failed with AllowEmptyTests: %v", rule, err)
	}
	if len(allowed) != 0 {
		t.Errorf("%s reports %v with AllowEmptyTests, want the pass", rule, allowed)
	}
}

func TestAdhereToWithoutAPredicateIsAUserErrorNamingTheVerb(t *testing.T) {
	// A chain that passes no function has said nothing about the files it selected, and calling the nil function
	// would take the test process down — so it is the user's error, reported before the project is read.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).Should().AdhereTo(nil, "be at most 400 lines long")

	violations, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "adhere to" {
		t.Errorf("UserError.Operation = %q, want the predicate `adhere to`", user.Operation)
	}
	if !errors.Is(err, fluentapi.ErrNoPredicate) {
		t.Errorf("Check error = %v, want it to wrap ErrNoPredicate", err)
	}
	if len(violations) != 0 {
		t.Errorf("Check reports %v beside the error, want nothing: a misused API is not a rule failure", violations)
	}
	if !strings.Contains(rule.String(), "rejected") {
		t.Errorf("%s renders without the rejection, want it visible in a test failure", rule)
	}
}

func TestAdhereToWithoutAMessageIsAUserErrorNamingTheVerb(t *testing.T) {
	// The message is the whole of what a failure of this rule can say, because no report can print a closure —
	// so a rule without one is a rule nobody could act on, in either mood.
	locator := fixtureLocator(t, writeFixtureProject(t))
	always := func(filesextraction.FileInfo) bool { return true }
	tests := []struct {
		name string
		rule fluentapi.FilesAdherenceCondition
	}{
		{name: "empty", rule: fluentapi.ProjectFiles(locator).Should().AdhereTo(always, "")},
		{name: "blank", rule: fluentapi.ProjectFiles(locator).ShouldNot().AdhereTo(always, "  \t ")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.rule.Check(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Check error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != "adhere to" {
				t.Errorf("UserError.Operation = %q, want the predicate `adhere to`", user.Operation)
			}
			if !errors.Is(err, fluentapi.ErrNoRequirement) {
				t.Errorf("Check error = %v, want it to wrap ErrNoRequirement", err)
			}
		})
	}
}

func TestAdhereToRendersTheMessageInPlaceOfTheFunction(t *testing.T) {
	// The sentence the user typed, read back — with the words they gave standing in for the one stage of the
	// grammar the library cannot print.
	locator := fixtureLocator(t, writeFixtureProject(t))
	always := func(filesextraction.FileInfo) bool { return true }

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/**").ShouldNot().AdhereTo(always, "mention the legacy client")

	want := `project files, path without filename matches "internal/**", should not, adhere to "mention the legacy client"`
	if rendered := rule.String(); rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
}

func TestAdhereToIsBuiltWithoutReadingAnythingOrCallingThePredicate(t *testing.T) {
	// A rule is a value, not an action: building one locates no project, reads no file and asks the user's
	// function nothing. Only the terminal does, and only when it is called.
	asked := 0
	predicate := func(filesextraction.FileInfo) bool {
		asked++
		return true
	}

	rule := fluentapi.ProjectFiles(&extraction.ProjectLocator{Directory: "no-such-directory"}).
		InFolder("internal/**").
		Should().
		AdhereTo(predicate, "be at most 400 lines long")

	if asked != 0 {
		t.Errorf("building %s asked the predicate %d times, want none", rule, asked)
	}
}

// adheringOffenders names the files an adherence rule reported, in the order it reported them, checking on the
// way that every violation is an AdherenceViolation of the file-adherence kind.
func adheringOffenders(t *testing.T, rule fluentapi.FilesAdherenceCondition, violations []assertion.Violation) []string {
	t.Helper()

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != filesassertion.KindFileAdherence {
			t.Errorf("%s reports a violation of kind %q, want %q", rule, kind, filesassertion.KindFileAdherence)
		}
		adherence, ok := violation.(filesassertion.AdherenceViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want an AdherenceViolation", rule, violation)
		}
		offenders = append(offenders, adherence.File)
	}
	return offenders
}
