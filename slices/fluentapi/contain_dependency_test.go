package fluentapi_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	slicesassertion "github.com/LukasNiessen/ArchUnitGo/slices/assertion"
	"github.com/LukasNiessen/ArchUnitGo/slices/fluentapi"
)

// A rule with a predicate in it is a Checkable, which is the one thing every consumer of a rule programs
// against; the slicing and the mood before it are not, because neither is yet a rule about anything.
var _ kernel.Checkable = fluentapi.SlicesDependencyCondition{}

func TestAForbiddenDependencyBetweenSlicesIsReportedWithTheFilesThatBrokeIt(t *testing.T) {
	// The sentence a slicing exists for. The violation is data, not a phrase — the two slices, the mood and the
	// concrete imports — because after the relabelling the files live nowhere else, and they are the reader's
	// next question.
	root := writeSlicedFixtureProject(t)
	rule := fixtureSlicing(t, root).ShouldNot().ContainDependency("api", "db")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("the rule reported %v, want the one dependency it forbids", messages(t, violations))
	}
	violation := sliceViolation(t, violations[0])
	if violation.Slice != "api" || violation.DependsOn != "db" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", violation.Slice, violation.DependsOn, "api", "db")
	}
	if violation.Mood != assertion.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want %s", violation.Mood, assertion.ShouldNot)
	}
	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if !slices.Equal(brokenBy(violation), want) {
		t.Errorf("the violation was broken by %v, want %v", brokenBy(violation), want)
	}
	if kind := violation.Kind(); kind != slicesassertion.KindSliceDependency {
		t.Errorf("the violation is of kind %q, want %q", kind, slicesassertion.KindSliceDependency)
	}
}

func TestARequiredDependencyTheProjectDoesNotHaveIsReported(t *testing.T) {
	// The positive mood, and the one violation in the library that is about something absent: the slices were
	// found and the projection was judged, and what is missing is one edge between them. There is nothing to
	// show, so the violation carries no files.
	root := writeSlicedFixtureProject(t)
	rule := fixtureSlicing(t, root).Should().ContainDependency("db", "domain")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("the rule reported %v, want the one dependency it requires", messages(t, violations))
	}
	violation := sliceViolation(t, violations[0])
	if violation.Slice != "db" || violation.DependsOn != "domain" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", violation.Slice, violation.DependsOn, "db", "domain")
	}
	if len(violation.Dependencies) != 0 {
		t.Errorf("the violation carries %v, want nothing: the dependency it reports is the missing one", brokenBy(violation))
	}
}

func TestARuleAboutSlicesThatHoldsReportsNothing(t *testing.T) {
	// The passes, and they are the two moods' answers to the same project: a forbidden dependency the fixture
	// does not have, and a required one it does.
	root := writeSlicedFixtureProject(t)
	tests := []struct {
		name string
		rule kernel.Checkable
	}{
		{
			name: "a forbidden dependency the project does not have",
			rule: fixtureSlicing(t, root).ShouldNot().ContainDependency("db", "api"),
		},
		{
			name: "a required dependency the project has",
			rule: fixtureSlicing(t, root).Should().ContainDependency("api", "domain"),
		},
		{
			name: "a forbidden dependency of a slice that reaches nothing",
			rule: fixtureSlicing(t, root).ShouldNot().ContainDependency("domain", "db"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations, err := test.rule.Check(nil)
			if err != nil {
				t.Fatalf("checking %s failed: %v", test.rule, err)
			}
			if len(violations) != 0 {
				t.Errorf("the rule reported %v, want nothing", messages(t, violations))
			}
		})
	}
}

func TestTheDirectionOfTheDependencyIsHalfTheSentence(t *testing.T) {
	// `api -> db` and `db -> api` are two different rules, and a project may break one and keep the other.
	// Forbidding both ways round is two rules, on purpose, because each is broken by different imports.
	root := writeSlicedFixtureProject(t)
	forward := fixtureSlicing(t, root).ShouldNot().ContainDependency("api", "db")
	backward := fixtureSlicing(t, root).ShouldNot().ContainDependency("db", "api")

	broken, err := forward.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", forward, err)
	}
	kept, err := backward.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", backward, err)
	}

	if len(broken) != 1 {
		t.Errorf("the rule about api -> db reported %v, want the dependency the fixture has", messages(t, broken))
	}
	if len(kept) != 0 {
		t.Errorf("the rule about db -> api reported %v, want nothing: it is the converse sentence", messages(t, kept))
	}
}

func TestARuleAboutSlicesThreadsTheCheckOptionsIntoTheExtraction(t *testing.T) {
	// AllowEmptyTests travels its own way to the guard, so every other knob on the bag reaches this terminal
	// through the extraction — and a rule judging a differently-extracted project would hold the slices over files
	// the user did not ask about, silently. IncludeTestFiles is the cheapest to observe: the fixture's db slice
	// reaches the api slice through its test file and through nothing else, so the one dependency this rule
	// forbids exists only when the knob is on.
	root := writeSlicedFixtureProject(t)
	rule := fixtureSlicing(t, root).ShouldNot().ContainDependency("db", "api")

	byDefault, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}
	withTests, err := rule.Check(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("checking %s with IncludeTestFiles failed: %v", rule, err)
	}

	if len(byDefault) != 0 {
		t.Errorf("the rule reported %v by default, want nothing: the db slice reaches the api slice from a test file alone",
			messages(t, byDefault))
	}
	if len(withTests) != 1 {
		t.Fatalf("the rule reported %v with IncludeTestFiles, want the dependency the db slice's test file makes",
			messages(t, withTests))
	}
	violation := sliceViolation(t, withTests[0])
	if violation.Slice != "db" || violation.DependsOn != "api" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", violation.Slice, violation.DependsOn, "db", "api")
	}
	want := []string{"internal/db/conn_test.go -> internal/api/handler.go", "internal/db/conn_test.go -> internal/api/router.go"}
	if broken := brokenBy(violation); !slices.Equal(broken, want) {
		t.Errorf("the violation was broken by %v, want %v", broken, want)
	}
}

func TestASlicingThatFoundNoSliceIsReportedRatherThanJudged(t *testing.T) {
	// The empty-test guard on this module's terminal, and the reason it is there: a slicing whose folder has
	// been renamed projects no dependencies at all, so `should not contain dependency` would be green forever.
	// One violation and not three — the slicing explains why both named slices are empty too.
	root := writeSlicedFixtureProject(t)
	rule := fluentapi.ProjectSlices(fixtureLocator(t, root)).
		DefinedBy("modules/(**)/**").
		ShouldNot().
		ContainDependency("api", "db")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("the rule reported %v, want the one slicing that found nothing", messages(t, violations))
	}
	empty := emptyTestViolation(t, violations[0])
	if empty.Subject != "slices" {
		t.Errorf("the guard reports %q, want the vocabulary the entry point names", empty.Subject)
	}
	if len(empty.Selectors) != 1 || !strings.Contains(empty.Selectors[0].String(), "modules/(**)/**") {
		t.Errorf("the guard reports the selectors %v, want the slicing's own pattern", empty.Selectors)
	}
}

func TestARuleAboutASliceNobodyIsInIsReportedRatherThanJudged(t *testing.T) {
	// The other stale glob: the slicing works, and the rule names a slice the project no longer has. Both moods
	// of such a rule are vacuous, so the guard names the slice as well as the vocabulary — the pattern alone
	// would not say which half of the sentence a reader has to go and fix.
	root := writeSlicedFixtureProject(t)
	rule := fixtureSlicing(t, root).ShouldNot().ContainDependency("api", "transport")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 1 {
		t.Fatalf("the rule reported %v, want the one slice nobody is in", messages(t, violations))
	}
	if subject := emptyTestViolation(t, violations[0]).Subject; subject != `files in slice "transport"` {
		t.Errorf("the guard reports %q, want the slice named in it", subject)
	}
}

func TestBothEndsOfARuleAboutSlicesNobodyIsInAreReportedAtOnce(t *testing.T) {
	// A reader who renamed two folders is told about both, rather than fixing one name and coming back for the
	// other. The guard reports every population it is given.
	root := writeSlicedFixtureProject(t)
	rule := fixtureSlicing(t, root).ShouldNot().ContainDependency("transport", "storage")

	violations, err := rule.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 2 {
		t.Errorf("the rule reported %v, want one violation per slice nobody is in", messages(t, violations))
	}
}

func TestARuleAboutAnEmptySliceIsJudgedWhenTheUserAsksForIt(t *testing.T) {
	// AllowEmptyTests is the opt-out, and it is the same knob on the same bag every terminal threads into the
	// guard: with it the rule is judged, and the slice nobody is in simply has no dependencies.
	root := writeSlicedFixtureProject(t)
	rule := fixtureSlicing(t, root).ShouldNot().ContainDependency("api", "transport")

	violations, err := rule.Check(&kernel.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("checking %s failed: %v", rule, err)
	}

	if len(violations) != 0 {
		t.Errorf("the rule reported %v, want nothing: the user asked for the empty slice to be judged", messages(t, violations))
	}
}

func TestASliceNamedWithTheEmptyStringIsAUserError(t *testing.T) {
	// A slice is a name a pattern cut out of an identifier, and the projection never produces an empty one, so
	// such a rule would judge nothing at all whichever mood it is written in.
	slicing := fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**")
	rules := map[string]kernel.Checkable{
		"the depending slice":   slicing.ShouldNot().ContainDependency("", "db"),
		"the slice it is about": slicing.Should().ContainDependency("api", ""),
	}

	for half, rule := range rules {
		t.Run(half, func(t *testing.T) {
			_, err := rule.Check(nil)

			if !errors.Is(err, fluentapi.ErrUnnamedSlice) {
				t.Fatalf("checking %s returned %v, want ErrUnnamedSlice", rule, err)
			}
			if operation := userError(t, err).Operation; operation != "contain dependency" {
				t.Errorf("UserError.Operation = %q, want the predicate at fault, %q", operation, "contain dependency")
			}
		})
	}
}

func TestASliceDependingOnItselfIsAUserError(t *testing.T) {
	// A slice may always depend on itself and the projection does not even carry that dependency, so the negated
	// rule would hold forever and the positive one could never hold: neither is a rule about the code, and both
	// are rejected where they are written rather than reported as violations.
	slicing := fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**")
	rules := map[string]fluentapi.SlicesDependencyCondition{
		"should":     slicing.Should().ContainDependency("api", "api"),
		"should not": slicing.ShouldNot().ContainDependency("api", "api"),
	}

	for mood, rule := range rules {
		t.Run(mood, func(t *testing.T) {
			_, err := rule.Check(nil)

			if !errors.Is(err, fluentapi.ErrSelfDependency) {
				t.Fatalf("checking %s returned %v, want ErrSelfDependency", rule, err)
			}
			if subject := userError(t, err).Subject; subject != "api" {
				t.Errorf("UserError.Subject = %q, want the name that was given twice, %q", subject, "api")
			}
			if !strings.Contains(rule.String(), "rejected") {
				t.Errorf("%s renders without the rejection, want it visible in a test failure", rule)
			}
		})
	}
}

func TestARejectedRuleAboutSlicesIsReportedBeforeTheProjectIsRead(t *testing.T) {
	// What the user typed is wrong whatever the project turns out to be, and reading the project first would
	// answer a typo with a complaint about the locator.
	root := writeSlicedFixtureProject(t)
	rule := fixtureSlicing(t, root).ShouldNot().ContainDependency("api", "api")

	violations, err := rule.Check(nil)

	if !errors.Is(err, fluentapi.ErrSelfDependency) {
		t.Fatalf("checking %s returned %v, want the rejected rule", rule, err)
	}
	if len(violations) != 0 {
		t.Errorf("the rule reported %v alongside the error, want nothing: there was no runnable rule", messages(t, violations))
	}
}

// messages are these violations as their own log lines, for a test failure that has to say what a rule
// reported.
func messages(t *testing.T, violations []assertion.Violation) []string {
	t.Helper()

	rendered := make([]string, 0, len(violations))
	for _, violation := range violations {
		rendered = append(rendered, fmt.Sprintf("%s: %v", violation.Kind(), violation))
	}
	return rendered
}

// sliceViolation is this violation as the slices module's own type, failing the test when a rule reported
// anything else.
func sliceViolation(t *testing.T, violation assertion.Violation) slicesassertion.DependencyViolation {
	t.Helper()

	reported, ok := violation.(slicesassertion.DependencyViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want a DependencyViolation", violation)
	}
	return reported
}

// emptyTestViolation is this violation as the empty-test guard's own type, failing the test when a rule
// reported anything else.
func emptyTestViolation(t *testing.T, violation assertion.Violation) assertion.EmptyTestViolation {
	t.Helper()

	reported, ok := violation.(assertion.EmptyTestViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want an EmptyTestViolation", violation)
	}
	return reported
}

// brokenBy are the file dependencies this violation was broken by, as `a.go -> b.go`.
func brokenBy(violation slicesassertion.DependencyViolation) []string {
	rendered := make([]string, 0, len(violation.Dependencies))
	for _, edge := range violation.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}
