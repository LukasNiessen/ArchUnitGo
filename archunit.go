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
// The chain starts at an entry point — ProjectFiles and ProjectLayers today, one per rule family as they
// land — and every entry point takes an optional *ProjectLocator, where nil means the project the test
// itself is in.
package archunit

import (
	"github.com/LukasNiessen/ArchUnitGo/archtest"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	filesextraction "github.com/LukasNiessen/ArchUnitGo/files/extraction"
	filesapi "github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
	layersapi "github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
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
