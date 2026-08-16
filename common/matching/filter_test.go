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

func TestFilterExcludingMatchersUsesTheExclusionsOwnTarget(t *testing.T) {
	// The other half of an exclusion: a folder selector qualified by an exclusion about filenames, which
	// is `in folder "internal/**" except with name "*_test.go"`.
	filter := FolderMatcher(mustGlob(t, "internal/**", nil)).
		ExcludingMatchers(FilenameMatcher(mustGlob(t, "*_test.go", nil)))

	if !filter.Matches("internal/api/handler.go") {
		t.Error("internal/api/handler.go should match")
	}
	if filter.Matches("internal/api/handler_test.go") {
		t.Error("internal/api/handler_test.go should be excluded by its filename")
	}
	if filter.Matches("internal/legacy/handler_test.go") {
		t.Error("an exclusion is read against the whole identifier, whatever folder the file is in")
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

func TestSiblingFiltersDoNotShareAnExclusionArray(t *testing.T) {
	// TestFiltersAreImmutable checks the parent, which is not enough: it passes even if Excluding
	// drops the slices.Clone, because every append in a chain built through the public API happens
	// to allocate an exactly-fitting array, so the hazard never arises.
	//
	// The hazard is a parent whose exclusion slice has spare capacity. Two siblings then append into
	// the same slot and the second silently overwrites the first. This test therefore builds that
	// precondition directly through the unexported field — creating the hazard is the whole point,
	// and an in-package test is the only thing that can.
	parent := FilenameMatcher(mustGlob(t, "*.go", nil))
	parent.exclusions = append(make([]Filter, 0, 4), FilenameMatcher(mustGlob(t, "zz_generated.go", nil)))

	left := parent.Excluding(mustGlob(t, "*_test.go", nil))
	right := parent.Excluding(mustGlob(t, "mock_*.go", nil))

	if left.Matches("handler_test.go") {
		t.Error("left should exclude *_test.go")
	}
	if right.Matches("mock_handler.go") {
		t.Error("right should exclude mock_*.go")
	}
	if !left.Matches("mock_handler.go") {
		t.Error("left can see right's exclusion: the two share a backing array")
	}
	if !right.Matches("handler_test.go") {
		t.Error("right can see left's exclusion: the two share a backing array")
	}
	if !parent.Matches("handler_test.go") || !parent.Matches("mock_handler.go") {
		t.Error("the parent gained an exclusion from one of its children")
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

func TestFilterNormalisesTheIdentifierBeforeExtracting(t *testing.T) {
	// Filter.Matches normalises separators before the target extracts its part of the identifier, and
	// that normalisation is invisible to every other test here: Pattern.Matches normalises a second
	// time, so a backslash identifier still matches at the Pattern level even if the Filter forgot.
	// Only a Filter whose target slices the identifier can tell the difference — drop the call in
	// Filter.Matches and path.Base returns the whole backslash string while path.Dir returns ".".
	windows := `internal\api\handler.go`

	for _, filter := range []Filter{
		FilenameMatcher(mustGlob(t, "*.go", nil)),
		FolderMatcher(mustGlob(t, "internal/api", nil)),
		PathMatcher(mustGlob(t, "internal/**", nil)),
	} {
		if !filter.Matches(windows) {
			t.Errorf("(%s).Matches(%q) = false: the identifier was not normalised before extraction", filter, windows)
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
		{
			name: "with a targeted exclusion",
			filter: FolderMatcher(mustGlob(t, "app/**", nil)).
				ExcludingMatchers(FilenameMatcher(mustGlob(t, "*_gen.go", nil))),
			want: `path without filename matches "app/**", excluding filename matches "*_gen.go"`,
		},
		{
			// A bare pattern written after a targeted exclusion has to repeat the word too, or the folder
			// pattern below reads as a second filename the `*_gen.go` exclusion is about.
			name: "with a bare exclusion after a targeted one",
			filter: FolderMatcher(mustGlob(t, "app/**", nil)).
				ExcludingMatchers(FilenameMatcher(mustGlob(t, "*_gen.go", nil))).
				Excluding(mustGlob(t, "**/generated", nil)),
			want: `path without filename matches "app/**", ` +
				`excluding filename matches "*_gen.go", excluding "**/generated"`,
		},
		{
			// And the bare patterns on either side of it are still each listed once: the word is repeated
			// where the kind of exclusion changes and nowhere else.
			name: "with bare exclusions on both sides of a targeted one",
			filter: FolderMatcher(mustGlob(t, "app/**", nil)).
				Excluding(mustGlob(t, "**/legacy", nil), mustGlob(t, "**/vendor", nil)).
				ExcludingMatchers(FilenameMatcher(mustGlob(t, "*_gen.go", nil))).
				Excluding(mustGlob(t, "**/generated", nil), mustGlob(t, "**/mocks", nil)),
			want: `path without filename matches "app/**", ` +
				`excluding "**/legacy", "**/vendor", excluding filename matches "*_gen.go", ` +
				`excluding "**/generated", "**/mocks"`,
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
