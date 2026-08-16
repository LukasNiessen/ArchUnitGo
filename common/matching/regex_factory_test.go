package matching

import (
	"errors"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
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

func TestNewLiteralPattern(t *testing.T) {
	tests := []struct {
		name    string
		literal string
		matches []string
		rejects []string
	}{
		{
			name:    "glob metacharacters are literal",
			literal: "handler_v[1]*.go",
			matches: []string{"handler_v[1]*.go"},
			rejects: []string{"handler_v1.go", "handler_v[1]x.go"},
		},
		{
			name:    "regex metacharacters are literal",
			literal: `a+b(c).go`,
			matches: []string{"a+b(c).go"},
			rejects: []string{"aab(c).go", "abc.go"},
		},
		{
			name:    "a whole identifier, not a fragment of one",
			literal: "internal/api/handler.go",
			matches: []string{"internal/api/handler.go", `internal\api\handler.go`},
			rejects: []string{"cmd/internal/api/handler.go", "internal/api/handler.go.bak", "internal/api"},
		},
		{
			name:    "separators normalise on both sides",
			literal: `internal\api\handler.go`,
			matches: []string{"internal/api/handler.go"},
			rejects: []string{"internal/api/handler.go.bak"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := NewLiteralPattern(test.literal, nil)
			if err != nil {
				t.Fatalf("NewLiteralPattern(%q): %v", test.literal, err)
			}
			if pattern.Source() != test.literal {
				t.Errorf("Source() = %q, want the literal as written, %q", pattern.Source(), test.literal)
			}
			for _, candidate := range test.matches {
				if !pattern.Matches(candidate) {
					t.Errorf("literal %q should match %q", test.literal, candidate)
				}
			}
			for _, candidate := range test.rejects {
				if pattern.Matches(candidate) {
					t.Errorf("literal %q should not match %q", test.literal, candidate)
				}
			}
		})
	}
}

func TestNewLiteralPatternRejectsAnEmptyLiteral(t *testing.T) {
	for _, literal := range []string{"", "   "} {
		_, err := NewLiteralPattern(literal, nil)
		if err == nil {
			t.Fatalf("NewLiteralPattern(%q) should have failed", literal)
		}
		if !errors.Is(err, ErrInvalidPattern) {
			t.Errorf("NewLiteralPattern(%q) error = %v, want ErrInvalidPattern", literal, err)
		}
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

func TestRegexFactoryMatchersPerSelector(t *testing.T) {
	globs := NewRegexFactory(nil)

	tests := []struct {
		name    string
		matcher func(string) (Filter, error)
		pattern string
		target  MatchTarget
		matches []string
		rejects []string
	}{
		{
			name:    "filename matcher",
			matcher: globs.FilenameMatcher,
			pattern: "*_test.go",
			target:  TargetFilename,
			matches: []string{"internal/api/handler_test.go", "handler_test.go"},
			rejects: []string{"internal/api/handler.go", "internal/test.go/x"},
		},
		{
			name:    "folder matcher",
			matcher: globs.FolderMatcher,
			pattern: "internal/api/**",
			target:  TargetPathWithoutFilename,
			matches: []string{"internal/api/handler.go", "internal/api/v1/handler.go"},
			rejects: []string{"internal/db/store.go", "cmd/internal/api/main.go"},
		},
		{
			name:    "path matcher",
			matcher: globs.PathMatcher,
			pattern: "internal/**/*.go",
			target:  TargetPath,
			matches: []string{"internal/handler.go", "internal/api/handler.go"},
			rejects: []string{"handler.go", "internal/api/handler.txt"},
		},
		{
			name:    "classname matcher",
			matcher: globs.ClassnameMatcher,
			pattern: "*Handler",
			target:  TargetClassname,
			matches: []string{"internal/api.RequestHandler", "api.Handler", "Handler"},
			rejects: []string{"internal/api.HandlerFactory"},
		},
		{
			name:    "exact file matcher",
			matcher: globs.ExactFileMatcher,
			pattern: "internal/api/handler.go",
			target:  TargetPath,
			matches: []string{"internal/api/handler.go"},
			rejects: []string{"internal/api/handler_test.go", "cmd/internal/api/handler.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter, err := test.matcher(test.pattern)
			if err != nil {
				t.Fatalf("matcher(%q): %v", test.pattern, err)
			}
			if filter.Target() != test.target {
				t.Errorf("Target() = %v, want %v", filter.Target(), test.target)
			}
			if filter.Pattern().Source() != test.pattern {
				t.Errorf("Pattern().Source() = %q, want the pattern as written, %q", filter.Pattern().Source(), test.pattern)
			}
			for _, candidate := range test.matches {
				if !filter.Matches(candidate) {
					t.Errorf("(%s).Matches(%q) = false, want true", filter, candidate)
				}
			}
			for _, candidate := range test.rejects {
				if filter.Matches(candidate) {
					t.Errorf("(%s).Matches(%q) = true, want false", filter, candidate)
				}
			}
		})
	}
}

func TestRegexFactoryZeroValueIsGlobAndCaseSensitive(t *testing.T) {
	var zero RegexFactory

	if zero.Syntax() != SyntaxGlob {
		t.Errorf("the zero factory's syntax = %d, want SyntaxGlob", zero.Syntax())
	}

	filter := mustMatch(t, zero.FolderMatcher, "internal/**")
	if !filter.Matches("internal/api/handler.go") {
		t.Error("the zero factory should read a pattern as a glob")
	}
	if filter.Matches("Internal/api/handler.go") {
		t.Error("the zero factory should be case-sensitive")
	}
	if nilOptions := NewRegexFactory(nil); nilOptions != zero {
		t.Error("NewRegexFactory(nil) should be the zero factory: nil means the defaults")
	}
}

func TestRegexFactoryReadsRegexSyntaxWhenAsked(t *testing.T) {
	regexes := NewRegexFactory(&RegexFactoryOptions{Syntax: SyntaxRegex})

	if regexes.Syntax() != SyntaxRegex {
		t.Errorf("Syntax() = %d, want SyntaxRegex", regexes.Syntax())
	}

	filter := mustMatch(t, regexes.FilenameMatcher, `handler_v[0-9]+\.go`)
	if !filter.Matches("internal/api/handler_v12.go") {
		t.Error("a regex-syntax factory should compile its pattern as a regular expression")
	}
	if filter.Matches("internal/api/handler_vX.go") {
		t.Error("a regex escape must survive the factory")
	}

	// The same string means different things in the two syntaxes, which is the whole point of the
	// syntax travels with the factory rather than being decided at the call site.
	asGlob := mustMatch(t, NewRegexFactory(nil).FilenameMatcher, "*.go")
	if !asGlob.Matches("handler.go") {
		t.Error("`*.go` is a valid glob")
	}
	if _, err := regexes.FilenameMatcher("*.go"); err == nil {
		t.Error("`*.go` is a dangling repetition as a regular expression, so it should fail")
	}
}

func TestRegexFactoryCaseInsensitive(t *testing.T) {
	factory := NewRegexFactory(&RegexFactoryOptions{CaseInsensitive: true})

	folder := mustMatch(t, factory.FolderMatcher, "internal/API/**")
	if !folder.Matches("internal/api/handler.go") {
		t.Error("a case-insensitive factory should compile a case-insensitive folder matcher")
	}
	if folder.Matches("internal/apis/handler.go") {
		t.Error("case insensitivity should not make the rest of the pattern looser")
	}

	exact := mustMatch(t, factory.ExactFileMatcher, "internal/api/Handler.go")
	if !exact.Matches("internal/api/handler.go") {
		t.Error("case insensitivity applies to an exact matcher too")
	}
}

func TestRegexFactoryExactFileMatcherIgnoresTheSyntax(t *testing.T) {
	// An exact match is neither a glob nor a regular expression, so both factories treat the string
	// the same way: as the name of one file.
	for name, factory := range map[string]RegexFactory{
		"glob":  NewRegexFactory(nil),
		"regex": NewRegexFactory(&RegexFactoryOptions{Syntax: SyntaxRegex}),
	} {
		t.Run(name, func(t *testing.T) {
			filter := mustMatch(t, factory.ExactFileMatcher, "internal/api/handler_v[1].go")

			if !filter.Matches("internal/api/handler_v[1].go") {
				t.Error("an exact matcher should match the file it names")
			}
			if filter.Matches("internal/api/handler_v1.go") {
				t.Error("an exact matcher must not read its argument as a pattern")
			}
		})
	}
}

func TestRegexFactoryCompilesExclusionsInItsOwnSyntax(t *testing.T) {
	regexes := NewRegexFactory(&RegexFactoryOptions{Syntax: SyntaxRegex})

	excluded, err := regexes.Compile(`.*_(test|mock)\.go`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	filter := mustMatch(t, regexes.PathMatcher, `internal/.*\.go`).Excluding(excluded)

	if !filter.Matches("internal/api/handler.go") {
		t.Error("an identifier matching no exclusion should still match")
	}
	for _, candidate := range []string{"internal/api/handler_test.go", "internal/api/handler_mock.go"} {
		if filter.Matches(candidate) {
			t.Errorf("(%s).Matches(%q) = true, want false", filter, candidate)
		}
	}
}

func TestRegexFactoryReportsAnInvalidPattern(t *testing.T) {
	globs := NewRegexFactory(nil)

	matchers := map[string]func(string) (Filter, error){
		"filename":   globs.FilenameMatcher,
		"folder":     globs.FolderMatcher,
		"path":       globs.PathMatcher,
		"classname":  globs.ClassnameMatcher,
		"exact file": globs.ExactFileMatcher,
	}

	for name, matcher := range matchers {
		t.Run(name, func(t *testing.T) {
			// The empty pattern is the one every matcher rejects, exact ones included.
			filter, err := matcher("")
			if err == nil {
				t.Fatalf("%s matcher should have rejected the empty pattern", name)
			}
			if !errors.Is(err, ErrInvalidPattern) {
				t.Errorf("error = %v, want ErrInvalidPattern", err)
			}
			if filter.Matches("internal/api/handler.go") {
				t.Error("a matcher that failed must return the zero Filter, which matches nothing")
			}
		})
	}

	if _, err := globs.FolderMatcher("internal/[api"); !errors.Is(err, ErrInvalidPattern) {
		t.Errorf("an unterminated character class error = %v, want ErrInvalidPattern", err)
	}
}

func TestRegexFactoryRejectsAnUnknownSyntax(t *testing.T) {
	// Not reachable through the exported API, which is why an in-package test is the only thing that
	// can check the guard: a syntax nobody taught the factory must fail loudly rather than fall back
	// to a syntax the user did not ask for.
	unknown := RegexFactory{syntax: SyntaxRegex + 1}

	if _, err := unknown.Compile("*.go"); !errors.Is(err, ErrInvalidPattern) {
		t.Errorf("Compile with an unknown syntax error = %v, want ErrInvalidPattern", err)
	}
	if _, err := unknown.PathMatcher("internal/**"); !errors.Is(err, ErrInvalidPattern) {
		t.Errorf("PathMatcher with an unknown syntax error = %v, want ErrInvalidPattern", err)
	}
}

// TestRegexFactorySelectsNodesOfAFixtureGraph is the level above the unit tests, and the shape the
// fluent API will use the factory in: pattern strings from the user in, selected nodes of a graph out.
func TestRegexFactorySelectsNodesOfAFixtureGraph(t *testing.T) {
	graph := extraction.NewGraph(
		extraction.NewEdge("cmd/server/main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/store.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "fmt", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler_test.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.SelfEdge("internal/db/store.go"),
	)

	globs := NewRegexFactory(nil)

	tests := []struct {
		name    string
		matcher func(string) (Filter, error)
		pattern string
		want    []string
	}{
		{
			name:    "a folder and everything under it",
			matcher: globs.FolderMatcher,
			pattern: "internal/**",
			want:    []string{"internal/api/handler.go", "internal/api/handler_test.go", "internal/db/store.go"},
		},
		{
			name:    "test files anywhere",
			matcher: globs.FilenameMatcher,
			pattern: "*_test.go",
			want:    []string{"internal/api/handler_test.go"},
		},
		{
			name:    "one named file",
			matcher: globs.ExactFileMatcher,
			pattern: "internal/db/store.go",
			want:    []string{"internal/db/store.go"},
		},
		{
			name:    "a pattern matching nothing selects nothing",
			matcher: globs.PathMatcher,
			pattern: "internal/apis/**",
			want:    []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter := mustMatch(t, test.matcher, test.pattern)

			selected := make([]string, 0, len(test.want))
			for _, node := range graph.Nodes() {
				if filter.Matches(node) {
					selected = append(selected, node)
				}
			}
			if !slices.Equal(selected, test.want) {
				t.Errorf("(%s) selected %v, want %v", filter, selected, test.want)
			}
		})
	}
}

func TestNewGlobCapturePatternNamesWhatItMatched(t *testing.T) {
	tests := []struct {
		name      string
		glob      string
		candidate string
		want      string
	}{
		{
			name:      "the folder under a prefix",
			glob:      "internal/(**)/**",
			candidate: "internal/api/handler.go",
			want:      "api",
		},
		{
			name:      "a capture is as short as it can be, so a nested file still names its folder",
			glob:      "internal/(**)/**",
			candidate: "internal/api/sub/deep/handler.go",
			want:      "api",
		},
		{
			name:      "the folder itself is under the folder, so it names itself",
			glob:      "internal/(**)/**",
			candidate: "internal/api",
			want:      "api",
		},
		{
			name:      "the top-level folder",
			glob:      "(*)/**",
			candidate: "files/fluentapi/mood.go",
			want:      "files",
		},
		{
			name:      "the filename",
			glob:      "**/(*)",
			candidate: "internal/api/handler.go",
			want:      "handler.go",
		},
		{
			// A leading `**/` crosses zero segments here, exactly as it does in a plain glob: a file at the
			// root of the project is named by the same pattern that names one in a folder.
			name:      "the filename of a file no folder is under",
			glob:      "**/(*)",
			candidate: "main.go",
			want:      "main.go",
		},
		{
			// And a crossing `/**/` crosses zero segments too, so the file one folder down is named by the
			// same pattern as the one three folders down.
			name:      "the folder of a file directly under it",
			glob:      "internal/(*)/**/order.go",
			candidate: "internal/api/order.go",
			want:      "api",
		},
		{
			name:      "a capture spanning segments when that is what was asked for",
			glob:      "internal/(**)",
			candidate: "internal/api/handler.go",
			want:      "api/handler.go",
		},
		{
			name:      "separators are normalised in the glob as well as the candidate",
			glob:      `internal\(**)\**`,
			candidate: `internal\api\handler.go`,
			want:      "api",
		},
		{
			name:      "the capture may sit inside a segment",
			glob:      "**/(*)_test.go",
			candidate: "internal/api/handler_test.go",
			want:      "handler",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern := mustGlobCapture(t, test.glob, nil)

			got, ok := pattern.Capture(test.candidate)
			if !ok {
				t.Fatalf("(%s).Capture(%q) named nothing, want %q", pattern, test.candidate, test.want)
			}
			if got != test.want {
				t.Errorf("(%s).Capture(%q) = %q, want %q", pattern, test.candidate, got, test.want)
			}
			if !pattern.Matches(test.candidate) {
				t.Errorf("(%s) named %q in %q but does not match it", pattern, got, test.candidate)
			}
		})
	}
}

func TestCaptureNamesNothingWhenThereIsNoNameToRead(t *testing.T) {
	tests := []struct {
		name    string
		pattern Pattern
		// candidate is the identifier the pattern is asked about.
		candidate string
	}{
		{
			name:      "the zero pattern",
			pattern:   Pattern{},
			candidate: "internal/api/handler.go",
		},
		{
			name:      "a pattern the candidate does not match",
			pattern:   mustGlobCapture(t, "internal/(**)/**", nil),
			candidate: "cmd/server/main.go",
		},
		{
			name:      "a pattern compiled without a capture",
			pattern:   mustGlob(t, "internal/**", nil),
			candidate: "internal/api/handler.go",
		},
		{
			name:      "a capture that matched the empty string",
			pattern:   mustGlobCapture(t, "(**)_test.go", nil),
			candidate: "_test.go",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := test.pattern.Capture(test.candidate)
			if ok || got != "" {
				t.Errorf("Capture(%q) = %q, %v, want an unnamed candidate", test.candidate, got, ok)
			}
		})
	}
}

func TestACapturePatternWantsExactlyOneCapture(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		regex   string
		wantErr error
	}{
		{"a glob with no capture", "internal/**", "internal/.*", ErrOneCapture},
		{"a glob with two captures", "(*)/(*)/**", "([^/]*)/([^/]*)/.*", ErrOneCapture},
		{"an empty pattern", "", "", ErrInvalidPattern},
		{"a pattern that does not compile", "(a[b)", "(a[b)", ErrInvalidPattern},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewGlobCapturePattern(test.glob, nil); !errors.Is(err, test.wantErr) {
				t.Errorf("NewGlobCapturePattern(%q) error = %v, want %v", test.glob, err, test.wantErr)
			}
			if _, err := NewRegexCapturePattern(test.regex, nil); !errors.Is(err, test.wantErr) {
				t.Errorf("NewRegexCapturePattern(%q) error = %v, want %v", test.regex, err, test.wantErr)
			}
		})
	}
}

func TestNewGlobPatternKeepsParenthesesLiteral(t *testing.T) {
	// Captures are opt-in, because a filename may contain a parenthesis and the glob a user writes about
	// it is not suddenly a naming pattern.
	pattern := mustGlob(t, "internal/api/handler(1).go", nil)

	if !pattern.Matches("internal/api/handler(1).go") {
		t.Error("a parenthesis in a plain glob stands for itself")
	}
	if pattern.Matches("internal/api/handler1.go") {
		t.Error("a plain glob must not group: `(1)` is two literal parentheses around a literal 1")
	}
}

func TestNewRegexCapturePatternTakesTheExpressionAsWritten(t *testing.T) {
	pattern, err := NewRegexCapturePattern(`internal/([a-z]+)/(?:.*/)?[a-z]+\.go`, nil)
	if err != nil {
		t.Fatalf("NewRegexCapturePattern: %v", err)
	}

	got, ok := pattern.Capture("internal/api/sub/handler.go")
	if !ok || got != "api" {
		t.Errorf("Capture = %q, %v, want %q: a non-capturing group does not count as the capture", got, ok, "api")
	}
	if pattern.Source() != `internal/([a-z]+)/(?:.*/)?[a-z]+\.go` {
		t.Errorf("Source() = %q, want the expression as written", pattern.Source())
	}
}

func TestCapturePatternFollowsTheFactorySyntaxAndOptions(t *testing.T) {
	tests := []struct {
		name    string
		options *RegexFactoryOptions
		pattern string
	}{
		{"glob by default", nil, "internal/(**)/**"},
		{"regex when asked", &RegexFactoryOptions{Syntax: SyntaxRegex}, `internal/(.*?)(?:/.*)?`},
		{
			name:    "case-insensitively when asked",
			options: &RegexFactoryOptions{CaseInsensitive: true},
			pattern: "INTERNAL/(**)/**",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factory := NewRegexFactory(test.options)

			pattern, err := factory.CapturePattern(test.pattern)
			if err != nil {
				t.Fatalf("CapturePattern(%q): %v", test.pattern, err)
			}
			if got, ok := pattern.Capture("internal/api/handler.go"); !ok || got != "api" {
				t.Errorf("CapturePattern(%q).Capture = %q, %v, want %q", test.pattern, got, ok, "api")
			}
		})
	}
}

func TestCapturePatternRejectsAnUnknownSyntax(t *testing.T) {
	unknown := RegexFactory{syntax: PatternSyntax(7)}

	if _, err := unknown.CapturePattern("(*)/**"); !errors.Is(err, ErrInvalidPattern) {
		t.Errorf("CapturePattern with an unknown syntax error = %v, want ErrInvalidPattern", err)
	}
}

func mustMatch(t *testing.T, matcher func(string) (Filter, error), pattern string) Filter {
	t.Helper()
	filter, err := matcher(pattern)
	if err != nil {
		t.Fatalf("matcher(%q): %v", pattern, err)
	}
	return filter
}

func mustGlob(t *testing.T, glob string, options *PatternOptions) Pattern {
	t.Helper()
	pattern, err := NewGlobPattern(glob, options)
	if err != nil {
		t.Fatalf("NewGlobPattern(%q): %v", glob, err)
	}
	return pattern
}

func mustGlobCapture(t *testing.T, glob string, options *PatternOptions) Pattern {
	t.Helper()
	pattern, err := NewGlobCapturePattern(glob, options)
	if err != nil {
		t.Fatalf("NewGlobCapturePattern(%q): %v", glob, err)
	}
	return pattern
}
