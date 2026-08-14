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

// fixtureExternalDependencies are the projected edges of `project files, ..., depend on external modules,
// matching "*.*/**"` over the fixture selection: one selected file leaves the project twice, for two packages
// of one module, and no other selected file leaves it at all — which is what makes both moods have something
// to say.
func fixtureExternalDependencies() []kernelprojection.ProjectedEdge {
	return []kernelprojection.ProjectedEdge{
		kernelprojection.NewProjectedEdge("internal/db/conn.go", "gorm.io/gorm",
			extraction.NewEdge("internal/db/conn.go", "gorm.io/gorm", true, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("internal/db/conn.go", "gorm.io/gorm/clause",
			extraction.NewEdge("internal/db/conn.go", "gorm.io/gorm/clause", true, extraction.ImportKindPlain)),
	}
}

func TestGatherExternalDependencyViolationsOfTheNegatedMoodReportsTheFilesThatDepend(t *testing.T) {
	// `should not depend on external modules matching "gorm.io/**"`, which is the mood nearly every
	// third-party policy is written in: the one selected file that leaves the project for such a module is
	// reported, and it is reported once, carrying every import path it was broken by.
	required := []matching.Filter{pathMatcher(t, "gorm.io/**")}

	violations := assertion.GatherExternalDependencyViolations(
		fixtureSelection(), fixtureExternalDependencies(), required, kernel.ShouldNot)

	if offenders := externallyDependingOffenders(t, violations); !slices.Equal(offenders, []string{"internal/db/conn.go"}) {
		t.Fatalf("GatherExternalDependencyViolations reported %v, want the one file that depends on the object", offenders)
	}
	violation, ok := violations[0].(assertion.ExternalDependencyViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want an ExternalDependencyViolation", violations[0])
	}
	want := []string{"gorm.io/gorm", "gorm.io/gorm/clause"}
	if !slices.Equal(violation.Modules, want) {
		t.Errorf("the violation carries %v, want every module it was broken by: %v", violation.Modules, want)
	}
	if len(violation.Required) != 1 || violation.Required[0].Pattern().Source() != "gorm.io/**" {
		t.Errorf("the violation carries %v, want the object selectors passed through untouched", violation.Required)
	}
	if violation.Mood != kernel.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want %s", violation.Mood, kernel.ShouldNot)
	}
}

func TestGatherExternalDependencyViolationsOfThePositiveMoodReportsTheFilesThatDoNot(t *testing.T) {
	// `should depend on external modules matching "gorm.io/**"`: the same walk over the same projection, one
	// flag apart — so the offenders are exactly the files the negated mood was happy with, each carrying no
	// module, because the absence is the offense.
	required := []matching.Filter{pathMatcher(t, "gorm.io/**")}

	violations := assertion.GatherExternalDependencyViolations(
		fixtureSelection(), fixtureExternalDependencies(), required, kernel.Should)

	want := []string{"internal/api/handler.go", "internal/api/handler_test.go", "main.go"}
	if offenders := externallyDependingOffenders(t, violations); !slices.Equal(offenders, want) {
		t.Fatalf("GatherExternalDependencyViolations reported %v, want the files that reach none of the object: %v",
			offenders, want)
	}
	for _, violation := range violations {
		reported, ok := violation.(assertion.ExternalDependencyViolation)
		if !ok {
			t.Fatalf("a violation is a %T, want an ExternalDependencyViolation", violation)
		}
		if len(reported.Modules) != 0 {
			t.Errorf("the violation of %s carries %v, want none: it was reported for depending on nothing",
				reported.File, reported.Modules)
		}
	}
}

func TestGatherExternalDependencyViolationsInTheTwoMoodsPartitionTheSelection(t *testing.T) {
	// The property that makes one gather function enough for both moods: every selected file offends exactly
	// one of a rule and its negation, so nothing is judged twice and nothing escapes judgement.
	required := []matching.Filter{pathMatcher(t, "gorm.io/**")}

	positive := externallyDependingOffenders(t, assertion.GatherExternalDependencyViolations(
		fixtureSelection(), fixtureExternalDependencies(), required, kernel.Should))
	negated := externallyDependingOffenders(t, assertion.GatherExternalDependencyViolations(
		fixtureSelection(), fixtureExternalDependencies(), required, kernel.ShouldNot))

	both := slices.Concat(positive, negated)
	slices.Sort(both)
	if !slices.Equal(both, fixtureSelection()) {
		t.Errorf("the two moods reported %v between them, want each selected file exactly once: %v",
			both, fixtureSelection())
	}
}

func TestGatherExternalDependencyViolationsJudgesTheSelectionAndNotTheProjection(t *testing.T) {
	// The subjects of the rule are the files the scope selected, so a dependency whose source is not among them
	// is not this rule's dependency — a projection that reached wider than the scope would report a file the
	// user never asked about, and there would be no pattern in the report to explain it.
	dependencies := append(fixtureExternalDependencies(),
		kernelprojection.NewProjectedEdge("cmd/tool/main.go", "gorm.io/gorm",
			extraction.NewEdge("cmd/tool/main.go", "gorm.io/gorm", true, extraction.ImportKindPlain)))

	violations := assertion.GatherExternalDependencyViolations(fixtureSelection(), dependencies, nil, kernel.ShouldNot)

	if offenders := externallyDependingOffenders(t, violations); !slices.Equal(offenders, []string{"internal/db/conn.go"}) {
		t.Errorf("GatherExternalDependencyViolations reported %v, want only the selected file among them", offenders)
	}
}

func TestGatherExternalDependencyViolationsReportsInTheOrderTheFilesArrived(t *testing.T) {
	// The order is the selection's, which projection.SelectFiles sorted, so a report is reproducible and reads
	// down the project rather than in the order a map happened to be walked.
	violations := assertion.GatherExternalDependencyViolations(fixtureSelection(), nil, nil, kernel.Should)

	if offenders := externallyDependingOffenders(t, violations); !slices.Equal(offenders, fixtureSelection()) {
		t.Errorf("GatherExternalDependencyViolations reported %v, want the whole selection in its own order: %v",
			offenders, fixtureSelection())
	}
}

func TestGatherExternalDependencyViolationsOfTheNegatedMoodPassesWhenNoModuleWasReached(t *testing.T) {
	// The ordinary answer for this family, and the reason its object is not put through the empty-test guard:
	// a pattern that matched no module and a project that imports no such module are one statement, and for
	// `should not` that statement is the pass — a guard here would fail every well-behaved project.
	violations := assertion.GatherExternalDependencyViolations(
		fixtureSelection(), nil, []matching.Filter{pathMatcher(t, "github.com/deprecated/**")}, kernel.ShouldNot)

	if len(violations) != 0 {
		t.Errorf("GatherExternalDependencyViolations reported %v, want the pass: no file reached a forbidden module",
			violations)
	}
}

func TestGatherExternalDependencyViolationsOfAnEmptySelectionReportsNothing(t *testing.T) {
	// A rule that selected no file is the empty-test guard's answer, not this one's: there is no file here to
	// judge, in either mood, and inventing a violation for a population that does not exist would report a
	// failure no file's name could be printed beside.
	for _, mood := range []kernel.Mood{kernel.Should, kernel.ShouldNot} {
		t.Run(mood.String(), func(t *testing.T) {
			violations := assertion.GatherExternalDependencyViolations(nil, fixtureExternalDependencies(), nil, mood)

			if len(violations) != 0 {
				t.Errorf("GatherExternalDependencyViolations reported %v, want nothing said about an empty selection",
					violations)
			}
		})
	}
}

// externallyDependingOffenders names the files a third-party dependency rule reported, in the order it
// reported them, checking on the way that every violation is an ExternalDependencyViolation of the
// file-external-dependency kind.
func externallyDependingOffenders(t *testing.T, violations []kernel.Violation) []string {
	t.Helper()

	if len(violations) == 0 {
		return nil
	}
	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		if violation.Kind() != assertion.KindFileExternalDependency {
			t.Errorf("a violation is of kind %q, want %q", violation.Kind(), assertion.KindFileExternalDependency)
		}
		dependency, ok := violation.(assertion.ExternalDependencyViolation)
		if !ok {
			t.Fatalf("a violation is a %T, want an ExternalDependencyViolation", violation)
		}
		offenders = append(offenders, dependency.File)
	}
	return offenders
}
