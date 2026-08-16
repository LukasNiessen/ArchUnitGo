package assertion_test

import (
	"slices"
	"strings"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/extraction"
)

// fixtureFiles is the file population every test here judges: the selection of fixtureSelection, described
// the way extraction.ExtractFileInfo hands it over — one FileInfo per file, in the order it was sorted into.
func fixtureFiles() []extraction.FileInfo {
	return []extraction.FileInfo{
		extraction.NewFileInfo("internal/api/handler.go", "package api\n\nfunc Handle() {}\n"),
		extraction.NewFileInfo("internal/api/handler_test.go", "package api\n"),
		extraction.NewFileInfo("internal/db/conn.go", "package db\n\nimport \"database/sql\"\n\nfunc Connect() *sql.DB { return nil }\n"),
		extraction.NewFileInfo("main.go", "package main\n\nfunc main() {}\n"),
	}
}

// isSmall is the kind of predicate a user writes: a question about one file, here about its size, satisfied
// by every file of the fixture but internal/db/conn.go — which is three non-blank lines long where the rest
// are one or two.
func isSmall(file extraction.FileInfo) bool {
	return file.NonBlankLineCount <= 2
}

func TestGatherAdherenceViolationsReportsTheFilesThePredicateSaysNoAbout(t *testing.T) {
	// `should adhere to`: the requirement is held over the whole selection, so every file the user's own
	// function answers no about is one violation and the rest are silent.
	violations := assertion.GatherAdherenceViolations(fixtureFiles(), isSmall, "be at most two lines long", kernel.Should)

	want := []string{"internal/db/conn.go"}
	if offenders := adherenceOffendersOf(t, violations); !slices.Equal(offenders, want) {
		t.Errorf("GatherAdherenceViolations reported %v, want %v", offenders, want)
	}
}

func TestGatherAdherenceViolationsOfTheNegatedMoodReportsTheFilesItSaysYesAbout(t *testing.T) {
	// `should not adhere to`: the same walk over the same function, one flag apart — so the offender is
	// exactly the file the positive mood was happy with.
	violations := assertion.GatherAdherenceViolations(fixtureFiles(), isSmall, "be at most two lines long", kernel.ShouldNot)

	want := []string{"internal/api/handler.go", "internal/api/handler_test.go", "main.go"}
	if offenders := adherenceOffendersOf(t, violations); !slices.Equal(offenders, want) {
		t.Errorf("GatherAdherenceViolations reported %v, want %v", offenders, want)
	}
}

func TestGatherAdherenceViolationsInTheTwoMoodsPartitionTheSelection(t *testing.T) {
	// The property that makes one gather function enough for both moods: every selected file offends exactly
	// one of a rule and its negation, so nothing is judged twice and nothing escapes judgement.
	positive := adherenceOffendersOf(t, assertion.GatherAdherenceViolations(fixtureFiles(), isSmall, "be small", kernel.Should))
	negated := adherenceOffendersOf(t, assertion.GatherAdherenceViolations(fixtureFiles(), isSmall, "be small", kernel.ShouldNot))

	both := slices.Concat(positive, negated)
	slices.Sort(both)
	if !slices.Equal(both, fixtureSelection()) {
		t.Errorf("the two moods reported %v between them, want each selected file exactly once: %v", both, fixtureSelection())
	}
}

func TestGatherAdherenceViolationsHandsThePredicateTheWholeFile(t *testing.T) {
	// Everything a user may ask about is on the FileInfo, and it is the one the fluent stage read: the
	// identifier the rule's patterns matched, the name, the folder, the source text and the size.
	var seen []extraction.FileInfo
	remember := func(file extraction.FileInfo) bool {
		seen = append(seen, file)
		return true
	}

	assertion.GatherAdherenceViolations(fixtureFiles(), remember, "be looked at", kernel.Should)

	if len(seen) != len(fixtureFiles()) {
		t.Fatalf("the predicate saw %d files, want the %d it was given", len(seen), len(fixtureFiles()))
	}
	if !slices.Equal(seen, fixtureFiles()) {
		t.Errorf("the predicate saw %+v, want the files unchanged: %+v", seen, fixtureFiles())
	}
	if seen[2].Path != "internal/db/conn.go" || !strings.Contains(seen[2].Source, "database/sql") {
		t.Errorf("the predicate saw %+v third, want internal/db/conn.go with its own source text", seen[2])
	}
}

func TestGatherAdherenceViolationsAsksThePredicateOncePerFile(t *testing.T) {
	// A user's function is asked exactly once about each selected file, in the order the selection arrived:
	// a predicate that logs, counts or memoises may rely on that, and nothing here may ask twice.
	asked := map[string]int{}
	count := func(file extraction.FileInfo) bool {
		asked[file.Path]++
		return false
	}

	assertion.GatherAdherenceViolations(fixtureFiles(), count, "be counted", kernel.Should)

	for _, file := range fixtureFiles() {
		if asked[file.Path] != 1 {
			t.Errorf("the predicate was asked %d times about %s, want once", asked[file.Path], file.Path)
		}
	}
	if len(asked) != len(fixtureFiles()) {
		t.Errorf("the predicate was asked about %d files, want the %d it was given", len(asked), len(fixtureFiles()))
	}
}

func TestGatherAdherenceViolationsOfASatisfiedSelectionIsThePass(t *testing.T) {
	// Every file satisfying the rule is no violations, in either mood: an empty result is what every rule in
	// the library returns when it holds, and there is no boolean beside the list.
	tests := []struct {
		name      string
		predicate assertion.FilePredicate
		mood      kernel.Mood
	}{
		{name: "should", predicate: func(file extraction.FileInfo) bool { return file.Extension == ".go" }, mood: kernel.Should},
		{name: "should not", predicate: func(file extraction.FileInfo) bool { return file.Extension == ".java" }, mood: kernel.ShouldNot},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := assertion.GatherAdherenceViolations(fixtureFiles(), test.predicate, "be written in Go", test.mood)

			if len(violations) != 0 {
				t.Errorf("GatherAdherenceViolations reported %v, want the pass", violations)
			}
		})
	}
}

func TestGatherAdherenceViolationsOfAnEmptySelectionIsSilentInBothMoods(t *testing.T) {
	// A selection with nothing in it has nothing to report here, which is exactly why the empty-test guard
	// exists a stage earlier: this function would call a stale glob green whichever mood it was written in.
	for _, files := range [][]extraction.FileInfo{nil, {}} {
		for _, mood := range []kernel.Mood{kernel.Should, kernel.ShouldNot} {
			violations := assertion.GatherAdherenceViolations(files, isSmall, "be small", mood)

			if len(violations) != 0 {
				t.Errorf("GatherAdherenceViolations(%v, ..., %s) reported %v, want nothing", files, mood, violations)
			}
		}
	}
}

func TestGatherAdherenceViolationsCarriesTheRequirementAndTheMoodItJudgedWith(t *testing.T) {
	// The violation carries the file, the words the rule was phrased in and the mood — which is all a report
	// can have, because the rule itself was a Go function and no report can print one.
	never := func(extraction.FileInfo) bool { return false }

	violations := assertion.GatherAdherenceViolations(fixtureFiles(), never, "declare a package comment", kernel.Should)

	if len(violations) != len(fixtureFiles()) {
		t.Fatalf("GatherAdherenceViolations reported %d violations, want all %d files", len(violations), len(fixtureFiles()))
	}
	for _, violation := range violations {
		adherence, ok := violation.(assertion.AdherenceViolation)
		if !ok {
			t.Fatalf("GatherAdherenceViolations reported a %T, want an AdherenceViolation", violation)
		}
		if adherence.Requirement != "declare a package comment" {
			t.Errorf("the violation of %s requires %q, want the message the user wrote", adherence.File, adherence.Requirement)
		}
		if adherence.Mood != kernel.Should {
			t.Errorf("the violation of %s was judged in mood %s, want %s", adherence.File, adherence.Mood, kernel.Should)
		}
	}
}

func TestGatherAdherenceViolationsOfANilPredicateReportsEverySelectedFile(t *testing.T) {
	// A missing predicate satisfies nothing, the way a zero matching.Filter matches nothing: a positive rule
	// written with one fails everywhere rather than passing quietly, and calling the nil function instead
	// would take the host test process down. The fluent API returns it as the user's error long before here.
	reported := assertion.GatherAdherenceViolations(fixtureFiles(), nil, "be judged by a function nobody gave", kernel.Should)
	if offenders := adherenceOffendersOf(t, reported); !slices.Equal(offenders, fixtureSelection()) {
		t.Errorf("GatherAdherenceViolations reported %v, want every selected file %v", offenders, fixtureSelection())
	}

	silent := assertion.GatherAdherenceViolations(fixtureFiles(), nil, "be judged by a function nobody gave", kernel.ShouldNot)
	if len(silent) != 0 {
		t.Errorf("GatherAdherenceViolations reported %v under `should not`, want nothing: no file satisfied anything", silent)
	}
}

// adherenceOffendersOf names the files the violations were reported for, in the order they were reported, and
// checks on the way that every one of them is an AdherenceViolation of the file-adherence kind.
func adherenceOffendersOf(t *testing.T, violations []kernel.Violation) []string {
	t.Helper()

	if len(violations) == 0 {
		return nil
	}
	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if violation.Kind() != assertion.KindFileAdherence {
			t.Errorf("a violation is of kind %q, want %q", violation.Kind(), assertion.KindFileAdherence)
		}
		adherence, ok := violation.(assertion.AdherenceViolation)
		if !ok {
			t.Fatalf("GatherAdherenceViolations reported a %T, want an AdherenceViolation", violation)
		}
		offenders = append(offenders, adherence.File)
	}
	return offenders
}
