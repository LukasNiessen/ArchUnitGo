// Package fluentapi is the chain a user types to describe a dependency-graph report. It is the only part of
// the graph module that is public API, and the entry point of every report in it is ProjectGraph:
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		FocusOn("internal/api/**", 1).
//		CollapseToFolderDepth(2).
//		Titled("what the api layer touches").
//		Snapshot()
//
// A report is a value, not an action. Every stage here returns a new builder and does no work — no
// filesystem, no toolchain, nothing but a user pattern compiled to a regex — so a half-built report can be
// stored in a variable and branched from as often as it is useful, which is what makes one query with three
// output formats, or one collapse with three different focuses, a matter of reusing a variable.
//
// This module has no mood, no predicate and no violations, and that is the grammar rather than an omission: a
// report is not a rule. Nothing here judges a codebase, so there is nothing for it to disagree with and
// nothing to assert — which is also why the module has a projection package and no assertion package. What
// the chain has instead of a mood is modifiers, and there are nine of them, every one optional, chainable and
// order-independent: `including external dependencies`, `including self dependencies`, `focus on`, `reachable
// from`, `dependents of`, `collapse to folder depth`, `collapse by pattern`, `titled` and `with check
// options`.
//
// Four of those take a pattern, and each of them takes the exclusion of except.go: `except` qualifies the
// pattern modifier the chain wrote most recently, so `focus on "app/**" within 1 hop, except "**/generated/**"`
// is one modifier and not a tenth kind of query. It is the one word here that is not order-independent, and it
// cannot be: an exclusion belongs to the clause it was typed in.
//
// There are thirteen terminals. Snapshot hands the report back as data, and the other twelve hand it back as a
// document: `to dot`, `to mermaid`, `to d2`, `to csv`, `to json` and `to html` as a string, and `export as
// dot` and its five siblings as a file on disk. Every one of them is Snapshot followed by one function of
// graph/rendering, and that two-step split is the reason this module is shaped the way it is: a query builds a
// projection.Snapshot, and rendering is a function of that snapshot alone. So a new output format is one
// function that nothing here has to know about, and a modifier added here is understood by every format the
// day it lands.
package fluentapi

import (
	"errors"
	"strconv"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// The two ways a dependency-graph report can be typed wrongly, as sentinels a caller can recognize with
// errors.Is. Each of them is reported as an archerror.UserError naming the modifier at fault: the library is
// working and the code has not been judged, there is simply no report to render.
var (
	// ErrUnnamedGroup says `collapse by pattern` was given the empty string for a group name. A group is a
	// node of the diagram and a node has to be called something, so a nameless group would draw every file
	// it matched under a blank label.
	ErrUnnamedGroup = errors.New("collapse group without a name")
	// ErrInvalidFolderDepth says `collapse to folder depth` was given a depth below one. Depth is a number
	// of path segments to keep, so zero segments is not a folder any file lives in; asking not to collapse
	// at all is not calling the modifier.
	ErrInvalidFolderDepth = errors.New("folder depth below one")
)

// GraphBuilder is a dependency-graph report as far as it has been described: `project graph`, plus every
// modifier chained onto it. It is what ProjectGraph returns, what every modifier returns, and what the
// terminal is asked of.
//
// A GraphBuilder is immutable. Every method takes a value receiver and hands back a new builder, so storing
// one and branching from it is safe and is the point — a query is the expensive half of a report to write,
// and asking two questions about one part of a system should not mean typing it twice:
//
//	api := archunit.ProjectGraph(nil).FocusOn("internal/api/**", 0)
//	inside := api.CollapseToFolderDepth(3)
//	whatItUses := api.IncludingExternalDependencies()
//
// It is both the whole grammar of this module and its terminal, unlike the rule families, where a scope
// cannot be checked until a mood and a predicate have been added. Every modifier is optional, so `project
// graph` on its own is already a report — the whole project, one node per file — and there is no stage it is
// waiting for.
//
// The zero value is that report over the whole project, auto-detected: the same builder ProjectGraph(nil)
// returns.
type GraphBuilder struct {
	// locator is where the project is, and nil means auto-detect, as it does at every entry point.
	locator *extraction.ProjectLocator
	// factory compiles the strings the pattern modifiers take. It is the one place this module decides that
	// a user pattern is a glob, which is also why no modifier below mentions glob syntax.
	factory matching.RegexFactory
	// query is what the modifiers have said about the report: which nodes, drawn as what, under what title.
	// It is the whole of what the projection is given, and it is a value rather than a pointer so that a
	// builder cannot share it with the copy it came from.
	query projection.SnapshotOptions
	// qualified is which of the query's pattern modifiers the chain wrote most recently, which is the one an
	// `except` qualifies. It is remembered rather than derived, because the four of them append to four
	// different fields and nothing in a resolved query says which was written last.
	qualified patternModifier
	// check is how the project is read, from `with check options`, and nil means the defaults. It is a copy
	// of what the user passed, for the reason the locator is: a report that changes when the bag it was
	// built from is edited afterwards is not immutable.
	check *kernel.CheckOptions
	// err is the first thing the user typed that this library cannot understand — a pattern that will not
	// compile, a group without a name, a folder depth below one — kept until the terminal can return it. A
	// fluent method has nowhere to put an error, and failing at the end of the chain is what lets the chain
	// read as a sentence.
	err error
}

// ProjectGraph is the entry point of every dependency-graph report: `project graph`.
//
// The locator says where the project is and is always optional — nil means auto-detect, which walks up from
// the working directory to the nearest go.mod and is what a test run inside the project it is about always
// wants:
//
//	thisProject := archunit.ProjectGraph(nil)
//	thatProject := archunit.ProjectGraph(&archunit.ProjectLocator{Directory: dir})
//
// Nothing is read here. The returned builder describes a report of a project that has not been looked at, and
// only the terminal resolves it.
//
// The locator is copied rather than kept, so a caller may reuse one struct to build a report per directory and
// each report still means the directory it was built with.
func ProjectGraph(locator *extraction.ProjectLocator) GraphBuilder {
	if locator != nil {
		copied := *locator
		locator = &copied
	}
	return GraphBuilder{locator: locator, factory: matching.NewRegexFactory(nil)}
}

// DependencyGraph is ProjectGraph under the other name the family gives it, for a chain that reads better as
// a noun. The two are one entry point: `dependency graph, collapse to folder depth 2` and `project graph,
// collapse to folder depth 2` describe the same report.
func DependencyGraph(locator *extraction.ProjectLocator) GraphBuilder {
	return ProjectGraph(locator)
}

// Titled names the report — `the modules of this project` — which is what a renderer prints as its headline.
//
// It is a modifier like the others and it is optional; a report with no title leaves the headline to the
// format, because what an untitled diagram should say at the top is the renderer's business and differs per
// format. Titling twice keeps the last title: a title is one string, not a list.
func (b GraphBuilder) Titled(title string) GraphBuilder {
	titled := b.modifying()
	titled.query.Title = title
	return titled
}

// WithCheckOptions says how the project is to be read — build tags, test files, ignored import kinds, a
// cleared cache — for the report this chain describes. A nil bag means the defaults.
//
// It is a modifier rather than an argument to the terminal, unlike `check(options?)` in every rule family,
// and the reason is that this module's terminals are not one but thirteen: a snapshot, six rendered documents
// and six files written to disk would each have to take the same bag, and the chain is where a thing that is
// said once belongs. AllowEmptyTests is honored here too, as the way to permit a report of nothing.
//
// The bag is copied, so editing it afterwards does not change a report already described. Calling this twice
// keeps the last bag.
func (b GraphBuilder) WithCheckOptions(options *kernel.CheckOptions) GraphBuilder {
	resolved := options.WithDefaults()
	configured := b.modifying()
	configured.check = &resolved
	return configured
}

// String renders the report as far as it has been described, as `project graph, including external
// dependencies, focus on path matches "internal/api/**" within 1 hop, collapse to folder depth 2, titled
// "the api layer"`.
//
// The modifiers are printed in this module's own order rather than the order they were chained in, because
// they are order-independent: two chains that describe the same report should read the same, and a reader
// comparing two reports should not have to notice that one of them said `titled` first.
func (b GraphBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.rejected()
}

// stages are the parts of the sentence this report has been described by, ready to be joined with ", ": the
// entry point, then every modifier that has something to say, in the canonical order.
//
// It is a fresh slice, and the rejection below is rendered separately rather than as the last stage, because
// a rejected pattern ends the sentence instead of sitting inside it.
func (b GraphBuilder) stages() []string {
	stages := make([]string, 0, 8)
	stages = append(stages, "project graph")
	if b.query.IncludeExternalDependencies {
		stages = append(stages, "including external dependencies")
	}
	if b.query.IncludeSelfDependencies {
		stages = append(stages, "including self dependencies")
	}
	for _, focus := range b.query.Focus {
		stages = append(stages, "focus on "+focus.String())
	}
	for _, selector := range b.query.ReachableFrom {
		stages = append(stages, "reachable from "+selector.String())
	}
	for _, selector := range b.query.DependentsOf {
		stages = append(stages, "dependents of "+selector.String())
	}
	if b.query.CollapseToFolderDepth > 0 {
		stages = append(stages, "collapse to folder depth "+strconv.Itoa(b.query.CollapseToFolderDepth))
	}
	for _, group := range b.query.CollapseGroups {
		stages = append(stages, "collapse by pattern "+group.String())
	}
	if b.query.Title != "" {
		stages = append(stages, `titled "`+b.query.Title+`"`)
	}
	if b.check != nil {
		stages = append(stages, "with check options")
	}
	return stages
}

// rejected renders the thing the user typed that this library could not understand as a parenthesis closing
// the sentence, and the empty string when the whole chain compiled. A rejected modifier narrowed nothing, so
// without it a builder would render as the report the user thought they asked for.
func (b GraphBuilder) rejected() string {
	if b.err == nil {
		return ""
	}
	return " (rejected: " + b.err.Error() + ")"
}

// modifying is the copy every modifier writes into: a new builder whose query owns its own slices, so that
// appending a second `focus on` cannot reach into the query of the builder it was chained from.
//
// A struct copy shares a slice's backing array, and a builder is meant to be stored and branched from, so
// this is the one line that keeps `base.FocusOn(a)` and `base.FocusOn(b)` from being the same report.
func (b GraphBuilder) modifying() GraphBuilder {
	modified := b
	modified.query = b.query.WithDefaults()
	return modified
}

// rejecting records that the user typed something this library cannot understand: a UserError naming the
// modifier at fault and quoting the argument as it was written, wrapping the reason.
//
// The first rejection wins and the ones after it are dropped, because the first is the one a user has to fix
// and a chain reporting the last would point at the wrong line. The rejected modifier does not join the
// query: a zero Filter matches nothing, so a report would come out empty as though the project were, instead
// of reporting the typo.
func (b GraphBuilder) rejecting(modifier, subject string, cause error) GraphBuilder {
	if b.err != nil {
		return b
	}
	rejected := b
	rejected.err = archerror.NewUserError(modifier, subject, cause)
	return rejected
}
