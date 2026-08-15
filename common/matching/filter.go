package matching

import (
	"slices"
	"strings"
)

// Filter is everything needed to decide whether one identifier is in the set a user described: a
// compiled Pattern, the part of the identifier to look at, whether matching the pattern means in or
// out, and the exclusions that veto a match.
//
// Because the target travels with the filter, there is exactly one matching function — Matches —
// and callers never choose between a filename comparison and a path comparison.
//
// A Filter is immutable: every method returns a new value. Build one with a matcher —
// FilenameMatcher, PathMatcher, FolderMatcher, ClassnameMatcher — and note that the zero Filter
// matches nothing.
type Filter struct {
	pattern  Pattern
	target   MatchTarget
	matching bool
	// exclusions are the filters that veto a match, and they are filters rather than patterns because
	// an exclusion may look at a different part of an identifier than the pattern it qualifies: `in
	// folder "app/**" except with name "*_gen.go"` is one selector whose exclusion is about filenames.
	// An exclusion built by Excluding looks at this filter's own target, which is what an exclusion
	// written as a bare pattern means.
	exclusions []Filter
}

// FilenameMatcher matches a pattern against the last segment of an identifier.
func FilenameMatcher(pattern Pattern) Filter {
	return newFilter(pattern, TargetFilename)
}

// PathMatcher matches a pattern against the whole identifier.
func PathMatcher(pattern Pattern) Filter {
	return newFilter(pattern, TargetPath)
}

// FolderMatcher matches a pattern against the identifier without its last segment, which is the
// folder a file lives in.
func FolderMatcher(pattern Pattern) Filter {
	return newFilter(pattern, TargetPathWithoutFilename)
}

// ClassnameMatcher matches a pattern against a declared name, with any package or path
// qualification stripped.
func ClassnameMatcher(pattern Pattern) Filter {
	return newFilter(pattern, TargetClassname)
}

// Pattern is the compiled pattern this filter matches with, for reports and violation data.
func (f Filter) Pattern() Pattern {
	return f.pattern
}

// Target is the part of an identifier this filter looks at.
func (f Filter) Target() MatchTarget {
	return f.target
}

// Excluding returns a filter that rejects any identifier matching one of patterns, whatever the
// main pattern says. Exclusions are matched against the same target as the main pattern, so a
// filename filter excludes filenames.
//
// It is the plain form of an exclusion — a pattern and nothing else, read against whatever the filter
// it qualifies is read against — and ExcludingMatchers is the form for an exclusion that names a
// target of its own.
func (f Filter) Excluding(patterns ...Pattern) Filter {
	exclusions := make([]Filter, 0, len(patterns))
	for _, pattern := range patterns {
		exclusions = append(exclusions, newFilter(pattern, f.target))
	}
	return f.ExcludingMatchers(exclusions...)
}

// ExcludingMatchers returns a filter that rejects any identifier one of these matchers accepts,
// whatever the main pattern says. Each matcher carries the part of an identifier it looks at, so this
// is how a folder selector is qualified by an exclusion about filenames.
//
// A matcher is a Filter, so an exclusion is asked the same question the filter it qualifies is asked,
// and it is asked of the whole identifier rather than of the part the main pattern was reduced to.
// Excluding is the same thing for the common case where the exclusion inherits the target.
func (f Filter) ExcludingMatchers(matchers ...Filter) Filter {
	excluded := f
	excluded.exclusions = append(slices.Clone(f.exclusions), matchers...)
	return excluded
}

// NotMatching returns this filter inverted: an identifier is in the set when it does not match the
// pattern. Exclusions are not inverted — they always reject.
func (f Filter) NotMatching() Filter {
	inverted := f
	inverted.matching = !f.matching
	return inverted
}

// Matches reports whether identifier is in the set this filter describes. It is the library's one
// matching function: filename, path, folder and classname rules all come through here, differing
// only in the filter's target.
//
// The zero Filter matches nothing, because a filter with no pattern is a mistake and matching
// everything would hide it.
func (f Filter) Matches(identifier string) bool {
	if f.pattern.regex == nil {
		return false
	}
	normalized := normalizeSeparators(identifier)
	candidate, extracted := f.target.extract(normalized)
	if !extracted {
		return false
	}
	for _, exclusion := range f.exclusions {
		// The exclusion is asked about the whole identifier, not about the candidate this filter's own
		// target reduced it to: an exclusion carries a target of its own, so extracting twice would ask
		// a filename exclusion for the filename of a folder.
		if exclusion.Matches(normalized) {
			return false
		}
	}
	return f.pattern.Matches(candidate) == f.matching
}

// String renders the filter for logs and test failures, as `filename matches "*.go"`, with any
// exclusions after it: `path without filename matches "app/**", excluding "**/generated/**"`.
//
// Consecutive bare patterns are listed after one `excluding`, and the word is repeated for an exclusion
// naming a target of its own and for the first bare one after such an exclusion — `excluding
// "**/generated", excluding filename matches "*_gen.go", excluding "**/legacy"` — because a bare list
// would read as a second selector, or as a second pattern of the targeted exclusion in front of it,
// rather than as a second exclusion.
func (f Filter) String() string {
	verb := "matches"
	if !f.matching {
		verb = "does not match"
	}
	var rendered strings.Builder
	rendered.WriteString(f.target.String() + " " + verb + ` "` + f.pattern.Source() + `"`)
	for index, exclusion := range f.exclusions {
		if index == 0 || !f.plain(exclusion) || !f.plain(f.exclusions[index-1]) {
			rendered.WriteString(", excluding ")
		} else {
			rendered.WriteString(", ")
		}
		rendered.WriteString(f.describe(exclusion))
	}
	return rendered.String()
}

// describe renders one of this filter's exclusions: the pattern on its own when the exclusion looks at
// the same part of an identifier as the filter it qualifies, and the whole filter when it does not.
//
// The two spellings are the two ways an exclusion is written — a bare pattern, and one that names its
// target — so a reader sees back what they typed instead of `path without filename matches "app/**",
// excluding path without filename matches "**/generated/**"`.
func (f Filter) describe(exclusion Filter) string {
	if f.plain(exclusion) {
		return `"` + exclusion.pattern.Source() + `"`
	}
	return exclusion.String()
}

// plain reports whether this exclusion is one a user wrote as a bare pattern: it reads the same part of
// an identifier as the filter it qualifies, it excludes rather than inverts, and it carries no exclusion
// of its own. Anything else names something of itself and has to say so when it is rendered.
func (f Filter) plain(exclusion Filter) bool {
	return exclusion.target == f.target && exclusion.matching && len(exclusion.exclusions) == 0
}

func newFilter(pattern Pattern, target MatchTarget) Filter {
	return Filter{pattern: pattern, target: target, matching: true}
}
