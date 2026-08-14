package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// FilesNamingCondition is the terminal of the three self-contained rules about how the files a scope
// selected are named and where they live — `project files, in folder "internal/**", should, have name
// "*.go"` — and it is a fluentapi.Checkable, which is the one thing every consumer of a rule programs
// against:
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/**").Should().HaveName("*.go")
//	violations, err := rule.Check(nil)
//
// It is what HaveName, BeInFolder and BeInPath return, on either mood, and the three are one terminal
// because they are one rule with one requirement: a pattern, and the part of a file's identifier it is
// matched against. That part travels on the compiled matching.Filter, so the predicate is chosen once,
// where the user typed it, and nothing here branches on which of the three it was.
//
// It carries the scope and the mood it was asked of unchanged, and it is immutable like every stage before
// it — so a rule can be stored, passed to a helper and checked as often as it is useful. Nothing has been
// read when it is built: the project is located, extracted, selected and judged by Check, and by nothing
// else.
//
// None of the three predicates has an object stage. Each is a sentence on its own — the files it is about
// are the ones the scope named, and the pattern is the predicate's own argument — so this terminal is the
// end of the chain.
type FilesNamingCondition struct {
	// rule is the scope and the mood the predicate was asked of.
	rule filesRule
	// verb is the predicate as the user typed it — `have name`, `be in folder`, `be in path` — for the
	// sentence String renders and for the UserError a rejected pattern is reported as.
	verb string
	// pattern is the pattern as the user wrote it. It is kept beside the compiled filter because a
	// pattern this library could not compile has no filter to be read back from, and a rule must still
	// render as the sentence the user typed.
	pattern string
	// required is what the rule asks of each selected file: the compiled pattern together with the part
	// of an identifier the predicate looks at. It is the zero Filter when the pattern was rejected, which
	// no check reaches — the rejection is returned as an error first.
	required matching.Filter
}

// Check runs the rule: one violation per selected file that is not named, or not placed, the way the
// predicate requires, and an empty result when every file satisfies it, which is the pass. A nil
// *CheckOptions means the defaults.
//
// It is the whole pipeline in three steps — locate and extract the project, select the scope's files, judge
// each of them against the requirement — and the only stage of the chain that reads anything. There is no
// projection of dependencies in it, because these three predicates are about the files themselves; the
// violations are the files module's own assertion.NamingViolation values, each carrying the file, the
// requirement and the mood, or the one EmptyTestViolation of a scope that selected no file at all.
//
// The error is technical or the user's — a pattern the scope or the predicate could not compile, a locator
// naming no Go project, a project that will not load — and never a failing rule. When it is non-nil the
// violations say nothing.
func (c FilesNamingCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	// The graph is deliberately dropped: what a file is called and where it lives is on the file, so this
	// rule is about the selection and not about the dependencies between the files in it.
	_, selected, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}

	if empty := options.GatherEmptyTestViolations(c.rule.selection(len(selected))); len(empty) > 0 {
		// A rule with no subject is reported instead of being judged: every file of an empty selection
		// satisfies every requirement, in either mood, so such a rule would otherwise pass forever.
		return empty, nil
	}

	return filesassertion.GatherNamingViolations(selected, c.required, c.rule.mood), nil
}

// String renders the whole rule as the sentence the user typed, as `project files, path without filename
// matches "internal/**", should, have name "*.go"`.
//
// The predicate renders as the words the user wrote rather than as its filter's description, which is what
// the scope verbs before it do: after the mood the sentence has to read as English, and `should, filename
// matches "*.go"` is not the predicate anybody typed.
func (c FilesNamingCondition) String() string {
	return c.rule.render(c.verb + ` "` + c.pattern + `"`)
}

// requiring is every one of the three naming predicates: compile the pattern the user typed with the
// scope's own factory, and hand back the terminal that judges the selected files against it. Which part of
// an identifier a predicate looks at is the compile function it passes in, so that pairing is stated once
// per predicate and nowhere else — the same shape FilesBuilder.selecting gives the scope verbs.
//
// A pattern this library cannot understand is deferred to the terminal exactly as a scope verb's is: the
// rejection joins the scope, so the first pattern the user has to fix is the one reported, the rule renders
// with the rejection visible, and Check returns it as a UserError naming the predicate before the project is
// read. The requirement is left as the zero Filter, which no check reaches.
func (r filesRule) requiring(verb, pattern string, compile func(string) (matching.Filter, error)) FilesNamingCondition {
	required, err := compile(pattern)
	if err != nil {
		rejected := r
		rejected.scope = r.scope.rejecting(verb, pattern, err)
		return FilesNamingCondition{rule: rejected, verb: verb, pattern: pattern}
	}
	return FilesNamingCondition{rule: r, verb: verb, pattern: pattern, required: required}
}
