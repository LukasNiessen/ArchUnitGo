package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	slicesassertion "github.com/LukasNiessen/ArchUnitGo/slices/assertion"
)

// SlicesDependencyCondition is the predicate and the terminal of a rule about what the slices of a project
// may depend on — `project slices, defined by "internal/(**)/**", should not, contain dependency "api" ->
// "db"` — and it is a fluentapi.Checkable, which is the one thing every consumer of a rule programs against:
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		ShouldNot().
//		ContainDependency("api", "db")
//	violations, err := rule.Check(nil)
//
// It is what ContainDependency returns on either mood. Both ends of the dependency are arguments of that one
// verb, so this stage is a terminal and nothing else: there is no object stage to chain, because a dependency
// is a pair and naming half of it would be no rule at all.
//
// It carries the slicing and the mood it was asked of unchanged, and it is immutable like every stage before
// it — so a rule can be stored, passed to a helper and checked as often as it is useful. Nothing has been
// read when it is built: the project is located, extracted, sliced and judged by Check, and by nothing else.
type SlicesDependencyCondition struct {
	// rule is the slicing and the mood the predicate was asked of.
	rule slicesRule
	// from and to are the two slices the rule is about, as the user named them: the depending slice and the
	// one it may or must depend on. They are names, not patterns — the slicing is what turns a pattern into
	// names, and a rule speaks the vocabulary the slicing produced.
	from string
	to   string
}

// ContainDependency is the positive mood of the predicate: `should contain dependency "api" -> "domain"`,
// a dependency the project is required to have.
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		Should().
//		ContainDependency("api", "domain")
//
// It is broken by a dependency that is not there, and the violation then carries no files at all, because
// what it reports is that there were none. That is the rule for a dependency the architecture relies on — a
// slice that must go through another one rather than around it — and it is the less common of the two moods.
//
// Both names are names the slicing produced, matched exactly and not as patterns. A slice nothing is in makes
// the rule vacuous, so it is reported by the empty-test guard rather than judged, and a name the slicing
// never produces at all is that same report.
func (b SlicesShouldBuilder) ContainDependency(from, to string) SlicesDependencyCondition {
	return newDependencyCondition(b.rule, from, to)
}

// ContainDependency is the negated mood of the predicate: `should not contain dependency "api" -> "db"`, a
// dependency the project may not have. It is the sentence a slicing exists for.
//
//	rule := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		ShouldNot().
//		ContainDependency("api", "db")
//
// It is broken by a dependency that is there, and the violation carries every file dependency the pair of
// slices stood for — which is what a reader has to go and unpick. It is the positive rule with
// assertion.Mood threaded into the same assertion, not a second implementation.
//
// The direction is the sentence: `contain dependency "api" -> "db"` says nothing about whether the db may
// reach the api, and the converse rule is the one that says so. Forbidding both ways round is two rules, on
// purpose, because the two are broken by different imports and a report has to name the one that was.
func (b SlicesShouldNotBuilder) ContainDependency(from, to string) SlicesDependencyCondition {
	return newDependencyCondition(b.rule, from, to)
}

// Check runs the rule: one violation when the project has the dependency and the mood forbade it, or does not
// have it and the mood required it, and an empty result when the project agrees with the rule, which is the
// pass. A nil *CheckOptions means the defaults.
//
// It is the whole pipeline in four steps — locate and extract the project, cut a slice name out of every
// file's identifier, project the dependencies between the slices, judge the one pair the rule is about — and
// the only stage of the chain that reads anything.
//
// The violations are the slices module's own assertion.DependencyViolation values, carrying the two slices,
// the mood and the file dependencies found, or the EmptyTestViolations of a slicing that found no slices at
// all or a rule about a slice nobody is in.
//
// The error is technical or the user's — a pattern with no capture in it, a chain with no slicing, a locator
// naming no Go project, a project that will not load — and never a failing rule. When it is non-nil the
// violations say nothing.
func (c SlicesDependencyCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
	graph, membership, err := c.rule.scope.resolve(options)
	if err != nil {
		return nil, err
	}

	if empty := options.GatherEmptyTestViolations(c.populations(membership)...); len(empty) > 0 {
		// A rule about a slice nobody is in is reported instead of being judged: there is no dependency to
		// find either way, so the negated mood would pass forever and the positive one could never pass.
		return empty, nil
	}

	dependencies := kernelprojection.ProjectEdges(graph, c.rule.scope.mapper())
	return slicesassertion.GatherDependencyViolations(c.from, c.to, dependencies, c.rule.mood), nil
}

// String renders the whole rule as the sentence the user typed, as `project slices, path matches
// "internal/(**)/**", should not, contain dependency "api" -> "db"`.
//
// The predicate renders as the words the user wrote and the two slices as they were named, with the arrow
// that says which way round the dependency goes — the direction is half the meaning of the rule.
func (c SlicesDependencyCondition) String() string {
	return c.rule.render(c.stages()...)
}

// stages are the parts of the sentence this predicate adds, ready for slicesRule.render: the predicate and
// the pair of slices, as one stage, because they are one verb.
func (c SlicesDependencyCondition) stages() []string {
	return []string{`contain dependency "` + c.from + `" -> "` + c.to + `"`}
}

// populations are what the empty-test guard is asked about: the slices the slicing found, and — when it found
// any — the two the rule is about.
//
// Either is the stale glob the guard exists for. A slicing whose pattern no longer matches anything finds no
// slice at all, and a rule over it is vacuous; so is a rule naming a slice that the slicing does produce for
// other files but that this one is empty of, which is what a renamed folder looks like when the slicing is by
// folder.
//
// The two ends are guarded only when the slicing found something, because a slicing that found nothing
// already explains why both of them are empty, and the guard reports every population it is given — three
// violations for one renamed pattern would bury the one a reader has to fix. The subject names the slice as
// well as the vocabulary — `files in slice "api"` — for the reason LayersBuilder.populations gives.
func (c SlicesDependencyCondition) populations(membership map[string][]string) []kernel.EmptyTestPopulation {
	populations := []kernel.EmptyTestPopulation{c.rule.selection(len(membership))}
	if len(membership) == 0 {
		return populations
	}
	for _, slice := range []string{c.from, c.to} {
		populations = append(populations, kernel.EmptyTestPopulation{
			Subject:   `files in slice "` + slice + `"`,
			Matched:   len(membership[slice]),
			Selectors: c.rule.scope.selectors(),
		})
	}
	return populations
}

// newDependencyCondition is both moods' predicate: the rule about this pair of slices, with a pair no slicing
// could ever produce rejected on the way.
//
// A slice named with the empty string is ErrUnnamedSlice and a slice depending on itself is
// ErrSelfDependency, and both are rejected here rather than reported as violations, because neither is a
// question about the code: the projection carries no unnamed slice and no dependency of a slice on itself, so
// one mood of such a rule would pass forever and the other could never pass. The rejection is deferred to the
// terminal like every other one in the family, and the first one the user typed is the one reported.
func newDependencyCondition(rule slicesRule, from, to string) SlicesDependencyCondition {
	condition := SlicesDependencyCondition{rule: rule, from: from, to: to}
	switch {
	case from == "":
		condition.rule.scope = rule.scope.rejecting("contain dependency", from, ErrUnnamedSlice)
	case to == "":
		condition.rule.scope = rule.scope.rejecting("contain dependency", to, ErrUnnamedSlice)
	case from == to:
		condition.rule.scope = rule.scope.rejecting("contain dependency", from, ErrSelfDependency)
	}
	return condition
}
