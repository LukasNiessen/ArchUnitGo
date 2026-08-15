// Package fluentapi is the chain a user types to describe a rule about slices. It is the only part of the
// slices module that is public API, and the entry point of every rule in it is ProjectSlices:
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		ShouldNot().
//		ContainDependency("api", "db")
//	violations, err := rule.Check(nil)
//
// A slice is a name cut out of a file's identifier, and the capture in that pattern is where the name comes
// from: `internal/(**)/**` says that the slices of this project are its folders under internal, so the file
// `internal/api/handler.go` is in the slice `api`. Nothing else declares them — there is no list of slices
// anywhere, which is the whole difference from a layer policy, where every layer is named before any file is
// read.
//
// A rule is a value, not an action. Every stage here returns a new builder and does no work — no filesystem,
// no toolchain, nothing but a user pattern compiled to a regex — so a slicing can be stored in a variable and
// branched from as often as it is useful:
//
//	slicing := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**")
//	noDatabase := slicing.ShouldNot().ContainDependency("api", "db")
//	viaDomain := slicing.Should().ContainDependency("api", "domain")
//
// The chain is the family's ordinary one: the entry point, one scope verb — `defined by` or `defined by
// regex` — the mood, the predicate `contain dependency`, and the terminal `check`. The scope is exactly one
// verb rather than the usual chain of them, because a slicing is a projection and not a selection: two
// slicings would be two different vocabularies to talk about the project in, so a second one is
// ErrSlicedTwice instead of a narrower rule.
package fluentapi

import (
	"errors"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/slices/projection"
)

// The four ways a rule about slices can be typed wrongly, as sentinels a caller can recognize with
// errors.Is. Each of them is reported as an archerror.UserError naming the step of the chain at fault: the
// library is working and the code has not been judged, there is simply no runnable rule to judge it with.
var (
	// ErrNoSlicing says the chain reached its mood without a slicing: nobody said what the slices of this
	// project are, so there is no vocabulary for the rule to be about. It is the one rejection this module
	// cannot leave to the type system, because the mood is asked of the entry point itself.
	ErrNoSlicing = errors.New("no slicing")
	// ErrSlicedTwice says two slicing verbs were chained. A slicing is a function from a file to the name of
	// its slice, so a second one is not a narrower rule the way a second scope verb is elsewhere in the
	// family — it is a different set of names for the same project, and there is no reading of the two
	// together that a user could have meant.
	ErrSlicedTwice = errors.New("slicing declared twice")
	// ErrUnnamedSlice says a predicate named a slice with the empty string. A slice is a name a pattern cut
	// out of an identifier, and the projection never produces an empty one, so such a rule would judge
	// nothing at all.
	ErrUnnamedSlice = errors.New("slice without a name")
	// ErrSelfDependency says a predicate named one slice twice — `contain dependency "api", "api"`. A slice
	// may always depend on itself and the projection does not even carry that dependency, so the negated
	// rule would hold forever and the positive one could never hold: neither is a rule about the code.
	ErrSelfDependency = errors.New("dependency of a slice on itself")
)

// SlicesBuilder is the entry point and the scope of a rule about slices: `project slices`, plus the slicing
// that says what the project's slices are.
//
// A SlicesBuilder is immutable. Every method takes a value receiver and hands back a new builder, so storing
// one and branching from it is safe and is the point — the slicing is the half of the rule that is worth
// typing once, and two rules over one slicing should not mean writing it twice.
//
// It is not a Checkable, and that is the grammar rather than an omission: a chain that has a slicing but no
// mood and no predicate is not yet a rule about anything, so there is nothing for a terminal to report. The
// Checkable is SlicesDependencyCondition, which the predicate returns.
//
// The zero value is `project slices` over the whole project, auto-detected, with no slicing — the same
// builder ProjectSlices(nil) returns.
type SlicesBuilder struct {
	// locator is where the project is, and nil means auto-detect, as it does at every entry point.
	locator *extraction.ProjectLocator
	// globs compiles what DefinedBy takes. The syntax travels with a factory, which is the whole of the
	// difference between the two slicing verbs: neither of them decides how a pattern is spelled at the point
	// where it is matched. Only this one is a field, because it is the zero RegexFactory — DefinedByRegex
	// builds its regex-syntax factory where it is used, which is what keeps the zero builder above the same
	// builder ProjectSlices(nil) returns.
	globs matching.RegexFactory
	// selector is the slicing: a Filter over the capture pattern the slicing verb compiled. One value serves
	// both halves of a check, because the pattern it carries is what names the slices and the filter itself
	// is what the empty-test guard reports as the thing that selected nothing — so what a report says the
	// rule was about cannot drift from what was judged.
	selector matching.Filter
	// sliced says whether a slicing verb has been accepted. It is what makes a second one ErrSlicedTwice and
	// none at all ErrNoSlicing, and it is a flag of its own because a pattern this library rejected leaves
	// the selector zero without the user having failed to type one.
	sliced bool
	// err is the first thing the user typed that this library cannot understand — a pattern that will not
	// compile, a pattern with no capture in it, a slicing declared twice, a slice named with the empty
	// string — kept until a terminal can return it. A fluent method has nowhere to put an error, and failing
	// at the end of the chain is what lets the chain read as a sentence.
	err error
}

// ProjectSlices is the entry point of every rule about slices: `project slices`.
//
// The locator says where the project is and is always optional — nil means auto-detect, which walks up from
// the working directory to the nearest go.mod and is what a test run inside the project it is about always
// wants:
//
//	thisProject := archunit.ProjectSlices(nil)
//	thatProject := archunit.ProjectSlices(&archunit.ProjectLocator{Directory: dir})
//
// Nothing is read here. The returned builder describes slices that do not exist yet, and only a terminal
// resolves them against a project.
//
// The locator is copied rather than kept, so a caller may reuse one struct to build a rule per directory and
// each rule still means the directory it was built with.
//
// There is no shorter alias for it, unlike `project files` and `project layers`: `slices` alone is the name
// of a standard library package, and a chain starting with it would read as one.
func ProjectSlices(locator *extraction.ProjectLocator) SlicesBuilder {
	if locator != nil {
		copied := *locator
		locator = &copied
	}
	return SlicesBuilder{locator: locator, globs: matching.NewRegexFactory(nil)}
}

// SelectSliceFiles resolves the slicing against the project: the identifiers of the files in each slice,
// sorted, keyed by the slice's name. A nil *CheckOptions means the defaults.
//
// It is the half of every rule about slices that runs before anything is judged — locate the project,
// extract it, cut a slice name out of every file's identifier — so it is how a user sees what a half-built
// rule is talking about, and what a report of a slicing that found nothing is built from.
//
// The slices of the result are the ones some file is in. A slicing that describes no file resolves to no
// slices at all, which is neither an error nor a violation here: whether an empty slicing is a failure is a
// question only a rule that judges something can ask, so the empty-test guard belongs to the terminal.
//
// The error is something the chain could not make sense of — a pattern that will not compile, a pattern with
// no capture in it, a slicing declared twice, no slicing at all — or a project that cannot be located or
// extracted. It is never a rule failure.
func (b SlicesBuilder) SelectSliceFiles(options *kernel.CheckOptions) (map[string][]string, error) {
	_, membership, err := b.resolve(options)
	return membership, err
}

// String renders the rule as far as it has been built, as `project slices, path matches
// "internal/(**)/**"`.
//
// The slicing describes itself as the filter it compiled to, which is what the scope verbs of the files
// module do; user-facing violation messages are built in the testing layer, not here.
func (b SlicesBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.rejected()
}

// resolve is the SOURCE-and-EXTRACT-plus-slicing half of a rule about slices, in one call: the graph the rule
// is to be judged against, and the files of each slice the slicing found in it.
//
// It is what SelectSliceFiles hands out the second half of and what the terminal runs first. A terminal needs
// both — the slices, to count for the empty-test guard, and the graph, because the dependencies between the
// slices' files are edges of it — and asking for them separately would extract the project twice or, worse,
// resolve the slices against a second graph.
//
// Anything the user typed that this library could not understand is returned before the project is read, and
// it is the first such thing rather than the last. A chain with no slicing at all is one of them, checked here
// as well as at the mood, because a builder that nobody asked a mood of can be resolved directly and an empty
// map is not an answer to a question nobody asked. The error is otherwise a project that cannot be located or
// extracted, and it is never a rule failure.
func (b SlicesBuilder) resolve(options *kernel.CheckOptions) (extraction.Graph, map[string][]string, error) {
	if b.err != nil {
		return nil, nil, b.err
	}
	if !b.sliced {
		return nil, nil, archerror.NewUserError("project slices", "", ErrNoSlicing)
	}
	graph, err := options.ExtractGraph(b.locator)
	if err != nil {
		return nil, nil, err
	}
	return graph, projection.SelectSliceFiles(graph, b.mapper()), nil
}

// mapper is the slicing as the projection wants it: the MapFunction that labels every edge of the graph with
// the slices of its two ends.
//
// It is built from the very pattern the empty-test guard reports on, because the two must not be able to
// disagree — a report naming one pattern while another one was judged is worse than no report. A builder with
// no slicing yields the mapper of the zero Pattern, which projects nothing; ErrNoSlicing is where that is
// reported, so this function has no error of its own.
func (b SlicesBuilder) mapper() kernelprojection.MapFunction {
	return projection.SliceByCapture(b.selector.Pattern())
}

// selectors are the slicing as the empty-test guard is asked about it: the one filter that described the
// files, or nothing at all when no slicing has been accepted.
//
// The guard wants a list because most rules in the family are selected by a chain of verbs; a slicing is
// exactly one, so this is a list of one. A rule with no slicing never reaches the guard — resolve returns
// ErrNoSlicing first — and the empty list is what keeps that unreachable case from claiming a pattern that
// was never typed.
func (b SlicesBuilder) selectors() []matching.Filter {
	if !b.sliced {
		return nil
	}
	return []matching.Filter{b.selector}
}

// stages are the parts of the sentence this builder has been built from, in the order the user typed them:
// the entry point and, once one has been accepted, the slicing, ready to be joined with ", ".
//
// It is a fresh slice, and the rejection below is rendered separately rather than as the last stage, because
// a rejected pattern ends the sentence instead of sitting inside it.
func (b SlicesBuilder) stages() []string {
	stages := make([]string, 0, 2)
	stages = append(stages, "project slices")
	if b.sliced {
		stages = append(stages, b.selector.String())
	}
	return stages
}

// rejected renders the thing the user typed that this library could not understand as a parenthesis closing
// the sentence, and the empty string when the whole chain compiled. A rejected pattern sliced nothing, so
// without it a rule would render as the one the user thought they wrote.
func (b SlicesBuilder) rejected() string {
	if b.err == nil {
		return ""
	}
	return " (rejected: " + b.err.Error() + ")"
}

// slicing is both slicing verbs: compile the string the user typed with the factory of the verb's own syntax,
// and hand back a new builder sliced by the resulting pattern. Which syntax a verb reads is the compile
// function it passes in, so that pairing is stated once per verb — the shape LayerBuilder.defining gives the
// `defined by` verbs of the layers module.
//
// A second slicing is rejected before the pattern is even compiled, so that the verb a user has to delete is
// the one the error names. A pattern this library cannot understand is deferred to the terminal as everywhere
// else in the family: the rejection joins the builder, Check returns it as a UserError naming this verb
// before the project is read, and the slicing is not recorded — the zero Pattern names nothing, so recording
// it anyway would report a slicing that found nothing instead of the typo the user has to fix.
func (b SlicesBuilder) slicing(verb, pattern string, compile func(string) (matching.Pattern, error)) SlicesBuilder {
	if b.sliced {
		return b.rejecting(verb, pattern, ErrSlicedTwice)
	}
	compiled, err := compile(pattern)
	if err != nil {
		return b.rejecting(verb, pattern, err)
	}
	if b.err != nil {
		// An earlier stage was rejected. The first rejection is the one to report, and this slicing must not
		// join a rule that cannot run.
		return b
	}
	sliced := b
	sliced.selector = matching.PathMatcher(compiled)
	sliced.sliced = true
	return sliced
}

// rejecting records that the user typed something this library cannot understand: a UserError naming the step
// of the chain at fault and quoting the argument as it was written, wrapping the reason.
//
// The first rejection wins and the ones after it are dropped, because the first is the one a user has to fix
// and a chain reporting the last would point at the wrong line.
func (b SlicesBuilder) rejecting(verb, subject string, cause error) SlicesBuilder {
	if b.err != nil {
		return b
	}
	rejected := b
	rejected.err = archerror.NewUserError(verb, subject, cause)
	return rejected
}
