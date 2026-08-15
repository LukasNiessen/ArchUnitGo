package matching

import (
	"errors"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

func TestExceptingQualifiesTheLastSelector(t *testing.T) {
	selectors := []Filter{
		FolderMatcher(mustGlob(t, "internal/**", nil)),
		FilenameMatcher(mustGlob(t, "*.go", nil)),
	}

	excepted, err := Excepting(selectors, NewRegexFactory(nil), []string{"*_test.go"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	want := []string{
		`path without filename matches "internal/**"`,
		`filename matches "*.go", excluding "*_test.go"`,
	}
	if got := render(excepted); !slices.Equal(got, want) {
		t.Errorf("Excepting produced %v, want %v", got, want)
	}
}

func TestExceptingReadsAPlainPatternAgainstTheQualifiedSelectorsTarget(t *testing.T) {
	// A plain exclusion inherits the target of the selector it qualifies, so the same pattern means a
	// folder after `in folder` and a filename after `with name`.
	factory := NewRegexFactory(nil)

	folder, err := Excepting([]Filter{FolderMatcher(mustGlob(t, "internal/**", nil))}, factory, []string{"**/legacy"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}
	name, err := Excepting([]Filter{FilenameMatcher(mustGlob(t, "*.go", nil))}, factory, []string{"*_test.go"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	if folder[0].Matches("internal/legacy/store.go") {
		t.Errorf("(%s) should exclude a file of the excluded folder", folder[0])
	}
	if !folder[0].Matches("internal/api/handler.go") {
		t.Errorf("(%s) should still match a file outside it", folder[0])
	}
	if name[0].Matches("internal/api/handler_test.go") {
		t.Errorf("(%s) should exclude the test file", name[0])
	}
	if !name[0].Matches("internal/api/handler.go") {
		t.Errorf("(%s) should still match the file beside it", name[0])
	}
}

func TestExceptingTakesTheTargetTheCallerNames(t *testing.T) {
	selectors := []Filter{FolderMatcher(mustGlob(t, "internal/**", nil))}

	excepted, err := Excepting(selectors, NewRegexFactory(nil), []string{"*_test.go"}, FilenameMatcher)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	if excepted[0].Matches("internal/api/handler_test.go") {
		t.Errorf("(%s) should exclude the file by its name", excepted[0])
	}
	if !excepted[0].Matches("internal/api/handler.go") {
		t.Errorf("(%s) should match the file beside it", excepted[0])
	}
}

func TestExceptingCompilesEveryPatternInTheFactorysSyntax(t *testing.T) {
	regex := NewRegexFactory(&RegexFactoryOptions{Syntax: SyntaxRegex})
	pattern, patternErr := NewRegexPattern(".*", nil)
	if patternErr != nil {
		t.Fatalf("compiling the selector's own pattern failed: %v", patternErr)
	}
	selectors := []Filter{PathMatcher(pattern)}

	excepted, err := Excepting(selectors, regex, []string{`.*_test\.go`, `.*/mock/.*`}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	for _, excluded := range []string{"internal/api/handler_test.go", "internal/mock/db.go"} {
		if excepted[0].Matches(excluded) {
			t.Errorf("(%s) should exclude %q", excepted[0], excluded)
		}
	}
	if !excepted[0].Matches("internal/api/handler.go") {
		t.Errorf("(%s) should match what no exclusion names", excepted[0])
	}
}

func TestExceptingIsRepeatable(t *testing.T) {
	factory := NewRegexFactory(nil)
	selectors := []Filter{FilenameMatcher(mustGlob(t, "*.go", nil))}

	once, err := Excepting(selectors, factory, []string{"*_test.go"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}
	twice, err := Excepting(once, factory, []string{"doc.go"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	want := `filename matches "*.go", excluding "*_test.go", "doc.go"`
	if got := twice[0].String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	for _, excluded := range []string{"api/handler_test.go", "api/doc.go"} {
		if twice[0].Matches(excluded) {
			t.Errorf("(%s) should exclude %q", twice[0], excluded)
		}
	}
}

func TestExceptingLeavesTheSelectorsItWasGivenAlone(t *testing.T) {
	// The chain a builder holds must not change when a copy of it gains an exclusion, and the hazard is
	// a slice with spare capacity: two siblings would otherwise write into the same slot.
	selectors := append(make([]Filter, 0, 4), FilenameMatcher(mustGlob(t, "*.go", nil)))

	left, err := Excepting(selectors, NewRegexFactory(nil), []string{"*_test.go"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}
	right, err := Excepting(selectors, NewRegexFactory(nil), []string{"doc.go"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	if !selectors[0].Matches("api/handler_test.go") {
		t.Errorf("(%s) gained an exclusion from a chain derived from it", selectors[0])
	}
	if !left[0].Matches("api/doc.go") {
		t.Errorf("(%s) can see the sibling chain's exclusion", left[0])
	}
	if !right[0].Matches("api/handler_test.go") {
		t.Errorf("(%s) can see the sibling chain's exclusion", right[0])
	}
}

func TestExceptingRejectsAnExclusionThatQualifiesNothing(t *testing.T) {
	_, err := Excepting(nil, NewRegexFactory(nil), []string{"**/generated/**"}, nil)

	if !errors.Is(err, ErrExclusionWithoutSelector) {
		t.Errorf("Excepting error = %v, want ErrExclusionWithoutSelector", err)
	}
}

func TestExceptingRejectsAnExclusionWithNoPattern(t *testing.T) {
	selectors := []Filter{FolderMatcher(mustGlob(t, "internal/**", nil))}

	_, err := Excepting(selectors, NewRegexFactory(nil), nil, nil)

	if !errors.Is(err, ErrExclusionWithoutPattern) {
		t.Errorf("Excepting error = %v, want ErrExclusionWithoutPattern", err)
	}
}

func TestExceptingRejectsAnExclusionAboutAnotherPopulation(t *testing.T) {
	// An exclusion is read against the identifier its selector is read against, and a classname is not a
	// path: asked for the folder of `internal/api.UserService` this would answer `internal`, which is worse
	// than matching nothing because nothing about the rule would look wrong.
	classes := []Filter{ClassnameMatcher(mustGlob(t, "*Service", nil))}
	files := []Filter{FolderMatcher(mustGlob(t, "internal/**", nil))}

	_, folderOfAClass := Excepting(classes, NewRegexFactory(nil), []string{"internal/legacy"}, FolderMatcher)
	_, classOfAFile := Excepting(files, NewRegexFactory(nil), []string{"*Service"}, ClassnameMatcher)

	if !errors.Is(folderOfAClass, ErrExclusionOfAnotherPopulation) {
		t.Errorf("Excepting error = %v, want ErrExclusionOfAnotherPopulation", folderOfAClass)
	}
	if !errors.Is(classOfAFile, ErrExclusionOfAnotherPopulation) {
		t.Errorf("Excepting error = %v, want ErrExclusionOfAnotherPopulation", classOfAFile)
	}
}

func TestExceptingTakesAnExclusionOfTheSamePopulation(t *testing.T) {
	// The other side of the guard, so it cannot pass by refusing everything: a class selector excepting a
	// class, and a file selector excepting a part of a path, are both what the verb is for.
	classes, err := Excepting([]Filter{ClassnameMatcher(mustGlob(t, "*Service", nil))}, NewRegexFactory(nil),
		[]string{"*TestService"}, ClassnameMatcher)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}
	files, err := Excepting([]Filter{FolderMatcher(mustGlob(t, "internal/**", nil))}, NewRegexFactory(nil),
		[]string{"*_test.go"}, FilenameMatcher)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	if classes[0].Matches("internal/api.UserTestService") {
		t.Errorf("(%s) should exclude the class it names", classes[0])
	}
	if !classes[0].Matches("internal/api.UserService") {
		t.Errorf("(%s) should still match the class beside it", classes[0])
	}
	if files[0].Matches("internal/api/handler_test.go") {
		t.Errorf("(%s) should exclude the file it names", files[0])
	}
}

func TestExceptingRejectsAPatternThatWillNotCompile(t *testing.T) {
	selectors := []Filter{FolderMatcher(mustGlob(t, "internal/**", nil))}

	_, err := Excepting(selectors, NewRegexFactory(nil), []string{"internal/[legacy"}, nil)

	if !errors.Is(err, ErrInvalidPattern) {
		t.Errorf("Excepting error = %v, want ErrInvalidPattern", err)
	}
}

// TestExceptingSelectsNodesOfAFixtureGraph is the level above the unit tests: the sentence the issue
// asks for — everything under a folder, but not the generated part of it — applied to the identifiers of
// a hand-built graph, which is the shape every rule will use it in.
func TestExceptingSelectsNodesOfAFixtureGraph(t *testing.T) {
	graph := extraction.NewGraph(
		extraction.SelfEdge("app/api/handler.go"),
		extraction.SelfEdge("app/api/handler_test.go"),
		extraction.SelfEdge("app/generated/schema.go"),
		extraction.SelfEdge("cmd/server/main.go"),
	)
	factory := NewRegexFactory(nil)
	folder := []Filter{FolderMatcher(mustGlob(t, "app/**", nil))}

	plain, err := Excepting(folder, factory, []string{"**/generated"}, nil)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}
	targeted, err := Excepting(folder, factory, []string{"*_test.go"}, FilenameMatcher)
	if err != nil {
		t.Fatalf("Excepting failed: %v", err)
	}

	tests := []struct {
		name   string
		filter Filter
		want   []string
	}{
		{
			name:   "a folder, except one folder under it",
			filter: plain[0],
			want:   []string{"app/api/handler.go", "app/api/handler_test.go"},
		},
		{
			name:   "a folder, except the files named a certain way",
			filter: targeted[0],
			want:   []string{"app/api/handler.go", "app/generated/schema.go"},
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

// render is every filter of a chain as it describes itself, which is what a test comparing two chains
// needs: a Filter holds a compiled regexp, so two of them are never equal by value.
func render(selectors []Filter) []string {
	rendered := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		rendered = append(rendered, selector.String())
	}
	return rendered
}
