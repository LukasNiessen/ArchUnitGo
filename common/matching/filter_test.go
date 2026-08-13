package matching

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

func TestFilterMatchesPerTarget(t *testing.T) {
	identifier := "internal/api/handler.go"

	tests := []struct {
		name   string
		filter Filter
		want   bool
	}{
		{"filename matcher on the filename", FilenameMatcher(mustGlob(t, "*.go", nil)), true},
		{"filename matcher is not given the path", FilenameMatcher(mustGlob(t, "internal/**", nil)), false},
		{"path matcher on the whole identifier", PathMatcher(mustGlob(t, "internal/**/*.go", nil)), true},
		{"path matcher is not given the filename alone", PathMatcher(mustGlob(t, "handler.go", nil)), false},
		{"folder matcher on the folder", FolderMatcher(mustGlob(t, "internal/api", nil)), true},
		{"folder matcher covering subfolders", FolderMatcher(mustGlob(t, "internal/**", nil)), true},
		{"folder matcher is not given the filename", FolderMatcher(mustGlob(t, "internal/api/*.go", nil)), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.filter.Matches(identifier); got != test.want {
				t.Errorf("(%s).Matches(%q) = %v, want %v", test.filter, identifier, got, test.want)
			}
		})
	}
}

func TestClassnameMatcherIgnoresQualification(t *testing.T) {
	filter := ClassnameMatcher(mustGlob(t, "Handler", nil))

	for _, identifier := range []string{"Handler", "api.Handler", "internal/api.Handler"} {
		if !filter.Matches(identifier) {
			t.Errorf("(%s).Matches(%q) = false, want true", filter, identifier)
		}
	}
	if filter.Matches("internal/api.HandlerFactory") {
		t.Error("a classname filter is anchored to the whole declared name")
	}
}

func TestFilterExcluding(t *testing.T) {
	filter := FilenameMatcher(mustGlob(t, "*.go", nil)).
		Excluding(mustGlob(t, "*_test.go", nil), mustGlob(t, "zz_generated.go", nil))

	if !filter.Matches("internal/api/handler.go") {
		t.Error("an identifier matching no exclusion should still match")
	}
	for _, identifier := range []string{"internal/api/handler_test.go", "internal/api/zz_generated.go"} {
		if filter.Matches(identifier) {
			t.Errorf("(%s).Matches(%q) = true, want false", filter, identifier)
		}
	}
}

func TestFilterExcludingUsesTheFiltersOwnTarget(t *testing.T) {
	// Exclusions look at the same part of the identifier as the main pattern, which for a folder
	// matcher is the folder.
	filter := FolderMatcher(mustGlob(t, "internal/**", nil)).Excluding(mustGlob(t, "internal/legacy/**", nil))

	if !filter.Matches("internal/api/handler.go") {
		t.Error("internal/api/handler.go should match")
	}
	if filter.Matches("internal/legacy/handler.go") {
		t.Error("internal/legacy/handler.go should be excluded")
	}
}

func TestFilterNotMatching(t *testing.T) {
	filter := FilenameMatcher(mustGlob(t, "*_test.go", nil)).NotMatching()

	if !filter.Matches("internal/api/handler.go") {
		t.Error("an inverted filter should accept what the pattern rejects")
	}
	if filter.Matches("internal/api/handler_test.go") {
		t.Error("an inverted filter should reject what the pattern accepts")
	}
	if !filter.NotMatching().Matches("internal/api/handler_test.go") {
		t.Error("inverting twice should return to the original sense")
	}
}

func TestFilterNotMatchingStillHonoursExclusions(t *testing.T) {
	// An exclusion always rejects; it is not part of what the mood inverts.
	filter := FilenameMatcher(mustGlob(t, "*_test.go", nil)).
		Excluding(mustGlob(t, "doc.go", nil)).
		NotMatching()

	if !filter.Matches("internal/api/handler.go") {
		t.Error("handler.go is not a test file, so the inverted filter should accept it")
	}
	if filter.Matches("internal/api/doc.go") {
		t.Error("an excluded identifier is out whatever the mood")
	}
}

func TestFiltersAreImmutable(t *testing.T) {
	base := FilenameMatcher(mustGlob(t, "*.go", nil))
	excluding := base.Excluding(mustGlob(t, "*_test.go", nil))
	inverted := base.NotMatching()

	if !base.Matches("handler_test.go") {
		t.Error("Excluding must not have modified the filter it was called on")
	}
	if !excluding.Matches("handler.go") {
		t.Error("the derived filter should still match what its base matched")
	}
	if inverted.Matches("handler.go") {
		t.Error("NotMatching must return a new filter, not mutate the receiver")
	}
	if base.Target() != TargetFilename || base.Pattern().Source() != "*.go" {
		t.Errorf("the base filter changed: %s", base)
	}
}

func TestZeroFilterMatchesNothing(t *testing.T) {
	var filter Filter

	for _, identifier := range []string{"", ".", "internal/api/handler.go"} {
		if filter.Matches(identifier) {
			t.Errorf("the zero Filter must match nothing, but matched %q", identifier)
		}
	}
}

func TestFilterMatchesNothingWhenTheIdentifierIsEmpty(t *testing.T) {
	filter := PathMatcher(mustGlob(t, "**", nil))

	if filter.Matches("") {
		t.Error("the empty string is the absence of an identifier, so nothing describes it")
	}
}

func TestFilterString(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   string
	}{
		{
			name:   "plain",
			filter: FolderMatcher(mustGlob(t, "internal/**", nil)),
			want:   `path without filename matches "internal/**"`,
		},
		{
			name:   "inverted",
			filter: FilenameMatcher(mustGlob(t, "*_test.go", nil)).NotMatching(),
			want:   `filename does not match "*_test.go"`,
		},
		{
			name:   "with exclusions",
			filter: PathMatcher(mustGlob(t, "**", nil)).Excluding(mustGlob(t, "**/legacy/**", nil), mustGlob(t, "doc.go", nil)),
			want:   `path matches "**", excluding "**/legacy/**", "doc.go"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.filter.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestFilterSelectsNodesOfAFixtureGraph is the level above the unit tests: a filter applied to the
// identifiers of a hand-built graph, which is the shape every rule will use it in.
func TestFilterSelectsNodesOfAFixtureGraph(t *testing.T) {
	graph := extraction.NewGraph(
		extraction.NewEdge("cmd/server/main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/store.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "fmt", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler_test.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.SelfEdge("internal/db/store.go"),
	)

	tests := []struct {
		name   string
		filter Filter
		want   []string
	}{
		{
			name:   "everything under a folder",
			filter: FolderMatcher(mustGlob(t, "internal/**", nil)),
			want:   []string{"internal/api/handler.go", "internal/api/handler_test.go", "internal/db/store.go"},
		},
		{
			name:   "everything outside a folder",
			filter: FolderMatcher(mustGlob(t, "internal/api/**", nil)).NotMatching(),
			want:   []string{"cmd/server/main.go", "fmt", "internal/db/store.go"},
		},
		{
			name:   "a folder, excluding one subfolder",
			filter: FolderMatcher(mustGlob(t, "internal/**", nil)).Excluding(mustGlob(t, "**/db", nil)),
			want:   []string{"internal/api/handler.go", "internal/api/handler_test.go"},
		},
		{
			name:   "entry points by filename",
			filter: FilenameMatcher(mustGlob(t, "main.go", nil)),
			want:   []string{"cmd/server/main.go"},
		},
		{
			name:   "a pattern matching nothing selects nothing",
			filter: FolderMatcher(mustGlob(t, "internal/apis/**", nil)),
			want:   []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected := make([]string, 0, len(test.want))
			for _, node := range graph.Nodes() {
				if test.filter.Matches(node) {
					selected = append(selected, node)
				}
			}
			if !slices.Equal(selected, test.want) {
				t.Errorf("(%s) selected %v, want %v", test.filter, selected, test.want)
			}
		})
	}
}
