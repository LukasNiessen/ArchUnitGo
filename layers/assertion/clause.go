// Package assertion is the layers module's half of the ASSERT stage: it judges the projected
// dependencies between a project's layers and reports one violation per dependency the policy does not
// allow.
//
// A layer policy is a list of Clause values — `where layer "api", may not depend on layers "db"` — and
// GatherDependencyViolations is the one function that judges them, reporting DependencyViolation values.
// Both halves are data: a violation says which layer depended on which, in which clause's words, and
// carries the concrete file dependencies it was broken by, but not a word about them. Message
// construction belongs to the testing layer, where one place controls phrasing, numbering and color.
//
// The mood is what makes the two clauses one piece of logic. `may only depend on layers` is the positive
// mood — an allowlist, satisfied by a dependency the clause named — and `may not depend on layers` is the
// negated one, a blocklist, so Clause.Allows is assertion.Mood.Holds over one membership test and there
// is no second code path for the second clause.
//
// The package is pure, like every assertion package in the library: no filesystem, no clock, no globals,
// and nothing in it knows Go. It takes the projected structure common/projection produced and hands back
// a []assertion.Violation, so a policy's judgement can be tested against a hand-built projection before
// any project is extracted at all.
package assertion

import (
	"slices"
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// Clause is one clause of a layer policy: the layer it is about, the layers it names, and which of the
// two ways round it was written.
//
// It is what `where layer "api", may only depend on layers "domain", "db"` compiles to, and a policy is a
// list of them. Each clause is a rule of its own and all of them apply, so two clauses about one layer
// are read as both being in force; which of them a report blames for a given dependency is
// GatherDependencyViolations's to say.
//
// The mood is the whole difference between the two clauses. Should is `may only depend on layers`, the
// allowlist: the dependency has to be one of the layers named. ShouldNot is `may not depend on layers`,
// the blocklist: it has to be none of them. Both are Allows, which is one membership test handed to
// assertion.Mood.Holds.
//
// A Clause is immutable, and the zero Clause is a nameless allowlist that permits nothing — which is what
// a sealed layer is, so it is a value with a meaning rather than a mistake.
type Clause struct {
	layer string
	named []string
	mood  kernel.Mood
}

// NewClause records that this layer may only, or may not, depend on the layers named — mood being which
// of the two.
//
// The named layers are copied, because a Clause is immutable and is held by a builder a user may have
// stored: spreading a caller's slice into a variadic parameter shares its backing array. They are kept in
// the order the user typed them rather than sorted, because a clause renders as the sentence that was
// written.
//
// Naming no layer at all is meaningful in exactly one mood. Under Should it is the sealed layer —
// `may only depend on no layers` — which permits nothing outside itself, and it is why this constructor
// rejects nothing: whether an empty list is a rule anybody meant is the fluent API's question, asked
// where the user typed it, and by then this layer has only a policy to judge.
func NewClause(layer string, named []string, mood kernel.Mood) Clause {
	return Clause{layer: layer, named: slices.Clone(named), mood: mood}
}

// Layer is the layer this clause is about: the depending end of every dependency it judges.
func (c Clause) Layer() string {
	return c.layer
}

// Named are the layers this clause named, in the order the user typed them — the ones allowed under
// `may only depend on layers` and the ones forbidden under `may not depend on layers`.
//
// The result is the caller's own copy, because a Clause that has been stored must not change afterwards.
func (c Clause) Named() []string {
	return slices.Clone(c.named)
}

// Mood is which way round the clause was written: assertion.Should for `may only depend on layers` and
// assertion.ShouldNot for `may not depend on layers`.
func (c Clause) Mood() kernel.Mood {
	return c.mood
}

// Predicate is the clause's own words, `may only depend on layers` or `may not depend on layers`, as the
// user typed them — for a UserError naming the step at fault, and for the sentence String renders.
//
// It is the predicate that was written and not the phrasing of a report; that is the testing layer's, so
// that one place controls how a failure reads.
func (c Clause) Predicate() string {
	return c.verb() + " layers"
}

// Allows reports whether this clause permits the layer it is about to depend on target.
//
// It is the whole judgement of a clause, and it is one membership test: the allowlist permits what it
// named and the blocklist permits what it did not, which is assertion.Mood.Holds and the only place
// anything about a layer policy is inverted. A sealed layer — the allowlist that named nothing — allows
// no target at all, which falls out of the same test rather than being a case of its own.
//
// A dependency of a layer this clause is not about is not this clause's business, and answering that is
// GatherDependencyViolations's job rather than this one's: Allows is asked only about the dependencies of
// its own layer.
func (c Clause) Allows(target string) bool {
	return c.mood.Holds(slices.Contains(c.named, target))
}

// String renders the clause as the sentence the user typed, as `where layer "api", may only depend on
// layers "domain", "db"`, for a log line or a test failure.
//
// A sealed layer — `may only depend on layers` with nothing named — renders as `may only depend on no
// layers`, which is the one reading of an empty list that is still English. A blocklist naming nothing
// renders the same way round, `may not depend on no layers`, and is the clause the fluent API rejects
// rather than one a user gets to see here.
func (c Clause) String() string {
	return `where layer "` + c.layer + `", ` + c.verb() + " " + c.namedLayers()
}

// verb is the clause's mood in the words the user typed, `may only depend on` or `may not depend on`.
//
// This is the same thing assertion.Mood.String does for `should` and `should not`, and it is here rather
// than there because a layer policy is the one rule family in the library whose two moods the user spells
// as part of the predicate: `should, may not depend on layers` is not a sentence anybody would type.
func (c Clause) verb() string {
	if c.mood.Negated() {
		return "may not depend on"
	}
	return "may only depend on"
}

// namedLayers renders the object of the clause: the layers it named, quoted and comma-separated after the
// noun, or `no layers` when it named none. It is the one place the empty list is spelled, because both
// moods read it and a sealed layer is what the empty one means.
func (c Clause) namedLayers() string {
	if len(c.named) == 0 {
		return "no layers"
	}
	quoted := make([]string, 0, len(c.named))
	for _, name := range c.named {
		quoted = append(quoted, `"`+name+`"`)
	}
	return "layers " + strings.Join(quoted, ", ")
}
