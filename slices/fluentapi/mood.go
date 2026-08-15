package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
)

// SlicesShouldBuilder is the positive mood of a rule about slices — `project slices, defined by
// "internal/(**)/**", should` — and the stage a predicate is asked of:
//
//	rule := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").Should()
//
// It is what SlicesBuilder.Should returns, it carries the slicing it was asked of unchanged, and it is
// immutable like every other stage. There is no mood verb on it: the grammar allows exactly one mood per
// rule, and a mood stage that had a Should of its own would let a chain say the word twice.
//
// Together with SlicesShouldNotBuilder it is one of the two thin types over one shared rule — see slicesRule
// — so a predicate with a meaningful negation is implemented once, for both moods, and the mood reaches the
// assertion as assertion.Mood rather than as a second code path.
type SlicesShouldBuilder struct {
	rule slicesRule
}

// SlicesShouldNotBuilder is the negated mood of a rule about slices — `project slices, defined by
// "internal/(**)/**", should not` — and is SlicesShouldBuilder's twin: the same slicing, the same terminals,
// the same predicate, one flag apart.
//
//	rule := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").ShouldNot()
//
// It is what SlicesBuilder.ShouldNot returns, and it is the mood a rule about slices is nearly always written
// in: a slicing exists to say which parts of a project may not reach which others. The negation is not a
// second set of rules — it is assertion.Mood threaded into the same assertion, which is what keeps the
// negative half of the API free of logic of its own.
type SlicesShouldNotBuilder struct {
	rule slicesRule
}

// Should is the positive mood of a rule about slices: `should`.
//
// It closes the slicing and opens the predicate — `project slices, defined by "internal/(**)/**", should,
// contain dependency "api" -> "domain"` — and it does no work, like every stage before it.
//
// A chain that reaches a mood without a slicing is rejected here and reported by the terminal as a UserError
// wrapping ErrNoSlicing: the mood is asked of the entry point itself, so this is the first stage at which the
// missing `defined by` can be seen at all.
//
// There is no synonym for it. `must`, `always` and `has to` are not part of this library's grammar in any
// language it is ported to; the negative is ShouldNot and there are no other moods.
func (b SlicesBuilder) Should() SlicesShouldBuilder {
	return SlicesShouldBuilder{rule: b.ruleIn(assertion.Should)}
}

// ShouldNot is the negated mood of a rule about slices: `should not`.
//
// It is the mood nearly every rule about slices is written in — a slicing exists in order to forbid the
// dependencies that cut across it — and it is the same rule as Should with one flag flipped:
//
//	slicing := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**")
//	forbidden := slicing.ShouldNot().ContainDependency("api", "db")
//	required := slicing.Should().ContainDependency("api", "domain")
//
// A chain that reaches it without a slicing is rejected exactly as Should rejects one. There is no synonym
// for it either: no `must not`, no `may not`, no `never`, no `cannot`.
func (b SlicesBuilder) ShouldNot() SlicesShouldNotBuilder {
	return SlicesShouldNotBuilder{rule: b.ruleIn(assertion.ShouldNot)}
}

// Mood is assertion.Should, the mood this builder is the positive half of. It is the flag the predicate
// passes to assertion.GatherDependencyViolations, and the one thing that distinguishes this builder from
// SlicesShouldNotBuilder.
func (b SlicesShouldBuilder) Mood() assertion.Mood {
	return b.rule.mood
}

// SelectSliceFiles resolves the rule's slicing against the project, as SlicesBuilder.SelectSliceFiles does:
// the files of each slice, sorted, keyed by the slice's name, and a nil *CheckOptions means the defaults.
//
// The mood says nothing about what the slices of a project are, so this is the slicing's answer unchanged. It
// is the half of a check that runs before anything is judged.
func (b SlicesShouldBuilder) SelectSliceFiles(options *kernel.CheckOptions) (map[string][]string, error) {
	return b.rule.scope.SelectSliceFiles(options)
}

// String renders the rule as far as it has been built, as `project slices, path matches
// "internal/(**)/**", should`.
func (b SlicesShouldBuilder) String() string {
	return b.rule.String()
}

// Mood is assertion.ShouldNot, the mood this builder is the negated half of. It is the flag the predicate
// passes to assertion.GatherDependencyViolations, and the one thing that distinguishes this builder from
// SlicesShouldBuilder.
func (b SlicesShouldNotBuilder) Mood() assertion.Mood {
	return b.rule.mood
}

// SelectSliceFiles resolves the rule's slicing against the project, as SlicesBuilder.SelectSliceFiles does:
// the files of each slice, sorted, keyed by the slice's name, and a nil *CheckOptions means the defaults.
//
// The mood says nothing about what the slices of a project are, so this is the slicing's answer unchanged. It
// is the half of a check that runs before anything is judged.
func (b SlicesShouldNotBuilder) SelectSliceFiles(options *kernel.CheckOptions) (map[string][]string, error) {
	return b.rule.scope.SelectSliceFiles(options)
}

// String renders the rule as far as it has been built, as `project slices, path matches
// "internal/(**)/**", should not`.
func (b SlicesShouldNotBuilder) String() string {
	return b.rule.String()
}

// ruleIn is the rule both moods are built from: this slicing, in that mood, with a chain that has no slicing
// at all rejected on the way.
//
// The rejection lives here because this is where it becomes visible: the entry point cannot know that no
// `defined by` is coming, and by the mood it is too late for one. It is written once for both moods, and it
// is a rejection rather than a compile error because the mood is a method on the entry point — the type
// system cannot ask for a slicing the way LayerBuilder asks for a layer's pattern.
//
// SlicesBuilder.resolve rejects the same chain a second time, for the builder nobody asked a mood of. Doing it
// here as well is what makes the missing slicing the *first* rejection of the chain, so a rule that also names
// its slices wrongly reports the mistake a reader has to fix first, and what the rule renders as says so too.
func (b SlicesBuilder) ruleIn(mood assertion.Mood) slicesRule {
	scope := b
	if !b.sliced {
		scope = b.rejecting("project slices", "", ErrNoSlicing)
	}
	return slicesRule{scope: scope, mood: mood}
}

// slicesRule is the half of a rule about slices that both moods carry: the slicing the mood was asked of, and
// the mood itself. It is the one shared value the two builders above are thin types over.
//
// The predicate takes one of these and hands the mood on to assertion.GatherDependencyViolations, where
// assertion.Mood.Holds is the single comparison the two moods differ by. Nothing in the module reads the flag
// in order to choose between two implementations — that is the duplication the mood exists to prevent.
type slicesRule struct {
	// scope is the stage the mood was asked of, kept whole rather than unpacked: it already carries the
	// locator, the two pattern factories, the slicing and anything this library rejected, and a terminal
	// needs all of them.
	scope SlicesBuilder
	// mood is the flag, and the only difference between the two builders above.
	mood assertion.Mood
}

// String renders the slicing and then the mood, joined as the sentence the user typed. It is the two moods'
// one rendering, so neither builder can phrase itself differently from the other, and a pattern this library
// rejected is still visible — after the mood, because the rejection ends the sentence rather than sitting
// inside it.
func (r slicesRule) String() string {
	return r.render()
}

// render is String with the stages the predicate adds: `project slices, path matches "internal/(**)/**",
// should not, contain dependency "api" -> "db"`.
//
// Every stage after the mood renders through it, so the sentence is joined in one place and a rejected
// pattern stays at the end of it however many stages were chained on.
func (r slicesRule) render(stages ...string) string {
	sentence := append(r.scope.stages(), r.mood.String())
	sentence = append(sentence, stages...)
	return strings.Join(sentence, ", ") + r.scope.rejected()
}

// selection is the population this rule's slicing found, as the empty-test guard is asked about it: what the
// rule was about, how many slices the slicing came to, and the pattern that described them.
//
// It is what makes a renamed folder a failure rather than a green rule: a slicing that finds no slice at all
// projects no dependencies, so `should not contain dependency` would hold forever. The word is `slices` —
// the vocabulary the entry point names — and it is spelled here so that every terminal this module gains
// reports it identically.
func (r slicesRule) selection(matched int) kernel.EmptyTestPopulation {
	return kernel.EmptyTestPopulation{Subject: "slices", Matched: matched, Selectors: r.scope.selectors()}
}
