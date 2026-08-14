package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// FilesShouldBuilder is the positive mood of a rule about files — `project files, in folder
// "internal/api/**", should` — and the stage a predicate is asked of:
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/api/**").Should()
//
// It is what FilesBuilder.Should returns, it carries the scope it was asked of unchanged, and it is
// immutable like every other stage. There is no mood verb on it: the grammar allows exactly one mood
// per rule, and a mood stage that had a Should of its own would let a chain say the word twice.
//
// Together with FilesShouldNotBuilder it is one of the two thin types over one shared rule — see
// filesRule — so a predicate with a meaningful negation is implemented once, for both moods, and the
// mood reaches it as assertion.Mood rather than as a second code path. The one predicate that is not on
// both is HaveNoCycles, whose negation would have nothing to report; it is offered here, on the positive
// mood alone, for the reason assertion.GatherCycleViolations gives.
type FilesShouldBuilder struct {
	rule filesRule
}

// FilesShouldNotBuilder is the negated mood of a rule about files — `project files, in folder
// "internal/api/**", should not` — and is FilesShouldBuilder's twin: the same scope, the same
// terminals, and every predicate that has a meaningful negation, one flag apart.
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/api/**").ShouldNot()
//
// It is what FilesBuilder.ShouldNot returns. The negation is not a second set of rules: it is
// assertion.Mood threaded into the same assertion, which is what keeps the negative half of the API
// free of logic of its own.
//
// A predicate whose negation would have nothing to report is the one thing the twins do not share:
// HaveNoCycles is on FilesShouldBuilder alone and deliberately absent here, so that `should not have no
// cycles` — a rule that fails on the absence of a cycle, with no data to name — cannot be typed. The
// reason is spelled out in assertion.GatherCycleViolations.
type FilesShouldNotBuilder struct {
	rule filesRule
}

// Should is the positive mood of a rule about files: `should`.
//
// It closes the scope and opens the predicate — `project files, in folder "internal/api/**", should,
// depend on files, in folder "internal/db/**"` — and it does no work, like every stage before it.
//
// There is no synonym for it. `must`, `always` and `has to` are not part of this library's grammar in
// any language it is ported to; the negative is ShouldNot and there are no other moods.
func (b FilesBuilder) Should() FilesShouldBuilder {
	return FilesShouldBuilder{rule: filesRule{scope: b, mood: assertion.Should}}
}

// ShouldNot is the negated mood of a rule about files: `should not`.
//
// It is the mood most architecture rules are written in — a boundary is a sentence about what may not
// depend on what — and it is the same rule as Should with one flag flipped:
//
//	base := archunit.ProjectFiles(nil).InFolder("internal/api/**")
//	allowed := base.Should()
//	forbidden := base.ShouldNot()
//
// There is no synonym for it either: no `must not`, no `may not`, no `never`, no `cannot`.
func (b FilesBuilder) ShouldNot() FilesShouldNotBuilder {
	return FilesShouldNotBuilder{rule: filesRule{scope: b, mood: assertion.ShouldNot}}
}

// Mood is assertion.Should, the mood this builder is the positive half of. It is the flag a predicate
// passes to its `gather <thing> violations` function, and the one thing that distinguishes this
// builder from FilesShouldNotBuilder.
func (b FilesShouldBuilder) Mood() assertion.Mood {
	return b.rule.mood
}

// Selectors are the compiled scope verbs the rule was built from, as FilesBuilder.Selectors returns
// them: the caller's own copy, and the data a report needs in order to say which pattern selected
// nothing.
func (b FilesShouldBuilder) Selectors() []matching.Filter {
	return b.rule.scope.Selectors()
}

// SelectFiles resolves the rule's scope against the project, as FilesBuilder.SelectFiles does: the
// identifiers of the files the rule is about, sorted, and a nil *CheckOptions means the defaults.
//
// The mood says nothing about which files a rule is about, so this is the scope's answer unchanged. It
// is the half of a check that runs before anything is judged.
func (b FilesShouldBuilder) SelectFiles(options *kernel.CheckOptions) ([]string, error) {
	return b.rule.scope.SelectFiles(options)
}

// String renders the rule as far as it has been built, as `project files, path without filename
// matches "internal/api/**", should`.
func (b FilesShouldBuilder) String() string {
	return b.rule.String()
}

// Mood is assertion.ShouldNot, the mood this builder is the negated half of. It is the flag a
// predicate passes to its `gather <thing> violations` function, and the one thing that distinguishes
// this builder from FilesShouldBuilder.
func (b FilesShouldNotBuilder) Mood() assertion.Mood {
	return b.rule.mood
}

// Selectors are the compiled scope verbs the rule was built from, as FilesBuilder.Selectors returns
// them: the caller's own copy, and the data a report needs in order to say which pattern selected
// nothing.
func (b FilesShouldNotBuilder) Selectors() []matching.Filter {
	return b.rule.scope.Selectors()
}

// SelectFiles resolves the rule's scope against the project, as FilesBuilder.SelectFiles does: the
// identifiers of the files the rule is about, sorted, and a nil *CheckOptions means the defaults.
//
// The mood says nothing about which files a rule is about, so this is the scope's answer unchanged. It
// is the half of a check that runs before anything is judged.
func (b FilesShouldNotBuilder) SelectFiles(options *kernel.CheckOptions) ([]string, error) {
	return b.rule.scope.SelectFiles(options)
}

// String renders the rule as far as it has been built, as `project files, path without filename
// matches "internal/api/**", should not`.
func (b FilesShouldNotBuilder) String() string {
	return b.rule.String()
}

// filesRule is the half of a rule about files that both moods carry: the scope the mood was asked of,
// and the mood itself. It is the one shared value the two builders above are thin types over.
//
// Every predicate this module gains takes one of these and hands the mood on to a
// `gather <thing> violations` function in files/assertion, where assertion.Mood.Holds is the single
// comparison the two moods differ by. Nothing in the module reads the flag in order to choose between
// two implementations — that is the duplication the mood exists to prevent.
type filesRule struct {
	// scope is the stage the mood was asked of, kept whole rather than unpacked: it already carries
	// the locator, the pattern factory, the selectors and a pattern a scope verb rejected, and a
	// terminal needs all four.
	scope FilesBuilder
	// mood is the flag, and the only difference between the two builders above.
	mood assertion.Mood
}

// String renders the scope and then the mood, joined as the sentence the user typed. It is the two
// moods' one rendering, so neither builder can phrase itself differently from the other, and a
// pattern a scope verb rejected is still visible — after the mood, because the rejection ends the
// sentence rather than sitting inside it.
func (r filesRule) String() string {
	return r.render()
}

// render is String with the stages a predicate and its object add: `project files, path without
// filename matches "internal/**", should, have no cycles`.
//
// Every stage after the mood renders through it, so the sentence is joined in one place and a pattern a
// scope verb rejected stays at the end of it however many stages were chained on.
func (r filesRule) render(stages ...string) string {
	sentence := append(r.scope.stages(), r.mood.String())
	sentence = append(sentence, stages...)
	return strings.Join(sentence, ", ") + r.scope.rejected()
}

// emptyTestOptions are the empty-test guard's options for this rule: the check options' own
// AllowEmptyTests, the scope verbs the rule was built from, and what it was selecting.
//
// Every terminal in this module wires the guard in through it, so `files` — the word the entry point
// names, and the vocabulary a report says the rule selected nothing of — is spelled once.
func (r filesRule) emptyTestOptions(options *kernel.CheckOptions) *assertion.EmptyTestOptions {
	return options.EmptyTestOptions("files", r.scope.selectors...)
}
