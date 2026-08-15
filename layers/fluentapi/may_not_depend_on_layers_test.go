package fluentapi_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
)

func TestABlocklistReportsTheLayerItForbade(t *testing.T) {
	// The clause for the one edge a team cares about: the domain must not reach the database, and in the
	// fixture it does.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("domain").MayNotDependOnLayers("db")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"domain -> db"}) {
		t.Fatalf("the policy reported %v, want the one dependency it forbids", pairs)
	}
	if mood := layerViolation(t, violations[0]).Mood; mood != kernel.ShouldNot {
		t.Errorf("the violation was judged in mood %s, want the blocklist's %s", mood, kernel.ShouldNot)
	}
}

func TestABlocklistLeavesTheRestOfThePolicysLayersAlone(t *testing.T) {
	// The whole difference from the allowlist: forbidding one direction says nothing about the others, so a
	// policy can be tightened one edge at a time without enumerating everything a layer legitimately uses.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("api").MayNotDependOnLayers("db")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"api -> db"}) {
		t.Errorf("the policy reported %v, want the forbidden dependency alone: the api may still reach the domain", pairs)
	}
}

func TestABlocklistThatForbidsNothingIsRejected(t *testing.T) {
	// A blocklist naming no layer holds forever, which is the failure this library refuses to pass silently
	// anywhere. The sealed layer is the *other* clause with no argument, and it forbids everything instead.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).WhereLayer("domain").MayNotDependOnLayers()

	violations, err := policy.Check(nil)

	if !errors.Is(err, fluentapi.ErrNoLayersNamed) {
		t.Fatalf("the empty blocklist failed with %v, want ErrNoLayersNamed", err)
	}
	if len(violations) != 0 {
		t.Errorf("the rejected policy reported %v, want nothing: it was never a runnable rule", violations)
	}
	var userError *archerror.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("the error is a %T, want a UserError naming the step at fault", err)
	}
	if userError.Operation != "may not depend on layers" || userError.Subject != "domain" {
		t.Errorf("the error blames `%s %q`, want the clause the user left empty", userError.Operation, userError.Subject)
	}
}

func TestABlockedDependencyIsBlamedOnTheBlocklistRatherThanOnAnAllowlist(t *testing.T) {
	// The policy's evaluation order, and it is about the report rather than the pass: the reader wrote a
	// sentence about this very pair of layers, so that is the line the violation sends them to.
	root := writeLayeredFixtureProject(t)
	policy := fixturePolicy(t, root).
		WhereLayer("domain").MayOnlyDependOnLayers("api").
		WhereLayer("domain").MayNotDependOnLayers("db")

	violations, err := policy.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", policy, err)
	}
	if pairs := offendingPairs(t, violations); !slices.Equal(pairs, []string{"domain -> db"}) {
		t.Fatalf("the policy reported %v, want one violation for the one offending dependency", pairs)
	}
	violation := layerViolation(t, violations[0])
	if violation.Mood != kernel.ShouldNot || !slices.Equal(violation.Named, []string{"db"}) {
		t.Errorf("the violation blames `%s %v`, want the blocklist that forbade the pair", violation.Mood, violation.Named)
	}
}
