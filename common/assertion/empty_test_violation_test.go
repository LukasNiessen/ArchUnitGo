package assertion

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

func TestEmptyTestViolationCarriesTheSelectorsThatMatchedNothing(t *testing.T) {
	folder := matching.FolderMatcher(mustGlob(t, "internal/apis/**"))
	filename := matching.FilenameMatcher(mustGlob(t, "*.go"))

	violation := NewEmptyTestViolation("files", folder, filename)

	if violation.Kind() != KindEmptyTest {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), KindEmptyTest)
	}
	if violation.Subject != "files" {
		t.Errorf("Subject = %q, want %q", violation.Subject, "files")
	}
	if len(violation.Selectors) != 2 {
		t.Fatalf("Selectors = %v, want the two filters the rule was built from", violation.Selectors)
	}
	// The pattern the user typed, and the part of an identifier it was matched against: enough for
	// the testing layer to say which selector was wrong, without the violation phrasing anything.
	if source := violation.Selectors[0].Pattern().Source(); source != "internal/apis/**" {
		t.Errorf("Selectors[0] pattern source = %q, want %q", source, "internal/apis/**")
	}
	if target := violation.Selectors[0].Target(); target != matching.TargetPathWithoutFilename {
		t.Errorf("Selectors[0] target = %q, want %q", target, matching.TargetPathWithoutFilename)
	}
	if source := violation.Selectors[1].Pattern().Source(); source != "*.go" {
		t.Errorf("Selectors[1] pattern source = %q, want %q", source, "*.go")
	}
}

func TestEmptyTestViolationRendersItselfForALogLine(t *testing.T) {
	// Every violation type in the library renders itself for a log line and a test failure, and this is the
	// shape they share: the subject that disagreed with the rule, the requirement in the words the rule was
	// written in, then what was found.
	tests := []struct {
		name      string
		violation EmptyTestViolation
		want      string
	}{
		{
			name:      "the selection that came to nothing",
			violation: NewEmptyTestViolation("files", matching.FolderMatcher(mustGlob(t, "internal/apis/**"))),
			want:      `files: path without filename matches "internal/apis/**" -> nothing`,
		},
		{
			name: "every selector, in the order the user chained them",
			violation: NewEmptyTestViolation("files",
				matching.FolderMatcher(mustGlob(t, "internal/**")),
				matching.FilenameMatcher(mustGlob(t, "*_repository.go"))),
			want: `files: path without filename matches "internal/**", filename matches "*_repository.go" -> nothing`,
		},
		{
			name:      "a rule the guard was told nothing about the selection of",
			violation: NewEmptyTestViolation("files"),
			want:      "files -> nothing",
		},
		{
			name:      "and one it was told nothing at all about",
			violation: NewEmptyTestViolation(""),
			want:      "the rule's subject -> nothing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.violation.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEmptyTestViolationCopiesItsSelectors(t *testing.T) {
	// Spreading a slice into a variadic parameter hands over the caller's backing array, so without
	// the clone the caller can still rewrite a violation it has already reported. Spare capacity is
	// not needed for the hazard — the shared array is.
	selectors := []matching.Filter{matching.FolderMatcher(mustGlob(t, "internal/apis/**"))}

	violation := NewEmptyTestViolation("files", selectors...)
	selectors[0] = matching.FilenameMatcher(mustGlob(t, "*.go"))

	if source := violation.Selectors[0].Pattern().Source(); source != "internal/apis/**" {
		t.Errorf("the violation's selector changed with the caller's slice: %q", source)
	}
}

func TestGatherEmptyTestViolations(t *testing.T) {
	selector := matching.FolderMatcher(mustGlob(t, "internal/apis/**"))

	tests := []struct {
		name    string
		matched int
		options *EmptyTestOptions
		want    bool
	}{
		{
			name:    "a rule that selected something is not an empty test",
			matched: 3,
			options: &EmptyTestOptions{Subject: "files", Selectors: []matching.Filter{selector}},
			want:    false,
		},
		{
			name:    "one match is enough",
			matched: 1,
			options: &EmptyTestOptions{Subject: "files"},
			want:    false,
		},
		{
			name:    "zero matches is a violation, not a pass",
			matched: 0,
			options: &EmptyTestOptions{Subject: "files", Selectors: []matching.Filter{selector}},
			want:    true,
		},
		{
			name:    "allowEmptyTests is how a user opts out",
			matched: 0,
			options: &EmptyTestOptions{Subject: "files", Selectors: []matching.Filter{selector}, AllowEmptyTests: true},
			want:    false,
		},
		{
			name:    "nil options means the guard is on",
			matched: 0,
			options: nil,
			want:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GatherEmptyTestViolations(test.matched, test.options)

			// An empty list is the pass: there is no boolean beside it.
			if (len(got) > 0) != test.want {
				t.Fatalf("GatherEmptyTestViolations(%d, %+v) = %v, want a violation: %v", test.matched, test.options, got, test.want)
			}
			if !test.want {
				return
			}
			if len(got) != 1 {
				t.Fatalf("an empty test is one violation, got %d", len(got))
			}
			violation, ok := got[0].(EmptyTestViolation)
			if !ok {
				t.Fatalf("got %T, want an EmptyTestViolation", got[0])
			}
			if test.options == nil {
				return
			}
			if violation.Subject != test.options.Subject {
				t.Errorf("Subject = %q, want %q", violation.Subject, test.options.Subject)
			}
			if len(violation.Selectors) != len(test.options.Selectors) {
				t.Errorf("Selectors = %v, want the rule's %v", violation.Selectors, test.options.Selectors)
			}
		})
	}
}

func TestGatherEmptyTestViolationsTreatsANegativeCountAsNoMatches(t *testing.T) {
	// Nothing should produce one, but a count that is not positive is not a selection, and silently
	// passing is the outcome this guard exists to prevent.
	if got := GatherEmptyTestViolations(-1, nil); len(got) != 1 {
		t.Errorf("GatherEmptyTestViolations(-1, nil) = %v, want one violation", got)
	}
}

// TestEmptyTestGuardOnAFixtureGraph is the level above the unit tests: the guard where every terminal
// will call it, over the nodes a filter selected from a hand-built graph.
func TestEmptyTestGuardOnAFixtureGraph(t *testing.T) {
	graph := extraction.NewGraph(
		extraction.NewEdge("cmd/server/main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/store.go", false, extraction.ImportKindPlain),
		extraction.SelfEdge("internal/db/store.go"),
	)

	tests := []struct {
		name     string
		selector matching.Filter
		want     bool
	}{
		{
			name:     "a scope that selects files has nothing to report",
			selector: matching.FolderMatcher(mustGlob(t, "internal/**")),
			want:     false,
		},
		{
			name:     "a typo in the folder name selects nothing, and that is the violation",
			selector: matching.FolderMatcher(mustGlob(t, "internal/apis/**")),
			want:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched := 0
			for _, node := range graph.Nodes() {
				if test.selector.Matches(node) {
					matched++
				}
			}

			violations := GatherEmptyTestViolations(matched, &EmptyTestOptions{
				Subject:   "files",
				Selectors: []matching.Filter{test.selector},
			})

			if (len(violations) > 0) != test.want {
				t.Fatalf("(%s) selected %d nodes and gathered %v, want a violation: %v", test.selector, matched, violations, test.want)
			}
			if !test.want {
				return
			}
			violation, ok := violations[0].(EmptyTestViolation)
			if !ok {
				t.Fatalf("got %T, want an EmptyTestViolation", violations[0])
			}
			if violation.Selectors[0].Pattern().Source() != test.selector.Pattern().Source() {
				t.Errorf("the violation quotes %q, want the selector the user wrote, %q",
					violation.Selectors[0].Pattern().Source(), test.selector.Pattern().Source())
			}
		})
	}
}

func mustGlob(t *testing.T, glob string) matching.Pattern {
	t.Helper()

	pattern, err := matching.NewGlobPattern(glob, nil)
	if err != nil {
		t.Fatalf("NewGlobPattern(%q, nil) failed: %v", glob, err)
	}
	return pattern
}
