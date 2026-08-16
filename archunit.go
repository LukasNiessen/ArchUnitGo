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
// Every selector in the library takes an exclusion, and it is spelled `except` in all four families: `in
// folder "app/**", except "**/generated"` is one clause, so the rule a team means — everything under a folder
// but not the generated part of it — is written the way they say it rather than inverted into a rule about the
// generated folder. An exclusion qualifies the selector it follows and nothing else, its patterns are read
// against the same part of an identifier as that selector unless a targeted form such as `except with name`
// says otherwise, and it is repeatable. What each family spells is in its own except.go: four forms of the
// verb in the files module, five in metrics, three in layers, and the plain one in the graph module, where every
// pattern is matched against the whole identifier already.
//
// Not every chain describes a rule. ProjectGraph describes a report, so it has no mood, no predicate and no
// violations: its terminals hand back the diagram — as data, as a document in one of six formats, or as a file
// written to disk — rather than a list of what the code disagreed with. Everything else about it is the same —
// a value, chainable modifiers, the same optional locator. A metrics chain can end in a report too: Count and
// Distance each close with ExportAsHTML, and NewMetricsExporter writes the same page from numbers a caller has
// already measured. A slicing has such a terminal before its mood too: ToPlantUML and ExportAsPlantUML draw the
// project's slices as the component diagram `should adhere to diagram` judges one against.
package archunit

import (
	"github.com/LukasNiessen/ArchUnitGo/archtest"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	filesextraction "github.com/LukasNiessen/ArchUnitGo/files/extraction"
	filesapi "github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
	graphapi "github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
	graphprojection "github.com/LukasNiessen/ArchUnitGo/graph/projection"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
	layersapi "github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
	metricsassertion "github.com/LukasNiessen/ArchUnitGo/metrics/assertion"
	metricscalculation "github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	metricsapi "github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
	metricsrendering "github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
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

// LogOptions is the log one check writes while it runs: where it goes, how much of it is written, and — when
// a build wants to keep it — the file it is archived as. It is what CheckOptions.Logging holds, and it is the
// whole of how logging is turned on.
//
//	violations, err := rule.Check(&archunit.CheckOptions{
//		Logging: &archunit.LogOptions{Writer: os.Stderr, Level: archunit.LogLevelDebug},
//	})
//
// Logging is off by default and there is no way to turn it on globally: the destination is injected per
// check, a nil *LogOptions logs nothing, and a bag with neither a writer nor a file in it logs nothing
// either. That is what lets one test assert on a log while the rest of the suite runs beside it.
type LogOptions = logging.Options

// LogLevel is how much of what a check has to say reaches the log. A record is written when its own level is
// at or above the level the bag holds, so the bag's level names the quietest thing worth seeing.
type LogLevel = logging.Level

const (
	// LogLevelDebug adds the step-by-step: one record per stage of a check, saying what it resolved and how
	// much of the project that came to. It is the level to ask for when a rule reports something surprising.
	LogLevelDebug = logging.LevelDebug
	// LogLevelInfo is what a check says about itself when it is working — the rule, its outcome and the
	// numbers a metrics rule measured — and it is the default, because every default in this library is a
	// zero value.
	LogLevelInfo = logging.LevelInfo
	// LogLevelWarn is violations alone: the log of a suite that should be reporting nothing at all.
	LogLevelWarn = logging.LevelWarn
	// LogLevelError is the checks that could not be run at all. The failure itself still travels as the error
	// Check returns; a log line is never how this library reports something.
	LogLevelError = logging.LevelError
)

// Logger is an open log, and the five records a check writes to it: start check, end check, log progress, log
// violation, log metric. CheckOptions.Logger is how a caller assembling something of its own opens one, and
// closing it is that caller's job because a log file is a file.
//
// Every rule this library offers logs through it already, so a user writing rules never has to touch one.
type Logger = logging.Logger

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

// MetricsZoneViolation says that one package sits in one of the two corners of the abstractness and instability
// plane a rule forbade — the zone of pain, concrete and depended upon, or the zone of uselessness, abstract and
// used by nobody. It is what `should not be in zone of pain` and `should not be in zone of uselessness` report,
// one per offending package, carrying the package, the zone in the words the rule was written in, the two
// coordinates it was judged by and the mood.
type MetricsZoneViolation = metricsassertion.ZoneViolation

// MetricsThresholdViolation says that one number a metric read is not on the side of the figure a rule held it
// to, or is on it where the rule forbade that. It is what the five comparing predicates — `should be below`,
// `should be above`, `should be`, `should be below or equal`, `should be above or equal` — report, one per
// offending measurement, carrying the subject the number was read off, the metric's name, the number itself, the
// comparison in the words the rule was written in, the figure and the mood.
type MetricsThresholdViolation = metricsassertion.ThresholdViolation

// MetricsSatisfactionViolation says that one number a metric read does not satisfy the predicate a rule was
// given, or does satisfy it where the rule forbade it. It is what `should satisfy` reports, one per offending
// measurement — carrying the subject the number was read off, the metric's name, the number itself, the
// requirement in the words the user wrote beside their function, and the mood.
type MetricsSatisfactionViolation = metricsassertion.SatisfactionViolation

// SliceDependencyViolation says that one slice of a project depends on another and the rule does not allow it,
// or that the rule required that dependency and the project does not have it. It is what `should contain
// dependency` and `should not contain dependency` report, at most one per rule — carrying the two slices as the
// slicing named them, the mood the rule was written in and the concrete file dependencies that connect them,
// which a required dependency that is missing has none of.
type SliceDependencyViolation = slicesassertion.DependencyViolation

// SliceDiagramViolation says that a project and the component diagram somebody drew of it disagree in one
// place. It is what `should adhere to diagram` and `should adhere to diagram in file` report, one per
// disagreement rather than one per rule — carrying which of the three findings it is, the names it is about
// and, for a dependency the diagram does not draw, the concrete file dependencies that made it.
type SliceDiagramViolation = slicesassertion.DiagramViolation

// SliceDiagramFinding is which of the three ways a project and a diagram of it can disagree a
// SliceDiagramViolation reports, and the field a report reads before the others: the rest of the violation
// means what the finding says it means.
type SliceDiagramFinding = slicesassertion.DiagramFinding

const (
	// FindingUndrawnDependency is a dependency the project has and the diagram does not draw. It is the
	// finding a diagram is drawn for, and the only one no modifier switches off.
	FindingUndrawnDependency = slicesassertion.FindingUndrawnDependency
	// FindingUndeclaredSlice is a slice the project has and the diagram does not declare, which `ignoring
	// orphan slices` leaves out for the slices no dependency reaches.
	FindingUndeclaredSlice = slicesassertion.FindingUndeclaredSlice
	// FindingAbsentComponent is a component the diagram declares and the project has no slice for, which
	// `ignoring external slices` leaves out.
	FindingAbsentComponent = slicesassertion.FindingAbsentComponent
)

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
	// KindMetricsZone is the kind of MetricsZoneViolation.
	KindMetricsZone = metricsassertion.KindMetricsZone
	// KindMetricsThreshold is the kind of MetricsThresholdViolation.
	KindMetricsThreshold = metricsassertion.KindMetricsThreshold
	// KindMetricsSatisfaction is the kind of MetricsSatisfactionViolation.
	KindMetricsSatisfaction = metricsassertion.KindMetricsSatisfaction
	// KindSliceDependency is the kind of SliceDependencyViolation.
	KindSliceDependency = slicesassertion.KindSliceDependency
	// KindSliceDiagram is the kind of SliceDiagramViolation.
	KindSliceDiagram = slicesassertion.KindSliceDiagram
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
// slicing, the same predicate wherever a negation means anything, one flag apart — and it is the mood a rule
// about slices is nearly always written in. `adhere to diagram` is offered on SlicesShouldBuilder alone,
// because a diagram is a closed statement about a whole project and its negation would ask that a project
// contradict its own documentation somewhere.
type SlicesShouldNotBuilder = slicesapi.SlicesShouldNotBuilder

// SlicesDependencyCondition is the terminal of `project slices, defined by "internal/(**)/**", should not,
// contain dependency "api" -> "db"`, which ContainDependency returns on either mood. Both ends of the
// dependency are arguments of that one verb, so there is no object stage to chain, and it is a Checkable, so a
// built rule can be stored, passed to a helper or kept in a list of the suite's rules.
type SlicesDependencyCondition = slicesapi.SlicesDependencyCondition

// SlicesDiagramCondition is the predicate and the terminal of `project slices, defined by "internal/(**)/**",
// should, adhere to diagram in file "docs/architecture.puml"`, which AdhereToDiagram and AdhereToDiagramInFile
// return on the positive mood. Its two modifiers — IgnoringOrphanSlices, IgnoringExternalSlices — are chainable
// in either order, and it is a Checkable, so a whole architecture's worth of rule can be stored, passed to a
// helper or kept in a list of the suite's rules as one rule.
type SlicesDiagramCondition = slicesapi.SlicesDiagramCondition

// MapFunction is a projection: what one dependency of the graph becomes when the library relabels it, or
// nothing at all when the projection is not about it. It is what SliceByPattern and its siblings return, and it
// is named here so that a caller can hold one in a variable or a struct field.
type MapFunction = kernelprojection.MapFunction

// MetricsBuilder is the scope stage of a rule about the numbers a project's code adds up to, which Metrics
// returns and every scope verb — WithName, InFolder, InPath, ForClassesMatching — hands back a new one of. It
// is named here so that one scope can be stored and branched into a rule per metric. Count and Distance open
// the two groups of metrics the library names, and CustomMetric is the one a user defines themselves.
type MetricsBuilder = metricsapi.MetricsBuilder

// MetricsCountBuilder is the stage between a metrics rule's scope and its number, which Count returns:
// `metrics, in folder "internal/**", count`, waiting for one of the eight counting verbs — LinesOfCode,
// Statements, Imports, Functions, Classes, Interfaces, MethodCount, FieldCount. ExportAsHTML is the one
// terminal of the group itself: it writes all eight counts of the scope to a file as one report.
type MetricsCountBuilder = metricsapi.MetricsCountBuilder

// MetricsDistanceBuilder is the stage between a metrics rule's scope and its number for the metrics about a
// package rather than a file, which Distance returns: `metrics, in folder "internal/**", distance`, waiting for
// one of the five verbs — Abstractness, Instability, DistanceFromMainSequence, NormalizedDistance,
// CouplingFactor — or for one of the two zone checks, which are rules rather than numbers, or for ExportAsHTML,
// which writes all five numbers of the scope to a file as one report.
type MetricsDistanceBuilder = metricsapi.MetricsDistanceBuilder

// MetricBuilder is a metrics rule whose metric has been chosen — `metrics, ..., count, lines of code` — which
// each counting verb, each distance verb and CustomMetric return. Measure is what it hands back the numbers
// themselves with, one per file for a metric about a file, one per class for a metric about a class and one per
// folder for a metric about a package, and the six threshold predicates are what judges them: ShouldBeBelow,
// ShouldBeAbove, ShouldBe, ShouldBeBelowOrEqual and ShouldBeAboveOrEqual hold every number to a figure, and
// ShouldSatisfy to a comparison the user writes. Those six are the whole of the family's grammar and no synonym
// joins them.
type MetricBuilder = metricsapi.MetricBuilder

// MetricsThresholdCondition is the terminal of a metrics rule that holds its numbers to a figure — `metrics,
// ..., count, lines of code, should be below 400` — which the five comparing predicates return: ShouldBeBelow,
// ShouldBeAbove, ShouldBe, ShouldBeBelowOrEqual and ShouldBeAboveOrEqual. One terminal serves all five, because
// what differs between them is the comparison and not the rule. There is no mood stage: each of the five spells
// its own mood, as all six threshold predicates do. It is a Checkable, so a built rule can be stored, passed to a
// helper or kept in a list of the suite's rules.
type MetricsThresholdCondition = metricsapi.MetricsThresholdCondition

// MetricsZoneCondition is the terminal of the two rules about where a package sits in the abstractness and
// instability plane — `metrics, ..., distance, should not be in zone of pain`, and the same for the zone of
// uselessness — which ShouldNotBeInZoneOfPain and ShouldNotBeInZoneOfUselessness return. There is no mood stage
// in this pair, as in the layers family: the corner a rule names and the mood it names it in are one verb. It is
// a Checkable, so a built rule can be stored, passed to a helper or kept in a list of the suite's rules.
type MetricsZoneCondition = metricsapi.MetricsZoneCondition

// MetricsSatisfactionCondition is the terminal of the metrics rule whose comparison the user writes themselves
// — `metrics, ..., count, method count, should satisfy "be at most 10 methods wide"` — which
// MetricBuilder.ShouldSatisfy returns, and it is the sixth of the six threshold predicates beside
// MetricsThresholdCondition's five. There is no mood stage: `should satisfy` spells its own mood, as all six
// threshold predicates do. It is a Checkable, so a built rule can be stored, passed to a helper or kept in a
// list of the suite's rules.
type MetricsSatisfactionCondition = metricsapi.MetricsSatisfactionCondition

// Measurement is one number a metric read off one subject: what was measured, the file, class or folder it was
// measured about, and the answer. It is what MetricBuilder.Measure returns, one per subject, and the first
// argument of the predicate `should satisfy` takes.
type Measurement = metricscalculation.Measurement

// MetricsClassInfo is one of the project's declared types as a user's own function sees it: its name, its
// identifier, the file it was declared in, whether it is an interface, how many fields and methods it has, and
// which of its fields each of its methods reaches. It is what the function passed to `custom metric` is handed,
// one per selected class, and the second argument of the predicate `should satisfy` takes — the zero value
// there for a number that was read off a file or a package rather than a class.
type MetricsClassInfo = metricsextraction.ClassInfo

// MetricsFieldInfo is one field of a MetricsClassInfo, and which of that class's methods reach it.
type MetricsFieldInfo = metricsextraction.FieldInfo

// MetricsMethodInfo is one method of a MetricsClassInfo, and which of that class's fields it reaches.
type MetricsMethodInfo = metricsextraction.MethodInfo

// MetricsClassMeasure is the metric a user writes themselves: one number read off one class. It is the third
// argument of `custom metric`, which asks it once about every selected class.
type MetricsClassMeasure = metricscalculation.ClassMeasure

// MetricsSatisfaction is the comparison a user writes themselves: one question about one measurement, answered
// yes or no. It is the first argument of `should satisfy`, which requires it to answer yes about every number
// the rule measured.
type MetricsSatisfaction = metricsassertion.Satisfaction

// MetricsExporter writes a metrics report to a file: measurements somebody has already taken, rendered as one
// self-contained HTML page and put where they asked for it. NewMetricsExporter is how one is built.
//
// It is the metrics family's report terminal for numbers that did not come from one rule — grouped per folder,
// per release, per metric of the caller's own — where MetricsCountBuilder.ExportAsHTML and
// MetricsDistanceBuilder.ExportAsHTML are the shorthand for a report of one scope.
type MetricsExporter = metricsapi.MetricsExporter

// MetricsReportData is what a metrics report is written from: the measurements, grouped under the heading each
// group of them is listed under. A report written off a rule has one group per metric, and a caller assembling
// their own groups them however they mean them to be compared.
type MetricsReportData = metricsrendering.ReportData

// MetricsReportOptions is what an exported metrics report says about itself: the title it is headlined with, the
// timestamp it carries — the zero time, the default, means none, because this library reads no clock of its own
// — and a stylesheet added after the library's. A nil *MetricsReportOptions means the defaults.
type MetricsReportOptions = metricsrendering.ReportOptions

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
//
// The scope verbs and the object verbs both take `except`, and both have the three targeted forms of it —
// `except with name`, `except in folder`, `except in path` — so a rule carves the hole where it is meant:
// `in folder "app/**", except "**/generated"` is a rule about less code, and `should not depend on files, in
// folder "internal/db/**", except "internal/db/dto/**"` is a boundary with one documented door in it.
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
//
// A declaration takes `except`, and `except in folder` and `except in path` are the same verb with the target
// said out loud: `Layer("api").DefinedByFolder("internal/api/**").Except("**/generated")` is the layer a folder
// is with one package taken back out of it. A file excluded that way is in no layer, so every dependency it is
// an end of is ignored.
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
// The other rule a slicing can be asked for is the whole architecture at once, against the component diagram
// somebody drew of it — and the reverse of it, which draws the diagram out of the project as it is:
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		Should().
//		AdhereToDiagramInFile("docs/architecture.puml")
//	err := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").ExportAsPlantUML("docs/architecture.puml", nil)
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
//	violations, err := archunit.Metrics(nil).
//		InFolder("internal/api/**").
//		Count().
//		LinesOfCode().
//		ShouldBeBelow(400).
//		Check(nil)
//
// There are exactly six threshold predicates, and each of them spells its own mood, so this family has no mood
// stage. Five hold every number a rule measured to a figure — `should be below`, `should be above`, `should be`,
// `should be below or equal`, `should be above or equal` — and the sixth, `should satisfy`, holds it to a
// comparison the user writes. There is no seventh: `should equal`, `should be at most` and every other synonym of
// one of the five are deliberately absent, because two spellings of one comparison mean every reader of a suite
// has to learn which of them the author picked.
//
// The four scope verbs are chainable and combined with AND, three of them describing files and
// ForClassesMatching describing declared types, and each of them takes `except` — plus the four targeted forms
// of it, one per scope verb, of which ExceptClassesMatching is the class population's own. An exclusion is
// about the same population as the verb it qualifies, which is a rule only this family has to state. The
// family's own name for this entry point is `metrics` alone, so unlike the others it has no second spelling.
//
// Which numbers there are to ask for is decided by the group the scope is followed by: `count` for the eight
// metrics about a file or a class, `distance` for the five about a package — abstractness, instability, distance
// from the main sequence, its normalised twin, and the coupling factor. The distance group also holds the two
// rules this family judges without a threshold, where the corner and the mood are one verb:
//
//	violations, err := archunit.Metrics(nil).
//		Distance().
//		ShouldNotBeInZoneOfPain().
//		Check(nil)
//
// Either group also closes without naming a metric at all, and then the chain is a report rather than a rule:
// `export as html` measures every number of the group over the one scope and writes them to a file as one
// self-contained page, which is the form to reach for when the numbers are for a person rather than a threshold.
//
//	err := archunit.Metrics(nil).InFolder("internal/**").Count().ExportAsHTML("build/metrics.html", nil)
//
// NewMetricsExporter is the same page for measurements a caller has already taken and grouped their own way, and
// the way to a title, a timestamp and a stylesheet of their own.
//
// CustomMetric is the third thing a scope can be followed by, and the family's escape hatch: a name, the words
// saying what the number means, and the user's own function for reading it off one class. It is a metric like
// any other, so the same Measure and the same threshold predicates follow it — and ShouldSatisfy is the
// predicate for the comparisons no threshold expresses, holding every number a rule measured to a function the
// user writes:
//
//	violations, err := archunit.Metrics(nil).
//		ForClassesMatching("*Service").
//		CustomMetric("public surface", "how many methods and fields a type exposes",
//			func(class archunit.MetricsClassInfo) float64 {
//				return float64(class.MethodCount + class.FieldCount)
//			}).
//		ShouldSatisfy(func(measurement archunit.Measurement, class archunit.MetricsClassInfo) bool {
//			return measurement.Value <= 20 || class.Interface
//		}, "expose at most 20 methods and fields unless it is an interface").
//		Check(nil)
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
// The four that take a pattern each take `except`, which qualifies the one the chain wrote most recently: it is
// how a diagram leaves out the generated packages, and how `collapse by pattern "third party" "**"` keeps one
// module out of its catch-all group.
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

// NewMetricsExporter returns the exporter that writes a metrics report to a file. A nil *MetricsReportOptions
// means the defaults — an untitled, unstamped, plainly styled page:
//
//	err := archunit.NewMetricsExporter(&archunit.MetricsReportOptions{Title: "the numbers of this project"}).
//		ExportAsHTML(archunit.MetricsReportData{"lines of code": measurements}, "build/metrics.html")
//
// The timestamp on the options bag is the caller's own time.Now(), because a page that stamped itself would
// render different bytes on every run — a report committed beside the code would then show up in every diff.
func NewMetricsExporter(options *MetricsReportOptions) MetricsExporter {
	return metricsapi.NewMetricsExporter(options)
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
