// Package fluentapi is the chain a user types to describe a rule about files. It is the only part of
// the files module that is public API, and the entry point of every rule in it is ProjectFiles:
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/api/**").
//		WithName("*.go")
//
// A rule is a value, not an action. Every stage here returns a new builder and does no work — no
// filesystem, no toolchain, nothing but a user pattern compiled to a regex — so a half-built rule can
// be stored in a variable and branched from as often as it is useful.
//
// The scope verbs are `with name`, `in folder`, `in path` and `in file`, they are chainable, and they
// are combined with AND: each one narrows the selection, so their order never matters. What that
// selection means, given a graph, is files/projection.SelectFiles.
//
// Each of them takes an exclusion, which is `except` and its three targeted forms in except.go: it
// qualifies the verb it follows, so `in folder "app/**", except "**/generated"` is one clause and not an
// inverted rule. Object verbs take the same companion.
//
// After the scope comes the mood, and there are exactly two of it — Should and ShouldNot, with no
// synonyms — returning the two thin builders in mood.go. The predicate and the terminal are the stages
// after that.
package fluentapi

import (
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

// FilesBuilder is the scope stage of a rule about files: `project files`, plus every scope verb chained
// onto it. It is what ProjectFiles returns, what each scope verb returns, and what the mood is asked of.
//
// A FilesBuilder is immutable. Every method takes a value receiver and hands back a new builder, so
// storing one and branching from it is safe and is the point:
//
//	base := archunit.ProjectFiles(nil).InFolder("internal/**")
//	sources := base.WithName("*.go")
//	generated := base.WithName("*_gen.go")
//
// The zero value is `project files` over the whole project, auto-detected — the same builder
// ProjectFiles(nil) returns.
type FilesBuilder struct {
	// locator is where the project is, and nil means auto-detect, as it does at every entry point.
	locator *extraction.ProjectLocator
	// factory compiles the strings the scope verbs take. It is the one place this module decides that a
	// user pattern is a glob, which is also why no scope verb below mentions glob syntax.
	factory matching.RegexFactory
	// selectors are the compiled scope verbs, in the order they were chained, and they are combined
	// with AND.
	selectors []matching.Filter
	// err is the first pattern a scope verb rejected, kept until a terminal can return it. A fluent
	// method has nowhere to put an error, and failing at the end of the chain is what lets the chain
	// read as a sentence.
	err error
}

// ProjectFiles is the entry point of every rule about files: `project files`.
//
// The locator says where the project is and is always optional — nil means auto-detect, which walks up
// from the working directory to the nearest go.mod and is what a test run inside the project it is
// about always wants:
//
//	thisProject := archunit.ProjectFiles(nil)
//	thatProject := archunit.ProjectFiles(&archunit.ProjectLocator{Directory: dir})
//
// Nothing is read here. The returned builder describes a set of files and only a terminal resolves it.
//
// The locator is copied rather than kept, so a caller may reuse one struct to build a rule per directory
// and each rule still means the directory it was built with. A builder that is immutable in every stage
// but the argument it started from would be the more surprising of the two contracts.
func ProjectFiles(locator *extraction.ProjectLocator) FilesBuilder {
	if locator != nil {
		copied := *locator
		locator = &copied
	}
	return FilesBuilder{locator: locator, factory: matching.NewRegexFactory(nil)}
}

// Files is ProjectFiles under the shorter name the family also gives it, for a chain that reads better
// without the verb. The two are one entry point: `files, in folder "internal/**"` and `project files,
// in folder "internal/**"` build the same rule.
func Files(locator *extraction.ProjectLocator) FilesBuilder {
	return ProjectFiles(locator)
}

// WithName narrows the scope to the files whose name matches this pattern: the last segment of the
// identifier, so `*.go` means every Go file in the project and `handler.go` every file with that name
// wherever it lives.
func (b FilesBuilder) WithName(pattern string) FilesBuilder {
	return b.selecting("with name", pattern, b.factory.FilenameMatcher)
}

// InFolder narrows the scope to the files in a folder matching this pattern: the identifier without its
// last segment, so `internal/api` is that folder alone and `internal/api/**` is it together with
// everything below it. A file at the project root is in the folder `.`.
func (b FilesBuilder) InFolder(pattern string) FilesBuilder {
	return b.selecting("in folder", pattern, b.factory.FolderMatcher)
}

// InPath narrows the scope to the files whose whole identifier matches this pattern, folder and name at
// once — `internal/**/handler*.go`.
func (b FilesBuilder) InPath(pattern string) FilesBuilder {
	return b.selecting("in path", pattern, b.factory.PathMatcher)
}

// InFile narrows the scope to one named file. The identifier is taken literally rather than as a
// pattern, so a file whose name contains `*`, `[` or `.` needs no defensive spelling, and it is the
// whole project-relative identifier — `internal/api/handler.go`, not `handler.go`. Selecting by bare
// name is WithName's job.
//
// Chaining it more than once selects nothing, because the verbs are combined with AND and no file is
// two files. Two named files are two rules.
func (b FilesBuilder) InFile(identifier string) FilesBuilder {
	return b.selecting("in file", identifier, b.factory.ExactFileMatcher)
}

// Selectors are the compiled scope verbs this builder was built from, in the order they were chained.
// They are the data a report needs in order to say which pattern selected nothing — each one already
// knows the part of an identifier it looks at — and they are what assertion.EmptyTestViolation carries.
//
// The result is the caller's own copy, because a builder that has been stored must not change
// afterwards. A pattern a scope verb rejected is not among them; the rejection is reported as an error
// by the terminal instead.
func (b FilesBuilder) Selectors() []matching.Filter {
	return slices.Clone(b.selectors)
}

// SelectFiles resolves this scope against the project: the identifiers of the files it describes,
// sorted. A nil *CheckOptions means the defaults.
//
// It is the half of every rule about files that runs before the mood — locate the project, extract it,
// keep the files every scope verb accepts — so the predicates that follow are written over it, and it
// is how a user can see what a half-built rule is talking about.
//
// Selecting nothing is neither an error nor a violation here. Whether an empty selection is a failure
// is a question only a rule that judges something can ask, so the empty-test guard belongs to the
// terminal and Selectors is the data it reports with.
//
// The error is a pattern a scope verb rejected — a UserError naming the verb, returned before the
// project is read — or a project that cannot be located or extracted. It is never a rule failure.
func (b FilesBuilder) SelectFiles(options *kernel.CheckOptions) ([]string, error) {
	_, selected, err := b.resolve(options)
	return selected, err
}

// resolve is the SOURCE-and-EXTRACT-plus-scope half of a rule about files, in one call: the graph the
// rule is to be judged against, and the identifiers of the files its scope names, sorted.
//
// It is what SelectFiles hands out the second half of and what every terminal in this module runs
// first. A terminal needs both — the files, to count for the empty-test guard, and the graph, because
// the dependencies between those files are edges of it — and asking for them separately would extract
// the project twice or, worse, resolve the scope against a second graph.
//
// A pattern a scope verb rejected is returned before the project is read, and the error is otherwise a
// project that cannot be located or extracted. It is never a rule failure.
func (b FilesBuilder) resolve(options *kernel.CheckOptions) (extraction.Graph, []string, error) {
	if b.err != nil {
		return nil, nil, b.err
	}
	graph, err := options.ExtractGraph(b.locator)
	if err != nil {
		return nil, nil, err
	}
	return graph, projection.SelectFiles(graph, b.selectors...), nil
}

// String renders the scope for logs and test failures, as `project files, path without filename
// matches "internal/**", filename matches "*.go"`. Each selector describes itself, which is what a
// reader needs in order to see which part of an identifier a pattern was matched against; user-facing
// violation messages are built in the testing layer, not here.
func (b FilesBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.rejected()
}

// stages are the parts of the sentence this scope has been built from, in the order the user typed
// them: the entry point, then one per scope verb, ready to be joined with ", ".
//
// It is a fresh slice, because the stages that come after the scope append their own word to it — the
// mood does, in filesRule.String — and that is also why the rejection below is rendered separately
// instead of being the last stage.
func (b FilesBuilder) stages() []string {
	stages := make([]string, 0, len(b.selectors)+2)
	stages = append(stages, "project files")
	for _, selector := range b.selectors {
		stages = append(stages, selector.String())
	}
	return stages
}

// rejected renders the pattern a scope verb refused as a parenthesis closing the sentence, and the
// empty string when every pattern compiled. A rejected pattern narrowed nothing, so without it a
// builder would render as the rule the user thought they wrote.
func (b FilesBuilder) rejected() string {
	if b.err == nil {
		return ""
	}
	return " (rejected: " + b.err.Error() + ")"
}

// selecting is every scope verb: compile the string the user typed with this builder's factory, and
// hand back a new builder narrowed by the resulting filter. Which part of an identifier a verb looks at
// is the compile function it passes in, so that pairing is stated once per verb and nowhere else.
func (b FilesBuilder) selecting(verb, pattern string, compile func(string) (matching.Filter, error)) FilesBuilder {
	selector, err := compile(pattern)
	if err != nil {
		return b.rejecting(verb, pattern, err)
	}
	narrowed := b
	narrowed.selectors = append(slices.Clone(b.selectors), selector)
	return narrowed
}

// rejecting records that the user typed a pattern this library cannot understand: a UserError naming
// the scope verb at fault and quoting the pattern as it was written, wrapping the reason matching gave.
//
// The first rejection wins and the ones after it are dropped, because the first is the one a user has
// to fix and a chain reporting the last would point at the wrong line. The rejected pattern does not
// join the selectors: a zero Filter matches nothing, so the rule would report every file as unselected
// instead of reporting the typo.
func (b FilesBuilder) rejecting(verb, subject string, cause error) FilesBuilder {
	if b.err != nil {
		return b
	}
	rejected := b
	rejected.err = archerror.NewUserError(verb, subject, cause)
	return rejected
}
