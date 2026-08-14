package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// fixtureSelection is the file population every test here judges: one file at the project root and two
// folders below it, the way projection.SelectFiles hands a selection over — sorted, normalised identifiers.
func fixtureSelection() []string {
	return []string{
		"internal/api/handler.go",
		"internal/api/handler_test.go",
		"internal/db/conn.go",
		"main.go",
	}
}

func TestGatherNamingViolationsReportsTheFilesThatDoNotMatch(t *testing.T) {
	// `should have name "*_test.go"`: the one file that is named that way is silent, and every other selected
	// file is reported, because the requirement is held over the whole selection.
	violations := assertion.GatherNamingViolations(fixtureSelection(), filenameMatcher(t, "*_test.go"), kernel.Should)

	want := []string{"internal/api/handler.go", "internal/db/conn.go", "main.go"}
	if offenders := offendersOf(t, violations); !slices.Equal(offenders, want) {
		t.Errorf("GatherNamingViolations reported %v, want %v", offenders, want)
	}
}

func TestGatherNamingViolationsOfTheNegatedMoodReportsTheFilesThatDoMatch(t *testing.T) {
	// `should not have name "*_test.go"`: the same walk over the same filter, one flag apart — so the offender
	// is exactly the file the positive mood was happy with.
	violations := assertion.GatherNamingViolations(fixtureSelection(), filenameMatcher(t, "*_test.go"), kernel.ShouldNot)

	want := []string{"internal/api/handler_test.go"}
	if offenders := offendersOf(t, violations); !slices.Equal(offenders, want) {
		t.Errorf("GatherNamingViolations reported %v, want %v", offenders, want)
	}
}

func TestGatherNamingViolationsInTheTwoMoodsPartitionTheSelection(t *testing.T) {
	// The property that makes one gather function enough for both moods: every selected file offends exactly
	// one of a rule and its negation, so nothing is judged twice and nothing escapes judgement.
	required := folderMatcher(t, "internal/**")

	positive := offendersOf(t, assertion.GatherNamingViolations(fixtureSelection(), required, kernel.Should))
	negated := offendersOf(t, assertion.GatherNamingViolations(fixtureSelection(), required, kernel.ShouldNot))

	both := slices.Concat(positive, negated)
	slices.Sort(both)
	if !slices.Equal(both, fixtureSelection()) {
		t.Errorf("the two moods reported %v between them, want each selected file exactly once: %v", both, fixtureSelection())
	}
}

func TestGatherNamingViolationsLooksAtThePartOfTheIdentifierTheFilterNames(t *testing.T) {
	// The three predicates differ by nothing but the filter they pass in — `have name` a filename matcher,
	// `be in folder` a folder matcher, `be in path` a path matcher — so the same pattern text judges different
	// things and this function never asks which predicate called it.
	tests := []struct {
		name     string
		required matching.Filter
		want     []string
	}{
		{
			name:     "have name",
			required: filenameMatcher(t, "*.go"),
			want:     nil,
		},
		{
			name:     "be in folder",
			required: folderMatcher(t, "internal/**"),
			want:     []string{"main.go"},
		},
		{
			name:     "be in path",
			required: pathMatcher(t, "internal/**/*.go"),
			want:     []string{"main.go"},
		},
		{
			name:     "be in path, name and place at once",
			required: pathMatcher(t, "internal/api/*.go"),
			want:     []string{"internal/db/conn.go", "main.go"},
		},
		{
			name:     "a folder pattern read as a filename matches nothing",
			required: filenameMatcher(t, "internal/**"),
			want:     fixtureSelection(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := assertion.GatherNamingViolations(fixtureSelection(), test.required, kernel.Should)

			if offenders := offendersOf(t, violations); !slices.Equal(offenders, test.want) {
				t.Errorf("GatherNamingViolations under %s reported %v, want %v", test.required, offenders, test.want)
			}
		})
	}
}

func TestGatherNamingViolationsOfAComplyingSelectionIsThePass(t *testing.T) {
	// Every file satisfying the requirement is no violations, in either mood: an empty result is what every
	// rule in the library returns when it holds, and there is no boolean beside the list.
	tests := []struct {
		name     string
		required matching.Filter
		mood     kernel.Mood
	}{
		{name: "should", required: filenameMatcher(t, "*.go"), mood: kernel.Should},
		{name: "should not", required: filenameMatcher(t, "*.java"), mood: kernel.ShouldNot},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := assertion.GatherNamingViolations(fixtureSelection(), test.required, test.mood)

			if len(violations) != 0 {
				t.Errorf("GatherNamingViolations reported %v, want the pass", violations)
			}
		})
	}
}

func TestGatherNamingViolationsOfAnEmptySelectionIsSilentInBothMoods(t *testing.T) {
	// A selection with nothing in it has nothing to report here, which is exactly why the empty-test guard
	// exists a stage earlier: this function would call a stale glob green whichever mood it was written in.
	for _, files := range [][]string{nil, {}} {
		for _, mood := range []kernel.Mood{kernel.Should, kernel.ShouldNot} {
			violations := assertion.GatherNamingViolations(files, filenameMatcher(t, "*_service.go"), mood)

			if len(violations) != 0 {
				t.Errorf("GatherNamingViolations(%v, ..., %s) reported %v, want nothing", files, mood, violations)
			}
		}
	}
}

func TestGatherNamingViolationsCarriesTheRequirementAndTheMoodItJudgedWith(t *testing.T) {
	// The violation is data, not a sentence: whoever phrases it later gets the pattern the user typed, the
	// part of the identifier it was matched against and the mood — never a re-derived or inverted filter.
	required := pathMatcher(t, "**/legacy/**")

	violations := assertion.GatherNamingViolations(fixtureSelection(), required, kernel.Should)

	if len(violations) != len(fixtureSelection()) {
		t.Fatalf("GatherNamingViolations reported %d violations, want all %d files", len(violations), len(fixtureSelection()))
	}
	for _, violation := range violations {
		naming, ok := violation.(assertion.NamingViolation)
		if !ok {
			t.Fatalf("GatherNamingViolations reported a %T, want a NamingViolation", violation)
		}
		if naming.Required.String() != required.String() {
			t.Errorf("the violation of %s requires %s, want the filter as compiled: %s", naming.File, naming.Required, required)
		}
		if naming.Required.Target() != matching.TargetPath {
			t.Errorf("the violation of %s was matched against the %s, want the whole path", naming.File, naming.Required.Target())
		}
		if naming.Mood != kernel.Should {
			t.Errorf("the violation of %s was judged in mood %s, want %s", naming.File, naming.Mood, kernel.Should)
		}
	}
}

func TestGatherNamingViolationsOfAZeroFilterReportsEverySelectedFile(t *testing.T) {
	// A zero matching.Filter matches nothing, so a positive rule written with one fails everywhere rather than
	// passing quietly. No predicate can build one — a pattern that will not compile is the user's error,
	// returned by the terminal before the project is read — and this is what the mistake would look like.
	violations := assertion.GatherNamingViolations(fixtureSelection(), matching.Filter{}, kernel.Should)

	if offenders := offendersOf(t, violations); !slices.Equal(offenders, fixtureSelection()) {
		t.Errorf("GatherNamingViolations reported %v, want every selected file %v", offenders, fixtureSelection())
	}
}

// offendersOf names the files the violations were reported for, in the order they were reported, and checks
// on the way that every one of them is a NamingViolation of the file-naming kind.
func offendersOf(t *testing.T, violations []kernel.Violation) []string {
	t.Helper()

	if len(violations) == 0 {
		return nil
	}
	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if violation.Kind() != assertion.KindFileNaming {
			t.Errorf("a violation is of kind %q, want %q", violation.Kind(), assertion.KindFileNaming)
		}
		naming, ok := violation.(assertion.NamingViolation)
		if !ok {
			t.Fatalf("GatherNamingViolations reported a %T, want a NamingViolation", violation)
		}
		offenders = append(offenders, naming.File)
	}
	return offenders
}
