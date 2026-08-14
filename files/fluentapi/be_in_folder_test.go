package fluentapi_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

func TestBeInFolderPassesWhenEverySelectedFileLivesThere(t *testing.T) {
	// The sentence the predicate exists for, over a project on disk: every file called `handler.go` should be
	// in folder `internal/api/**` — a kind of file kept where it belongs. The pass is an empty result.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).WithName("handler.go").Should().BeInFolder("internal/api/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) != 0 {
		t.Errorf("%s reports %v, want the pass: the project's handler lives there", rule, violations)
	}
}

func TestBeInFolderReportsEachSelectedFileThatLivesElsewhere(t *testing.T) {
	// One violation per misplaced file, carrying the folder pattern the user typed and the part of the
	// identifier it was matched against — which is how a report can say `internal/**` was read as a folder.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).WithName("*.go").Should().BeInFolder("internal/**")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if offenders := namedOffenders(t, rule, violations); !slices.Equal(offenders, []string{"main.go"}) {
		t.Fatalf("%s reports %v, want the one file that is not under internal/", rule, offenders)
	}
	naming, ok := violations[0].(filesassertion.NamingViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a NamingViolation", rule, violations[0])
	}
	if source := naming.Required.Pattern().Source(); source != "internal/**" {
		t.Errorf("the violation quotes %q, want the pattern as the user typed it", source)
	}
	if target := naming.Required.Target(); target != matching.TargetPathWithoutFilename {
		t.Errorf("the violation was matched against the %s, want the folder `be in folder` reads", target)
	}
	want := `main.go: should, path without filename matches "internal/**"`
	if rendered := naming.String(); rendered != want {
		t.Errorf("the violation reads %q, want %q", rendered, want)
	}
}

func TestBeInFolderReadsAFolderPatternTheWayTheScopeVerbDoes(t *testing.T) {
	// `internal/api` is that folder alone, `internal/api/**` is it together with everything below it, and a
	// file at the project root is in the folder `.`. It is InFolder's own reading one stage later, so the two
	// halves of a rule cannot disagree about what a folder is.
	locator := fixtureLocator(t, writeFixtureProject(t))
	tests := []struct {
		name    string
		rule    fluentapi.FilesNamingCondition
		offends []string
	}{
		{
			name:    "the folder alone",
			rule:    fluentapi.ProjectFiles(locator).InFolder("internal/**").Should().BeInFolder("internal/api"),
			offends: []string{"internal/db/conn.go"},
		},
		{
			name:    "the folder and everything below it",
			rule:    fluentapi.ProjectFiles(locator).InFolder("internal/**").Should().BeInFolder("internal/**"),
			offends: nil,
		},
		{
			name:    "the project root",
			rule:    fluentapi.ProjectFiles(locator).WithName("main.go").Should().BeInFolder("."),
			offends: nil,
		},
		{
			name:    "the root is not below internal/",
			rule:    fluentapi.ProjectFiles(locator).WithName("main.go").Should().BeInFolder("internal"),
			offends: []string{"main.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations, err := test.rule.Check(nil)
			if err != nil {
				t.Fatalf("%s failed: %v", test.rule, err)
			}
			offenders := namedOffenders(t, test.rule, violations)
			if len(offenders) == 0 {
				offenders = nil
			}
			if !slices.Equal(offenders, test.offends) {
				t.Errorf("%s reports %v, want %v", test.rule, offenders, test.offends)
			}
		})
	}
}

func TestBeInFolderInTheNegatedMoodForbidsThePlace(t *testing.T) {
	// `should not be in folder` is the boundary written the way most architecture rules are: the same walk
	// with assertion.Mood flipped, so the offender is the file that *is* in the forbidden folder.
	locator := fixtureLocator(t, writeFixtureProject(t))

	forbidden := fluentapi.ProjectFiles(locator).WithName("*.go").ShouldNot().BeInFolder("internal/db/**")

	violations, err := forbidden.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", forbidden, err)
	}
	if offenders := namedOffenders(t, forbidden, violations); !slices.Equal(offenders, []string{"internal/db/conn.go"}) {
		t.Fatalf("%s reports %v, want the one file that is in that folder", forbidden, offenders)
	}
	naming, ok := violations[0].(filesassertion.NamingViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a NamingViolation", forbidden, violations[0])
	}
	if naming.Mood != assertion.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want %s", naming.Mood, assertion.ShouldNot)
	}
	want := `internal/db/conn.go: should not, path without filename matches "internal/db/**"`
	if rendered := naming.String(); rendered != want {
		t.Errorf("the violation reads %q, want the requirement as the rule stated it: %q", rendered, want)
	}
}

func TestBeInFolderInTheTwoMoodsPartitionsTheSelection(t *testing.T) {
	// The two moods over one stored scope split the selection between them: every file is either in the
	// folder or not, and exactly one of the two rules says so.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator)

	positive := scope.Should().BeInFolder("internal/api")
	negated := scope.ShouldNot().BeInFolder("internal/api")

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
