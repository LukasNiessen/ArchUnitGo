package fluentapi_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

func TestBeInPathPassesWhenEverySelectedIdentifierMatches(t *testing.T) {
	// The predicate for a convention that ties a name to a place: every file of the api package should have
	// an identifier of the form `internal/api/*.go`, folder and name judged at once.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/api").Should().BeInPath("internal/api/*.go")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) != 0 {
		t.Errorf("%s reports %v, want the pass: both files of that package are spelled that way", rule, violations)
	}
}

func TestBeInPathReportsEachSelectedFileWhoseIdentifierDoesNotMatch(t *testing.T) {
	// One violation per file, carrying the pattern the user typed and the whole path as the thing it was
	// matched against — which is the difference a report has to be able to show.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).Should().BeInPath("internal/**/*.go")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := namedOffenders(t, rule, violations); !slices.Equal(offenders, []string{"main.go"}) {
		t.Fatalf("%s reports %v, want the one file whose identifier is not spelled that way", rule, offenders)
	}
	naming, ok := violations[0].(filesassertion.NamingViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a NamingViolation", rule, violations[0])
	}
	if source := naming.Required.Pattern().Source(); source != "internal/**/*.go" {
		t.Errorf("the violation quotes %q, want the pattern as the user typed it", source)
	}
	if target := naming.Required.Target(); target != matching.TargetPath {
		t.Errorf("the violation was matched against the %s, want the whole path `be in path` reads", target)
	}
	want := `main.go: should, path matches "internal/**/*.go"`
	if rendered := naming.String(); rendered != want {
		t.Errorf("the violation reads %q, want %q", rendered, want)
	}
}

func TestBeInPathJudgesTheNameAndThePlaceAtOnce(t *testing.T) {
	// What neither of the other two predicates can say: the same file satisfies the name alone and the folder
	// alone, and still breaks the rule that ties them together.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**")

	byName, err := scope.Should().HaveName("*.go").Check(nil)
	if err != nil {
		t.Fatalf("the rule about names failed: %v", err)
	}
	byFolder, err := scope.Should().BeInFolder("internal/**").Check(nil)
	if err != nil {
		t.Fatalf("the rule about folders failed: %v", err)
	}
	tied := scope.Should().BeInPath("internal/api/*.go")
	byPath, err := tied.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", tied, err)
	}

	if len(byName) != 0 || len(byFolder) != 0 {
		t.Fatalf("the name and folder rules report %v and %v, want both to hold", byName, byFolder)
	}
	if offenders := namedOffenders(t, tied, byPath); !slices.Equal(offenders, []string{"internal/db/conn.go"}) {
		t.Errorf("%s reports %v, want the file that is named and placed acceptably but not together", tied, offenders)
	}
}

func TestBeInPathInTheNegatedMoodForbidsTheIdentifier(t *testing.T) {
	// `should not be in path` is the same walk with assertion.Mood flipped, and the mood the rule was written
	// in travels on the violation — a report cannot tell the two failures apart without it.
	locator := fixtureLocator(t, writeFixtureProject(t))

	forbidden := fluentapi.ProjectFiles(locator).ShouldNot().BeInPath("**/db/**")
	absent := fluentapi.ProjectFiles(locator).ShouldNot().BeInPath("**/legacy/**")

	violations, err := forbidden.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", forbidden, err)
	}
	if offenders := namedOffenders(t, forbidden, violations); !slices.Equal(offenders, []string{"internal/db/conn.go"}) {
		t.Fatalf("%s reports %v, want the one file whose identifier is spelled that way", forbidden, offenders)
	}
	naming, ok := violations[0].(filesassertion.NamingViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a NamingViolation", forbidden, violations[0])
	}
	if naming.Mood != assertion.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want %s", naming.Mood, assertion.ShouldNot)
	}

	nothing, err := absent.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", absent, err)
	}
	if len(nothing) != 0 {
		t.Errorf("%s reports %v, want the pass: that project has no such folder", absent, nothing)
	}
}

func TestBeInPathInTheTwoMoodsPartitionsTheSelection(t *testing.T) {
	// The two moods over one stored scope split the selection between them, as they do for every predicate
	// built on one gather function and one flag.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator)

	positive := scope.Should().BeInPath("internal/api/*.go")
	negated := scope.ShouldNot().BeInPath("internal/api/*.go")

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
}
