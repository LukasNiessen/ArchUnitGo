package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// fixtureDependencies are the projected edges of `project files, in folder "internal/api/**", ..., depend on
// files, in folder "internal/db/**"` over the fixture selection: the handler reaches the database twice and no
// other selected file reaches it at all, which is what makes both moods have something to say.
func fixtureDependencies() []kernelprojection.ProjectedEdge {
	return []kernelprojection.ProjectedEdge{
		kernelprojection.NewProjectedEdge("internal/api/handler.go", "internal/db/conn.go",
			extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("internal/api/handler.go", "internal/db/query.go",
			extraction.NewEdge("internal/api/handler.go", "internal/db/query.go", false, extraction.ImportKindPlain)),
	}
}

func TestGatherDependencyViolationsOfTheNegatedMoodReportsTheFilesThatDepend(t *testing.T) {
	// `should not depend on files in folder "internal/db/**"`: the one selected file that reaches the object is
	// reported, and it is reported once, carrying every dependency it was broken by.
	required := []matching.Filter{folderMatcher(t, "internal/db/**")}

	violations := assertion.GatherDependencyViolations(fixtureSelection(), fixtureDependencies(), required, kernel.ShouldNot)

	if offenders := dependingOffenders(t, violations); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Fatalf("GatherDependencyViolations reported %v, want the one file that depends on the object", offenders)
	}
	violation, ok := violations[0].(assertion.DependencyViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want a DependencyViolation", violations[0])
	}
	want := []string{"internal/db/conn.go", "internal/db/query.go"}
	if !slices.Equal(violation.Dependencies, want) {
		t.Errorf("the violation carries %v, want every dependency it was broken by: %v", violation.Dependencies, want)
	}
	if len(violation.Required) != 1 || violation.Required[0].Pattern().Source() != "internal/db/**" {
		t.Errorf("the violation carries %v, want the object selectors passed through untouched", violation.Required)
	}
	if violation.Mood != kernel.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want %s", violation.Mood, kernel.ShouldNot)
	}
}

func TestGatherDependencyViolationsOfThePositiveMoodReportsTheFilesThatDoNot(t *testing.T) {
	// `should depend on files in folder "internal/db/**"`: the same walk over the same projection, one flag
	// apart — so the offenders are exactly the files the negated mood was happy with, each carrying no
	// dependency, because the absence is the offense.
	required := []matching.Filter{folderMatcher(t, "internal/db/**")}

	violations := assertion.GatherDependencyViolations(fixtureSelection(), fixtureDependencies(), required, kernel.Should)

	want := []string{"internal/api/handler_test.go", "internal/db/conn.go", "main.go"}
	if offenders := dependingOffenders(t, violations); !slices.Equal(offenders, want) {
		t.Fatalf("GatherDependencyViolations reported %v, want the files that reach none of the object: %v", offenders, want)
	}
	for _, violation := range violations {
		reported, ok := violation.(assertion.DependencyViolation)
		if !ok {
			t.Fatalf("a violation is a %T, want a DependencyViolation", violation)
		}
		if len(reported.Dependencies) != 0 {
			t.Errorf("the violation of %s carries %v, want none: it was reported for depending on nothing",
				reported.File, reported.Dependencies)
		}
	}
}

func TestGatherDependencyViolationsInTheTwoMoodsPartitionTheSelection(t *testing.T) {
	// The property that makes one gather function enough for both moods: every selected file offends exactly one
	// of a rule and its negation, so nothing is judged twice and nothing escapes judgement.
	required := []matching.Filter{folderMatcher(t, "internal/db/**")}

	positive := dependingOffenders(t, assertion.GatherDependencyViolations(fixtureSelection(), fixtureDependencies(), required, kernel.Should))
	negated := dependingOffenders(t, assertion.GatherDependencyViolations(fixtureSelection(), fixtureDependencies(), required, kernel.ShouldNot))

	both := slices.Concat(positive, negated)
	slices.Sort(both)
	if !slices.Equal(both, fixtureSelection()) {
		t.Errorf("the two moods reported %v between them, want each selected file exactly once: %v", both, fixtureSelection())
	}
}

func TestGatherDependencyViolationsJudgesTheSelectionAndNotTheProjection(t *testing.T) {
	// The subjects of the rule are the files the scope selected, so a dependency whose source is not among them
	// is not this rule's dependency — a projection that reached wider than the scope would report a file the
	// user never asked about, and there would be no pattern in the report to explain it.
	dependencies := append(fixtureDependencies(),
		kernelprojection.NewProjectedEdge("cmd/tool/main.go", "internal/db/conn.go",
			extraction.NewEdge("cmd/tool/main.go", "internal/db/conn.go", false, extraction.ImportKindPlain)))

	violations := assertion.GatherDependencyViolations(fixtureSelection(), dependencies, nil, kernel.ShouldNot)

	if offenders := dependingOffenders(t, violations); !slices.Equal(offenders, []string{"internal/api/handler.go"}) {
		t.Errorf("GatherDependencyViolations reported %v, want only the selected file among them", offenders)
	}
}

func TestGatherDependencyViolationsReportsInTheOrderTheFilesArrived(t *testing.T) {
	// The order is the selection's, which projection.SelectFiles sorted, so a report is reproducible and reads
	// down the project rather than in the order a map happened to be walked.
	violations := assertion.GatherDependencyViolations(fixtureSelection(), nil, nil, kernel.Should)

	if offenders := dependingOffenders(t, violations); !slices.Equal(offenders, fixtureSelection()) {
		t.Errorf("GatherDependencyViolations reported %v, want the whole selection in its own order: %v",
			offenders, fixtureSelection())
	}
}

func TestGatherDependencyViolationsOfAnEmptySelectionReportsNothing(t *testing.T) {
	// A rule that selected no file is the empty-test guard's answer, not this one's: there is no file here to
	// judge, in either mood, and inventing a violation for a population that does not exist would report a
	// failure no file's name could be printed beside.
	for _, mood := range []kernel.Mood{kernel.Should, kernel.ShouldNot} {
		t.Run(mood.String(), func(t *testing.T) {
			violations := assertion.GatherDependencyViolations(nil, fixtureDependencies(), nil, mood)

			if len(violations) != 0 {
				t.Errorf("GatherDependencyViolations reported %v, want nothing said about an empty selection", violations)
			}
		})
	}
}

// dependingOffenders names the files a dependency rule reported, in the order it reported them, checking on
// the way that every violation is a DependencyViolation of the file-dependency kind.
func dependingOffenders(t *testing.T, violations []kernel.Violation) []string {
	t.Helper()

	if len(violations) == 0 {
		return nil
	}
	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if violation.Kind() != assertion.KindFileDependency {
			t.Errorf("a violation is of kind %q, want %q", violation.Kind(), assertion.KindFileDependency)
		}
		dependency, ok := violation.(assertion.DependencyViolation)
		if !ok {
			t.Fatalf("a violation is a %T, want a DependencyViolation", violation)
		}
		offenders = append(offenders, dependency.File)
	}
	return offenders
}
