package fluentapi_test

import (
	"bytes"
	"io"
	"reflect"
	"testing"

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
	if got := options.LogWriter(); got != nil {
		t.Errorf("LogWriter() = %v, want nil: a library logs nowhere until a writer is injected", got)
	}
	if options.IgnoresImportKind(extraction.ImportKindBlank) {
		t.Error("a nil options bag ignores blank imports; it should ignore nothing")
	}
}

func TestCheckOptionsWithDefaultsIsACopy(t *testing.T) {
	// Every field is set to a non-zero value, so that a resolution which drops one is visible. BuildTags
	// is the only field a struct copy does not separate: the copy shares its backing array.
	options := &fluentapi.CheckOptions{
		AllowEmptyTests:    true,
		Logging:            &bytes.Buffer{},
		ClearCache:         true,
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

	// A terminal resolves the bag once and may well pass the value around; the user's own options,
	// which a stored half-built rule shares, must not move underneath them.
	if options.BuildTags[0] != "integration" {
		t.Errorf("the caller's build tags changed with the resolved copy: %v", options.BuildTags)
	}
}

func TestCheckOptionsWithDefaultsCopiesAreIndependentOfEachOther(t *testing.T) {
	// Spare capacity in the parent slice is what makes the sibling case bite: without a copy, both
	// appends write the same array slot, and the second one wins for both.
	options := &fluentapi.CheckOptions{BuildTags: append(make([]string, 0, 4), "integration")}

	first := options.WithDefaults()
	second := options.WithDefaults()
	first.BuildTags = append(first.BuildTags, "linux")
	second.BuildTags = append(second.BuildTags, "darwin")

	// Two terminals resolve the same stored rule's options; neither may see the other's tags.
	if !reflect.DeepEqual(first.BuildTags, []string{"integration", "linux"}) {
		t.Errorf("the first copy's build tags are %v, want its own append", first.BuildTags)
	}
	if !reflect.DeepEqual(second.BuildTags, []string{"integration", "darwin"}) {
		t.Errorf("the second copy's build tags are %v, want its own append", second.BuildTags)
	}
	if !reflect.DeepEqual(options.BuildTags, []string{"integration"}) {
		t.Errorf("the caller's build tags are %v, want the one tag it was built with", options.BuildTags)
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

func mustGlob(t *testing.T, glob string) matching.Pattern {
	t.Helper()

	pattern, err := matching.NewGlobPattern(glob, nil)
	if err != nil {
		t.Fatalf("NewGlobPattern(%q, nil) failed: %v", glob, err)
	}
	return pattern
}
