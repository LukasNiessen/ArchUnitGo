package fluentapi_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

func TestHaveNamePassesWhenEverySelectedFileIsNamedThatWay(t *testing.T) {
	// The whole pipeline through the public surface: a project on disk, located, extracted, selected and
	// judged. An empty result is the pass — and the selection is worth naming, because only a rule with a
	// subject can pass at all.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator)

	rule := scope.Should().HaveName("*.go")

	selected := selectedFiles(t, scope)
	if !slices.Contains(selected, "main.go") || !slices.Contains(selected, "internal/db/conn.go") {
		t.Fatalf("%s is about %v, want the whole project — a file at the root and files below it", rule, selected)
	}
	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) != 0 {
		t.Errorf("%s reports %v, want the pass: every file of that project is a Go file", rule, violations)
	}
}

func TestHaveNameReportsEachSelectedFileThatIsNotNamedThatWay(t *testing.T) {
	// The violation the issue asks for: a naming convention held over a folder, one violation per file that
	// breaks it, carrying the file, the requirement and the mood rather than a sentence about them.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InFolder("internal/**").Should().HaveName("handler*.go")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	want := []string{"internal/api/router.go", "internal/db/conn.go"}
	if offenders := namedOffenders(t, rule, violations); !slices.Equal(offenders, want) {
		t.Fatalf("%s reports %v, want the files not named that way: %v", rule, offenders, want)
	}
	naming, ok := violations[0].(filesassertion.NamingViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a NamingViolation", rule, violations[0])
	}
	if source := naming.Required.Pattern().Source(); source != "handler*.go" {
		t.Errorf("the violation quotes %q, want the pattern as the user typed it", source)
	}
	if target := naming.Required.Target(); target != matching.TargetFilename {
		t.Errorf("the violation was matched against the %s, want the filename `have name` reads", target)
	}
	if naming.Mood != assertion.Should {
		t.Errorf("the violation was judged in mood %s, want the rule's own %s", naming.Mood, assertion.Should)
	}
	first := `internal/api/router.go: should, filename matches "handler*.go"`
	if rendered := naming.String(); rendered != first {
		t.Errorf("the violation reads %q, want %q", rendered, first)
	}
}

func TestHaveNameLooksAtTheLastSegmentAndNotAtThePlace(t *testing.T) {
	// `have name` is the scope verb WithName's reading one stage later: the same name is required of a file
	// wherever it lives, which is what makes it the predicate for a convention rather than for a boundary.
	locator := fixtureLocator(t, writeFixtureProject(t))

	rule := fluentapi.ProjectFiles(locator).InPath("**/conn.go").Should().HaveName("conn.go")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", rule, err)
	}
	if len(violations) != 0 {
		t.Errorf("%s reports %v, want the pass: the file is named that, wherever it sits", rule, violations)
	}

	// And a pattern written as a path is not quietly read as one, because the predicate chose its target: no
	// filename contains a separator, so every selected file is an offender.
	misread := fluentapi.ProjectFiles(locator).Should().HaveName("internal/**")
	reported, err := misread.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", misread, err)
	}
	if len(reported) != len(selectedFiles(t, fluentapi.ProjectFiles(locator))) {
		t.Errorf("%s reports %v, want every file of the project", misread, reported)
	}
}

func TestHaveNameInTheNegatedMoodForbidsTheName(t *testing.T) {
	// `should not have name` is the same walk with assertion.Mood flipped: the offender is the file the
	// positive mood would have been happy with, and the violation says which mood it was judged in.
	locator := fixtureLocator(t, writeFixtureProject(t))

	forbidden := fluentapi.ProjectFiles(locator).ShouldNot().HaveName("conn.go")
	absent := fluentapi.ProjectFiles(locator).ShouldNot().HaveName("*_deprecated.go")

	violations, err := forbidden.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", forbidden, err)
	}
	if offenders := namedOffenders(t, forbidden, violations); !slices.Equal(offenders, []string{"internal/db/conn.go"}) {
		t.Fatalf("%s reports %v, want the one file that is named that", forbidden, offenders)
	}
	naming, ok := violations[0].(filesassertion.NamingViolation)
	if !ok {
		t.Fatalf("%s reports a %T, want a NamingViolation", forbidden, violations[0])
	}
	if naming.Mood != assertion.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want the rule's own %s", naming.Mood, assertion.ShouldNot)
	}
	if rendered := naming.String(); rendered != `internal/db/conn.go: should not, filename matches "conn.go"` {
		t.Errorf("the violation reads %q, want the requirement as the rule stated it", rendered)
	}

	nothing, err := absent.Check(nil)
	if err != nil {
		t.Fatalf("%s failed: %v", absent, err)
	}
	if len(nothing) != 0 {
		t.Errorf("%s reports %v, want the pass: no file of that project is named that way", absent, nothing)
	}
}

func TestHaveNameInTheTwoMoodsPartitionsTheSelection(t *testing.T) {
	// One rule and its negation over one stored scope: every selected file offends exactly one of them,
	// which is the property that lets the two moods share a single assertion.
	locator := fixtureLocator(t, writeFixtureProject(t))
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**")

	positive := scope.Should().HaveName("*_test.go")
	negated := scope.ShouldNot().HaveName("*_test.go")

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

// namedOffenders names the files a naming rule reported, in the order it reported them, checking on the way
// that every violation is a NamingViolation of the file-naming kind.
func namedOffenders(t *testing.T, rule fluentapi.FilesNamingCondition, violations []assertion.Violation) []string {
	t.Helper()

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if kind := violation.Kind(); kind != filesassertion.KindFileNaming {
			t.Errorf("%s reports a violation of kind %q, want %q", rule, kind, filesassertion.KindFileNaming)
		}
		naming, ok := violation.(filesassertion.NamingViolation)
		if !ok {
			t.Fatalf("%s reports a %T, want a NamingViolation", rule, violation)
		}
		offenders = append(offenders, naming.File)
	}
	return offenders
}

// selectedFiles are the files a scope is about, which is what makes a count of violations readable.
func selectedFiles(t *testing.T, scope fluentapi.FilesBuilder) []string {
	t.Helper()

	selected, err := scope.SelectFiles(nil)
	if err != nil {
		t.Fatalf("%s could not resolve its scope: %v", scope, err)
	}
	return selected
}
