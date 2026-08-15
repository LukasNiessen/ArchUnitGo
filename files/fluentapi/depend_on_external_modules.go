package fluentapi

import (
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

// FilesExternalDependencyCondition is the object stage and the terminal of a rule about which of somebody
// else's modules the files a scope selected may depend on — `project files, in folder "internal/domain/**",
// should not, depend on external modules, matching "*.*/**"` — and it is a fluentapi.Checkable, which is the
// one thing every consumer of a rule programs against:
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/domain/**").
//		ShouldNot().
//		DependOnExternalModules().
//		Matching("*.*/**")
//	violations, err := rule.Check(nil)
//
// It is what DependOnExternalModules returns on either mood, and it is both an object builder and a terminal
// for the reason FilesDependencyCondition is: the object verb is chainable, so it has to hand back something a
// further verb can be chained onto, and the sentence is complete the moment its object is named.
//
// It is FilesDependencyCondition across the project's boundary, and there is one deliberate difference in the
// grammar. `Matching` is repeatable and its patterns are combined with **OR**, where every other chain in this
// library narrows with AND: a third-party policy is a list of alternatives — `matching "github.com/lib/pq"` or
// `matching "github.com/deprecated/**"` — and a module cannot be two modules at once, so ANDing two of them
// would name the empty set and make the second verb meaningless. The rendering says `or` where the rest of the
// library says `,`, so nothing about that reading has to be remembered. `except` is the one companion that verb
// takes, and it qualifies the alternative it follows rather than the list, for the same reason: an exclusion
// belongs to the clause it was written in.
//
// It carries the scope and the mood it was asked of unchanged, and it is immutable like every stage before it —
// so a rule can be stored, passed to a helper and checked as often as it is useful, and one object can be
// branched into two. Nothing has been read when it is built: the project is located, extracted, projected and
// judged by Check, and by nothing else.
type FilesExternalDependencyCondition struct {
	// rule is the scope and the mood the predicate was asked of.
	rule filesRule
	// modules are the compiled object verbs, in the order they were chained, and they are combined with OR. No
	// object verb at all is every module the project depends on, which is what `depend on external modules` on
	// its own means — the standard library included.
	modules []matching.Filter
}

// DependOnExternalModules is the third-party dependency predicate: `depend on external modules`.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/adapter/**").
//		Should().
//		DependOnExternalModules().
//		Matching("*.*/**")
//
// An external module is a dependency that leaves the project: an import path the project does not own, which is
// what the extractor marked as external and is settled there rather than here. The standard library is among
// them — `fmt` and `net/http` are code this project uses and does not own — so a rule that means third-party
// alone says so with a pattern, and `*.*/**` is the idiom for it: an import path whose first segment holds a
// dot is a domain, and no package of the standard library has one.
//
// It is satisfied per file, existentially: a selected file holds this rule when it depends on at least one of
// the modules the object names. So the positive mood requires each file of the scope to reach one of them,
// which is the rarer half — `every adapter must talk to some third-party library` — and the negation is the
// policy nearly every project writes.
//
// The object stage that follows is where the modules are named. Chaining nothing onto it is a rule about every
// module the project depends on: `should not depend on external modules` is a file that must import nothing but
// its own project, which is the strictest reading and a perfectly meaningful one for a domain package.
func (b FilesShouldBuilder) DependOnExternalModules() FilesExternalDependencyCondition {
	return FilesExternalDependencyCondition{rule: b.rule}
}

// DependOnExternalModules is the negated mood of the same predicate: `should not depend on external modules`,
// which forbids the dependency rather than requiring it.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/domain/**").
//		ShouldNot().
//		DependOnExternalModules().
//		Matching("github.com/deprecated/**")
//
// This is the mood a third-party policy is written in: a domain package that may not know any framework, a
// module the team is migrating off, a library one adapter is allowed to import and nothing else may. It is the
// positive rule with assertion.Mood threaded into the same assertion — one violation per selected file that
// *does* depend on such a module, carrying the import paths it was broken by — and not a second
// implementation.
func (b FilesShouldNotBuilder) DependOnExternalModules() FilesExternalDependencyCondition {
	return FilesExternalDependencyCondition{rule: b.rule}
}

// Matching names a module the rule is about, as a pattern over the whole import path: `gorm.io/**` is that
// module and every package of it, `github.com/lib/pq` is that one package, and `*.*/**` is every third-party
// module but no package of the standard library.
//
// The pattern is matched against the import path exactly as the importing file wrote it, so it names a package
// rather than the module it was published as — `golang.org/x/tools/go/packages`, never `golang.org/x/tools`.
// A rule about a whole module is written with a trailing `/**`, which covers the module path itself as well as
// everything under it.
//
// It is repeatable, and the patterns are combined with OR: each call widens the set of modules the rule is
// about, which is the one place in this library that chaining does. That is what makes a policy a list —
// `Matching("github.com/deprecated/**").Matching("gopkg.in/**")` forbids either — and it is why the order of
// the calls cannot matter.
func (c FilesExternalDependencyCondition) Matching(pattern string) FilesExternalDependencyCondition {
	return c.selecting("matching", pattern, c.rule.scope.factory.PathMatcher)
}

// Check runs the rule: one violation per selected file that depends on the external modules the object named
// where the mood forbids it, or on none of them where the mood requires it, and an empty result when every
// selected file agrees with the rule, which is the pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, resolve both halves of the sentence
// against it, project the dependencies that leave the project for the named modules, judge each selected file —
// and the only stage of the chain that reads anything. Both populations are resolved against one graph, because
// two extractions could not be compared with each other.
//
// Only the subject goes through the empty-test guard, which is the one place this terminal differs from
// FilesDependencyCondition.Check. This rule's object *is* a set of dependencies: "no module matched" and "no
// selected file depends on such a module" are one statement, and for the negated mood — which is the mood
// nearly every rule of this family is written in — that statement is exactly the pass. Guarding it would fail
// every project that obeys its own third-party policy, which is the opposite of what the guard is for. The
// stale-glob risk the guard exists for is still covered on the half where it is real: a scope naming a folder
// that has been renamed is reported as loudly here as anywhere else.
//
// The violations are the files module's own assertion.ExternalDependencyViolation values, each carrying the
// file, the object's selectors, the import paths found and the mood, or the EmptyTestViolation of a scope that
// selected no file at all.
//
// The error is technical or the user's — a pattern a scope verb or an object verb could not compile, a locator
// naming no Go project, a project that will not load — and never a failing rule. When it is non-nil the
// violations say nothing.
func (c FilesExternalDependencyCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	graph, selected, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}

	if empty := options.GatherEmptyTestViolations(c.rule.selection(len(selected))); len(empty) > 0 {
		// A rule with no subject is reported instead of being judged: there is no file to find a dependency
		// for, so one mood of such a rule would pass forever and the other would report nothing at all.
		return empty, nil
	}

	modules := projection.SelectExternalModules(graph, c.modules...)
	dependencies := kernelprojection.ProjectEdges(graph, projection.PerExternalDependencyEdge(selected, modules))
	return filesassertion.GatherExternalDependencyViolations(selected, dependencies, c.modules, c.rule.mood), nil
}

// String renders the whole rule as the sentence the user typed, as `project files, path without filename
// matches "internal/domain/**", should not, depend on external modules, path matches "*.*/**"`.
//
// An object of several verbs renders as one stage joined with ` or `, where the scope verbs and the object
// verbs of `depend on files` are joined with `, ` like everything else: that join *is* the difference between
// the two families, and a sentence that hid it would read as a requirement no module could ever meet.
func (c FilesExternalDependencyCondition) String() string {
	return c.rule.render(c.stages()...)
}

// stages are the parts of the sentence this predicate and its object add, ready for filesRule.render: the
// predicate as the user typed it, then the object verbs as the single alternative-joined stage they are.
func (c FilesExternalDependencyCondition) stages() []string {
	stages := []string{"depend on external modules"}
	if alternatives := c.alternatives(); alternatives != "" {
		stages = append(stages, alternatives)
	}
	return stages
}

// alternatives renders the object verbs joined with ` or `, and the empty string when the user named no module
// at all. It is the one place that join is spelled in this package, as projection.SelectExternalModules is the
// one place the OR itself lives.
func (c FilesExternalDependencyCondition) alternatives() string {
	sources := make([]string, 0, len(c.modules))
	for _, module := range c.modules {
		sources = append(sources, module.String())
	}
	return strings.Join(sources, " or ")
}

// selecting is the object verb: compile the string the user typed with the scope's own factory, and hand back a
// new condition whose object is widened by the resulting filter. It is FilesDependencyCondition.selecting with
// the one difference this family has — the new filter is an alternative rather than a further narrowing — and
// it defers a pattern this library cannot understand to the terminal in exactly the same way: the rejection
// joins the scope, so the rule renders with it visible and Check returns it as a UserError naming the object
// verb before the project is read, while the rejected pattern stays out of the object, where a zero Filter
// would silently match nothing.
func (c FilesExternalDependencyCondition) selecting(verb, pattern string, compile func(string) (matching.Filter, error)) FilesExternalDependencyCondition {
	selector, err := compile(pattern)
	if err != nil {
		rejected := c
		rejected.rule.scope = c.rule.scope.rejecting(verb, pattern, err)
		return rejected
	}
	widened := c
	widened.modules = append(slices.Clone(c.modules), selector)
	return widened
}
