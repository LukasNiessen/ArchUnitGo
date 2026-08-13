package matching

import (
	"errors"
	"testing"
)

func TestNewGlobPatternMatches(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		matches []string
		rejects []string
	}{
		{
			name:    "literal identifier",
			glob:    "internal/api/handler.go",
			matches: []string{"internal/api/handler.go"},
			rejects: []string{"internal/api/handler.go.bak", "x/internal/api/handler.go", "internal/api"},
		},
		{
			name:    "star stays inside one segment",
			glob:    "internal/api/*.go",
			matches: []string{"internal/api/handler.go", "internal/api/.go"},
			rejects: []string{"internal/api/sub/handler.go", "internal/api/handler.txt", "internal/handler.go"},
		},
		{
			name:    "star is not a dot",
			glob:    "*.go",
			matches: []string{"handler.go"},
			rejects: []string{"handlergo", "go"},
		},
		{
			name:    "double star crosses segments",
			glob:    "internal/**/handler.go",
			matches: []string{"internal/handler.go", "internal/api/handler.go", "internal/api/v1/handler.go"},
			rejects: []string{"handler.go", "internal/handler.go/x", "external/api/handler.go"},
		},
		{
			name:    "trailing double star covers the folder itself",
			glob:    "internal/api/**",
			matches: []string{"internal/api", "internal/api/handler.go", "internal/api/v1/handler.go"},
			rejects: []string{"internal", "internal/apix", "cmd/internal/api"},
		},
		{
			name:    "leading double star",
			glob:    "**/api/**",
			matches: []string{"api", "api/handler.go", "internal/api", "internal/api/v1/handler.go"},
			rejects: []string{"internal", "internal/apis/handler.go"},
		},
		{
			name:    "bare double star matches everything",
			glob:    "**",
			matches: []string{".", "main.go", "internal/api/handler.go"},
			rejects: []string{},
		},
		{
			name:    "double star inside a segment",
			glob:    "internal/**_test.go",
			matches: []string{"internal/api_test.go", "internal/api/handler_test.go"},
			rejects: []string{"internal/api.go"},
		},
		{
			name:    "question mark is exactly one character",
			glob:    "a?c.go",
			matches: []string{"abc.go", "a-c.go"},
			rejects: []string{"ac.go", "abbc.go", "a/c.go"},
		},
		{
			name:    "character class",
			glob:    "handler_v[123].go",
			matches: []string{"handler_v1.go", "handler_v3.go"},
			rejects: []string{"handler_v4.go", "handler_v12.go"},
		},
		{
			name:    "character class range",
			glob:    "[a-z]*.go",
			matches: []string{"handler.go", "a.go"},
			rejects: []string{"Handler.go", "1.go"},
		},
		{
			name:    "negated character class",
			glob:    "[!_]*.go",
			matches: []string{"handler.go"},
			rejects: []string{"_generated.go"},
		},
		{
			name:    "caret negates a class too",
			glob:    "[^_]*.go",
			matches: []string{"handler.go"},
			rejects: []string{"_generated.go"},
		},
		{
			name:    "closing bracket first in a class is literal",
			glob:    "v[]12].go",
			matches: []string{"v].go", "v1.go", "v2.go"},
			rejects: []string{"v3.go", "v.go"},
		},
		{
			name:    "class metacharacters are literal after the first position",
			glob:    "v[a^[].go",
			matches: []string{"va.go", "v^.go", "v[.go"},
			rejects: []string{"vb.go"},
		},
		{
			name:    "regex metacharacters are literal",
			glob:    "a+b(c).go",
			matches: []string{"a+b(c).go"},
			rejects: []string{"aab(c).go", "ab(c).go", "abc.go"},
		},
		{
			name:    "windows separators normalise",
			glob:    `internal\api\**`,
			matches: []string{"internal/api", "internal/api/handler.go"},
			rejects: []string{"internal"},
		},
		{
			name:    "duplicate and trailing separators normalise",
			glob:    "internal//api/",
			matches: []string{"internal/api"},
			rejects: []string{"internal/api/handler.go"},
		},
		{
			name:    "case sensitive by default",
			glob:    "internal/API/**",
			matches: []string{"internal/API/handler.go"},
			rejects: []string{"internal/api/handler.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern := mustGlob(t, test.glob, nil)

			for _, candidate := range test.matches {
				if !pattern.Matches(candidate) {
					t.Errorf("glob %q should match %q", test.glob, candidate)
				}
			}
			for _, candidate := range test.rejects {
				if pattern.Matches(candidate) {
					t.Errorf("glob %q should not match %q", test.glob, candidate)
				}
			}
		})
	}
}

func TestNewGlobPatternIsCaseInsensitiveOnRequest(t *testing.T) {
	pattern := mustGlob(t, "internal/API/*.GO", &PatternOptions{CaseInsensitive: true})

	for _, candidate := range []string{"internal/API/Handler.GO", "internal/api/handler.go"} {
		if !pattern.Matches(candidate) {
			t.Errorf("case-insensitive glob %q should match %q", pattern.Source(), candidate)
		}
	}
	if pattern.Matches("internal/api/handler.txt") {
		t.Error("case insensitivity should not make the rest of the glob looser")
	}
}

func TestNewGlobPatternNormalisesTheCandidateSeparators(t *testing.T) {
	// A rule must behave the same on every operating system, so a Windows-shaped candidate matches
	// the same glob as its canonical form.
	pattern := mustGlob(t, "internal/api/**", nil)

	if !pattern.Matches(`internal\api\handler.go`) {
		t.Error("a candidate with backslash separators should match the same glob")
	}
}

func TestNewGlobPatternRejectsBadGlobs(t *testing.T) {
	tests := []struct {
		name string
		glob string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"unterminated class", "handler_v[123.go"},
		{"unterminated negated class", "[!_"},
		{"reversed range", "[z-a].go"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGlobPattern(test.glob, nil)
			if err == nil {
				t.Fatalf("NewGlobPattern(%q) should have failed", test.glob)
			}
			if !errors.Is(err, ErrInvalidPattern) {
				t.Errorf("NewGlobPattern(%q) error = %v, want ErrInvalidPattern", test.glob, err)
			}
		})
	}
}

func TestNewGlobPatternKeepsTheSourceAsWritten(t *testing.T) {
	// Violations quote what the user typed, not the regex it compiled to.
	glob := `internal\api\**`
	pattern := mustGlob(t, glob, nil)

	if pattern.Source() != glob {
		t.Errorf("Source() = %q, want %q", pattern.Source(), glob)
	}
	if pattern.String() != glob {
		t.Errorf("String() = %q, want %q", pattern.String(), glob)
	}
}

func TestNewRegexPattern(t *testing.T) {
	pattern, err := NewRegexPattern(`internal/[a-z]+/handler\.go`, nil)
	if err != nil {
		t.Fatalf("NewRegexPattern: %v", err)
	}

	if !pattern.Matches("internal/api/handler.go") {
		t.Error("regex should match internal/api/handler.go")
	}
	if pattern.Matches("cmd/internal/api/handler.go") {
		t.Error("a regex pattern is anchored: it describes the whole identifier")
	}
	if pattern.Matches("internal/api/handlerXgo") {
		t.Error("a regex escape must survive compilation")
	}
}

func TestNewRegexPatternAnchorsAlternationAsAWhole(t *testing.T) {
	pattern, err := NewRegexPattern("main.go|doc.go", nil)
	if err != nil {
		t.Fatalf("NewRegexPattern: %v", err)
	}

	for _, candidate := range []string{"main.go", "doc.go"} {
		if !pattern.Matches(candidate) {
			t.Errorf("alternation should match %q", candidate)
		}
	}
	for _, candidate := range []string{"cmd/main.go", "doc.go.bak"} {
		if pattern.Matches(candidate) {
			t.Errorf("alternation is anchored as a whole, so %q must not match", candidate)
		}
	}
}

func TestNewRegexPatternRejectsBadExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"empty", ""},
		{"unclosed group", "internal/(api"},
		{"dangling repetition", "*.go"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRegexPattern(test.expression, nil)
			if err == nil {
				t.Fatalf("NewRegexPattern(%q) should have failed", test.expression)
			}
			if !errors.Is(err, ErrInvalidPattern) {
				t.Errorf("NewRegexPattern(%q) error = %v, want ErrInvalidPattern", test.expression, err)
			}
		})
	}
}

func TestZeroPatternMatchesNothing(t *testing.T) {
	var pattern Pattern

	for _, candidate := range []string{"", ".", "internal/api/handler.go"} {
		if pattern.Matches(candidate) {
			t.Errorf("the zero Pattern must match nothing, but matched %q", candidate)
		}
	}
}

func TestNormalizeSeparators(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already canonical", "internal/api/**", "internal/api/**"},
		{"windows separators", `internal\api\**`, "internal/api/**"},
		{"mixed separators", `internal\api/**`, "internal/api/**"},
		{"duplicate separators", "internal///api", "internal/api"},
		{"trailing separator", "internal/api/", "internal/api"},
		{"surrounding whitespace", "  internal/api  ", "internal/api"},
		{"lone separator is kept", "/", "/"},
		{"glob syntax survives", "**/*?[a-z]", "**/*?[a-z]"},
		{"empty", "", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeSeparators(test.in); got != test.want {
				t.Errorf("normalizeSeparators(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func mustGlob(t *testing.T, glob string, options *PatternOptions) Pattern {
	t.Helper()
	pattern, err := NewGlobPattern(glob, options)
	if err != nil {
		t.Fatalf("NewGlobPattern(%q): %v", glob, err)
	}
	return pattern
}
