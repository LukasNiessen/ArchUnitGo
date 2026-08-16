package fluentapi_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
	"github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
)

// A policy with a clause in it is a Checkable, which is the one thing every consumer of a rule programs
// against, and the declaration stages are not: a chain that has declared layers and written no clause is not
// yet a rule about anything.
var _ kernel.Checkable = fluentapi.LayersPolicyCondition{}

func TestAWholePolicyIsOneRuleCheckedInOnePass(t *testing.T) {
	// The reason this module exists: an N-layer policy is one chain and one check, where the same statement as
	// file rules is N² sentences. Every clause is in force, and each offending dependency is reported once.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).
		WhereLayer("api").MayOnlyDependOnLayers("domain").
		WhereLayer("domain").MayOnlyDependOnLayers().
		WhereLayer("db").MayNotDependOnLayers("api")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	want := []string{"api -> db", "domain -> db"}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, want) {
		t.Errorf("the policy reported %v, want %v: every clause of it applies", pairs, want)
	}
}

func TestAPolicyReportsTheClausesAndTheLayersItWasWrittenWith(t *testing.T) {
	// A violation is data, not a sentence: the two layers, the clause that was broken and the concrete file
	// dependencies. The words are the testing layer's, which is what keeps one place in charge of phrasing.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("domain").MayNotDependOnLayers("db")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if len(violations) != 1 {
		t.Fatalf("the policy reported %v, want the one violation the fixture has", messages(t, violations))
	}

	violation := layerViolation(t, violations[0])
	if violation.Kind() != layersassertion.KindLayerDependency {
		t.Errorf("the violation is of kind %q, want %q", violation.Kind(), layersassertion.KindLayerDependency)
	}
	if violation.Layer != "domain" || violation.DependsOn != "db" {
		t.Errorf("the violation is about %q -> %q, want %q -> %q", violation.Layer, violation.DependsOn, "domain", "db")
	}
	if len(violation.Dependencies) != 1 {
		t.Errorf("the violation carries %v, want the one file dependency that broke it", brokenBy(violation))
	}
}

func TestAPolicyWhoseLayerNoFileIsInIsReportedRatherThanJudged(t *testing.T) {
	// The empty-test guard on this module's terminal. Every clause about a layer nobody is in is vacuous, so a
	// policy whose folder has been renamed would be green forever — and the guard names the layer as well as the
	// pattern, because a policy has one population per layer and a reader has to know which to go and fix.
	root := writeLayeredFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").
		Layer("transport").DefinedByFolder("internal/transport/**").
		WhereLayer("api").MayNotDependOnLayers("transport")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if len(violations) != 1 {
		t.Fatalf("the policy reported %v, want the one empty layer it has", messages(t, violations))
	}
	empty, ok := violations[0].(assertion.EmptyTestViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want an EmptyTestViolation", violations[0])
	}
	if empty.Subject != `files in layer "transport"` {
		t.Errorf("the guard reports %q, want the layer named in it", empty.Subject)
	}
	if len(empty.Selectors) != 1 || !strings.Contains(empty.Selectors[0].String(), "internal/transport/**") {
		t.Errorf("the guard reports the selectors %v, want the layer's own patterns", empty.Selectors)
	}
}

func TestAPolicyReportsEveryEmptyLayerAtOnce(t *testing.T) {
	// A reader who renamed two folders is told about both, rather than fixing one pattern and coming back for
	// the other.
	root := writeLayeredFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("transport").DefinedByFolder("internal/transport/**").
		Layer("storage").DefinedByFolder("internal/storage/**").
		WhereLayer("transport").MayNotDependOnLayers("storage")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if len(violations) != 2 {
		t.Errorf("the policy reported %v, want one violation per empty layer", messages(t, violations))
	}
}

func TestAPolicyWithAnEmptyLayerIsJudgedWhenTheUserAsksForIt(t *testing.T) {
	// AllowEmptyTests is the opt-out, and it is the same knob on the same bag every terminal threads into the
	// guard: with it the policy is judged, and the layer nobody is in simply says nothing.
	root := writeLayeredFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").
		Layer("transport").DefinedByFolder("internal/transport/**").
		WhereLayer("api").MayNotDependOnLayers("transport")

	violations, err := policy.Check(&kernel.CheckOptions{AllowEmptyTests: true})
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if len(violations) != 0 {
		t.Errorf("the policy reported %v, want nothing: nothing is in the layer it forbids", messages(t, violations))
	}
}

func TestALayerPolicyThreadsTheCheckOptionsIntoTheExtraction(t *testing.T) {
	// AllowEmptyTests travels its own way to the guard, so every other knob on the bag reaches this terminal
	// through the extraction — and a policy judging a differently-extracted project would hold the layers over
	// files the user did not ask about, silently. IncludeTestFiles is the cheapest to observe: the fixture's db
	// layer reaches the api layer through its test file and through nothing else, so the one dependency this
	// blocklist forbids exists only when the knob is on.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("db").MayNotDependOnLayers("api")

	byDefault, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	withTests, err := policy.Check(&kernel.CheckOptions{IncludeTestFiles: true})
	if err != nil {
		t.Fatalf("checking %s with IncludeTestFiles failed: %v", policy, err)
	}

	if len(byDefault) != 0 {
		t.Errorf("the policy reported %v by default, want nothing: the db layer reaches the api layer from a test file alone",
			messages(t, byDefault))
	}
	if pairs := offendingPairs(t, withTests); !slices.Equal(pairs, []string{"db -> api"}) {
		t.Errorf("the policy reported %v with IncludeTestFiles, want the dependency the db layer's test file makes", pairs)
	}
}

func TestAPolicyIsAValueThatCanBeCheckedTwice(t *testing.T) {
	// A rule is a value, not an action: nothing is read until Check, and checking one twice answers the same
	// thing — which is what lets a suite keep its policies in a list and hand them around.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("domain").MayNotDependOnLayers("db")

	first, firstErr := policy.Check(nil)
	second, secondErr := policy.Check(nil)

	if firstErr != nil || secondErr != nil {
		t.Fatalf("checking %s failed: %v, %v", policy, firstErr, secondErr)
	}
	if !slices.Equal(offendingPairs(t, first), offendingPairs(t, second)) {
		t.Errorf("the policy reported %v and then %v, want the same answer twice",
			offendingPairs(t, first), offendingPairs(t, second))
	}
}

func TestTheClausesOfAPolicyMayBeTypedInEitherOrder(t *testing.T) {
	// Every clause of a policy is in force and each offending dependency is judged against all of them, so the
	// order the clauses were written in changes which sentence a violation blames and nothing else.
	root := writeLayeredFixtureProject(t)
	apiFirst := fixturePolicy(t, root).
		WhereLayer("api").MayOnlyDependOnLayers("domain").
		WhereLayer("domain").MayOnlyDependOnLayers()
	domainFirst := fixturePolicy(t, root).
		WhereLayer("domain").MayOnlyDependOnLayers().
		WhereLayer("api").MayOnlyDependOnLayers("domain")

	first, err := apiFirst.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", apiFirst, err)
	}
	second, err := domainFirst.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", domainFirst, err)
	}

	want := []string{"api -> db", "domain -> db"}
	if pairs := offendingPairs(t, first); !slices.Equal(pairs, want) {
		t.Errorf("the policy reported %v, want %v", pairs, want)
	}
	if !slices.Equal(offendingPairs(t, first), offendingPairs(t, second)) {
		t.Errorf("the two orders reported %v and %v, want the same policy either way",
			offendingPairs(t, first), offendingPairs(t, second))
	}
}

func TestAPolicyRendersTheWholeSentenceFromItsTerminal(t *testing.T) {
	// The terminal renders what the user typed, all of it, so a failing rule can be printed beside its
	// violations without the reader having to reconstruct which policy it was.
	policy := fluentapi.ProjectLayers(nil).
		Layer("api").DefinedByFolder("internal/api/**").
		WhereLayer("api").MayOnlyDependOnLayers()

	want := `project layers, layer "api" defined by path without filename matches "internal/api/**", ` +
		`where layer "api", may only depend on no layers`
	if rendered := policy.String(); rendered != want {
		t.Errorf("the policy reads\n%q, want\n%q", rendered, want)
	}
}

// offendingPairs are the pairs of layers these violations are about, as `api -> db`, in the order they were
// reported.
func offendingPairs(t *testing.T, violations []assertion.Violation) []string {
	t.Helper()

	pairs := make([]string, 0, len(violations))
	for _, violation := range violations {
		reported := layerViolation(t, violation)
		pairs = append(pairs, reported.Layer+" -> "+reported.DependsOn)
	}
	return pairs
}

// layerViolation is this violation as the layers module's own type, failing the test when a policy reported
// anything else.
func layerViolation(t *testing.T, violation assertion.Violation) layersassertion.DependencyViolation {
	t.Helper()

	reported, ok := violation.(layersassertion.DependencyViolation)
	if !ok {
		t.Fatalf("the violation is a %T, want a DependencyViolation", violation)
	}
	return reported
}

// brokenBy are the file dependencies this violation was broken by, as `a.go -> b.go`.
func brokenBy(violation layersassertion.DependencyViolation) []string {
	rendered := make([]string, 0, len(violation.Dependencies))
	for _, edge := range violation.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return rendered
}

// messages renders whatever a policy reported, of whichever kind, for a failure message about violations a
// test did not expect at all.
func messages(t *testing.T, violations []assertion.Violation) []string {
	t.Helper()

	rendered := make([]string, 0, len(violations))
	for _, violation := range violations {
		rendered = append(rendered, fmt.Sprintf("%s: %v", violation.Kind(), violation))
	}
	return rendered
}
