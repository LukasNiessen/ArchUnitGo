package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/layers/assertion"
)

func TestAnAllowlistClausePermitsTheLayersItNamedAndNoOther(t *testing.T) {
	// `where layer "api", may only depend on layers "domain", "db"`. An allowlist is the clause a layered
	// architecture is written as, and a layer added to the policy later is forbidden until it is named here.
	clause := assertion.NewClause("api", []string{"domain", "db"}, kernel.Should)

	for _, allowed := range []string{"domain", "db"} {
		if !clause.Allows(allowed) {
			t.Errorf("%s forbids a dependency on %q, want it allowed: the clause names it", clause, allowed)
		}
	}
	if clause.Allows("transport") {
		t.Errorf(`%s allows a dependency on "transport", want it forbidden: the clause does not name it`, clause)
	}
}

func TestABlocklistClauseForbidsTheLayersItNamedAndNoOther(t *testing.T) {
	// `where layer "db", may not depend on layers "api"`. The clause for the one edge a team cares about,
	// leaving the rest of the policy's layers alone — which is the whole difference from the allowlist, and it
	// is one membership test through the mood rather than a second code path.
	clause := assertion.NewClause("db", []string{"api"}, kernel.ShouldNot)

	if clause.Allows("api") {
		t.Errorf(`%s allows a dependency on "api", want it forbidden: the clause names it`, clause)
	}
	if !clause.Allows("domain") {
		t.Errorf(`%s forbids a dependency on "domain", want it allowed: a blocklist leaves the rest alone`, clause)
	}
}

func TestASealedLayerAllowsNothing(t *testing.T) {
	// `may only depend on layers` with nothing named: the allowlist that named no layer forbids every
	// dependency on another declared layer, which is the domain that may not reach the code around it. It
	// falls out of the same membership test rather than being a case of its own.
	sealed := assertion.NewClause("domain", nil, kernel.Should)

	for _, target := range []string{"api", "db", ""} {
		if sealed.Allows(target) {
			t.Errorf("%s allows a dependency on %q, want a sealed layer to allow nothing", sealed, target)
		}
	}
}

func TestTheZeroClauseIsASealedNamelessLayer(t *testing.T) {
	// A value with a meaning rather than a mistake: the zero Clause is an allowlist that named nothing, which
	// is what a sealed layer is.
	var zero assertion.Clause

	if zero.Allows("api") {
		t.Errorf("the zero clause allows a dependency on %q, want it to allow nothing", "api")
	}
	if zero.Mood() != kernel.Should {
		t.Errorf("the zero clause is in mood %s, want the allowlist", zero.Mood())
	}
}

func TestAClauseRendersAsTheSentenceItWasWrittenAs(t *testing.T) {
	// The mood is the clause's own verb here, not `should`/`should not`: a layer policy is the one family
	// whose user spells the mood as part of the predicate. A sealed layer reads `no layers`, which is the one
	// reading of an empty list that is still English.
	sentences := map[string]assertion.Clause{
		`where layer "api", may only depend on layers "domain", "db"`: assertion.NewClause(
			"api", []string{"domain", "db"}, kernel.Should),
		`where layer "db", may not depend on layers "api"`: assertion.NewClause(
			"db", []string{"api"}, kernel.ShouldNot),
		`where layer "domain", may only depend on no layers`: assertion.NewClause("domain", nil, kernel.Should),
	}

	for want, clause := range sentences {
		if rendered := clause.String(); rendered != want {
			t.Errorf("the clause reads %q, want %q", rendered, want)
		}
	}
}

func TestAClausesPredicateIsTheStepOfTheChainItWasWrittenAt(t *testing.T) {
	// What a UserError names when a clause is wrong: the verb the user typed, and not the object it was given.
	predicates := map[kernel.Mood]string{
		kernel.Should:    "may only depend on layers",
		kernel.ShouldNot: "may not depend on layers",
	}

	for mood, want := range predicates {
		if predicate := assertion.NewClause("api", nil, mood).Predicate(); predicate != want {
			t.Errorf("the predicate of a %s clause is %q, want %q", mood, predicate, want)
		}
	}
}

func TestAClauseKeepsTheLayersItNamedInTheOrderTheyWereTypedAndDoesNotShareThem(t *testing.T) {
	// A clause renders as the sentence that was written, so the order is the user's rather than sorted — and
	// it is a copy, because a policy is a value a user may have stored and spreading a slice into a variadic
	// parameter shares its backing array.
	named := []string{"domain", "db"}
	clause := assertion.NewClause("api", named, kernel.Should)

	named[0] = "transport"
	clause.Named()[1] = "transport"

	if want := []string{"domain", "db"}; !slices.Equal(clause.Named(), want) {
		t.Errorf("the clause names %v, want %v in the order they were typed", clause.Named(), want)
	}
	if clause.Allows("transport") {
		t.Errorf(`%s allows "transport", want the layers it was given to have been copied`, clause)
	}
}
