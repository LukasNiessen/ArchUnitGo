package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	metricsassertion "github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

// The two zone checks are rules, so they are the fluentapi.Checkable every consumer of a rule programs
// against, asserted at compile time rather than in a test that could be deleted.
var _ kernel.Checkable = fluentapi.MetricsZoneCondition{}

func TestShouldNotBeInZoneOfPainReportsTheConcreteAndDependedUponPackages(t *testing.T) {
	// The whole rule through the public chain: `internal/db` is depended on by both other packages and declares
	// nothing but a struct, which is the corner every change is paid for in.
	rule := fluentapi.Metrics(zonedProject(t)).Distance().ShouldNotBeInZoneOfPain()

	violations := check(t, rule, nil)

	if got := zoneOffendersOf(violations); !slices.Equal(got, []string{"internal/db"}) {
		t.Errorf("%s reported %v, want [internal/db]", rule, got)
	}
	reported, ok := violations[0].(metricsassertion.ZoneViolation)
	if !ok {
		t.Fatalf("reported a %T, want a metrics ZoneViolation", violations[0])
	}
	if reported.Zone != "zone of pain" || reported.Abstractness != 0 || reported.Instability != 0 {
		t.Errorf("reported %s, want internal/db at the concrete, depended-upon corner", reported)
	}
}

func TestShouldNotBeInZoneOfUselessnessReportsTheAbstractAndUnusedPackages(t *testing.T) {
	// `internal/port` is nothing but an interface and nothing depends on it, which is an abstraction nobody
	// asked for.
	rule := fluentapi.Metrics(zonedProject(t)).Distance().ShouldNotBeInZoneOfUselessness()

	violations := check(t, rule, nil)

	if got := zoneOffendersOf(violations); !slices.Equal(got, []string{"internal/port"}) {
		t.Errorf("%s reported %v, want [internal/port]", rule, got)
	}
	reported, ok := violations[0].(metricsassertion.ZoneViolation)
	if !ok {
		t.Fatalf("reported a %T, want a metrics ZoneViolation", violations[0])
	}
	if reported.Zone != "zone of uselessness" || reported.Abstractness != 1 || reported.Instability != 1 {
		t.Errorf("reported %s, want internal/port at the abstract, unused corner", reported)
	}
}

func TestAZoneCheckLeavesThePackagesOffTheCornersAlone(t *testing.T) {
	// The fixture the rest of the metrics tests measure: `internal/api` is on the main sequence, at A 0.5 and
	// I 0.5, and `.` is at the good concrete end of it, so neither rule has anything to say about either —
	// which a check reporting every package it read could not manage.
	locator := measuredProject(t)

	pain := check(t, fluentapi.Metrics(locator).Distance().ShouldNotBeInZoneOfPain(), nil)
	uselessness := check(t, fluentapi.Metrics(locator).Distance().ShouldNotBeInZoneOfUselessness(), nil)

	if got := zoneOffendersOf(pain); !slices.Equal(got, []string{"internal/db"}) {
		t.Errorf("`should not be in zone of pain` reported %v, want the one concrete, depended-upon folder", got)
	}
	if len(uselessness) != 0 {
		t.Errorf("`should not be in zone of uselessness` reported %v, want nothing",
			zoneOffendersOf(uselessness))
	}
}

func TestAZoneCheckIsJudgedOverThePackagesItsScopeSelected(t *testing.T) {
	// The consequence worth knowing: a package's coupling is its coupling to the rest of the *selection*, so a
	// scope narrow enough to hide it changes where the package sits. `internal/port` alone depends on nothing
	// selected, which puts its instability at 0 and takes it out of the corner it is really in — so a rule
	// about the corners is usually written over the whole project.
	whole := fluentapi.Metrics(zonedProject(t)).Distance().ShouldNotBeInZoneOfUselessness()
	narrowed := fluentapi.Metrics(zonedProject(t)).InFolder("internal/port").Distance().ShouldNotBeInZoneOfUselessness()

	if got := zoneOffendersOf(check(t, whole, nil)); !slices.Equal(got, []string{"internal/port"}) {
		t.Errorf("%s reported %v, want [internal/port]", whole, got)
	}
	if violations := check(t, narrowed, nil); len(violations) != 0 {
		t.Errorf("%s reported %v, want nothing: it depends on nothing the rule selected",
			narrowed, zoneOffendersOf(violations))
	}
}

func TestAZoneCheckThatSelectedNoPackageIsAnEmptyTestViolation(t *testing.T) {
	// The highest-value defensive decision in the library, wired in here too: a stale glob is reported rather
	// than passing forever, because no package selected means no package in a zone.
	rule := fluentapi.Metrics(zonedProject(t)).InFolder("nowhere/**").Distance().ShouldNotBeInZoneOfPain()

	violations := check(t, rule, nil)

	if len(violations) != 1 {
		t.Fatalf("%s reported %v, want the one empty-test violation", rule, violations)
	}
	empty, ok := violations[0].(assertion.EmptyTestViolation)
	if !ok {
		t.Fatalf("reported a %T, want an EmptyTestViolation", violations[0])
	}
	if empty.Subject != "components" {
		t.Errorf("the empty selection was reported about %q, want the vocabulary the rule judged in", empty.Subject)
	}
	if len(empty.Selectors) != 1 {
		t.Errorf("the empty selection reported %v, want the scope verb that selected nothing", empty.Selectors)
	}
}

func TestAZoneCheckOfAnEmptySelectionCanBeAllowed(t *testing.T) {
	// The opt-out is the same one every other rule takes, from the same options bag.
	rule := fluentapi.Metrics(zonedProject(t)).InFolder("nowhere/**").Distance().ShouldNotBeInZoneOfPain()

	violations := check(t, rule, &kernel.CheckOptions{AllowEmptyTests: true})

	if len(violations) != 0 {
		t.Errorf("%s reported %v with AllowEmptyTests, want nothing", rule, violations)
	}
}

func TestAZoneCheckReportsAnEmptySelectionInsteadOfJudgingAnything(t *testing.T) {
	// The short-circuit: a report must never say that a rule both did and did not have a subject.
	rule := fluentapi.Metrics(zonedProject(t)).InFolder("nowhere/**").Distance().ShouldNotBeInZoneOfUselessness()

	for _, violation := range check(t, rule, nil) {
		if violation.Kind() == metricsassertion.KindMetricsZone {
			t.Errorf("%s reported %s beside the empty selection, want the empty selection alone", rule, violation)
		}
	}
}

func TestTheCheckOptionsReachAZoneCheck(t *testing.T) {
	// The same options every other rule takes, and they change the answer here: `internal/db` depends on
	// nothing at all in the code the project ships, which is what puts it in the corner, but its test file
	// imports `internal/testutil` — so counting the test files gives it an outgoing coupling of its own and
	// takes it out.
	rule := fluentapi.Metrics(zonedProject(t)).Distance().ShouldNotBeInZoneOfPain()

	byDefault := check(t, rule, nil)
	withTests := check(t, rule, &kernel.CheckOptions{IncludeTestFiles: true})

	if got := zoneOffendersOf(byDefault); !slices.Equal(got, []string{"internal/db"}) {
		t.Errorf("%s reported %v by default, want [internal/db]", rule, got)
	}
	if len(withTests) != 0 {
		t.Errorf("%s reported %v with IncludeTestFiles, want nothing", rule, zoneOffendersOf(withTests))
	}
}

func TestAZoneCheckRejectsAPatternItsScopeCouldNotCompile(t *testing.T) {
	rule := fluentapi.Metrics(nil).InFolder("[unclosed").Distance().ShouldNotBeInZoneOfPain()

	violations, err := rule.Check(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Check error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("UserError.Operation = %q, want the scope verb `in folder`", user.Operation)
	}
	if len(violations) != 0 {
		t.Errorf("Check returned %v beside the error, want nothing", violations)
	}
	if !strings.Contains(rule.String(), "rejected") {
		t.Errorf("%s renders without the rejection, want it visible in a test failure", rule)
	}
}

func TestAZoneCheckRejectsALocatorThatIsNotAProject(t *testing.T) {
	rule := fluentapi.Metrics(&extraction.ProjectLocator{Directory: t.TempDir()}).
		Distance().
		ShouldNotBeInZoneOfPain()

	_, err := rule.Check(nil)

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("Check error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
}

func TestAZoneCheckRendersTheSentenceTheUserTyped(t *testing.T) {
	tests := []struct {
		rule fluentapi.MetricsZoneCondition
		want string
	}{
		{
			rule: fluentapi.Metrics(nil).InFolder("internal/**").Distance().ShouldNotBeInZoneOfPain(),
			want: `metrics, path without filename matches "internal/**", distance, should not be in zone of pain`,
		},
		{
			rule: fluentapi.Metrics(nil).Distance().ShouldNotBeInZoneOfUselessness(),
			want: "metrics, distance, should not be in zone of uselessness",
		},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if rendered := test.rule.String(); rendered != test.want {
				t.Errorf("String() = %q, want %q", rendered, test.want)
			}
		})
	}
}

func TestAZoneCheckCanBeStoredAndCheckedTwice(t *testing.T) {
	// A rule is a value: nothing is read when it is built, and checking it does not change it.
	rule := fluentapi.Metrics(zonedProject(t)).Distance().ShouldNotBeInZoneOfPain()

	first := zoneOffendersOf(check(t, rule, nil))
	second := zoneOffendersOf(check(t, rule, nil))

	if !slices.Equal(first, second) {
		t.Errorf("the rule reported %v and then %v, want the same answer twice", first, second)
	}
}

// check runs a rule and fails the test if it could not be run at all, which is what separates a technical
// error from a rule the project breaks.
func check(t *testing.T, rule kernel.Checkable, options *kernel.CheckOptions) []assertion.Violation {
	t.Helper()

	violations, err := rule.Check(options)
	if err != nil {
		t.Fatalf("%s failed to check: %v", rule, err)
	}
	return violations
}

// zoneOffendersOf names the components a zone check reported, in order, for a failure message.
func zoneOffendersOf(violations []assertion.Violation) []string {
	reported := make([]string, 0, len(violations))
	for _, violation := range violations {
		if zone, ok := violation.(metricsassertion.ZoneViolation); ok {
			reported = append(reported, zone.Component)
		}
	}
	return reported
}

// zonedProject writes a project with one package in each zone and hands back a locator naming it. The graph
// cache is keyed by project, and cleared so that a rule resolved here never reads a graph another test
// extracted.
func zonedProject(t *testing.T) *extraction.ProjectLocator {
	t.Helper()

	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	return &extraction.ProjectLocator{Directory: writeZonedProject(t)}
}

// writeZonedProject writes a project whose four packages sit in four different places of the
// abstractness/instability plane, so that each zone check has one package to find and three to leave alone:
//
//	internal/db        one struct, depended on by two others     A 0, I 0  the zone of pain
//	internal/port      one interface, depended on by nothing     A 1, I 1  the zone of uselessness
//	internal/api       one struct, depended on by nothing        A 0, I 1  the good concrete corner
//	internal/testutil  one interface, coupled to nothing         A 1, I 0  the good abstract corner
//
// The one test file in it is what makes the check options observable: it is `internal/db`'s, it imports
// `internal/testutil`, and reading it is the difference between `internal/db` depending on nothing and
// depending on one of the other three.
func writeZonedProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/zoned\n\ngo 1.26\n",
		// Concrete and depended upon: nothing about it can change without changing what depends on it, and
		// nothing about it is a promise a substitute could keep.
		"internal/db/conn.go": "package db\n\ntype Connection struct{}\n\nfunc Connect() *Connection { return nil }\n",
		"internal/db/conn_test.go": "package db\n\nimport (\n\t\"testing\"\n\n" +
			"\t\"example.com/zoned/internal/testutil\"\n)\n\nvar _ testutil.Helper\n\n" +
			"func TestConnect(t *testing.T) {\n\tif Connect() != nil {\n\t\tt.Fatal(\"want nil\")\n\t}\n}\n",
		// Abstract and depended on by nothing: a promise nobody asked for.
		"internal/port/port.go": "package port\n\nimport \"example.com/zoned/internal/db\"\n\n" +
			"type Store interface {\n\tOpen() *db.Connection\n}\n",
		// Concrete and depended on by nothing, which is the end of the main sequence rather than a corner off
		// it — so a check reporting every package it read would fail here.
		"internal/api/handler.go": "package api\n\nimport \"example.com/zoned/internal/db\"\n\n" +
			"type Handler struct {\n\tconn *db.Connection\n}\n",
		// Abstract and coupled to nothing, which is the other end of the line: a check that mistook either
		// axis for the other would report it.
		"internal/testutil/util.go": "package testutil\n\ntype Helper interface {\n\tHelp()\n}\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating the folder of %q failed: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %q failed: %v", name, err)
		}
	}
	return root
}
