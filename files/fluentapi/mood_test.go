package fluentapi_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

func TestBothMoodsCanBeAskedOfOneStoredScope(t *testing.T) {
	// The branching AGENTS.md asks for, at the mood: one half-built rule, both moods taken from it, and
	// the scope unchanged by either.
	base := fluentapi.ProjectFiles(nil).InFolder("internal/**").WithName("*.go")

	positive := base.Should()
	negated := base.ShouldNot()

	if mood := positive.Mood(); mood != assertion.Should {
		t.Errorf("Should().Mood() = %s, want %s", mood, assertion.Should)
	}
	if mood := negated.Mood(); mood != assertion.ShouldNot {
		t.Errorf("ShouldNot().Mood() = %s, want %s", mood, assertion.ShouldNot)
	}
	if selectors := base.Selectors(); len(selectors) != 2 {
		t.Errorf("the stored scope's Selectors() = %v, want the two verbs it was built with", selectors)
	}
	if positive.Mood() == negated.Mood() {
		t.Error("both moods report the same flag, want the one difference between them")
	}
}

func TestAMoodDoesNotChangeWhichFilesARuleIsAbout(t *testing.T) {
	// The mood says what the rule claims, never what it is about. Both moods therefore resolve to the
	// scope's own selection, which is what every predicate after them is written over.
	scope := fluentapi.ProjectFiles(nil).InFolder("internal/**")
	want := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}

	moods := map[string][]matching.Filter{
		"should":     scope.Should().Selectors(),
		"should not": scope.ShouldNot().Selectors(),
	}

	for mood, selectors := range moods {
		t.Run(mood, func(t *testing.T) {
			if selected := projection.SelectFiles(fixtureGraph(), selectors...); !slices.Equal(selected, want) {
				t.Errorf("`... %s` is about %v, want the scope's own files %v", mood, selected, want)
			}
		})
	}
}

func TestAMoodResolvesTheScopeAgainstARealProject(t *testing.T) {
	// The level above the unit tests: the mood stage carries the locator and the check options the scope
	// was built with, so a rule stays about the project it names.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	locator := &extraction.ProjectLocator{Directory: writeFixtureProject(t)}
	scope := fluentapi.ProjectFiles(locator).InFolder("internal/**").WithName("*.go")

	positive, err := scope.Should().SelectFiles(nil)
	if err != nil {
		t.Fatalf("`should` failed to resolve the scope it was asked of: %v", err)
	}
	negated, err := scope.ShouldNot().SelectFiles(nil)
	if err != nil {
		t.Fatalf("`should not` failed to resolve the scope it was asked of: %v", err)
	}

	want := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
	if !slices.Equal(positive, want) {
		t.Errorf("`... should` is about %v, want %v", positive, want)
	}
	if !slices.Equal(negated, positive) {
		t.Errorf("`... should not` is about %v, want what `... should` is about, %v", negated, positive)
	}
}

func TestAMoodRendersTheSentenceSoFar(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
		want     string
	}{
		{
			name:     "the mood on its own",
			rendered: fluentapi.ProjectFiles(nil).ShouldNot().String(),
			want:     "project files, should not",
		},
		{
			name:     "should, after a scope",
			rendered: fluentapi.ProjectFiles(nil).InFolder("internal/**").Should().String(),
			want:     `project files, path without filename matches "internal/**", should`,
		},
		{
			name:     "should not, after a scope",
			rendered: fluentapi.ProjectFiles(nil).InFolder("internal/**").WithName("*.go").ShouldNot().String(),
			want:     `project files, path without filename matches "internal/**", filename matches "*.go", should not`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.rendered != test.want {
				t.Errorf("String() = %q, want %q", test.rendered, test.want)
			}
		})
	}
}

func TestAMoodStillReportsAPatternTheScopeRejected(t *testing.T) {
	// A rejected pattern is deferred to the terminal, so taking a mood must neither swallow it nor
	// render the rule as the one the user thought they wrote.
	rule := fluentapi.ProjectFiles(nil).InFolder("[unclosed").ShouldNot()

	_, err := rule.SelectFiles(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("SelectFiles error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the scope verb `in folder`", user.Operation)
	}
	rendered := rule.String()
	if !strings.Contains(rendered, "should not") {
		t.Errorf("String() = %q, want the mood visible", rendered)
	}
	if !strings.Contains(rendered, "rejected") {
		t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
	}
}

func TestTheMoodIsTwoWordsWithNoSynonyms(t *testing.T) {
	// The issue's whole first line, held to mechanically: the two moods and nothing else. A synonym is
	// how a fluent API stops sounding like one language, and it is added by someone who did not know the
	// word was already taken — so the check is on the method set rather than on a reviewer's memory.
	synonyms := []string{
		"Must", "MustNot", "May", "MayNot", "Never", "Always", "Shall", "ShallNot",
		"Ought", "OughtNot", "Cannot", "CanNot", "Is", "IsNot", "Are", "AreNot",
		"Does", "DoesNot", "Do", "DoNot", "Will", "WillNot", "Has", "HasNot",
		"Require", "Requires", "Expect", "Expects", "Forbid", "Forbids",
	}
	stages := []struct {
		name     string
		methods  []string
		wantMood bool
	}{
		{name: "the scope stage", methods: methodsOf(fluentapi.ProjectFiles(nil)), wantMood: true},
		{name: "`should`", methods: methodsOf(fluentapi.ProjectFiles(nil).Should())},
		{name: "`should not`", methods: methodsOf(fluentapi.ProjectFiles(nil).ShouldNot())},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			for _, mood := range []string{"Should", "ShouldNot"} {
				// Exactly one mood per rule: the scope offers both, and a mood stage offers neither, so
				// a chain cannot say the word twice.
				if slices.Contains(stage.methods, mood) != stage.wantMood {
					t.Errorf("%s has %s among %v, want %t", stage.name, mood, stage.methods, stage.wantMood)
				}
			}
			for _, method := range stage.methods {
				if slices.Contains(synonyms, method) {
					t.Errorf("%s has %s, want the mood spelled `should` and `should not` and no other way", stage.name, method)
				}
				if strings.HasPrefix(method, "Should") && method != "Should" && method != "ShouldNot" {
					t.Errorf("%s has %s, want no mood but the two", stage.name, method)
				}
			}
		})
	}
}

func TestTheMoodReachesAnAssertionAsAFlag(t *testing.T) {
	// What the mood is for, in the shape the coming predicates will have it: one assertion over one
	// selection, the mood passed to it, and the two moods reporting exactly complementary sets.
	scope := fluentapi.ProjectFiles(nil).InFolder("internal/**")
	positive := scope.Should()
	negated := scope.ShouldNot()

	satisfying := gatherFixtureOffenders(t, negated.Selectors(), negated.Mood())
	failing := gatherFixtureOffenders(t, positive.Selectors(), positive.Mood())

	if want := []string{"internal/api/handler.go"}; !slices.Equal(satisfying, want) {
		t.Errorf("`%s, depend on internal/db/conn.go` reports %v, want the files that do, %v", negated, satisfying, want)
	}
	if want := []string{"internal/api/router.go", "internal/db/conn.go"}; !slices.Equal(failing, want) {
		t.Errorf("`%s, depend on internal/db/conn.go` reports %v, want the files that do not, %v", positive, failing, want)
	}
	both := slices.Concat(satisfying, failing)
	slices.Sort(both)
	want := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
	if !slices.Equal(both, want) {
		t.Errorf("the two moods report %v between them, want every selected file exactly once, %v", both, want)
	}
}

// gatherFixtureOffenders is the shape every predicate of this module will have, written out here
// because none has landed yet: the scope resolved to files, one predicate asked the positive question
// per file, and the mood deciding which of the answers are violations. Nothing in it branches on the
// mood.
func gatherFixtureOffenders(t *testing.T, selectors []matching.Filter, mood assertion.Mood) []string {
	t.Helper()

	graph := fixtureGraph()
	offenders := make([]string, 0, len(graph))
	for _, file := range projection.SelectFiles(graph, selectors...) {
		// The dependencies rather than the whole graph: a file's self-edge is how it appears as a node
		// and is not a dependency on itself.
		_, depends := graph.Dependencies().Find(file, "internal/db/conn.go")
		if mood.Holds(depends) {
			continue
		}
		offenders = append(offenders, file)
	}
	return offenders
}

// methodsOf names the methods a fluent stage offers, which is the vocabulary a user can type at it.
func methodsOf(stage any) []string {
	stageType := reflect.TypeOf(stage)
	names := make([]string, 0, stageType.NumMethod())
	for index := range stageType.NumMethod() {
		names = append(names, stageType.Method(index).Name)
	}
	return names
}
