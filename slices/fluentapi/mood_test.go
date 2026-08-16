package fluentapi_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/slices/fluentapi"
)

func TestTheTwoMoodsOfARuleAboutSlicesAreOneRuleOneFlagApart(t *testing.T) {
	// The mood is a flag threaded into one assertion, not two implementations, and the two builders differ by
	// nothing else. There are no synonyms for either word in any language this library is ported to.
	slicing := fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**")

	if mood := slicing.Should().Mood(); mood != assertion.Should {
		t.Errorf("`should` is the mood %s, want %s", mood, assertion.Should)
	}
	if mood := slicing.ShouldNot().Mood(); mood != assertion.ShouldNot {
		t.Errorf("`should not` is the mood %s, want %s", mood, assertion.ShouldNot)
	}
}

func TestAMoodSaysNothingAboutWhatTheSlicesOfAProjectAre(t *testing.T) {
	// Both moods resolve to the slicing's own answer, unchanged: what a rule is about is the slicing's half of
	// it, and the mood is only what is said about it.
	root := writeSlicedFixtureProject(t)
	slicing := fixtureSlicing(t, root)

	want, err := slicing.SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("resolving the slicing failed: %v", err)
	}
	positive, err := slicing.Should().SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("resolving the slicing of `should` failed: %v", err)
	}
	negative, err := slicing.ShouldNot().SelectSliceFiles(nil)
	if err != nil {
		t.Fatalf("resolving the slicing of `should not` failed: %v", err)
	}

	if !maps.EqualFunc(positive, want, slices.Equal) {
		t.Errorf("`should` is about %v, want the slices the slicing found, %v", positive, want)
	}
	if !maps.EqualFunc(negative, want, slices.Equal) {
		t.Errorf("`should not` is about %v, want the slices the slicing found, %v", negative, want)
	}

	// Resolved a second time with a bag on, because handing the slicing a bag is the whole of what these two
	// methods do: a mood that dropped its caller's options would answer with the default project and read as
	// the same passthrough. The db slice is where the difference shows, since the fixture's test file lives in
	// the folder the slicing names it after.
	withTests := &kernel.CheckOptions{IncludeTestFiles: true}
	wantWithTests := []string{"internal/db/conn.go", "internal/db/conn_test.go"}

	positive, err = slicing.Should().SelectSliceFiles(withTests)
	if err != nil {
		t.Fatalf("resolving the slicing of `should` with IncludeTestFiles failed: %v", err)
	}
	negative, err = slicing.ShouldNot().SelectSliceFiles(withTests)
	if err != nil {
		t.Fatalf("resolving the slicing of `should not` with IncludeTestFiles failed: %v", err)
	}

	if !slices.Equal(positive["db"], wantWithTests) {
		t.Errorf(`the slice "db" of "should" came to %v, want %v: the mood passed the bag on`, positive["db"], wantWithTests)
	}
	if !slices.Equal(negative["db"], wantWithTests) {
		t.Errorf(`the slice "db" of "should not" came to %v, want %v: the mood passed the bag on`, negative["db"], wantWithTests)
	}
}

func TestAMoodRendersTheSentenceAsFarAsItHasBeenBuilt(t *testing.T) {
	// A rule caught mid-chain in a log line still says what it was about, and the two moods share one rendering
	// so that neither can phrase itself differently from the other.
	slicing := fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**")

	sentences := map[string]string{
		slicing.Should().String():    `project slices, path matches "internal/(**)/**", should`,
		slicing.ShouldNot().String(): `project slices, path matches "internal/(**)/**", should not`,
	}

	for rendered, want := range sentences {
		if rendered != want {
			t.Errorf("the rule reads %q, want %q", rendered, want)
		}
	}
}
