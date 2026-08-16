package fluentapi_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/slices/fluentapi"
)

func TestDefinedByNamesEachSliceByWhatItsCaptureMatched(t *testing.T) {
	// The capture is the slice's name and everything around it says which files are sliced at all, so one
	// pattern answers both questions. The three slicings below are the same project cut three ways.
	root := writeSlicedFixtureProject(t)
	tests := []struct {
		name    string
		pattern string
		want    map[string][]string
	}{
		{
			name:    "the folder under internal, which is what a slice of a Go project usually is",
			pattern: "internal/(**)/**",
			want: map[string][]string{
				"api":    {"internal/api/handler.go", "internal/api/router.go"},
				"db":     {"internal/db/conn.go"},
				"domain": {"internal/domain/order.go"},
			},
		},
		{
			name:    "the name of a file, which slices across the folders",
			pattern: "internal/api/(*).go",
			want: map[string][]string{
				"handler": {"internal/api/handler.go"},
				"router":  {"internal/api/router.go"},
			},
		},
		{
			name:    "a top-level folder, which no file of this project is directly in",
			pattern: "(*)/**",
			want: map[string][]string{
				"internal": {
					"internal/api/handler.go",
					"internal/api/router.go",
					"internal/db/conn.go",
					"internal/domain/order.go",
				},
				"main.go": {"main.go"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			slicing := fluentapi.ProjectSlices(fixtureLocator(t, root)).DefinedBy(test.pattern)

			membership, err := slicing.SelectSliceFiles(nil)
			if err != nil {
				t.Fatalf("resolving %s failed: %v", slicing, err)
			}
			if !maps.EqualFunc(membership, test.want, slices.Equal) {
				t.Errorf("`defined by %q` cuts the project into %v, want %v", test.pattern, membership, test.want)
			}
		})
	}
}

func TestDefinedByRegexReadsTheExpressionAsGoWritesIt(t *testing.T) {
	// The same verb with the pattern read as Go's own regexp syntax, for the slicing a glob cannot express: a
	// character class, and an alternation that slices part of a project and leaves the rest of it unsliced.
	// The expression is anchored at both ends, like every pattern in this library, so it describes the whole
	// identifier and not just the part the slice's name is cut from.
	root := writeSlicedFixtureProject(t)
	tests := []struct {
		name       string
		expression string
		want       map[string][]string
	}{
		{
			name:       "the folder under internal, spelled as a character class",
			expression: `internal/([^/]+)/.*`,
			want: map[string][]string{
				"api":    {"internal/api/handler.go", "internal/api/router.go"},
				"db":     {"internal/db/conn.go"},
				"domain": {"internal/domain/order.go"},
			},
		},
		{
			name:       "an alternation, which slices two folders and leaves the rest of the project unsliced",
			expression: `internal/(api|db)/.*`,
			want: map[string][]string{
				"api": {"internal/api/handler.go", "internal/api/router.go"},
				"db":  {"internal/db/conn.go"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			slicing := fluentapi.ProjectSlices(fixtureLocator(t, root)).DefinedByRegex(test.expression)

			membership, err := slicing.SelectSliceFiles(nil)
			if err != nil {
				t.Fatalf("resolving %s failed: %v", slicing, err)
			}
			if !maps.EqualFunc(membership, test.want, slices.Equal) {
				t.Errorf("`defined by regex %q` cuts the project into %v, want %v", test.expression, membership, test.want)
			}
		})
	}
}

func TestDefinedByRegexReadsAnExpressionAsSuchOnTheZeroBuilderToo(t *testing.T) {
	// The zero SlicesBuilder is the builder ProjectSlices(nil) returns, and this verb is where that could
	// quietly stop being true: the zero RegexFactory is glob syntax, so a regex-syntax factory kept in a field
	// would make `var b SlicesBuilder` compile a regular expression as a glob. The expression below is the
	// witness — `(?:...)` is one group in Go's syntax and two capture delimiters in this library's glob syntax,
	// so read as a glob it is ErrOneCapture and the builder renders as rejected.
	var zero fluentapi.SlicesBuilder

	slicing := zero.DefinedByRegex(`internal/(?:api|db)/([^/]+)\.go`)

	if strings.Contains(slicing.String(), "rejected") {
		t.Errorf("%s was rejected, want the expression read as a regular expression rather than as a glob", slicing)
	}
}

func TestASecondSlicingIsRejectedRatherThanNarrowingTheFirst(t *testing.T) {
	// A slicing is a function from a file to the name of its slice, so a second one is not a narrower rule the
	// way a second scope verb is elsewhere in the family — it is a second vocabulary for the same project, and
	// no reading of the two together is one the user could have meant.
	tests := []struct {
		name          string
		slicing       fluentapi.SlicesBuilder
		wantOperation string
		wantSubject   string
	}{
		{
			name:          "two globs",
			slicing:       fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**").DefinedBy("(*)/**"),
			wantOperation: "defined by",
			wantSubject:   "(*)/**",
		},
		{
			name:          "a glob and then a regular expression",
			slicing:       fluentapi.ProjectSlices(nil).DefinedBy("internal/(**)/**").DefinedByRegex(`([a-z]+)/.*`),
			wantOperation: "defined by regex",
			wantSubject:   `([a-z]+)/.*`,
		},
		{
			name:          "a regular expression and then a glob",
			slicing:       fluentapi.ProjectSlices(nil).DefinedByRegex(`([a-z]+)/.*`).DefinedBy("internal/(**)/**"),
			wantOperation: "defined by",
			wantSubject:   "internal/(**)/**",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.slicing.SelectSliceFiles(nil)

			if !errors.Is(err, fluentapi.ErrSlicedTwice) {
				t.Fatalf("resolving %s returned %v, want ErrSlicedTwice", test.slicing, err)
			}
			rejection := userError(t, err)
			if rejection.Operation != test.wantOperation || rejection.Subject != test.wantSubject {
				t.Errorf("the error blames `%s %q`, want `%s %q`: the slicing verb the user has to delete, with the pattern they typed",
					rejection.Operation, rejection.Subject, test.wantOperation, test.wantSubject)
			}
			if !strings.Contains(test.slicing.String(), "rejected") {
				t.Errorf("%s renders without the rejection, want it visible in a test failure", test.slicing)
			}
		})
	}
}

func TestASlicingWantsExactlyOneCapture(t *testing.T) {
	// A pattern that names nothing cannot say what a slice is called, and one that names two things cannot say
	// which of them was meant. Both are the user's mistake and neither can be guessed at.
	tests := []struct {
		name          string
		slicing       fluentapi.SlicesBuilder
		wantOperation string
		wantSubject   string
	}{
		{
			name:          "a glob with no capture",
			slicing:       fluentapi.ProjectSlices(nil).DefinedBy("internal/**"),
			wantOperation: "defined by",
			wantSubject:   "internal/**",
		},
		{
			name:          "a glob with two",
			slicing:       fluentapi.ProjectSlices(nil).DefinedBy("(*)/(*)/**"),
			wantOperation: "defined by",
			wantSubject:   "(*)/(*)/**",
		},
		{
			name:          "a regular expression with no capture",
			slicing:       fluentapi.ProjectSlices(nil).DefinedByRegex(`internal/.*`),
			wantOperation: "defined by regex",
			wantSubject:   `internal/.*`,
		},
		{
			name:          "a regular expression with two",
			slicing:       fluentapi.ProjectSlices(nil).DefinedByRegex(`(\w+)/(\w+)/.*`),
			wantOperation: "defined by regex",
			wantSubject:   `(\w+)/(\w+)/.*`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.slicing.SelectSliceFiles(nil)

			if !errors.Is(err, matching.ErrOneCapture) {
				t.Errorf("resolving %s returned %v, want matching.ErrOneCapture", test.slicing, err)
			}
			rejection := userError(t, err)
			if rejection.Operation != test.wantOperation || rejection.Subject != test.wantSubject {
				t.Errorf("the error blames `%s %q`, want `%s %q`: the slicing verb at fault, with the pattern the user typed",
					rejection.Operation, rejection.Subject, test.wantOperation, test.wantSubject)
			}
		})
	}
}

func TestASlicingPatternThatWillNotCompileIsReportedBeforeTheProjectIsRead(t *testing.T) {
	// What the user typed is wrong whatever the project turns out to be, and reading the project first would
	// answer a typo with a complaint about the locator.
	slicing := fluentapi.ProjectSlices(&extraction.ProjectLocator{Directory: t.TempDir()}).DefinedBy("internal/([unclosed")

	_, err := slicing.SelectSliceFiles(nil)

	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("resolving %s returned %v, want the rejected pattern rather than the missing project", slicing, err)
	}
	if errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("resolving %s returned %v, want the project left unread", slicing, err)
	}
	rejection := userError(t, err)
	if rejection.Operation != "defined by" || rejection.Subject != "internal/([unclosed" {
		t.Errorf("the error blames `%s %q`, want the verb and the pattern the user typed",
			rejection.Operation, rejection.Subject)
	}
}

func TestARejectedSlicingSlicesNothing(t *testing.T) {
	// A zero Pattern names no slice, so recording a rejected one would report a slicing that found nothing
	// instead of the typo the user has to fix — and a second, valid slicing must not rescue the chain either.
	rejected := fluentapi.ProjectSlices(nil).DefinedBy("internal/([unclosed")
	repaired := rejected.DefinedBy("internal/(**)/**")

	if _, err := rejected.SelectSliceFiles(nil); !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("resolving %s returned %v, want the pattern that did not compile", rejected, err)
	}
	_, err := repaired.SelectSliceFiles(nil)
	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("resolving %s returned %v, want the first mistake rather than the pattern that followed it", repaired, err)
	}
}
