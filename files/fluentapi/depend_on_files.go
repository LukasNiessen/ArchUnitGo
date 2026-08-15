package fluentapi

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

// FilesDependencyCondition is the object stage and the terminal of a rule about what the files a scope
// selected may depend on — `project files, in folder "internal/api/**", should not, depend on files, in
// folder "internal/db/**"` — and it is a fluentapi.Checkable, which is the one thing every consumer of a
// rule programs against:
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/api/**").
//		ShouldNot().
//		DependOnFiles().
//		InFolder("internal/db/**")
//	violations, err := rule.Check(nil)
//
// It is what DependOnFiles returns on either mood, and it is the one stage of the grammar that is both an
// object builder and a terminal: the object verbs are chainable, so each of them has to hand back something
// a further verb can be chained onto, and `depend on files` is a sentence the moment its object is named.
// Two types — one to chain on and one to check — would be the same three verbs written twice.
//
// The object verbs are `with name`, `in folder` and `in path`, they read exactly as the scope verbs of the
// same name one stage earlier, and they are combined with AND: each narrows the set of files the rule is
// about depending on, so their order never matters. Each of them takes the `except` companion of except.go,
// which is how a boundary gets its one documented hole.
//
// It carries the scope and the mood it was asked of unchanged, and it is immutable like every stage before it
// — so a rule can be stored, passed to a helper and checked as often as it is useful, and one object can be
// branched into two. Nothing has been read when it is built: the project is located, extracted, projected and
// judged by Check, and by nothing else.
type FilesDependencyCondition struct {
	// rule is the scope and the mood the predicate was asked of.
	rule filesRule
	// objects are the compiled object verbs, in the order they were chained, and they are combined with AND.
	// No object verb at all is every file of the project, which is what `depend on files` on its own means.
	objects []matching.Filter
}

// DependOnFiles is the relational predicate, and the rule most architecture policy is written as: `depend on
// files`.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/api/**").
//		Should().
//		DependOnFiles().
//		InFolder("internal/domain/**")
//
// It is satisfied per file, existentially: a selected file holds this rule when it depends on at least one of
// the files the object names. So `should depend on files in folder "internal/domain/**"` requires each file
// of the scope to reach that folder, and the negation forbids any of them from reaching it at all — which is
// how a boundary is spelled, and why ShouldNot is the mood most of these rules are written in.
//
// The object stage that follows is where the other half of the sentence is named. Chaining nothing onto it is
// a rule about every file of the project — `should not depend on files` is a file that must import nothing of
// its own project, and `should depend on files` is one that must import something — which is deliberately the
// loud reading: a chain whose object the user forgot to type fails, rather than passing because it forbade an
// empty set.
func (b FilesShouldBuilder) DependOnFiles() FilesDependencyCondition {
	return FilesDependencyCondition{rule: b.rule}
}

// DependOnFiles is the negated mood of the same predicate: `should not depend on files`, which forbids the
// dependency rather than requiring it.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/api/**").
//		ShouldNot().
//		DependOnFiles().
//		InFolder("internal/db/**")
//
// This is the sentence AGENTS.md opens with and the one most architecture rules are: a layer, a folder or a
// kind of file that may not reach another one. It is the positive rule with assertion.Mood threaded into the
// same assertion — one violation per selected file that *does* depend on the object's files, carrying the
// dependencies it was broken by — and not a second implementation.
func (b FilesShouldNotBuilder) DependOnFiles() FilesDependencyCondition {
	return FilesDependencyCondition{rule: b.rule}
}

// WithName narrows the object to the files whose name matches this pattern: the last segment of the
// identifier, so `*_repository.go` is every file named that way wherever it lives.
func (c FilesDependencyCondition) WithName(pattern string) FilesDependencyCondition {
	return c.selecting("with name", pattern, c.rule.scope.factory.FilenameMatcher)
}

// InFolder narrows the object to the files in a folder matching this pattern: the identifier without its last
// segment, so `internal/db` is that folder alone and `internal/db/**` is it together with everything below
// it. A file at the project root is in the folder `.`.
//
// It is the object verb almost every boundary rule is written with, because a boundary in Go is a folder.
func (c FilesDependencyCondition) InFolder(pattern string) FilesDependencyCondition {
	return c.selecting("in folder", pattern, c.rule.scope.factory.FolderMatcher)
}

// InPath narrows the object to the files whose whole identifier matches this pattern, folder and name at once
// — `internal/**/*_repository.go`.
func (c FilesDependencyCondition) InPath(pattern string) FilesDependencyCondition {
	return c.selecting("in path", pattern, c.rule.scope.factory.PathMatcher)
}

// Check runs the rule: one violation per selected file that depends on the files the object named where the
// mood forbids it, or on none of them where the mood requires it, and an empty result when every selected
// file agrees with the rule, which is the pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, resolve both halves of the
// sentence against it, project the dependencies from the one to the other, judge each selected file — and the
// only stage of the chain that reads anything. The project is extracted once and both populations are
// resolved against that one graph, because two extractions could not be compared with each other.
//
// The violations are the files module's own assertion.DependencyViolation values, each carrying the file, the
// object's selectors, the dependencies found and the mood, or the EmptyTestViolations of a sentence one of
// whose halves named no file at all.
//
// The error is technical or the user's — a pattern a scope verb or an object verb could not compile, a locator
// naming no Go project, a project that will not load — and never a failing rule. When it is non-nil the
// violations say nothing.
func (c FilesDependencyCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	graph, selected, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}
	objects := projection.SelectFiles(graph, c.objects...)

	if empty := options.GatherEmptyTestViolations(c.populations(len(selected), len(objects))...); len(empty) > 0 {
		// A sentence with no subject, or with no object, is reported instead of being judged: there is no
		// dependency to find either way, so one mood of such a rule would pass forever and the other would
		// report every file it selected.
		return empty, nil
	}

	dependencies := kernelprojection.ProjectEdges(graph, projection.PerDependencyEdge(selected, objects))
	return filesassertion.GatherDependencyViolations(selected, dependencies, c.objects, c.rule.mood), nil
}

// String renders the whole rule as the sentence the user typed, as `project files, path without filename
// matches "internal/api/**", should not, depend on files, path without filename matches "internal/db/**"`.
//
// The predicate renders as the words the user wrote and the object verbs after it as their filters'
// descriptions, which is what the scope verbs before the mood do: the object is a selection, and a reader
// needs to see which part of an identifier each of its patterns was matched against.
func (c FilesDependencyCondition) String() string {
	return c.rule.render(c.stages()...)
}

// stages are the parts of the sentence this predicate and its object add, ready for filesRule.render: the
// predicate as the user typed it, then one per object verb, in the order they were chained.
func (c FilesDependencyCondition) stages() []string {
	stages := make([]string, 0, len(c.objects)+1)
	stages = append(stages, "depend on files")
	for _, object := range c.objects {
		stages = append(stages, object.String())
	}
	return stages
}

// selecting is every object verb: compile the string the user typed with the scope's own factory, and hand
// back a new condition whose object is narrowed by the resulting filter. Which part of an identifier a verb
// looks at is the compile function it passes in, so that pairing is stated once per verb — the same shape
// FilesBuilder.selecting gives the scope verbs, over the other half of the sentence.
//
// A pattern this library cannot understand is deferred to the terminal exactly as a scope verb's is: the
// rejection joins the scope, so the first pattern the user has to fix is the one reported however many stages
// later the second typo sits, the rule renders with the rejection visible, and Check returns it as a UserError
// naming the object verb before the project is read. The rejected pattern does not join the object, because a
// zero Filter matches nothing and the rule would then report an empty object instead of the typo.
func (c FilesDependencyCondition) selecting(verb, pattern string, compile func(string) (matching.Filter, error)) FilesDependencyCondition {
	selector, err := compile(pattern)
	if err != nil {
		rejected := c
		rejected.rule.scope = c.rule.scope.rejecting(verb, pattern, err)
		return rejected
	}
	narrowed := c
	narrowed.objects = append(slices.Clone(c.objects), selector)
	return narrowed
}

// populations are both halves of the sentence, for the empty-test guard: the files the scope selected, and
// the files the object named. A relational rule has two populations, and either of them being empty is the
// stale glob the guard exists for — an object naming a folder that has been renamed is exactly the `should
// not depend on` rule that is green forever.
//
// Both are handed over when both are empty, because both patterns are then wrong and a reader fixing one
// would come back for the other; the guard reports every population it is given. The object is guarded only
// when the user named one: `depend on files` with nothing chained onto it is every file of the project, so
// an empty object there is an empty project, which the subject guard has already said.
func (c FilesDependencyCondition) populations(subjects, objects int) []kernel.EmptyTestPopulation {
	populations := []kernel.EmptyTestPopulation{c.rule.selection(subjects)}
	if len(c.objects) == 0 {
		return populations
	}
	// The object's subject word is its own vocabulary — `files to depend on` rather than `files` — because the
	// two halves of the sentence are selected the same way, so a report that only carried the selectors could
	// not say which half of the rule the user has to go and fix.
	return append(populations, kernel.EmptyTestPopulation{
		Subject:   "files to depend on",
		Matched:   objects,
		Selectors: c.objects,
	})
}
