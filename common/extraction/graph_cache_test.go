package extraction

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// cachedGraph runs one project through a cache and fails the test if it could not be extracted.
func cachedGraph(t *testing.T, cache *graphCache, root string, options *SourceOptions) Graph {
	t.Helper()

	graph, err := cache.graph(root, options)
	if err != nil {
		t.Fatalf("the cache failed to extract %q: %v", root, err)
	}
	return graph
}

// oneFileProject is the smallest project an extraction has anything to say about: a module and a file
// that is a node of it.
func oneFileProject() map[string]string {
	return map[string]string{"main.go": "package main\n\nfunc main() {}\n"}
}

// addSourceFile writes a second file into an already extracted project, which is how a test makes the
// source disagree with a cached graph. It imports nothing, so the only difference it makes to a graph
// is its own node.
func addSourceFile(t *testing.T, root string) {
	t.Helper()

	writeProjectFile(t, root, "later.go", "package main\n\nfunc later() {}\n")
}

func TestGraphCacheExtractsOneProjectOnce(t *testing.T) {
	var cache graphCache
	root := writeSourceProject(t, oneFileProject())

	first := cachedGraph(t, &cache, root, nil)
	addSourceFile(t, root)
	second := cachedGraph(t, &cache, root, nil)

	// The graph is memoised, so the second ask does not read the source at all — which is exactly why
	// the file written between the two asks must be absent from it. Extraction is the expensive half of
	// a check and a suite runs dozens of rules over one project; this is what makes the second rule
	// onwards a map lookup.
	if want := NewGraph(SelfEdge("main.go")); !slices.Equal(first, want) {
		t.Fatalf("graph =\n%s\n\nwant\n%s", first, want)
	}
	if !slices.Equal(second, first) {
		t.Errorf("the second extraction of one project returned\n%s\n\nwant the cached\n%s", second, first)
	}
}

func TestGraphCacheClearMakesTheNextAskExtractAgain(t *testing.T) {
	var cache graphCache
	root := writeSourceProject(t, oneFileProject())

	cachedGraph(t, &cache, root, nil)
	addSourceFile(t, root)
	cache.clear()
	afterClear := cachedGraph(t, &cache, root, nil)

	// The escape hatch's whole job: the source moved underneath the library, and the graph has to be
	// read from it again rather than from the memo.
	want := NewGraph(SelfEdge("main.go"), SelfEdge("later.go"))
	if !slices.Equal(afterClear, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", afterClear, want)
	}
}

func TestGraphCacheTellsTwoAnalysesOfOneProjectApart(t *testing.T) {
	var cache graphCache
	root := writeSourceProject(t, map[string]string{
		"main.go":      "package main\n\nfunc main() {}\n",
		"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(*testing.T) {}\n",
	})

	production := cachedGraph(t, &cache, root, nil)
	withTests := cachedGraph(t, &cache, root, &SourceOptions{IncludeTestFiles: true})

	// The failure mode the key exists to prevent: one project asked about twice under different options
	// must not be answered from one entry, because the second rule would then be judging source that was
	// never read that way.
	if want := NewGraph(SelfEdge("main.go")); !slices.Equal(production, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", production, want)
	}
	want := NewGraph(
		SelfEdge("main.go"),
		SelfEdge("main_test.go"),
		NewEdge("main_test.go", "testing", true, ImportKindPlain),
	)
	if !slices.Equal(withTests, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", withTests, want)
	}
}

func TestGraphCacheKeepsOneEntryForTwoSpellingsOfOneRoot(t *testing.T) {
	var cache graphCache
	root := writeSourceProject(t, oneFileProject())

	first := cachedGraph(t, &cache, root, nil)
	addSourceFile(t, root)
	// The same directory, spelled the way a locator inside the project resolves it: cleaning happens
	// before the key is built, so this is a hit rather than a second entry for one project.
	detour := filepath.Join(root, "internal") + string(filepath.Separator) + ".."
	second := cachedGraph(t, &cache, detour, nil)

	if !slices.Equal(second, first) {
		t.Errorf("asking about %q returned\n%s\n\nwant the graph already cached for the same project\n%s", detour, second, first)
	}
}

func TestGraphCacheHandsOutACopyOfTheGraph(t *testing.T) {
	var cache graphCache
	root := writeSourceProject(t, oneFileProject())

	invented := NewEdge("main.go", "example.com/invented", true, ImportKindPlain)
	want := NewGraph(SelfEdge("main.go"))

	// A Graph is a slice, so a cache sharing the one it holds would let any reader write through into
	// every later reader's graph — a whole suite judging a graph one rule edited. Both halves matter: the
	// graph the extraction produced is the caller's, and so is every copy handed out after it.
	extracted := cachedGraph(t, &cache, root, nil)
	extracted[0] = invented
	fromCache := cachedGraph(t, &cache, root, nil)
	if !slices.Equal(fromCache, want) {
		t.Errorf("after writing through the extracted graph, the cache holds\n%s\n\nwant\n%s", fromCache, want)
	}

	fromCache[0] = invented
	again := cachedGraph(t, &cache, root, nil)
	if !slices.Equal(again, want) {
		t.Errorf("after writing through a cached graph, the cache holds\n%s\n\nwant\n%s", again, want)
	}
}

func TestGraphCacheDoesNotCacheAFailure(t *testing.T) {
	var cache graphCache
	root := writeSourceProject(t, oneFileProject())
	// A directory that resolves as a root and holds source, so that the failure happens inside the
	// extraction rather than before the key is built — which is the only way a memoised failure would be
	// visible at all. Its module file is not one, so the toolchain refuses the project.
	nested := filepath.Join(root, "later")
	writeProjectFile(t, nested, moduleFileName, "not a module file\n")
	writeProjectFile(t, nested, "later.go", "package later\n")

	if _, err := cache.graph(nested, nil); err == nil {
		t.Fatalf("the cache extracted %q, whose module file is not one", nested)
	}
	writeProjectFile(t, nested, moduleFileName, "module example.com/later\n\ngo 1.26\n")
	graph := cachedGraph(t, &cache, nested, nil)

	// An extraction fails because of the environment, and the next rule in the suite deserves the same
	// chance rather than a memoised error.
	if want := NewGraph(SelfEdge("later.go")); !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
}

func TestCachedGraphAndClearGraphCacheAreTheProcessWideMemo(t *testing.T) {
	// The two public functions over the shared cache, in the shape a suite uses them: every rule shares
	// one graph, and clearing is what a test that writes its own fixture does between two of them.
	t.Cleanup(ClearGraphCache)
	ClearGraphCache()
	root := writeSourceProject(t, oneFileProject())

	first, err := CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph failed: %v", err)
	}
	addSourceFile(t, root)
	second, err := CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph failed: %v", err)
	}
	if !slices.Equal(second, first) {
		t.Errorf("a second check of one project extracted\n%s\n\nwant the shared\n%s", second, first)
	}

	ClearGraphCache()
	afterClear, err := CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph failed: %v", err)
	}
	want := NewGraph(SelfEdge("main.go"), SelfEdge("later.go"))
	if !slices.Equal(afterClear, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", afterClear, want)
	}
}

func TestCachedGraphRejectsARootThatIsNotAProject(t *testing.T) {
	root := writeSourceProject(t, oneFileProject())

	_, err := CachedGraph(filepath.Join(root, "main.go"), nil)

	// The root is resolved before it is keyed on, so a root the extractor would reject is rejected here
	// too, with the same error rather than a cache miss on a nonsensical key.
	if !errors.Is(err, ErrNotADirectory) {
		t.Errorf("CachedGraph error = %v, want it to wrap ErrNotADirectory", err)
	}
}

func TestClearGraphCacheOnAnEmptyCacheDoesNothing(t *testing.T) {
	var cache graphCache

	cache.clear()

	if _, found := cache.lookup(graphCacheKey("/project", nil)); found {
		t.Error("an empty cache holds an entry")
	}
}

func TestGraphCacheKeyIsOneKeyForEverySpellingOfTheDefaults(t *testing.T) {
	defaults := graphCacheKey("/project", nil)

	tests := []struct {
		name    string
		options *SourceOptions
	}{
		{name: "the zero bag", options: &SourceOptions{}},
		{name: "the defaults written out", options: &SourceOptions{ExcludedFolders: DefaultExcludedFolders()}},
		{
			name:    "the defaults in another order",
			options: &SourceOptions{ExcludedFolders: reversed(DefaultExcludedFolders())},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Three ways of asking for the same analysis. A key that told them apart would extract one
			// project twice and lose the whole point of the cache.
			if got := graphCacheKey("/project", test.options); got != defaults {
				t.Errorf("key =\n%s\n\nwant the key of a nil options bag\n%s", got, defaults)
			}
		})
	}
}

func TestGraphCacheKeyChangesWithEveryInputThatChangesTheGraph(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		options *SourceOptions
	}{
		{name: "the defaults", root: "/project", options: nil},
		{name: "another project", root: "/elsewhere", options: nil},
		{name: "the test files", root: "/project", options: &SourceOptions{IncludeTestFiles: true}},
		{name: "a build tag", root: "/project", options: &SourceOptions{BuildTags: []string{"integration"}}},
		{name: "another build tag", root: "/project", options: &SourceOptions{BuildTags: []string{"e2e"}}},
		{name: "one more exclusion", root: "/project", options: &SourceOptions{
			ExcludedFolders: append(DefaultExcludedFolders(), "generated"),
		}},
		{name: "no exclusion at all", root: "/project", options: &SourceOptions{ExcludedFolders: []string{}}},
		{name: "an import kind dropped", root: "/project", options: &SourceOptions{
			IgnoredImportKinds: NewImportKindSet(ImportKindBlank),
		}},
		{name: "another import kind dropped", root: "/project", options: &SourceOptions{
			IgnoredImportKinds: NewImportKindSet(ImportKindDot),
		}},
	}

	keys := make(map[string]string, len(tests))
	for _, test := range tests {
		key := graphCacheKey(test.root, test.options)
		if owner, found := keys[key]; found {
			t.Errorf("%q and %q share the cache key\n%s", owner, test.name, key)
			continue
		}
		keys[key] = test.name
	}
}

func TestGraphCacheKeyNamesEveryFieldOfTheSourceOptions(t *testing.T) {
	// The tripwire. Forgetting an input in graphCacheKey is the one way this cache can be wrong — it
	// hands a rule the graph of a different analysis of the same project — so a field added to
	// SourceOptions has to fail a test rather than pass review.
	defaults := graphCacheKey("/project", nil)
	options := reflect.TypeFor[SourceOptions]()

	for index := range options.NumField() {
		field := options.Field(index)
		t.Run(field.Name, func(t *testing.T) {
			varied := &SourceOptions{}
			if !varyField(reflect.ValueOf(varied).Elem().Field(index)) {
				t.Fatalf("this test cannot vary a field of kind %s: teach it how, and make sure graphCacheKey names %s",
					field.Type.Kind(), field.Name)
			}
			if got := graphCacheKey("/project", varied); got == defaults {
				t.Errorf("%s does not reach the cache key: it is still\n%s", field.Name, defaults)
			}
		})
	}
}

// reversed is a folder list in the other order. Order says nothing about what a walk excludes, so the
// two lists are one analysis and have to be one key.
func reversed(folders []string) []string {
	other := slices.Clone(folders)
	slices.Reverse(other)
	return other
}

// varyField sets a field of a zero SourceOptions to something the defaults are not, reporting false for
// a kind it has no value for — which is the tripwire above going off.
func varyField(field reflect.Value) bool {
	switch field.Kind() {
	case reflect.Bool:
		field.SetBool(true)
	case reflect.Uint8: // an ImportKindSet, which is a bit set over a uint8
		field.SetUint(1)
	case reflect.Slice:
		if field.Type().Elem().Kind() != reflect.String {
			return false
		}
		field.Set(reflect.ValueOf([]string{"archunit"}))
	default:
		return false
	}
	return true
}
