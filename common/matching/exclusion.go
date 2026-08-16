package matching

import (
	"errors"
	"fmt"
	"slices"
)

// The three ways an exclusion can be typed wrongly, as sentinels a caller can recognize with errors.Is.
// Each of them is reported by the fluent API as an archerror.UserError naming the `except` verb at
// fault: the library is working and the code has not been judged, the chain simply says nothing that
// could be run.
var (
	// ErrExclusionWithoutSelector says `except` was written before any selector. An exclusion qualifies
	// the selector it follows — that is where it gets the identifiers it is read against — so an
	// exclusion with nothing in front of it describes no set at all. Excluding everything is not a
	// selector either: `project files, except "**"` would be a rule about nothing.
	ErrExclusionWithoutSelector = errors.New("exclusion without a selector")
	// ErrExclusionWithoutPattern says `except` was called with no pattern. A verb the user typed that
	// narrows nothing is a mistake worth reporting rather than a no-op worth keeping, for the reason
	// AGENTS.md gives about zero matches: silently passing is the worst outcome.
	ErrExclusionWithoutPattern = errors.New("exclusion without a pattern")
	// ErrExclusionOfAnotherPopulation says the exclusion names a target the selector it qualifies could
	// never be read against. An exclusion is asked about the identifier its selector is asked about, and a
	// classname is not a path: `for classes matching "*Service" except in folder "internal/legacy"` would
	// ask for the folder of `internal/api.UserService`, which is a question with a wrong answer rather than
	// no answer, so it is refused instead of quietly matching nothing.
	ErrExclusionOfAnotherPopulation = errors.New("exclusion of another population")
)

// Excepting attaches every one of these patterns to the last of selectors as an exclusion, and hands
// back the chain with that selector narrowed. It is the `except` companion of every selector in the
// library, in one place, so that a module adding it spells the verb rather than the semantics.
//
// The exclusion qualifies the *last* selector because that is the one the user just wrote: `in folder
// "app/**", except "**/generated/**"` is one clause, and a chain of selectors is a chain of clauses. The
// selectors before it are untouched, so a scope narrowed twice and then excepted once still means what
// it reads as.
//
// build is what a pattern becomes, and it is one of this package's matcher factories — FilenameMatcher,
// FolderMatcher, PathMatcher, ClassnameMatcher — for an exclusion the user gave a target of its own. A
// nil build means the last selector's own target, which is what a plain exclusion pattern is
// interpreted against and the common case: an exclusion of a folder selector is about folders.
//
// factory compiles the patterns, so an exclusion is spelled the same way as the selector it qualifies —
// glob or regex, case-sensitive or not — and nobody chooses that twice in one chain.
//
// The error is a pattern that will not compile, or one of the three sentinels above; the caller turns it
// into the UserError naming the verb, because only the caller knows which verb was typed. The selectors
// are never modified: the result is a new slice, as it has to be for a builder a user may have stored.
func Excepting(selectors []Filter, factory RegexFactory, patterns []string, build func(Pattern) Filter) ([]Filter, error) {
	if len(selectors) == 0 {
		return nil, ErrExclusionWithoutSelector
	}
	if len(patterns) == 0 {
		return nil, ErrExclusionWithoutPattern
	}
	qualified := selectors[len(selectors)-1]
	if build == nil {
		build = func(pattern Pattern) Filter { return newFilter(pattern, qualified.Target()) }
	}
	exclusions := make([]Filter, 0, len(patterns))
	for _, pattern := range patterns {
		compiled, err := factory.Compile(pattern)
		if err != nil {
			return nil, err
		}
		exclusion := build(compiled)
		if (exclusion.Target() == TargetClassname) != (qualified.Target() == TargetClassname) {
			return nil, fmt.Errorf("%w: an exclusion matching a %s cannot qualify a selector matching a %s",
				ErrExclusionOfAnotherPopulation, exclusion.Target(), qualified.Target())
		}
		exclusions = append(exclusions, exclusion)
	}
	excepted := slices.Clone(selectors)
	excepted[len(excepted)-1] = qualified.ExcludingMatchers(exclusions...)
	return excepted, nil
}
