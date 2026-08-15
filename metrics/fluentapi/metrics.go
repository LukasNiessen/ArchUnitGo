// Package fluentapi is the chain a user types to describe a rule about the numbers a project's code adds
// up to. It is the only part of the metrics module that is public API, and the entry point of every rule in
// it is Metrics:
//
//	measurements, err := archunit.Metrics(nil).
//		InFolder("internal/api/**").
//		Count().
//		LinesOfCode().
//		Measure(nil)
//
// A rule is a value, not an action. Every stage here returns a new builder and does no work — no
// filesystem, no toolchain, nothing but a user pattern compiled to a regex — so a half-built rule can be
// stored in a variable and branched from as often as it is useful.
//
// The scope verbs are `with name`, `in folder`, `in path` and `for classes matching`, they are chainable,
// and they are combined with AND: each one narrows the selection, so their order never matters. The first
// three describe files and the last describes classes, and what a scope means, given a project, is
// metrics/projection.SelectFiles followed by metrics/projection.SelectSubjects.
//
// After the scope comes the metric: `count` groups the eight counts this library can take, and one of them
// closes it. Measure is the resolution door of what that describes — the numbers themselves, one per
// subject — and the six threshold predicates that judge them against a number are the mood and the
// predicate stages, which land with them.
package fluentapi

import (
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

// MetricsBuilder is the scope stage of a rule about numbers: `metrics`, plus every scope verb chained onto
// it. It is what Metrics returns, what each scope verb returns, and what the metric is asked of.
//
// A MetricsBuilder is immutable. Every method takes a value receiver and hands back a new builder, so
// storing one and branching from it is safe and is the point:
//
//	base := archunit.Metrics(nil).InFolder("internal/**")
//	size := base.Count().LinesOfCode()
//	fanOut := base.Count().Imports()
//
// The zero value is `metrics` over the whole project, auto-detected — the same builder Metrics(nil)
// returns.
type MetricsBuilder struct {
	// locator is where the project is, and nil means auto-detect, as it does at every entry point.
	locator *extraction.ProjectLocator
	// factory compiles the strings the scope verbs take. It is the one place this module decides that a
	// user pattern is a glob, which is also why no scope verb below mentions glob syntax.
	factory matching.RegexFactory
	// selectors are the compiled scope verbs, in the order they were chained, and they are combined with
	// AND. Both kinds are in here, because a filter already knows which part of an identifier it looks at:
	// the ones matching a classname narrow the classes and the rest narrow the files, so what a verb
	// selects is stated by the verb and never tracked twice.
	selectors []matching.Filter
	// err is the first pattern a scope verb rejected, kept until a terminal can return it. A fluent method
	// has nowhere to put an error, and failing at the end of the chain is what lets the chain read as a
	// sentence.
	err error
}

// Metrics is the entry point of every rule about numbers: `metrics`.
//
// The locator says where the project is and is always optional — nil means auto-detect, which walks up from
// the working directory to the nearest go.mod and is what a test run inside the project it is about always
// wants:
//
//	thisProject := archunit.Metrics(nil)
//	thatProject := archunit.Metrics(&archunit.ProjectLocator{Directory: dir})
//
// Nothing is read here. The returned builder describes a set of files and only a resolving stage reads
// them.
//
// The family gives this entry point one name rather than two: there is no `project metrics`, because the
// noun is already the thing being described rather than the project it is described of.
//
// The locator is copied rather than kept, so a caller may reuse one struct to build a rule per directory and
// each rule still means the directory it was built with.
func Metrics(locator *extraction.ProjectLocator) MetricsBuilder {
	if locator != nil {
		copied := *locator
		locator = &copied
	}
	return MetricsBuilder{locator: locator, factory: matching.NewRegexFactory(nil)}
}

// WithName narrows the scope to the files whose name matches this pattern: the last segment of the
// identifier, so `*.go` means every Go file in the project and `handler.go` every file with that name
// wherever it lives.
func (b MetricsBuilder) WithName(pattern string) MetricsBuilder {
	return b.selecting("with name", pattern, b.factory.FilenameMatcher)
}

// InFolder narrows the scope to the files in a folder matching this pattern: the identifier without its last
// segment, so `internal/api` is that folder alone and `internal/api/**` is it together with everything
// below it. A file at the project root is in the folder `.`.
func (b MetricsBuilder) InFolder(pattern string) MetricsBuilder {
	return b.selecting("in folder", pattern, b.factory.FolderMatcher)
}

// InPath narrows the scope to the files whose whole identifier matches this pattern, folder and name at once
// — `internal/**/handler*.go`.
func (b MetricsBuilder) InPath(pattern string) MetricsBuilder {
	return b.selecting("in path", pattern, b.factory.PathMatcher)
}

// ForClassesMatching narrows the scope to the classes whose name matches this pattern — `*Service`, `Handler`
// — where a class is a declared type: a struct, an interface, or a name given to another type. Go has no
// classes; the vocabulary is the family's.
//
// The pattern is matched against the bare name, so `*Service` describes `internal/api.UserService` as well as
// `internal/db.UserService`, and a rule that means one package says so with InFolder as well.
//
// It narrows the files too, and that is worth knowing: a rule that names classes and then counts something
// about a file — its lines, its imports — is measured over the files declaring one of those classes rather
// than over every file the folder verbs kept. metrics/projection.SelectSubjects is where both narrowings
// happen, and the alternative would be a verb the user typed that quietly changed nothing.
func (b MetricsBuilder) ForClassesMatching(pattern string) MetricsBuilder {
	return b.selecting("for classes matching", pattern, b.factory.ClassnameMatcher)
}

// Selectors are the compiled scope verbs this builder was built from, in the order they were chained, both
// the ones about files and the ones about classes. They are the data a report needs in order to say which
// pattern selected nothing — each one already knows the part of an identifier it looks at — and they are what
// assertion.EmptyTestViolation carries.
//
// The result is the caller's own copy, because a builder that has been stored must not change afterwards. A
// pattern a scope verb rejected is not among them; the rejection is reported as an error by the resolving
// stage instead.
func (b MetricsBuilder) Selectors() []matching.Filter {
	return slices.Clone(b.selectors)
}

// String renders the scope for logs and test failures, as `metrics, path without filename matches
// "internal/**", classname matches "*Service"`. Each selector describes itself, which is what a reader needs
// in order to see which part of an identifier a pattern was matched against; user-facing violation messages
// are built in the testing layer, not here.
func (b MetricsBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.rejected()
}

// resolve is the SOURCE-and-EXTRACT-plus-scope half of every rule about numbers, in one call: the files this
// scope names, read and counted, and the classes among them it names.
//
// The two reads are one door apiece and both are needed. The graph is what says which files are the
// project's, so the scope is resolved against the same population every other rule in the library is
// resolved against; the root is where those identifiers are read from, which is why it is located again
// rather than guessed from the graph.
//
// Only the files the scope selected are read, so a rule about one folder pays for one folder. The
// consequence for a class's method count is ExtractFileInfo's to describe.
//
// A pattern a scope verb rejected is returned before the project is read, and the error is otherwise a
// project that cannot be located, extracted or read. It is never a rule failure.
func (b MetricsBuilder) resolve(options *kernel.CheckOptions) (projection.Subjects, error) {
	if b.err != nil {
		return projection.Subjects{}, b.err
	}
	graph, err := options.ExtractGraph(b.locator)
	if err != nil {
		return projection.Subjects{}, err
	}
	root, err := extraction.LocateProject(b.locator)
	if err != nil {
		return projection.Subjects{}, err
	}
	files, err := metricsextraction.ExtractFileInfo(root, projection.SelectFiles(graph, b.fileSelectors()...))
	if err != nil {
		return projection.Subjects{}, err
	}
	return projection.SelectSubjects(files, b.classSelectors()...), nil
}

// fileSelectors are the scope verbs that describe a file, and classSelectors the ones that describe a class.
// Which of the two a verb is, is the part of an identifier its filter looks at, so the split is derived from
// the selectors themselves instead of being remembered in a second field that could disagree with them.
func (b MetricsBuilder) fileSelectors() []matching.Filter {
	return b.selectorsFor(false)
}

func (b MetricsBuilder) classSelectors() []matching.Filter {
	return b.selectorsFor(true)
}

// selectorsFor keeps the selectors that look at a classname, or the ones that do not.
func (b MetricsBuilder) selectorsFor(classes bool) []matching.Filter {
	kept := make([]matching.Filter, 0, len(b.selectors))
	for _, selector := range b.selectors {
		if (selector.Target() == matching.TargetClassname) == classes {
			kept = append(kept, selector)
		}
	}
	return kept
}

// stages are the parts of the sentence this scope has been built from, in the order the user typed them: the
// entry point, then one per scope verb, ready to be joined with ", ".
//
// It is a fresh slice, because the stages that come after the scope append their own word to it — the metric
// does, in MetricsCountBuilder.stages — and that is also why the rejection below is rendered separately
// instead of being the last stage.
func (b MetricsBuilder) stages() []string {
	stages := make([]string, 0, len(b.selectors)+3)
	stages = append(stages, "metrics")
	for _, selector := range b.selectors {
		stages = append(stages, selector.String())
	}
	return stages
}

// rejected renders the pattern a scope verb refused as a parenthesis closing the sentence, and the empty
// string when every pattern compiled. A rejected pattern narrowed nothing, so without it a builder would
// render as the rule the user thought they wrote.
func (b MetricsBuilder) rejected() string {
	if b.err == nil {
		return ""
	}
	return " (rejected: " + b.err.Error() + ")"
}

// selecting is every scope verb: compile the string the user typed with this builder's factory, and hand back
// a new builder narrowed by the resulting filter. Which part of an identifier a verb looks at is the compile
// function it passes in, so that pairing is stated once per verb and nowhere else.
func (b MetricsBuilder) selecting(verb, pattern string, compile func(string) (matching.Filter, error)) MetricsBuilder {
	selector, err := compile(pattern)
	if err != nil {
		return b.rejecting(verb, pattern, err)
	}
	narrowed := b
	narrowed.selectors = append(slices.Clone(b.selectors), selector)
	return narrowed
}

// rejecting records that the user typed a pattern this library cannot understand: a UserError naming the scope
// verb at fault and quoting the pattern as it was written, wrapping the reason matching gave.
//
// The first rejection wins and the ones after it are dropped, because the first is the one a user has to fix
// and a chain reporting the last would point at the wrong line. The rejected pattern does not join the
// selectors: a zero Filter matches nothing, so the rule would report every file as unselected instead of
// reporting the typo.
func (b MetricsBuilder) rejecting(verb, subject string, cause error) MetricsBuilder {
	if b.err != nil {
		return b
	}
	rejected := b
	rejected.err = archerror.NewUserError(verb, subject, cause)
	return rejected
}
