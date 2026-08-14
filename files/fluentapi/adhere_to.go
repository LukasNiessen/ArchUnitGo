package fluentapi

import (
	"errors"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	filesextraction "github.com/LukasNiessen/ArchUnitGo/files/extraction"
)

// ErrNoPredicate is the reason `adhere to` was rejected: it is the one predicate whose rule *is* a Go
// function, so a chain that passes none has said nothing about the files it selected. The fix is to pass
// the function.
var ErrNoPredicate = errors.New("no predicate function given")

// ErrNoRequirement is the reason `adhere to` was rejected: the message beside the function is missing or
// blank. It is not decoration — a report cannot print a closure, so those words are the whole of what a
// failure would be able to say, and a rule nobody can read from its output is a rule nobody can fix.
var ErrNoRequirement = errors.New("no requirement given to describe the predicate")

// FilesAdherenceCondition is the terminal of the rule a user writes the predicate of themselves —
// `project files, in folder "internal/**", should, adhere to (a function), "be at most 400 lines long"` —
// and it is a fluentapi.Checkable, which is the one thing every consumer of a rule programs against:
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/**").
//		Should().
//		AdhereTo(func(file archunit.FileInfo) bool {
//			return file.NonBlankLineCount <= 400
//		}, "be at most 400 lines long")
//	violations, err := rule.Check(nil)
//
// It is what AdhereTo returns, on either mood, and it is the escape hatch of the files module: every other
// predicate here is about a file's name, its place or its dependencies, and this one is about whatever the
// user can compute from a file — its source text, its size, a convention no glob expresses.
//
// It carries the scope and the mood it was asked of unchanged, plus the function and the words describing
// it, and it is immutable like every stage before it — so a rule can be stored, passed to a helper and
// checked as often as it is useful. Nothing is read when it is built and the predicate is not called: the
// project is located, extracted, selected, read and judged by Check, and by nothing else.
//
// There is no object stage. `adhere to` is a sentence on its own — the files it is about are the ones the
// scope named, and what is asked of them is the user's own function — so this terminal ends the chain.
type FilesAdherenceCondition struct {
	// rule is the scope and the mood the predicate was asked of.
	rule filesRule
	// predicate is the user's own function, kept as it was given. It is nil only for a rule that was
	// rejected, which no check reaches: the rejection is returned as an error first.
	predicate filesassertion.FilePredicate
	// requirement is the message the user wrote beside the function, for the sentence String renders and for
	// the violations to carry. It is what stands in for a rule that cannot be printed.
	requirement string
}

// AdhereTo is the predicate that holds the files the scope selected to a rule the user writes themselves:
// `adhere to`.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/db/**").
//		Should().
//		AdhereTo(func(file archunit.FileInfo) bool {
//			return strings.Contains(file.Source, "context.Context")
//		}, "take a context.Context")
//
// The function is asked once about each selected file and is handed an archunit.FileInfo: the file's
// identifier, its name without the extension, that extension, its folder, its full source text and how many
// of its lines carry something. `should adhere to` requires it to answer yes about every selected file.
//
// The message is the second half of the predicate rather than a nicety. Everything else in this library
// renders itself — a pattern quotes the glob the user typed — but a Go function is not printable, so these
// words are what the rule reads as in a log line and what every violation carries. Phrase them as a bare
// infinitive, so that the mood reads onto them: `be at most 400 lines long`, not `file is too long`.
//
// This is the module's escape hatch, and it is deliberately not the first tool to reach for: a rule about
// where a file lives or what it depends on is expressible as `be in folder` or `depend on files`, and those
// report which pattern the file broke. Reach for this one for the conventions no glob can express — a
// forbidden call, a required build tag, a size limit.
func (b FilesShouldBuilder) AdhereTo(predicate filesassertion.FilePredicate, requirement string) FilesAdherenceCondition {
	return b.rule.adheringTo(predicate, requirement)
}

// AdhereTo is the negated mood of the same predicate: `should not adhere to`, which forbids what the
// function describes rather than requiring it.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/**").
//		ShouldNot().
//		AdhereTo(func(file archunit.FileInfo) bool {
//			return strings.Contains(file.Source, "database/sql")
//		}, "open a database connection of its own")
//
// It is the positive rule with assertion.Mood threaded into the same assertion — one violation per selected
// file the function *does* answer yes about — and not a second implementation. Everything AdhereTo says
// about the function, the FileInfo it is handed and how to phrase the message holds here unchanged.
func (b FilesShouldNotBuilder) AdhereTo(predicate filesassertion.FilePredicate, requirement string) FilesAdherenceCondition {
	return b.rule.adheringTo(predicate, requirement)
}

// Check runs the rule: one violation per selected file the user's predicate does not hold for, and an empty
// result when every file satisfies it, which is the pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, select the scope's files, read
// the source of each of them, ask the user's function about each — and the only stage of the chain that
// reads anything or calls the predicate. The graph is used for the selection alone: what is *in* a file is
// not in the dependency graph, which is why this is the one rule in the module with a gathering step of its
// own, files/extraction.ExtractFileInfo.
//
// The files are read only once the selection is known to be non-empty, so a rule whose glob matches nothing
// opens no file at all and is reported as the empty test it is.
//
// The violations are the files module's own assertion.AdherenceViolation values, each carrying the file,
// the requirement as the user phrased it and the mood, or the one EmptyTestViolation of a scope that
// selected no file at all.
//
// The error is technical or the user's — a missing function or message, a pattern the scope could not
// compile, a locator naming no Go project, a file of the project that cannot be read — and never a failing
// rule. When it is non-nil the violations say nothing.
func (c FilesAdherenceCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	// The graph is deliberately dropped: this rule judges each selected file on its own contents, so the
	// dependencies between them say nothing about it.
	_, selected, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}

	if empty := assertion.GatherEmptyTestViolations(len(selected), c.rule.emptyTestOptions(options)); len(empty) > 0 {
		// A rule with no subject is reported instead of being judged: every file of an empty selection
		// satisfies every predicate, in either mood, so such a rule would otherwise pass forever.
		return empty, nil
	}

	files, err := c.rule.readSources(selected)
	if err != nil {
		return nil, err
	}
	return filesassertion.GatherAdherenceViolations(files, c.predicate, c.requirement, c.rule.mood), nil
}

// String renders the whole rule as the sentence the user typed, as `project files, path without filename
// matches "internal/**", should, adhere to "be at most 400 lines long"`.
//
// The message is what stands in for the function, because a closure has no readable form: this is the one
// stage of the grammar whose rule the library cannot print, and the reason AdhereTo insists on being given
// words for it.
func (c FilesAdherenceCondition) String() string {
	return c.rule.render(`adhere to "` + c.requirement + `"`)
}

// adheringTo is both moods of the predicate: keep the user's function and the words describing it, and hand
// back the terminal that asks the function about every selected file.
//
// A rule the library cannot run is deferred to the terminal exactly as a rejected pattern is: the rejection
// joins the scope, so the rule renders with it visible and Check returns it as a UserError naming
// `adhere to` before the project is read. Two things are rejected, and neither is a rule failure — a
// missing function, which is a rule that says nothing, and a missing message, which is a failure nobody
// could read. The requirement is kept either way, so that the rejected rule still renders as what the user
// meant to type.
func (r filesRule) adheringTo(predicate filesassertion.FilePredicate, requirement string) FilesAdherenceCondition {
	condition := FilesAdherenceCondition{rule: r, predicate: predicate, requirement: requirement}
	switch {
	case predicate == nil:
		condition.rule.scope = r.scope.rejecting("adhere to", requirement, ErrNoPredicate)
	case strings.TrimSpace(requirement) == "":
		condition.rule.scope = r.scope.rejecting("adhere to", requirement, ErrNoRequirement)
	}
	return condition
}

// readSources reads the source of the files a rule selected, so that a user's predicate can be asked about
// their contents: one files/extraction.FileInfo per identifier, in the order the selection sorted them into.
//
// The project is located again here rather than threaded out of the extraction: a locator is immutable and
// LocateProject is a walk up to a go.mod, so the answer is the same root the graph was extracted from, and
// it is nothing next to reading the files themselves. What must not differ is the root the identifiers are
// relative to — they were minted against it — which is why the locator is the one the scope carries and
// never a second one.
func (r filesRule) readSources(selected []string) ([]filesextraction.FileInfo, error) {
	root, err := extraction.LocateProject(r.scope.locator)
	if err != nil {
		return nil, err
	}
	return filesextraction.ExtractFileInfo(root, selected)
}
