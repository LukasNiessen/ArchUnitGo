package matching

import (
	"slices"
	"strings"
)

// Filter is everything needed to decide whether one identifier is in the set a user described: a
// compiled Pattern, the part of the identifier to look at, whether matching the pattern means in or
// out, and the patterns that veto a match.
//
// Because the target travels with the filter, there is exactly one matching function — Matches —
// and callers never choose between a filename comparison and a path comparison.
//
// A Filter is immutable: every method returns a new value. Build one with a matcher —
// FilenameMatcher, PathMatcher, FolderMatcher, ClassnameMatcher — and note that the zero Filter
// matches nothing.
type Filter struct {
	pattern    Pattern
	target     MatchTarget
	matching   bool
	exclusions []Pattern
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
func (f Filter) Excluding(patterns ...Pattern) Filter {
	excluded := f
	excluded.exclusions = append(slices.Clone(f.exclusions), patterns...)
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
	candidate, extracted := f.target.extract(normalizeSeparators(identifier))
	if !extracted {
		return false
	}
	for _, exclusion := range f.exclusions {
		if exclusion.Matches(candidate) {
			return false
		}
	}
	return f.pattern.Matches(candidate) == f.matching
}

// String renders the filter for logs and test failures, as `filename matches "*.go"`.
func (f Filter) String() string {
	verb := "matches"
	if !f.matching {
		verb = "does not match"
	}
	description := f.target.String() + " " + verb + ` "` + f.pattern.Source() + `"`
	if len(f.exclusions) == 0 {
		return description
	}
	sources := make([]string, 0, len(f.exclusions))
	for _, exclusion := range f.exclusions {
		sources = append(sources, `"`+exclusion.Source()+`"`)
	}
	return description + ", excluding " + strings.Join(sources, ", ")
}

func newFilter(pattern Pattern, target MatchTarget) Filter {
	return Filter{pattern: pattern, target: target, matching: true}
}
