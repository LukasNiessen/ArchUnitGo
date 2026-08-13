// Package matching answers one question — is this identifier described by this user pattern — and
// is the only package in the library that knows how a pattern is spelled.
//
// The rule it exists to enforce is that globs are sugar and regex is the substrate. NewGlobPattern
// is the one place where glob syntax is understood; it hands back a Pattern wrapping a compiled
// regular expression, and nothing downstream ever sees a glob again. A Pattern plus a MatchTarget
// is a Filter, and Filter.Matches is the library's single matching function: filename, path, folder
// and classname rules all come through it.
package matching

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidPattern is returned when a pattern cannot be compiled. It means the user wrote a
// pattern the library cannot understand, so callers should surface it rather than retry.
var ErrInvalidPattern = errors.New("invalid pattern")

// Pattern is a user pattern compiled to a regular expression, anchored at both ends: a pattern
// describes a whole identifier, never a fragment of one. Write `.*` (or `**`) where a fragment is
// what you mean.
//
// Matching is case-sensitive by default, because Go import paths and identifiers are.
//
// A Pattern is immutable, and the zero Pattern matches nothing.
type Pattern struct {
	source string
	regex  *regexp.Regexp
}

// PatternOptions are the knobs on pattern compilation. A nil *PatternOptions means the defaults,
// which is what most callers pass.
type PatternOptions struct {
	// CaseInsensitive makes the compiled pattern ignore letter case. It is off by default.
	CaseInsensitive bool
}

// NewGlobPattern compiles a glob into a Pattern. This constructor, and the translation it calls,
// are the one place in the library that knows what a glob is.
//
// The syntax is the familiar one:
//
//   - `*` is any run of characters within one segment, never crossing a separator;
//   - `**` is any run of characters, crossing separators;
//   - `?` is exactly one character, never a separator;
//   - `[abc]` is one character from the class, `[a-z]` a range and `[!abc]` a negated class.
//
// Everything else is literal, so `.` and `+` mean themselves. Separators are normalised, so
// `internal\api\**` and `internal/api/**` are the same glob on every operating system — which is
// also why a glob has no escape character. Use NewRegexPattern when a pattern needs one.
//
// Two conveniences in how `**` spans segments, both there so that a rule written about a folder
// holds for the folder itself as well as its contents:
//
//	a/**/b  matches `a/b`, `a/x/b` and `a/x/y/b` — crossing zero segments counts
//	a/**    matches `a`, `a/b` and `a/b/c`
func NewGlobPattern(glob string, options *PatternOptions) (Pattern, error) {
	normalized := normalizeSeparators(glob)
	if normalized == "" {
		return Pattern{}, fmt.Errorf("%w: glob is empty", ErrInvalidPattern)
	}
	body, err := globToRegex(normalized)
	if err != nil {
		return Pattern{}, err
	}
	return compilePattern(glob, body, options)
}

// NewRegexPattern compiles a regular expression into a Pattern. The expression is taken as
// written, apart from being anchored at both ends: nothing normalises separators here, because in a
// regular expression a backslash is an escape and not a separator. Identifiers use forward slashes,
// so a regex pattern should too.
func NewRegexPattern(expression string, options *PatternOptions) (Pattern, error) {
	if expression == "" {
		return Pattern{}, fmt.Errorf("%w: regular expression is empty", ErrInvalidPattern)
	}
	return compilePattern(expression, expression, options)
}

// Source is the pattern as the user wrote it, for reports and violation data. The compiled regular
// expression is deliberately not exposed: it is an implementation detail of matching, and a
// violation should quote what the user typed.
func (p Pattern) Source() string {
	return p.source
}

// Matches reports whether candidate is described by this pattern, end to end. The zero Pattern
// matches nothing, not everything: an unset pattern is a mistake, and matching everything would
// hide it.
func (p Pattern) Matches(candidate string) bool {
	if p.regex == nil {
		return false
	}
	return p.regex.MatchString(normalizeSeparators(candidate))
}

// String renders the pattern as the user wrote it.
func (p Pattern) String() string {
	return p.source
}

// compilePattern anchors a regex body and compiles it. Anchoring happens in exactly one place, and
// the body is wrapped in a non-capturing group first: `^a|b$` would anchor only the alternatives it
// touches, which is not what a pattern means.
func compilePattern(source, body string, options *PatternOptions) (Pattern, error) {
	prefix := ""
	if options != nil && options.CaseInsensitive {
		prefix = "(?i)"
	}
	regex, err := regexp.Compile(prefix + "^(?:" + body + ")$")
	if err != nil {
		return Pattern{}, fmt.Errorf("%w %q: %w", ErrInvalidPattern, source, err)
	}
	return Pattern{source: source, regex: regex}, nil
}

// globToRegex translates a glob into an unanchored regular expression body. Glob syntax exists in
// this function and nowhere else.
func globToRegex(glob string) (string, error) {
	var body strings.Builder
	for index := 0; index < len(glob); {
		remainder := glob[index:]
		switch {
		case strings.HasPrefix(remainder, "/**/"):
			// Crossing zero segments counts, so `a/**/b` matches `a/b`.
			body.WriteString(`/(?:.*/)?`)
			index += len("/**/")
		case remainder == "/**":
			// A trailing `/**` covers the folder itself as well as everything under it.
			body.WriteString(`(?:/.*)?`)
			index += len("/**")
		case index == 0 && strings.HasPrefix(remainder, "**/"):
			body.WriteString(`(?:.*/)?`)
			index += len("**/")
		case strings.HasPrefix(remainder, "**"):
			body.WriteString(`.*`)
			index += len("**")
		case remainder[0] == '*':
			body.WriteString(`[^/]*`)
			index++
		case remainder[0] == '?':
			body.WriteString(`[^/]`)
			index++
		case remainder[0] == '[':
			class, width, err := globCharacterClass(remainder)
			if err != nil {
				return "", err
			}
			body.WriteString(class)
			index += width
		default:
			body.WriteString(regexp.QuoteMeta(remainder[:1]))
			index++
		}
	}
	return body.String(), nil
}

// globCharacterClass translates one `[...]` glob class into a regex class and reports how much of
// the glob it consumed. `[!abc]` is the glob spelling of a negated class, and a `]` immediately
// after the opening bracket is a literal one.
func globCharacterClass(glob string) (string, int, error) {
	var class strings.Builder
	class.WriteString("[")
	index := 1
	if index < len(glob) && (glob[index] == '!' || glob[index] == '^') {
		class.WriteString("^")
		index++
	}
	if index < len(glob) && glob[index] == ']' {
		class.WriteString(`\]`)
		index++
	}
	for index < len(glob) && glob[index] != ']' {
		if glob[index] == '^' || glob[index] == '[' {
			class.WriteString(`\`)
		}
		class.WriteByte(glob[index])
		index++
	}
	if index == len(glob) {
		return "", 0, fmt.Errorf("%w: character class in %q is not closed", ErrInvalidPattern, glob)
	}
	class.WriteString("]")
	return class.String(), index + 1, nil
}

// normalizeSeparators puts a glob, or a candidate string about to be matched, into the separator
// convention the whole library uses: forward slashes, no duplicates, no trailing slash. That is
// what makes a rule behave the same on every operating system.
//
// It is deliberately narrower than extraction.NormalizeIdentifier: lexical cleaning would eat the
// `**` a glob is made of, and identifiers arriving here are already canonical. It is deliberately
// never applied to a regular expression, where a backslash is an escape rather than a separator.
func normalizeSeparators(pattern string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(pattern), `\`, "/")
	for strings.Contains(normalized, "//") {
		normalized = strings.ReplaceAll(normalized, "//", "/")
	}
	if len(normalized) > 1 {
		normalized = strings.TrimSuffix(normalized, "/")
	}
	return normalized
}
