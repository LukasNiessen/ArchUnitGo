package fluentapi_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

func TestNilCheckOptionsMeansTheDefaults(t *testing.T) {
	var options *fluentapi.CheckOptions

	// Every knob's default is its zero value, so a nil bag and the zero bag are the same check. The
	// comparison is DeepEqual rather than == because BuildTags makes CheckOptions uncomparable.
	if got := options.WithDefaults(); !reflect.DeepEqual(got, fluentapi.CheckOptions{}) {
		t.Errorf("(*CheckOptions)(nil).WithDefaults() = %+v, want the zero options", got)
	}
	// The defaults spelled out, because they are a promise: strict about empty selections, silent,
	// free to use a cached graph, production code only, no import kind dropped, host build tags.
	defaults := options.WithDefaults()
	if defaults.AllowEmptyTests {
		t.Error("AllowEmptyTests defaults to true; zero matches must be a violation unless asked otherwise")
	}
	if defaults.ClearCache {
		t.Error("ClearCache defaults to true")
	}
	if defaults.IncludeTestFiles {
		t.Error("IncludeTestFiles defaults to true")
	}
	if !defaults.IgnoredImportKinds.Empty() {
		t.Errorf("IgnoredImportKinds defaults to %v, want no kind dropped", defaults.IgnoredImportKinds)
	}
	if defaults.BuildTags != nil {
		t.Errorf("BuildTags defaults to %v, want the toolchain's own", defaults.BuildTags)
	}
	if defaults.IgnoreScopes != nil {
		t.Errorf("IgnoreScopes defaults to %v, want no scoped ignore directive honored", defaults.IgnoreScopes)
	}
	if got := options.LogWriter(); got != nil {
		t.Errorf("LogWriter() = %v, want nil: a library logs nowhere until a writer is injected", got)
	}
	if options.IgnoresImportKind(extraction.ImportKindBlank) {
		t.Error("a nil options bag ignores blank imports; it should ignore nothing")
	}
}

func TestCheckOptionsWithDefaultsIsACopy(t *testing.T) {
	// Every field is set to a non-zero value, so that a resolution which drops one is visible. The two
	// slices are the fields a struct copy does not separate: the copy shares their backing arrays.
	options := &fluentapi.CheckOptions{
		AllowEmptyTests:    true,
		Logging:            &bytes.Buffer{},
		ClearCache:         true,
		IgnoreScopes:       []string{"layers"},
		IncludeTestFiles:   true,
		IgnoredImportKinds: extraction.NewImportKindSet(extraction.ImportKindBlank),
		BuildTags:          []string{"integration"},
	}

	resolved := options.WithDefaults()

	// WithDefaults is where every terminal resolves its options, so it must carry all of them across,
	// not just the ones a default happens to be spelled out for.
	if !reflect.DeepEqual(resolved, *options) {
		t.Errorf("WithDefaults() = %+v, want the caller's own options %+v", resolved, *options)
	}

	resolved.BuildTags[0] = "e2e"
	resolved.IgnoreScopes[0] = "slices"

	// A terminal resolves the bag once and may well pass the value around; the user's own options,
	// which a stored half-built rule shares, must not move underneath them.
	if options.BuildTags[0] != "integration" {
		t.Errorf("the caller's build tags changed with the resolved copy: %v", options.BuildTags)
	}
	if options.IgnoreScopes[0] != "layers" {
		t.Errorf("the caller's ignore scopes changed with the resolved copy: %v", options.IgnoreScopes)
	}
}

func TestCheckOptionsWithDefaultsCopiesAreIndependentOfEachOther(t *testing.T) {
	// Spare capacity in the parent slice is what makes the sibling case bite: without a copy, both
	// appends write the same array slot, and the second one wins for both.
	options := &fluentapi.CheckOptions{
		IgnoreScopes: append(make([]string, 0, 4), "layers"),
		BuildTags:    append(make([]string, 0, 4), "integration"),
	}

	first := options.WithDefaults()
	second := options.WithDefaults()
	first.BuildTags = append(first.BuildTags, "linux")
	second.BuildTags = append(second.BuildTags, "darwin")
	first.IgnoreScopes = append(first.IgnoreScopes, "slices")
	second.IgnoreScopes = append(second.IgnoreScopes, "files")

	// Two terminals resolve the same stored rule's options; neither may see the other's tags or scopes.
	if !reflect.DeepEqual(first.BuildTags, []string{"integration", "linux"}) {
		t.Errorf("the first copy's build tags are %v, want its own append", first.BuildTags)
	}
	if !reflect.DeepEqual(second.BuildTags, []string{"integration", "darwin"}) {
		t.Errorf("the second copy's build tags are %v, want its own append", second.BuildTags)
	}
	if !reflect.DeepEqual(options.BuildTags, []string{"integration"}) {
		t.Errorf("the caller's build tags are %v, want the one tag it was built with", options.BuildTags)
	}
	if !reflect.DeepEqual(first.IgnoreScopes, []string{"layers", "slices"}) {
		t.Errorf("the first copy's ignore scopes are %v, want its own append", first.IgnoreScopes)
	}
	if !reflect.DeepEqual(second.IgnoreScopes, []string{"layers", "files"}) {
		t.Errorf("the second copy's ignore scopes are %v, want its own append", second.IgnoreScopes)
	}
	if !reflect.DeepEqual(options.IgnoreScopes, []string{"layers"}) {
		t.Errorf("the caller's ignore scopes are %v, want the one scope it was built with", options.IgnoreScopes)
	}
}

func TestCheckOptionsLogWriterIsTheInjectedWriter(t *testing.T) {
	log := &bytes.Buffer{}
	options := &fluentapi.CheckOptions{Logging: log}

	writer := options.LogWriter()
	if writer != io.Writer(log) {
		t.Fatalf("LogWriter() = %v, want the injected writer", writer)
	}
	if _, err := io.WriteString(writer, "extracted 3 edges"); err != nil {
		t.Fatalf("writing to the log writer failed: %v", err)
	}
	if log.String() != "extracted 3 edges" {
		t.Errorf("the buffer holds %q, want what was written to the log writer", log.String())
	}

	if got := (&fluentapi.CheckOptions{}).LogWriter(); got != nil {
		t.Errorf("LogWriter() = %v on options with no writer, want nil", got)
	}
}

func TestCheckOptionsIgnoresImportKind(t *testing.T) {
	options := &fluentapi.CheckOptions{
		IgnoredImportKinds: extraction.NewImportKindSet(extraction.ImportKindBlank, extraction.ImportKindDot),
	}

	tests := []struct {
		name string
		kind extraction.ImportKind
		want bool
	}{
		{name: "a blank import is a registration, not a dependency", kind: extraction.ImportKindBlank, want: true},
		{name: "and so the user dropped dot imports too", kind: extraction.ImportKindDot, want: true},
		{name: "a plain import is still an edge", kind: extraction.ImportKindPlain, want: false},
		{name: "an aliased import is still an edge", kind: extraction.ImportKindAliased, want: false},
		{name: "a kind Go does not have is never ignored", kind: extraction.ImportKind(9), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := options.IgnoresImportKind(test.kind); got != test.want {
				t.Errorf("IgnoresImportKind(%v) = %v, want %v", test.kind, got, test.want)
			}
		})
	}
}

func TestCheckOptionsEmptyTestOptionsCarryTheGuardTheUserAskedFor(t *testing.T) {
	selector := matching.FolderMatcher(mustGlob(t, "internal/apis/**"))

	tests := []struct {
		name    string
		options *fluentapi.CheckOptions
		want    bool
	}{
		{name: "the default guard turns zero matches into a violation", options: &fluentapi.CheckOptions{}, want: true},
		{name: "nil options means the guard is on", options: nil, want: true},
		{name: "allowEmptyTests is how a user opts out", options: &fluentapi.CheckOptions{AllowEmptyTests: true}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			guard := test.options.EmptyTestOptions("files", selector)

			if guard.Subject != "files" {
				t.Errorf("Subject = %q, want the entry point's own vocabulary", guard.Subject)
			}
			if len(guard.Selectors) != 1 || guard.Selectors[0].Pattern().Source() != "internal/apis/**" {
				t.Errorf("Selectors = %v, want the selector the rule was built from", guard.Selectors)
			}
			// The point of the translation: the flag arrives where the guard reads it.
			violations := assertion.GatherEmptyTestViolations(0, guard)
			if (len(violations) > 0) != test.want {
				t.Fatalf("gathered %v for %+v, want a violation: %v", violations, test.options, test.want)
			}
		})
	}
}

func TestCheckOptionsEmptyTestOptionsCopyTheirSelectors(t *testing.T) {
	// Spreading a slice into a variadic parameter hands over the caller's backing array, and the guard
	// options are what the violation is built from.
	selectors := []matching.Filter{matching.FolderMatcher(mustGlob(t, "internal/apis/**"))}

	guard := (&fluentapi.CheckOptions{}).EmptyTestOptions("files", selectors...)
	selectors[0] = matching.FilenameMatcher(mustGlob(t, "*.go"))

	if source := guard.Selectors[0].Pattern().Source(); source != "internal/apis/**" {
		t.Errorf("the guard's selector changed with the caller's slice: %q", source)
	}
}

func TestCheckOptionsSourceOptionsCarryTheFilesTheUserAskedFor(t *testing.T) {
	tests := []struct {
		name    string
		options *fluentapi.CheckOptions
		want    bool
	}{
		{name: "a rule is about the production code by default", options: &fluentapi.CheckOptions{}, want: false},
		{name: "nil options means the same", options: nil, want: false},
		{name: "includeTestFiles holds tests to the same rules", options: &fluentapi.CheckOptions{IncludeTestFiles: true}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := test.options.SourceOptions()

			if source.IncludeTestFiles != test.want {
				t.Errorf("IncludeTestFiles = %v, want %v", source.IncludeTestFiles, test.want)
			}
			// Nothing on the check options says which folders to skip, so the enumeration's own defaults
			// have to survive the translation rather than being replaced by an empty list.
			if source.ExcludedFolders != nil {
				t.Errorf("ExcludedFolders = %v, want nil, which is the enumeration's defaults", source.ExcludedFolders)
			}
			if !source.ExcludesFolder("vendor") {
				t.Error("the translated options walk into vendor")
			}
		})
	}
}

func TestCheckOptionsSourceOptionsCarryTheGoSpecificKnobs(t *testing.T) {
	// The two knobs that bear on the graph rather than on the walk cross here, in the one place, so that
	// no terminal assembles a second extraction bag by hand and finds it disagreeing with the first.
	options := &fluentapi.CheckOptions{
		IgnoredImportKinds: extraction.NewImportKindSet(extraction.ImportKindBlank),
		BuildTags:          []string{"integration", "linux"},
	}

	source := options.SourceOptions()

	if !slices.Equal(source.BuildTags, []string{"integration", "linux"}) {
		t.Errorf("BuildTags = %v, want the tags the user asked for", source.BuildTags)
	}
	if !source.IgnoresImportKind(extraction.ImportKindBlank) {
		t.Error("the translated options count blank imports the user asked to ignore")
	}
	if source.IgnoresImportKind(extraction.ImportKindPlain) {
		t.Error("the translated options drop plain imports")
	}

	source.BuildTags[0] = "windows"

	// WithDefaults clones the tags on the way through, so the extraction bag does not share an array with
	// the user's own options — which a stored half-built rule shares.
	if options.BuildTags[0] != "integration" {
		t.Errorf("the user's build tags changed with the translated bag: %v", options.BuildTags)
	}
}

func TestCheckOptionsSourceOptionsCarryTheIgnoreScopes(t *testing.T) {
	// The scopes a check answers to have to reach the extractor, because that is where a directive in the
	// source is matched against them, and there is nothing else a scope does.
	options := &fluentapi.CheckOptions{IgnoreScopes: []string{"layers", "slices"}}

	source := options.SourceOptions()

	if !slices.Equal(source.IgnoreScopes, []string{"layers", "slices"}) {
		t.Errorf("IgnoreScopes = %v, want the scopes the user asked for", source.IgnoreScopes)
	}
	// The question the extractor actually asks, on both kinds of directive.
	unscoped := extraction.ImportInfo{Path: "example.com/dependency", Ignore: extraction.IgnoreDirective{Present: true}}
	if !source.IgnoresImport(unscoped) {
		t.Error("the translated options keep an import the file marked with a bare directive")
	}
	scoped := extraction.ImportInfo{
		Path:   "example.com/dependency",
		Ignore: extraction.IgnoreDirective{Present: true, Scopes: "layers"},
	}
	if !source.IgnoresImport(scoped) {
		t.Error("the translated options keep an import marked for a scope the check answers to")
	}
	elsewhere := extraction.ImportInfo{
		Path:   "example.com/dependency",
		Ignore: extraction.IgnoreDirective{Present: true, Scopes: "files"},
	}
	if source.IgnoresImport(elsewhere) {
		t.Error("the translated options drop an import marked for a scope the check does not answer to")
	}

	// Defaults: only the directives that need no configuration.
	byDefault := (&fluentapi.CheckOptions{}).SourceOptions()
	if len(byDefault.IgnoreScopes) != 0 {
		t.Errorf("IgnoreScopes = %v by default, want none", byDefault.IgnoreScopes)
	}
	if !byDefault.IgnoresImport(unscoped) || byDefault.IgnoresImport(scoped) {
		t.Error("the default options do not honor exactly the unscoped directives")
	}

	source.IgnoreScopes[0] = "files"

	// Cloned on the way through, for the reason the tags are: the extraction bag must not share an array
	// with the user's own options.
	if options.IgnoreScopes[0] != "layers" {
		t.Errorf("the user's ignore scopes changed with the translated bag: %v", options.IgnoreScopes)
	}
}

func TestCheckOptionsExtractGraphHonorsTheIgnoreDirectivesTheSourceWrote(t *testing.T) {
	// The whole convention through the public surface, which is the only place it is visible as a feature:
	// a file marks two of its imports, and a check decides which of the two marks it answers to.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	root := writeFixtureProject(t)
	writeFixtureFile(t, root, "handler.go", `package fixture

import (
	"errors" //archunit:ignore

	//archunit:ignore layers
	"strings"
	"sort"
)

var _ = []any{errors.New, strings.TrimSpace, sort.Strings}
`)
	locator := &extraction.ProjectLocator{Directory: root}

	byDefault := extractGraph(t, &fluentapi.CheckOptions{}, locator)
	scoped := extractGraph(t, &fluentapi.CheckOptions{IgnoreScopes: []string{"layers"}}, locator)

	if _, found := byDefault.Find("handler.go", "errors"); found {
		t.Errorf("graph =\n%s\n\nwant the import the file marked with a bare directive left out", byDefault)
	}
	if _, found := byDefault.Find("handler.go", "strings"); !found {
		t.Errorf("graph =\n%s\n\nwant the scoped directive ignored by a check that answers to no scope", byDefault)
	}
	if _, found := byDefault.Find("handler.go", "sort"); !found {
		t.Errorf("graph =\n%s\n\nwant the unmarked import kept", byDefault)
	}

	// The scopes reach the cache key as well as the extractor: were they not part of it, this second check
	// would be handed the graph the first one cached, with the dependency still in it.
	if _, found := scoped.Find("handler.go", "strings"); found {
		t.Errorf("graph =\n%s\n\nwant the import scoped to layers left out of a check answering to layers", scoped)
	}
	if _, found := scoped.Find("handler.go", "sort"); !found {
		t.Errorf("graph =\n%s\n\nwant the unmarked import kept", scoped)
	}
}

func TestCheckOptionsExtractGraphSharesOneGraphBetweenChecks(t *testing.T) {
	// What a terminal does with a rule, twice, the way a suite does it: locate the project, extract it,
	// and the second rule onwards gets the graph the first one paid for. The fixture gains a file between
	// the two checks, so a second extraction would be visible.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	root := writeFixtureProject(t)
	locator := &extraction.ProjectLocator{Directory: root}

	first := extractGraph(t, &fluentapi.CheckOptions{}, locator)
	writeFixtureFile(t, root, "later.go", "package fixture\n\nfunc later() {}\n")
	second := extractGraph(t, &fluentapi.CheckOptions{}, locator)

	if _, found := first.Find("fixture.go", "fixture.go"); !found {
		t.Fatalf("graph =\n%s\n\nwant the project's own file as a node", first)
	}
	if !slices.Equal(second, first) {
		t.Errorf("the second check extracted\n%s\n\nwant the graph the first one cached\n%s", second, first)
	}
}

func TestCheckOptionsClearCacheMakesTheCheckReadTheSourceAgain(t *testing.T) {
	// The escape hatch, on the flag a user sets: this test is itself the case it exists for — a fixture
	// project written and then changed inside one process.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	root := writeFixtureProject(t)
	locator := &extraction.ProjectLocator{Directory: root}

	extractGraph(t, &fluentapi.CheckOptions{}, locator)
	writeFixtureFile(t, root, "later.go", "package fixture\n\nfunc later() {}\n")
	cleared := extractGraph(t, &fluentapi.CheckOptions{ClearCache: true}, locator)

	if _, found := cleared.Find("later.go", "later.go"); !found {
		t.Errorf("graph =\n%s\n\nwant the file written since the last check", cleared)
	}
	// And the check that cleared the cache still filled it, so the rules after it in the suite share its
	// graph rather than each extracting one of their own. The fixture gains a third file first, so that a
	// check which cleared the cache *instead* of filling it — clearing after the extraction rather than
	// before — is visible as that file arriving in the next check's graph.
	writeFixtureFile(t, root, "latest.go", "package fixture\n\nfunc latest() {}\n")
	reused := extractGraph(t, &fluentapi.CheckOptions{}, locator)
	if !slices.Equal(reused, cleared) {
		t.Errorf("the next check extracted\n%s\n\nwant the graph the cleared one cached\n%s", reused, cleared)
	}
	if _, found := reused.Find("latest.go", "latest.go"); found {
		t.Errorf("graph =\n%s\n\nwant the memoised graph, which was extracted before latest.go existed", reused)
	}
}

func TestCheckOptionsExtractGraphTellsTwoAnalysesOfOneProjectApart(t *testing.T) {
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	root := writeFixtureProject(t)
	writeFixtureFile(t, root, "fixture_test.go", "package fixture\n\nimport \"testing\"\n\nfunc TestFixture(*testing.T) {}\n")
	locator := &extraction.ProjectLocator{Directory: root}

	production := extractGraph(t, &fluentapi.CheckOptions{}, locator)
	withTests := extractGraph(t, &fluentapi.CheckOptions{IncludeTestFiles: true}, locator)

	// The Go-specific knobs reach the cache key, not just the extractor: a rule that holds the tests to
	// the same rules must not be answered with the graph of the production code alone.
	if _, found := production.Find("fixture_test.go", "fixture_test.go"); found {
		t.Errorf("graph =\n%s\n\nwant a rule about the production code to leave the test file out", production)
	}
	if _, found := withTests.Find("fixture_test.go", "fixture_test.go"); !found {
		t.Errorf("graph =\n%s\n\nwant the test file the options asked for", withTests)
	}
}

func TestCheckOptionsExtractGraphRejectsALocatorThatIsNotAProject(t *testing.T) {
	// Not a rule failure and not a violation: the library was pointed at something that is not a Go
	// project, and only the caller can say where the project really is.
	notAProject := t.TempDir()

	_, err := (&fluentapi.CheckOptions{}).ExtractGraph(&extraction.ProjectLocator{Directory: notAProject})

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("ExtractGraph error = %v, want a *archerror.UserError", err)
	}
	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("ExtractGraph error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
}

// extractGraph is the SOURCE-and-EXTRACT half of a terminal: the one call a rule makes to get the graph
// it is checked against.
func extractGraph(t *testing.T, options *fluentapi.CheckOptions, locator *extraction.ProjectLocator) extraction.Graph {
	t.Helper()

	graph, err := options.ExtractGraph(locator)
	if err != nil {
		t.Fatalf("ExtractGraph failed: %v", err)
	}
	return graph
}

// writeFixtureProject writes the smallest project a check has anything to say about — a module and one
// file that is a node of it — into a directory of this test's own.
func writeFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.com/fixture\n\ngo 1.26\n")
	writeFixtureFile(t, root, "fixture.go", "package fixture\n\nfunc Fixture() {}\n")
	return root
}

func writeFixtureFile(t *testing.T, root, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatalf("writing %q failed: %v", name, err)
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
