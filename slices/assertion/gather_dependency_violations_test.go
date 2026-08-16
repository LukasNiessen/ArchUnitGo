package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/slices/assertion"
)

// fixtureDependencies are the projected dependencies of a three-slice project: the api reaches both other
// slices, and the db reaches back into the api, which is what makes both moods of a rule over them have
// something to say.
func fixtureDependencies() []kernelprojection.ProjectedEdge {
	return []kernelprojection.ProjectedEdge{
		kernelprojection.NewProjectedEdge("api", "db",
			extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
			extraction.NewEdge("internal/api/router.go", "internal/db/query.go", false, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("api", "domain",
			extraction.NewEdge("internal/api/handler.go", "internal/domain/order.go", false, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("db", "api",
			extraction.NewEdge("internal/db/conn.go", "internal/api/router.go", false, extraction.ImportKindPlain)),
	}
}

func TestGatherDependencyViolationsReportsAForbiddenDependencyThatExists(t *testing.T) {
	// `should not, contain dependency "api", "db"`: the dependency is there, so one violation, carrying every
	// file dependency the pair of slices stood for — that list is the reader's next question and after
	// relabelling it lives nowhere else.
	violations := assertion.GatherDependencyViolations("api", "db", fixtureDependencies(), kernel.ShouldNot)

	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"api -> db"}) {
		t.Fatalf("the rule reported %v, want the one dependency it forbids", pairs)
	}
	violation := sliceViolation(t, violations[0])
	want := []string{
		"internal/api/handler.go -> internal/db/conn.go",
		"internal/api/router.go -> internal/db/query.go",
	}
	if !slices.Equal(brokenBy(violation), want) {
		t.Errorf("the violation was broken by %v, want %v", brokenBy(violation), want)
	}
	if violation.Mood != kernel.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want %s", violation.Mood, kernel.ShouldNot)
	}
}

func TestGatherDependencyViolationsReportsARequiredDependencyThatIsMissing(t *testing.T) {
	// The same walk over the same projection, one flag apart. `should, contain dependency "db", "domain"` is
	// broken by a dependency that is not there, so the violation carries no file dependencies: what it reports
	// is that there were none.
	violations := assertion.GatherDependencyViolations("db", "domain", fixtureDependencies(), kernel.Should)

	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"db -> domain"}) {
		t.Fatalf("the rule reported %v, want the one dependency it required", pairs)
	}
	violation := sliceViolation(t, violations[0])
	if len(violation.Dependencies) != 0 {
		t.Errorf("the violation carries %v, want nothing: the dependency it is about is the missing one", brokenBy(violation))
	}
	if violation.Mood != kernel.Should {
		t.Errorf("the violation was judged in mood %s, want %s", violation.Mood, kernel.Should)
	}
}

func TestGatherDependencyViolationsReportsNothingWhenTheRuleHolds(t *testing.T) {
	// Both passes, and they are the two moods' answers to the projection above: a forbidden dependency that is
	// not there, and a required one that is.
	tests := []struct {
		name string
		from string
		to   string
		mood kernel.Mood
	}{
		{"a forbidden dependency the project does not have", "db", "domain", kernel.ShouldNot},
		{"a required dependency the project has", "api", "db", kernel.Should},
		{"a forbidden dependency of a slice nothing depends on", "domain", "api", kernel.ShouldNot},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := assertion.GatherDependencyViolations(test.from, test.to, fixtureDependencies(), test.mood)

			if len(violations) != 0 {
				t.Errorf("the rule reported %v, want nothing", offendingPairs(t, violations))
			}
		})
	}
}

func TestGatherDependencyViolationsAsksAboutTheDirectionTheRuleWasWrittenIn(t *testing.T) {
	// `api -> db` and `db -> api` are two different sentences, and a projection may hold one without the
	// other. The fixture holds both, so the observable case is a pair that exists in one direction only.
	dependencies := []kernelprojection.ProjectedEdge{
		kernelprojection.NewProjectedEdge("api", "domain",
			extraction.NewEdge("internal/api/handler.go", "internal/domain/order.go", false, extraction.ImportKindPlain)),
	}

	forward := assertion.GatherDependencyViolations("api", "domain", dependencies, kernel.ShouldNot)
	backward := assertion.GatherDependencyViolations("domain", "api", dependencies, kernel.ShouldNot)

	if pairs := offendingPairs(t, forward); !slices.Equal(pairs, []string{"api -> domain"}) {
		t.Errorf("the rule about api -> domain reported %v, want that dependency", pairs)
	}
	if len(backward) != 0 {
		t.Errorf("the rule about domain -> api reported %v, want nothing: it is the converse sentence", offendingPairs(t, backward))
	}
}

func TestGatherDependencyViolationsOnAnEmptyProjection(t *testing.T) {
	// A projection with no edges in it breaks nothing a negated rule can be broken by, and that silence is
	// somebody else's to report: the empty-test guard runs before this function, in the rule's terminal.
	// The positive mood is the exception — a required dependency is missing from an empty projection too, and
	// saying so is this function's own answer.
	if violations := assertion.GatherDependencyViolations("api", "db", nil, kernel.ShouldNot); len(violations) != 0 {
		t.Errorf("an empty projection reported %v for a forbidden dependency, want nothing", offendingPairs(t, violations))
	}
	violations := assertion.GatherDependencyViolations("api", "db", nil, kernel.Should)
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"api -> db"}) {
		t.Errorf("an empty projection reported %v for a required dependency, want it missing", pairs)
	}
}

// offendingPairs are the pairs of slices these violations are about, as `api -> db`, in the order they were
// reported.
func offendingPairs(t *testing.T, violations []kernel.Violation) []string {
	t.Helper()

	pairs := make([]string, 0, len(violations))
	for _, violation := range violations {
		reported := sliceViolation(t, violation)
		pairs = append(pairs, reported.Slice+" -> "+reported.DependsOn)
	}
	return pairs
}

// sliceViolation is this violation as the slices module's own type, failing the test when a rule reported
// anything else.
func sliceViolation(t *testing.T, violation kernel.Violation) assertion.DependencyViolation {
	t.Helper()

	reported, ok := violation.(assertion.DependencyViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want a DependencyViolation", violation)
	}
	return reported
}

// brokenBy are the file dependencies this violation was broken by, as `a.go -> b.go`.
func brokenBy(violation assertion.DependencyViolation) []string {
	rendered := make([]string, 0, len(violation.Dependencies))
	for _, edge := range violation.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}
