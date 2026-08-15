// Package matching answers one question — is this identifier described by this user pattern — and
// is the only package in the library that knows how a pattern is spelled.
//
// The rule it exists to enforce is that globs are sugar and regex is the substrate. NewGlobPattern
// is the one place where glob syntax is understood; it hands back a Pattern wrapping a compiled
// regular expression, and nothing downstream ever sees a glob again. A Pattern plus a MatchTarget
// is a Filter, and Filter.Matches is the library's single matching function: filename, path, folder
// and classname rules all come through it.
//
// A Filter also carries its exclusions, which is what the fluent API's `except` companion compiles to:
// Excepting is the one place that says an exclusion qualifies the selector it follows, and
// Filter.Excluding and Filter.ExcludingMatchers are its two forms — a bare pattern read against the
// qualified selector's own target, and one that names a target of its own.
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

// NewLiteralPattern compiles a string into a Pattern that matches exactly that string and nothing
// else. It is the third and last way a user pattern becomes a Pattern, and the one an exact selector
// needs: every character is escaped, so a filename containing `*`, `[`, `+` or `.` is matched as
// written rather than as a pattern the user never meant to type.
//
// Separators are normalised as they are for a glob, so an identifier spelled `internal\api\x.go`
// compiles to the same Pattern as `internal/api/x.go`.
func NewLiteralPattern(literal string, options *PatternOptions) (Pattern, error) {
	normalized := normalizeSeparators(literal)
	if normalized == "" {
		return Pattern{}, fmt.Errorf("%w: literal is empty", ErrInvalidPattern)
	}
	return compilePattern(literal, regexp.QuoteMeta(normalized), options)
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

// PatternSyntax says how the pattern strings handed to a RegexFactory are spelled. It is the value
// behind the difference between the `defined by` and `defined by regex` scope verbs: the syntax is
// chosen once, where the rule is built, instead of at every call site that matches something.
type PatternSyntax uint8

// The two ways a user can spell a pattern.
const (
	// SyntaxGlob reads a pattern as a glob, as NewGlobPattern describes. It is the zero value,
	// because a glob is the sugar users write and the substrate is nobody's default.
	SyntaxGlob PatternSyntax = iota
	// SyntaxRegex reads a pattern as a regular expression, taken as written and anchored.
	SyntaxRegex
)

// RegexFactory is the collection of Filter constructors the rest of the library builds selectors
// with: one method per selector — FilenameMatcher, FolderMatcher, PathMatcher, ClassnameMatcher,
// ExactFileMatcher — each taking a pattern as the user typed it and handing back a Filter with the
// pattern already compiled and the match target already chosen.
//
// It is the seam that keeps two promises. Adding a selector means adding a method here, not a
// matching branch somewhere downstream. And no caller ever chooses between glob and regex
// compilation at the point of use — the choice travels with the factory, which is why nothing outside
// this package needs to know that a glob was ever involved.
//
// A RegexFactory is immutable, and the zero value is the useful one: glob syntax, case-sensitive.
type RegexFactory struct {
	syntax  PatternSyntax
	options PatternOptions
}

// RegexFactoryOptions are the knobs on a factory, and they are set once for every pattern it
// compiles. A nil *RegexFactoryOptions means the defaults — glob syntax, case-sensitive — which is
// what most callers pass.
type RegexFactoryOptions struct {
	// Syntax is how this factory's pattern strings are spelled. Glob by default.
	Syntax PatternSyntax
	// CaseInsensitive makes every pattern this factory compiles ignore letter case. It is off by
	// default, because Go import paths and identifiers are case-sensitive.
	CaseInsensitive bool
}

// NewRegexFactory returns the factory described by options.
func NewRegexFactory(options *RegexFactoryOptions) RegexFactory {
	if options == nil {
		return RegexFactory{}
	}
	return RegexFactory{
		syntax:  options.Syntax,
		options: PatternOptions{CaseInsensitive: options.CaseInsensitive},
	}
}

// Syntax is how this factory reads a pattern string.
func (f RegexFactory) Syntax() PatternSyntax {
	return f.syntax
}

// Compile turns one pattern string into a Pattern in this factory's syntax. Every pattern-taking
// matcher below is this function plus a match target; ExactFileMatcher is the exception — it compiles
// a literal, so this factory's syntax does not reach it.
//
// It is exported for one reason: a filter's exclusions have to be compiled the same way as the
// pattern they qualify, and a caller reaching for NewGlobPattern directly would be deciding the
// syntax a second time.
func (f RegexFactory) Compile(pattern string) (Pattern, error) {
	switch f.syntax {
	case SyntaxGlob:
		return NewGlobPattern(pattern, &f.options)
	case SyntaxRegex:
		return NewRegexPattern(pattern, &f.options)
	default:
		return Pattern{}, fmt.Errorf("%w: unknown pattern syntax %d", ErrInvalidPattern, f.syntax)
	}
}

// FilenameMatcher compiles pattern and matches it against the last segment of an identifier. It is
// the string-facing twin of the package-level function of the same name, which takes a Pattern that
// is already compiled.
func (f RegexFactory) FilenameMatcher(pattern string) (Filter, error) {
	return f.matcher(pattern, FilenameMatcher)
}

// FolderMatcher compiles pattern and matches it against the identifier without its last segment,
// which is the folder a file lives in.
func (f RegexFactory) FolderMatcher(pattern string) (Filter, error) {
	return f.matcher(pattern, FolderMatcher)
}

// PathMatcher compiles pattern and matches it against the whole identifier.
func (f RegexFactory) PathMatcher(pattern string) (Filter, error) {
	return f.matcher(pattern, PathMatcher)
}

// ClassnameMatcher compiles pattern and matches it against a declared name, with any package or path
// qualification stripped.
func (f RegexFactory) ClassnameMatcher(pattern string) (Filter, error) {
	return f.matcher(pattern, ClassnameMatcher)
}

// ExactFileMatcher matches one identifier and nothing else. The string is taken literally, so a file
// whose name contains `*`, `[` or `.` needs no defensive spelling; the factory's syntax does not
// apply, because an exact match is neither a glob nor a regular expression, while case sensitivity
// still does.
//
// It matches the whole identifier, so it wants the identifier as the graph spells it —
// `internal/api/handler.go`, not `handler.go`. Selecting by bare filename is FilenameMatcher's job.
func (f RegexFactory) ExactFileMatcher(identifier string) (Filter, error) {
	literal, err := NewLiteralPattern(identifier, &f.options)
	if err != nil {
		return Filter{}, err
	}
	return PathMatcher(literal), nil
}

// matcher compiles pattern in this factory's syntax and hands the result to build, which is one of
// the package-level Filter factories in filter.go. Every matcher method above except
// ExactFileMatcher is this function with a different build, so which selector looks at which part of
// an identifier is still stated in exactly one place. A pattern that does not compile yields the zero Filter, which matches nothing.
func (f RegexFactory) matcher(pattern string, build func(Pattern) Filter) (Filter, error) {
	compiled, err := f.Compile(pattern)
	if err != nil {
		return Filter{}, err
	}
	return build(compiled), nil
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
