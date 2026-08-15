// Package archunit is the public surface of ArchUnitGo: architecture rules written as ordinary Go unit
// tests.
//
//	func TestTheApiDoesNotTouchTheDatabase(t *testing.T) {
//		files, err := archunit.ProjectFiles(nil).InFolder("internal/api/**").SelectFiles(nil)
//		...
//	}
//
// It re-exports and does nothing else. Every name here is defined in a package under common/ or in a
// domain module, and this package exists so that a user imports one path and never has to know the
// layout — which is also why nothing inside the library is allowed to depend on it.
//
// A rule is a value, not an action: building one does no work, and only a terminal reads the project.
// The chain starts at an entry point — ProjectFiles, ProjectLayers, ProjectSlices, Metrics and ProjectGraph
// today, one per family as they land — and every entry point takes an optional *ProjectLocator, where nil means
// the project the test itself is in.
//
// Not every chain describes a rule. ProjectGraph describes a report, so it has no mood, no predicate and no
// violations: its terminals hand back the diagram — as data, as a document in one of six formats, or as a file
// written to disk — rather than a list of what the code disagreed with. Everything else about it is the same —
// a value, chainable modifiers, the same optional locator.
package archunit

import (
	"github.com/LukasNiessen/ArchUnitGo/archtest"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	filesextraction "github.com/LukasNiessen/ArchUnitGo/files/extraction"
	filesapi "github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
	graphapi "github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
	graphprojection "github.com/LukasNiessen/ArchUnitGo/graph/projection"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
	layersapi "github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
	metricscalculation "github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	metricsapi "github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
	slicesassertion "github.com/LukasNiessen/ArchUnitGo/slices/assertion"
	slicesapi "github.com/LukasNiessen/ArchUnitGo/slices/fluentapi"
	slicesprojection "github.com/LukasNiessen/ArchUnitGo/slices/projection"
)

// ProjectLocator says where the project under analysis is. A nil *ProjectLocator means auto-detect,
// which walks up from the working directory to the nearest go.mod.
type ProjectLocator = extraction.ProjectLocator

// CheckOptions is the one options bag a terminal takes: everything about how a rule is run, as opposed
// to what it says. A nil *CheckOptions means the defaults.
type CheckOptions = fluentapi.CheckOptions

// Mood is which of the two moods a rule was written in, `should` or `should not`. It is what the mood
// stage of a chain reports, and the flag the library's assertions take so that a rule and its negation
// are one piece of logic.
type Mood = assertion.Mood

const (
	// Should is the positive mood: the rule holds where its predicate is satisfied.
	Should = assertion.Should
	// ShouldNot is the negated mood: the rule holds where its predicate is not satisfied.
	ShouldNot = assertion.ShouldNot
)

// Checkable is a rule that can be run: what every chain ends in, and the one thing a helper that loops
// over a list of rules has to know about them. Check returns one Violation per place the code disagrees
// with the rule, and an empty result is the pass.
type Checkable = fluentapi.Checkable

// Violation is one disagreement between the code and a rule: the atom of every rule's result. It carries
// the thing that disagreed rather than a sentence about it, and Kind is what says which family it belongs
// to.
type Violation = assertion.Violation

// ViolationKind identifies a family of violations — `empty-test`, `file-cycle` — spelled the same way in
// every ArchUnit port. It is what a report groups and phrases by, without asserting on a concrete type
// first.
type ViolationKind = assertion.ViolationKind

// EmptyTestViolation says a rule selected nothing at all: its scope matched no file, so there was nothing
// to judge and a pass would have meant nothing. Every terminal reports it, and CheckOptions.AllowEmptyTests
// is how a user who really means an empty selection opts out.
type EmptyTestViolation = assertion.EmptyTestViolation

// FileCycleViolation says that some of the files a rule selected depend on each other in a circle. It is
// what `should have no cycles` reports, one per cycle, and it carries the cycle itself — printable as the
// readable path `a.go -> b.go -> a.go`.
type FileCycleViolation = filesassertion.CycleViolation

// Circuit is one cycle: the chain of dependencies that leaves a node, returns to it and touches nothing
// twice on the way. It is what a FileCycleViolation carries, and it renders itself as a readable path.
type Circuit = cycles.Circuit

// FileNamingViolation says that one file is not named, or not placed, the way a rule requires. It is what
// `should have name`, `should be in folder` and `should be in path` report, and their negations, one per
// offending file — carrying the file, the requirement as a compiled pattern with the part of the identifier
// it was matched against, and the mood the rule was written in.
type FileNamingViolation = filesassertion.NamingViolation

// FileInfo is one of the project's files as a user's own predicate sees it: its identifier, its name without
// the extension, that extension, its folder, its full source text and how many of its lines carry something.
// It is what the function passed to `should adhere to` is handed, one per selected file.
type FileInfo = filesextraction.FileInfo

// FilePredicate is the rule a user writes themselves: a question about one file, answered yes or no. It is the
// first argument of `adhere to`, and `should` requires it to answer yes about every selected file while
// `should not` forbids it from ever doing so.
type FilePredicate = filesassertion.FilePredicate

// FileAdherenceViolation says that one file does not satisfy the predicate a rule was given, or does satisfy it
// where the rule forbade it. It is what `should adhere to` and `should not adhere to` report, one per offending
// file — carrying the file, the requirement in the words the user wrote beside their function, and the mood.
type FileAdherenceViolation = filesassertion.AdherenceViolation

// FileDependencyViolation says that one file depends on the files a rule named where the rule forbids it, or
// on none of them where the rule requires it. It is what `should depend on files` and `should not depend on
// files` report, one per offending file — carrying the file, the object's selectors, the dependencies actually
// found and the mood the rule was written in.
type FileDependencyViolation = filesassertion.DependencyViolation

// FileExternalDependencyViolation says that one file depends on the external modules a rule named where the rule
// forbids it, or on none of them where the rule requires it. It is what `should depend on external modules` and
// `should not depend on external modules` report, one per offending file — carrying the file, the object's
// selectors, the import paths actually found and the mood the rule was written in.
type FileExternalDependencyViolation = filesassertion.ExternalDependencyViolation

// LayerDependencyViolation says that one layer of a policy depends on another layer the policy does not allow
// it to. It is what `may only depend on layers` and `may not depend on layers` report, one per pair of layers
// rather than one per import — carrying the two layers, the layers the broken clause named, the mood it was
// written in and the concrete file dependencies that connect them.
type LayerDependencyViolation = layersassertion.DependencyViolation

// SliceDependencyViolation says that one slice of a project depends on another and the rule does not allow it,
// or that the rule required that dependency and the project does not have it. It is what `should contain
// dependency` and `should not contain dependency` report, at most one per rule — carrying the two slices as the
// slicing named them, the mood the rule was written in and the concrete file dependencies that connect them,
// which a required dependency that is missing has none of.
type SliceDependencyViolation = slicesassertion.DependencyViolation

const (
	// KindEmptyTest is the kind of EmptyTestViolation.
	KindEmptyTest = assertion.KindEmptyTest
	// KindFileCycle is the kind of FileCycleViolation.
	KindFileCycle = filesassertion.KindFileCycle
	// KindFileNaming is the kind of FileNamingViolation.
	KindFileNaming = filesassertion.KindFileNaming
	// KindFileDependency is the kind of FileDependencyViolation.
	KindFileDependency = filesassertion.KindFileDependency
	// KindFileExternalDependency is the kind of FileExternalDependencyViolation.
	KindFileExternalDependency = filesassertion.KindFileExternalDependency
	// KindFileAdherence is the kind of FileAdherenceViolation.
	KindFileAdherence = filesassertion.KindFileAdherence
	// KindLayerDependency is the kind of LayerDependencyViolation.
	KindLayerDependency = layersassertion.KindLayerDependency
	// KindSliceDependency is the kind of SliceDependencyViolation.
	KindSliceDependency = slicesassertion.KindSliceDependency
)

// FilesBuilder is the scope stage of a rule about files, which ProjectFiles and Files return and every
// scope verb hands back a new one of. It is named here so that a half-built rule can be stored in a
// struct field or passed to a helper.
type FilesBuilder = filesapi.FilesBuilder

// FilesShouldBuilder is the positive mood of a rule about files, which FilesBuilder.Should returns:
// `project files, in folder "internal/api/**", should`.
type FilesShouldBuilder = filesapi.FilesShouldBuilder

// FilesShouldNotBuilder is the negated mood of a rule about files, which FilesBuilder.ShouldNot
// returns: `project files, in folder "internal/api/**", should not`. It is the positive builder's twin —
// same scope, same terminals, every predicate with a meaningful negation, one flag apart — with
// HaveNoCycles the one predicate offered on the positive mood alone, because its negation would have
// nothing to report.
type FilesShouldNotBuilder = filesapi.FilesShouldNotBuilder

// FilesCyclesCondition is the terminal of `project files, ..., should, have no cycles`, which
// FilesShouldBuilder.HaveNoCycles returns. It is a Checkable, so a built rule can be stored in a struct
// field, passed to a helper or kept in a list of the suite's rules.
type FilesCyclesCondition = filesapi.FilesCyclesCondition

// FilesNamingCondition is the terminal of the three self-contained rules about how a project's files are
// named and where they live — `have name`, `be in folder`, `be in path` — which HaveName, BeInFolder and
// BeInPath return on either mood. It is a Checkable, so a built rule can be stored, passed to a helper or
// kept in a list of the suite's rules.
type FilesNamingCondition = filesapi.FilesNamingCondition

// FilesDependencyCondition is the object stage and the terminal of `project files, ..., should not, depend on
// files, in folder "internal/db/**"`, which DependOnFiles returns on either mood. Its object verbs — WithName,
// InFolder, InPath — are chainable and combined with AND, and it is a Checkable, so a built rule can be
// stored, passed to a helper or kept in a list of the suite's rules.
type FilesDependencyCondition = filesapi.FilesDependencyCondition

// FilesExternalDependencyCondition is the object stage and the terminal of `project files, ..., should not, depend
// on external modules, matching "*.*/**"`, which DependOnExternalModules returns on either mood. Its one object
// verb — Matching — is repeatable and combined with OR, which is the one chain in this library that widens rather
// than narrows, and it is a Checkable, so a built rule can be stored, passed to a helper or kept in a list of the
// suite's rules.
type FilesExternalDependencyCondition = filesapi.FilesExternalDependencyCondition

// FilesAdherenceCondition is the terminal of `project files, ..., should, adhere to (a function), "be at most
// 400 lines long"`, which AdhereTo returns on either mood. It is a Checkable, so a built rule can be stored,
// passed to a helper or kept in a list of the suite's rules.
type FilesAdherenceCondition = filesapi.FilesAdherenceCondition

// LayersBuilder is the declaration stage of a named-layer policy, which ProjectLayers and Layers return and
// every `defined by` verb hands back a new one of. It is named here so that a project's layers can be
// declared once, stored in a struct field or a package-level helper, and branched into as many policies as a
// suite needs.
type LayersBuilder = layersapi.LayersBuilder

// LayerBuilder is a layer that has been named and not yet described: what Layer returns, and what DefinedBy
// and DefinedByFolder close. It is a stage of its own because a layer with no files is a policy that passes
// forever, so the pattern is asked for here rather than left optional.
type LayerBuilder = layersapi.LayerBuilder

// LayerPolicyBuilder is a clause that has named its layer and not yet said anything about it: what WhereLayer
// returns, and what MayOnlyDependOnLayers and MayNotDependOnLayers close. There is no mood stage in this
// family — the two predicates are the two moods.
type LayerPolicyBuilder = layersapi.LayerPolicyBuilder

// LayersPolicyCondition is the terminal of a named-layer policy — `project layers, layer "api" defined by
// folder ..., where layer "db", may not depend on layers "api"` — which both predicates return. It is also
// chainable, so a policy's further clauses follow straight on — every layer has to be declared before the
// first clause, because a chain that went back to declaring would leave Checkable — and it is a Checkable, so
// a whole N-layer policy can be stored, passed to a helper or kept in a list of the suite's rules as one rule.
type LayersPolicyCondition = layersapi.LayersPolicyCondition

// SlicesBuilder is the entry point and the slicing of a rule about slices, which ProjectSlices returns and
// both slicing verbs hand back a new one of. It is named here because the slicing is the half of the rule
// worth typing once: one of these can be stored in a struct field or a package-level helper and branched into
// as many rules as a suite needs.
type SlicesBuilder = slicesapi.SlicesBuilder

// SlicesShouldBuilder is the positive mood of a rule about slices, which SlicesBuilder.Should returns:
// `project slices, defined by "internal/(**)/**", should`.
type SlicesShouldBuilder = slicesapi.SlicesShouldBuilder

// SlicesShouldNotBuilder is the negated mood of a rule about slices, which SlicesBuilder.ShouldNot returns:
// `project slices, defined by "internal/(**)/**", should not`. It is the positive builder's twin — the same
// slicing, the same predicate, the same terminal, one flag apart — and it is the mood a rule about slices is
// nearly always written in.
type SlicesShouldNotBuilder = slicesapi.SlicesShouldNotBuilder

// SlicesDependencyCondition is the terminal of `project slices, defined by "internal/(**)/**", should not,
// contain dependency "api" -> "db"`, which ContainDependency returns on either mood. Both ends of the
// dependency are arguments of that one verb, so there is no object stage to chain, and it is a Checkable, so a
// built rule can be stored, passed to a helper or kept in a list of the suite's rules.
type SlicesDependencyCondition = slicesapi.SlicesDependencyCondition

// MapFunction is a projection: what one dependency of the graph becomes when the library relabels it, or
// nothing at all when the projection is not about it. It is what SliceByPattern and its siblings return, and it
// is named here so that a caller can hold one in a variable or a struct field.
type MapFunction = kernelprojection.MapFunction

// MetricsBuilder is the scope stage of a rule about the numbers a project's code adds up to, which Metrics
// returns and every scope verb — WithName, InFolder, InPath, ForClassesMatching — hands back a new one of. It
// is named here so that one scope can be stored and branched into a rule per metric.
type MetricsBuilder = metricsapi.MetricsBuilder

// MetricsCountBuilder is the stage between a metrics rule's scope and its number, which Count returns:
// `metrics, in folder "internal/**", count`, waiting for one of the eight counting verbs — LinesOfCode,
// Statements, Imports, Functions, Classes, Interfaces, MethodCount, FieldCount.
type MetricsCountBuilder = metricsapi.MetricsCountBuilder

// MetricBuilder is a metrics rule whose metric has been chosen — `metrics, ..., count, lines of code` — which
// each counting verb returns. Measure is what it hands back the numbers themselves with, one per file for a
// metric about a file and one per class for a metric about a class.
type MetricBuilder = metricsapi.MetricBuilder

// Measurement is one number a metric read off one subject: what was measured, the file or class it was
// measured about, and the answer. It is what MetricBuilder.Measure returns, one per subject.
type Measurement = metricscalculation.Measurement

// GraphBuilder is a dependency-graph report as far as it has been described — `project graph`, plus every
// modifier chained onto it — which ProjectGraph returns and every modifier hands back a new one of. It is
// named here because a query is the expensive half of a report to write: one described report can be stored
// and branched into as many focuses, collapses and output formats as a suite wants. It is the whole chain and
// all thirteen terminals in one type, because a report has no mood and no predicate to pass through: Snapshot
// for the report as data, ToDot and its five siblings for it as a string, ExportAsDot and its five siblings
// for it as a file.
type GraphBuilder = graphapi.GraphBuilder

// GraphSnapshot is what a described report hands back: the nodes that survived the query, the dependencies
// between them, the title and the counts. It is the seam the graph module hangs from — rendering is two
// steps, build a snapshot and then render it, so each of the six output formats is a function of this one
// value and a query option nobody has written a renderer for is still in all of them.
type GraphSnapshot = graphprojection.Snapshot

// GraphNode is one box of a report: the label it is drawn under — a file, a folder or a named group,
// whichever the query collapsed onto — and whether what it stands for is somebody else's code.
type GraphNode = graphprojection.Node

// GraphEdge is one arrow of a report: the two labels it runs between, how many of the project's dependencies
// were aggregated into it, whether it leaves the project and the union of the kinds of import behind it. The
// count is what keeps a collapsed diagram honest — merging arrows loses no dependency.
type GraphEdge = graphprojection.Edge

// GraphSummary is a report's counts, as the headline above a diagram states them: nodes, arrows, the
// dependencies those arrows stand for, and how many of each leave the project. It is derived from the
// snapshot it belongs to, so it cannot disagree with the diagram it is printed over.
type GraphSummary = graphprojection.Summary

// Result is a whole rule's outcome as a test needs it: whether the rule holds, and the one message to print
// when it does not. It is what ResultFactory shapes a rule's violations into, and the two fields an adapter
// to a test framework reads — an adapter prints a report, it never builds one.
type Result = archtest.Result

// ResultFactory turns the violations a rule reported into a Result: the count, the numbered list and the
// pass flag. It is where a suite goes to print a failure, and NewResultFactory is how one is built.
type ResultFactory = archtest.ResultFactory

// ViolationFactory phrases one violation — the file that disagreed with the rule, the requirement it broke
// and what was found instead — for a caller assembling a report of its own shape rather than the library's.
// NewViolationFactory is how one is built.
type ViolationFactory = archtest.ViolationFactory

// MessageOptions is the options bag a report is written with: which colors it is painted in and how many
// violations it lists. A nil *MessageOptions means the defaults — plain text, every violation.
type MessageOptions = archtest.MessageOptions

// TestingT is the part of a test framework's handle AssertPasses needs: Error, and nothing else. *testing.T
// satisfies it, and so does any other framework's handle that has the method every framework has — which is
// what makes the assert helper work without registration or configuration.
type TestingT = archtest.TestingT

// TestingRunner is the standard library's own handle as AssertAllPass needs it: Error, Helper and Run, the
// subtest. Only *testing.T satisfies it, deliberately — a suite is reported in the shape Go's testing package
// gives a result, and a framework whose handle has no subtests still has AssertPasses.
type TestingRunner = archtest.TestingRunner

// AssertOptions is the one options bag the assert helpers take: how the rule is run, and how a failure is
// written, as the two bags each half already has. A nil *AssertOptions means the defaults.
type AssertOptions = archtest.AssertOptions

// Palette is which color each part of a report is painted in, one field per role a piece of a message plays:
// the offender, the rule it broke, what was found instead. The zero Palette paints nothing, so color is
// something a caller opts into.
type Palette = archtest.Palette

// Color is a terminal color a report paints one part of a message in. It is a closed set of names rather
// than an escape sequence, so that nothing but the library ever writes one.
type Color = archtest.Color

const (
	// ColorNone paints nothing: the text is left exactly as it is. It is the zero value of a Color.
	ColorNone = archtest.ColorNone
	// ColorRed is the failure color of the default palette.
	ColorRed = archtest.ColorRed
	// ColorGreen is the pass color of the default palette.
	ColorGreen = archtest.ColorGreen
	// ColorYellow is the requirement color of the default palette.
	ColorYellow = archtest.ColorYellow
	// ColorBlue is for a palette of a caller's own; the default palette does not use it.
	ColorBlue = archtest.ColorBlue
	// ColorMagenta is for a palette of a caller's own; the default palette does not use it.
	ColorMagenta = archtest.ColorMagenta
	// ColorCyan is the subject color of the default palette.
	ColorCyan = archtest.ColorCyan
	// ColorGray is the hint color of the default palette.
	ColorGray = archtest.ColorGray
)

// ProjectFiles is the entry point of every rule about files: `project files`. The locator is optional
// and nil means auto-detect.
func ProjectFiles(locator *ProjectLocator) FilesBuilder {
	return filesapi.ProjectFiles(locator)
}

// Files is ProjectFiles under the shorter name the family also gives it. The two are one entry point.
func Files(locator *ProjectLocator) FilesBuilder {
	return filesapi.Files(locator)
}

// ProjectLayers is the entry point of every named-layer policy: `project layers`. The locator is optional
// and nil means auto-detect.
//
// A policy declares its layers and then says what they may depend on, and the whole of it is one rule:
//
//	rule := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("domain").DefinedByFolder("internal/domain/**").
//		Layer("db").DefinedByFolder("internal/db/**").
//		WhereLayer("api").MayOnlyDependOnLayers("domain", "db").
//		WhereLayer("domain").MayOnlyDependOnLayers()
//
// Dependencies inside a layer are always allowed, dependencies with an end in no declared layer are ignored,
// and `MayOnlyDependOnLayers()` with nothing named is the sealed layer.
func ProjectLayers(locator *ProjectLocator) LayersBuilder {
	return layersapi.ProjectLayers(locator)
}

// Layers is ProjectLayers under the shorter name the family also gives it. The two are one entry point.
func Layers(locator *ProjectLocator) LayersBuilder {
	return layersapi.Layers(locator)
}

// ProjectSlices is the entry point of every rule about slices: `project slices`. The locator is optional and
// nil means auto-detect.
//
// A slice is a name cut out of a file's identifier, and the capture in the slicing pattern is where that name
// comes from — so a rule says what the project's slices are and then what they may depend on:
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		ShouldNot().
//		ContainDependency("api", "db")
//
// Nothing declares the slices: `internal/(**)/**` says that this project's slices are its folders under
// internal, so `internal/api/handler.go` is in the slice `api`, and a file the pattern does not match is in no
// slice at all. That is the difference from a layer policy, where every layer is named before any file is read.
//
// There is no shorter alias for it, unlike ProjectFiles and ProjectLayers: `slices` alone is the name of a
// standard library package, and a chain starting with it would read as one.
func ProjectSlices(locator *ProjectLocator) SlicesBuilder {
	return slicesapi.ProjectSlices(locator)
}

// SliceByPattern is the slicing behind `defined by`, for a caller who wants the projection itself: the mapper
// that labels every dependency of the graph with the slices of its two ends, named by what the glob's one
// capture matched. `internal/(**)/**` puts `internal/api/handler.go` in the slice `api`.
//
// The error is a glob that will not compile, or one that does not capture exactly one name.
func SliceByPattern(glob string) (MapFunction, error) {
	return slicesprojection.SliceByPattern(glob)
}

// SliceByRegex is SliceByPattern with the pattern read as Go's own regexp syntax, and the projection behind
// `defined by regex`: `internal/([^/]+)/.*` is `internal/(**)/**` written in the substrate. The expression is
// anchored at both ends, as every pattern in this library is.
//
// The error is an expression that will not compile, or one that does not have exactly one capturing group.
func SliceByRegex(expression string) (MapFunction, error) {
	return slicesprojection.SliceByRegex(expression)
}

// SliceByFileSuffix is the slicing that names a file by what kind of file it is rather than by where it lives:
// the last `_`-separated word of its name, so `order_handler.go` and `user_handler.go` are both in the slice
// `handler` wherever in the project they sit. A name with no `_` in it is its own slice.
//
// It is the slicing for a rule about a kind of file — every handler, every repository, every store — and the
// one slicing that has no pattern to get wrong, so it takes no argument and returns no error.
func SliceByFileSuffix() MapFunction {
	return slicesprojection.SliceByFileSuffix()
}

// Identity is the projection that relabels nothing: every dependency of the graph under the identifiers it
// already carries, self-dependencies included. It is the mapper a report of a project's own files speaks, and
// the one to reach for when what is wanted is the graph as it was extracted.
func Identity() MapFunction {
	return kernelprojection.Identity()
}

// Metrics is the entry point of every rule about the numbers a project's code adds up to: `metrics`. The
// locator is optional and nil means auto-detect.
//
// A rule says where it looks, which number it is about, and — with the threshold predicates that judge one —
// what that number has to be:
//
//	measurements, err := archunit.Metrics(nil).
//		InFolder("internal/api/**").
//		Count().
//		LinesOfCode().
//		Measure(nil)
//
// The four scope verbs are chainable and combined with AND, three of them describing files and
// ForClassesMatching describing declared types. The family's own name for this entry point is `metrics` alone,
// so unlike the others it has no second spelling.
func Metrics(locator *ProjectLocator) MetricsBuilder {
	return metricsapi.Metrics(locator)
}

// ProjectGraph is the entry point of every dependency-graph report: `project graph`. The locator is optional
// and nil means auto-detect.
//
// A report is described the way a rule is, and then asked for its snapshot:
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		FocusOn("internal/api/**", 1).
//		CollapseToFolderDepth(2).
//		Titled("what the api layer touches").
//		Snapshot()
//
// The nine modifiers are all optional, chainable and order-independent — `including external dependencies`,
// `including self dependencies`, `focus on`, `reachable from`, `dependents of`, `collapse to folder depth`,
// `collapse by pattern`, `titled` and `with check options` — and each of them narrows what the diagram draws
// or says how it is labeled. The default report is one node per file of the project's own code.
//
// The same described report renders as a document instead, in any of six formats, as a string or as a file:
//
//	err := archunit.ProjectGraph(nil).
//		CollapseToFolderDepth(2).
//		Titled("the modules of this project").
//		ExportAsHTML("build/architecture.html")
func ProjectGraph(locator *ProjectLocator) GraphBuilder {
	return graphapi.ProjectGraph(locator)
}

// DependencyGraph is ProjectGraph under the other name the family also gives it. The two are one entry point.
func DependencyGraph(locator *ProjectLocator) GraphBuilder {
	return graphapi.DependencyGraph(locator)
}

// NewResultFactory returns the factory that shapes a rule's violations into a Result. A nil *MessageOptions
// means the defaults, so NewResultFactory(nil) is the ordinary call:
//
//	violations, err := rule.Check(nil)
//	...
//	if result := archunit.NewResultFactory(nil).Result(violations); !result.Passed {
//		t.Error(result.Message)
//	}
func NewResultFactory(options *MessageOptions) ResultFactory {
	return archtest.NewResultFactory(options)
}

// NewViolationFactory returns the factory that phrases one violation at a time, for a caller writing a report
// of its own shape. A nil *MessageOptions means the defaults.
func NewViolationFactory(options *MessageOptions) ViolationFactory {
	return archtest.NewViolationFactory(options)
}

// AssertPasses checks the rule and fails the test with the formatted violations when it does not hold. It is
// how an architecture rule becomes a unit test, and the last line of one:
//
//	func TestTheApiDoesNotTouchTheDatabase(t *testing.T) {
//		rule := archunit.ProjectFiles(nil).
//			InFolder("internal/api/**").
//			ShouldNot().
//			DependOnFiles().
//			InFolder("internal/db/**")
//
//		archunit.AssertPasses(t, rule, nil)
//	}
//
// A rule that holds reports nothing; one that does not is a single t.Error carrying the rule as it was written
// and then the numbered violations. A nil *AssertOptions means the defaults, which is why this needs no
// configuration to be useful, and AssertOptions is how a suite asks for color, a violation limit or a check
// that allows an empty selection.
//
// This frame marks itself as a helper too, for the same reason the one it delegates to does: the first unmarked
// frame on the stack is the one a framework blames, and a re-export that did not mark itself would move every
// failure from the user's own assertion line to this file — which is exactly the call form the documentation
// prescribes. The probe is optional, as it is one layer down, so a framework without Helper() still gets the
// report.
func AssertPasses(t TestingT, rule Checkable, options *AssertOptions) {
	if marked, ok := t.(interface{ Helper() }); ok {
		marked.Helper()
	}
	archtest.AssertPasses(t, rule, options)
}

// AssertAllPass asserts a whole suite of rules at once, each in its own named subtest. It is the path a suite
// of more than one rule should reach for, and it needs no more setup than the one for a single rule does:
//
//	func TestTheArchitectureHolds(t *testing.T) {
//		archunit.AssertAllPass(t, map[string]archunit.Checkable{
//			"the api does not touch the database": archunit.ProjectFiles(nil).
//				InFolder("internal/api/**").
//				ShouldNot().
//				DependOnFiles().
//				InFolder("internal/db/**"),
//			"no file depends on another in a circle": archunit.ProjectFiles(nil).Should().HaveNoCycles(),
//		}, nil)
//	}
//
// Every rule is asserted through AssertPasses inside its own t.Run, so a rule that does not hold fails the
// subtest its author named it after and the rules around it are asserted all the same. The rules run in the
// sorted order of their names, so a suite's output is the same on every run, and `go test -run` selects one
// rule of it by that name. A suite with no rules in it is a failure rather than a pass, for the same reason a
// rule that selected no file is.
//
// A nil *AssertOptions means the defaults, and the bag is the whole suite's — the one rule that needs knobs of
// its own is asserted beside the suite with its own AssertPasses call.
//
// This frame marks itself as a helper too, for the reason the single-rule re-export does: the first unmarked
// frame is the one a framework blames, and every frame between a subtest's failure and this call is marked, so
// what a failing rule is filed against is the line of the caller's own AssertAllPass.
func AssertAllPass(t TestingRunner, rules map[string]Checkable, options *AssertOptions) {
	t.Helper()
	archtest.AssertAllPass(t, rules, options)
}

// DefaultPalette is the palette a caller who wants a colored report and does not want to choose asks for: the
// failing count and what was found in red, a rule that holds in green, the offending file in cyan, the rule it
// broke in yellow and the explanatory notes in gray.
func DefaultPalette() Palette {
	return archtest.DefaultPalette()
}

// ClearGraphCache forgets every dependency graph a check extracted earlier in this process, so the next
// rule reads the source again.
//
// A test suite asks about one project many times and extraction is the expensive half of a check, so the
// graph is memoised by default. Call this when the source has changed underneath the library — a test
// that writes a fixture project, or generated code produced between two checks — or set ClearCache on
// the check options, which is the same thing scoped to one rule.
func ClearGraphCache() {
	extraction.ClearGraphCache()
}
