package fluentapi

import (
	"errors"
	"maps"
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	slicesassertion "github.com/LukasNiessen/ArchUnitGo/slices/assertion"
	slicesextraction "github.com/LukasNiessen/ArchUnitGo/slices/extraction"
)

// ErrMissingDiagramPath says `adhere to diagram in file` was given the empty string for a path. A diagram
// has to be somewhere, and the working directory is a folder rather than a file, so there is nothing this
// could have meant.
//
// It is reported as an archerror.UserError naming the predicate at fault, the way a pattern that will not
// compile is: the library is working and the code has not been judged, the rule simply cannot be run as
// written.
var ErrMissingDiagramPath = errors.New("no path to the diagram")

const (
	// adhereToDiagramVerb is the predicate that takes a diagram as a string, named once for the sentence it
	// renders as and the rejections it reports.
	adhereToDiagramVerb = "adhere to diagram"
	// adhereToDiagramInFileVerb is the predicate that takes a path to one.
	adhereToDiagramInFileVerb = "adhere to diagram in file"
)

// SlicesDiagramCondition is the predicate and the terminal of a rule that judges a project against the
// component diagram somebody drew of it — `project slices, defined by "internal/(**)/**", should, adhere to
// diagram in file "docs/architecture.puml"` — and it is a fluentapi.Checkable like every other rule:
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		Should().
//		AdhereToDiagramInFile("docs/architecture.puml")
//	violations, err := rule.Check(nil)
//
// It is what AdhereToDiagram and AdhereToDiagramInFile return, and it is the one predicate in the library
// whose rule is a whole architecture rather than one question: a diagram says which components there are and
// which of them may depend on which, so a check against one reports every place the code and the drawing
// disagree, in one run. That is what makes it worth drawing — `contain dependency` states one pair at a time,
// and forty of them are forty rules nobody keeps up to date.
//
// Two modifiers relax which disagreements are reported, IgnoringOrphanSlices and IgnoringExternalSlices, and
// they are chainable in either order. Neither of them can switch off a dependency the diagram does not draw,
// because that is the finding a diagram is drawn for.
//
// It carries the slicing it was asked of unchanged, and it is immutable like every stage before it. Nothing
// has been read when it is built — with one exception that is not a filesystem access: a diagram given as a
// string is parsed as the rule is built, so that a text that is not a diagram is a rejection the user sees
// where a pattern that will not compile is seen. A diagram given as a path is read by Check, like the project
// itself.
type SlicesDiagramCondition struct {
	// rule is the slicing and the mood the predicate was asked of. The mood is always assertion.Should:
	// AdhereToDiagram is offered on the positive mood alone.
	rule slicesRule
	// diagram is the drawing the project is judged against, parsed when the rule was built. It is the zero
	// Diagram when the rule names a file instead, because that one is read by the terminal.
	diagram slicesextraction.Diagram
	// path is where the diagram is, for the rule that names a file, and the empty string for the rule that was
	// given the text itself. It is what tells the two forms apart, in this rule's sentence and in its terminal.
	path string
	// options are the two modifiers, in the shape the assertion takes them: which of the three findings this
	// rule reports. Keeping the assertion's own type here is what stops the two from drifting apart — a
	// modifier added to one is a modifier missing from the other, and the compiler says so.
	options slicesassertion.DiagramOptions
}

// AdhereToDiagram is the predicate over a diagram the rule carries as text: `should adhere to diagram`.
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		Should().
//		AdhereToDiagram(`
//			@startuml
//			component [api]
//			component [domain]
//			component [db]
//			[api] --> [domain]
//			[api] --> [db]
//			@enduml
//		`)
//
// The text is the component-diagram subset of PlantUML, and extraction.ParseDiagram documents the whole
// dialect: declarations, arrows, comments and the frame. Every arrow is a permission — `[api] --> [db]` says
// the api may depend on the db and says nothing about the other direction — so a project adheres to a diagram
// when every dependency it has is an arrow somebody drew, every slice it has is a component of the drawing,
// and every component of the drawing is a slice it has.
//
// This is the form for a diagram that belongs to one test: it is in the test that asserts on it, so it cannot
// go stale in another file. AdhereToDiagramInFile is the form for the drawing that is an artifact of its own.
//
// A text that is not a diagram is rejected here and reported by the terminal as a UserError naming this
// predicate — a line the dialect does not have, or a text with no component in it at all. That is the same
// treatment a pattern that will not compile gets, and for the same reason: there is no rule to run, so there
// is nothing to judge the code with.
//
// There is no negated mood. A diagram is a closed statement about a whole project, so `should not adhere to
// diagram` would be a rule asking that a project contradict its own documentation somewhere — which is why
// this predicate is offered on the positive mood alone, as `have no cycles` is.
func (b SlicesShouldBuilder) AdhereToDiagram(text string) SlicesDiagramCondition {
	diagram, err := slicesextraction.ParseDiagram(text)
	if err != nil {
		return SlicesDiagramCondition{rule: b.rejectedRule(adhereToDiagramVerb, "", err)}
	}
	return SlicesDiagramCondition{rule: b.rule, diagram: diagram}
}

// AdhereToDiagramInFile is the predicate over a diagram the rule names by its path: `should adhere to diagram
// in file`.
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		Should().
//		AdhereToDiagramInFile("docs/architecture.puml")
//
// It is AdhereToDiagram over the contents of a file, and what a project adheres to is documented there. This
// is the form for the drawing that is an artifact: one file, beside the code, reviewed when it changes, drawn
// by a tool for whoever does not read PlantUML — and checked by this rule, so that it is the architecture
// rather than a picture of what it used to be. ExportAsPlantUML writes the first version of such a file out
// of the project as it is now.
//
// The path is interpreted like any other path a test reads, relative to the working directory the test runs
// in. `.puml` is what such a file is conventionally called, and this library neither requires nor checks the
// extension.
//
// Nothing is read here: the file is read by Check, with the project, because a rule is a value and reading one
// at build time would make storing a rule in a variable an action. The empty path is rejected here as
// ErrMissingDiagramPath, a file that cannot be read is a technical error naming it, and a file that is not a
// diagram is the same rejection AdhereToDiagram reports for a text that is not one — under this predicate's
// own name.
func (b SlicesShouldBuilder) AdhereToDiagramInFile(path string) SlicesDiagramCondition {
	if path == "" {
		return SlicesDiagramCondition{rule: b.rejectedRule(adhereToDiagramInFileVerb, path, ErrMissingDiagramPath)}
	}
	return SlicesDiagramCondition{rule: b.rule, path: path}
}

// IgnoringOrphanSlices leaves the slices no dependency reaches out of the report: `ignoring orphan slices`.
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		Should().
//		AdhereToDiagramInFile("docs/architecture.puml").
//		IgnoringOrphanSlices()
//
// A slice that imports nothing in the project and that nothing in the project imports says nothing about what
// may depend on what, so an architect drawing the architecture rather than the folder tree may reasonably
// leave it out. Without this modifier such a slice is reported as one the diagram does not declare, which is
// the strict reading and the default.
//
// It leaves out nothing else. A slice that is an end of an arrow and is missing from the drawing is a hole in
// it, and is reported however this rule is modified.
func (c SlicesDiagramCondition) IgnoringOrphanSlices() SlicesDiagramCondition {
	ignoring := c
	ignoring.options.IgnoreOrphanSlices = true
	return ignoring
}

// IgnoringExternalSlices leaves the components this project has no slice for out of the report: `ignoring
// external slices`.
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		Should().
//		AdhereToDiagramInFile("docs/system.puml").
//		IgnoringExternalSlices()
//
// It is the modifier for the drawing that is about more than this one project — a system of several modules,
// checked against each of them in turn — where a component this project has no slice for is not a folder that
// went missing but somebody else's. Without it, such a drawing reports every component of every sibling.
//
// The two modifiers are independent and chainable in either order: each of them switches off one of the two
// findings about a name, and neither touches the dependencies.
func (c SlicesDiagramCondition) IgnoringExternalSlices() SlicesDiagramCondition {
	ignoring := c
	ignoring.options.IgnoreExternalSlices = true
	return ignoring
}

// Check runs the rule: one violation per place the project and the diagram disagree, and an empty result when
// the drawing and the code say the same thing, which is the pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline — locate and extract the project, cut a slice name out of every file's identifier,
// read the diagram, project the dependencies between the slices, judge them against the drawing — and the
// only stage of the chain that reads anything.
//
// The violations are the slices module's own assertion.DiagramViolation values, each carrying which of the
// three disagreements it is, the names it is about and, for a dependency the diagram does not draw, the files
// that made it. A slicing that found no slice at all is reported as an EmptyTestViolation instead: a stale
// pattern would otherwise be a project whose every component is missing from the drawing, which is a report
// about the pattern told as a report about the architecture.
//
// It runs under kernel.CheckOptions.LoggedCheck, so a check that was asked for a log writes the rule, the count
// each of those steps came to, every violation and the outcome. With no log asked for, which is the default,
// nothing is written and nothing else about the check changes.
//
// The error is technical or the user's — a pattern with no capture in it, a chain with no slicing, a text or a
// file that is not a diagram, a diagram file that cannot be read, a project that will not load, a log this check
// was asked for that could not be opened, written or closed — and never a failing rule. When it is non-nil the
// violations say nothing.
func (c SlicesDiagramCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	return options.LoggedCheck(c, func(log *logging.Logger) ([]assertion.Violation, error) {
		graph, membership, err := c.rule.scope.resolve(options)
		if err != nil {
			return nil, err
		}
		log.LogProgress("slices the slicing found", len(membership))

		diagram, err := c.drawing()
		if err != nil {
			return nil, err
		}
		log.LogProgress("components the diagram declares", len(diagram.Components()))

		if empty := options.GatherEmptyTestViolations(c.rule.selection(len(membership))); len(empty) > 0 {
			// A slicing that found nothing is reported instead of being judged: every component of the diagram
			// would be one this project has no slice for, and a reader would go looking for deleted folders
			// instead of for the pattern that stopped matching.
			return empty, nil
		}

		dependencies := kernelprojection.ProjectEdges(graph, c.rule.scope.mapper())
		log.LogProgress("dependencies between the slices", len(dependencies))

		present := slices.Sorted(maps.Keys(membership))
		return slicesassertion.GatherDiagramViolations(diagram, dependencies, present, c.options), nil
	})
}

// String renders the whole rule as the sentence the user typed, as `project slices, path matches
// "internal/(**)/**", should, adhere to diagram in file "docs/architecture.puml", ignoring orphan slices`.
//
// A diagram given as text renders as its two sizes rather than as itself — `adhere to diagram (3 components, 2
// dependencies)` — because a rule renders as one line and a whole document inside one would bury it.
func (c SlicesDiagramCondition) String() string {
	return c.rule.render(c.stages()...)
}

// stages are the parts of the sentence this predicate adds, ready for slicesRule.render: the predicate, then
// the modifiers.
//
// The modifiers render in one order however they were chained, because they are a pair of flags rather than
// steps — the same shape GraphBuilder.stages gives its own modifiers, and what makes two rules that were typed
// differently and mean the same thing read the same way.
func (c SlicesDiagramCondition) stages() []string {
	drawing := adhereToDiagramVerb + " (" + c.diagram.String() + ")"
	if c.path != "" {
		drawing = adhereToDiagramInFileVerb + ` "` + c.path + `"`
	}

	stages := make([]string, 0, 3)
	stages = append(stages, drawing)
	if c.options.IgnoreOrphanSlices {
		stages = append(stages, "ignoring orphan slices")
	}
	if c.options.IgnoreExternalSlices {
		stages = append(stages, "ignoring external slices")
	}
	return stages
}

// drawing is the diagram this rule judges against, at the moment it is needed: the one parsed when the rule
// was built, or the one in the file the rule names, read now.
//
// The two error types are what tells a failure's blame apart here. A file that cannot be read is the
// environment's fault and travels unchanged, because there is nothing in the rule to fix; anything else is
// something the user wrote, so it is wrapped as a UserError naming this predicate — which is the same report
// AdhereToDiagram makes for a text that is not a diagram, so a diagram in a file and the same diagram in a
// string fail identically.
func (c SlicesDiagramCondition) drawing() (slicesextraction.Diagram, error) {
	if c.path == "" {
		return c.diagram, nil
	}

	diagram, err := slicesextraction.ExtractDiagram(c.path)
	if err == nil {
		return diagram, nil
	}
	var technical *archerror.TechnicalError
	if errors.As(err, &technical) {
		return slicesextraction.Diagram{}, err
	}
	return slicesextraction.Diagram{}, archerror.NewUserError(adhereToDiagramInFileVerb, c.path, err)
}

// rejectedRule is this rule with something the user typed rejected on the way: the diagram that is not one, or
// the path that is not a path.
//
// It is the shape newDependencyCondition rejects a nameless slice in, written as a method of the mood because
// both predicates that need it are asked of the mood — and the rejection reaches the terminal through the
// slicing, where the first rejection of a chain already wins.
func (b SlicesShouldBuilder) rejectedRule(verb, subject string, cause error) slicesRule {
	rejected := b.rule
	rejected.scope = b.rule.scope.rejecting(verb, subject, cause)
	return rejected
}
