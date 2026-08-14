package fluentapi

import (
	"io"
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// CheckOptions is the one parameter a terminal takes beyond its required arguments: everything about
// *how* a rule is run, as opposed to what the rule says.
//
// It is a struct rather than a list of functional options, matching the sibling ports and reading
// better in the documentation, and a *CheckOptions is always allowed to be nil. Every knob's default
// is its zero value — a check is quiet, strict about empty selections, free to reuse a cached graph,
// and looks at the production code under the host platform's build constraints — so a nil bag, the
// zero bag and an explicitly empty one all describe the same check. Read a nil bag through the
// methods below, or through WithDefaults, rather than reaching for a field.
type CheckOptions struct {
	// AllowEmptyTests makes a rule that selected nothing a pass instead of an EmptyTestViolation.
	// False by default, because a selector matching nothing is far more often a stale glob or a
	// renamed folder than an intention, and such a rule is green forever. This is the one knob that
	// changes what a rule reports, which is why it is the flag every terminal has to thread into the
	// empty-test guard; EmptyTestOptions is how.
	AllowEmptyTests bool
	// Logging is where the check writes its progress log: which files were parsed, how many edges
	// were extracted, how long a stage took. Nil — the default — means it logs nothing at all.
	//
	// A library must not own a process-global logger, so the destination is injected rather than
	// configured: any io.Writer will do, log/slog over that writer is fine, and a test can pass a
	// bytes.Buffer. A technical failure is still an error returned from Check, never a log line.
	Logging io.Writer
	// ClearCache makes the check extract the project from source again instead of reusing a graph an
	// earlier check produced in the same process. Extraction is the expensive half of a check and
	// every rule in a test suite asks about the same project, so caching is how a suite stays fast.
	//
	// The escape hatch matters when the source has changed underneath the library — a test that
	// writes a fixture project, or generated code produced between two checks. No cache exists yet;
	// this flag is the contract the extractor is written against.
	ClearCache bool

	// The knobs below are Go-specific: they say how the Go toolchain is pointed at the project, and
	// have no counterpart in a sibling port.

	// IncludeTestFiles adds the project's _test.go files, and the packages built only from them, to
	// the extracted graph. It is the Tests flag on go/packages.
	//
	// Off by default: an architecture rule describes the shape of the production code, and a test
	// reaching across a boundary to build a fixture is rarely the violation the rule meant to catch.
	// Turn it on to hold tests to the same rules as the code.
	IncludeTestFiles bool
	// IgnoredImportKinds drops imports of these flavors before any edge is emitted, so a rule never
	// has to know they existed.
	//
	// The usual member is extraction.ImportKindBlank: `import _ "github.com/lib/pq"` registers a
	// driver and depends on no API, so counting it as a dependency makes every rule about the
	// database layer fire in main.go. Empty by default, because dropping an edge is a decision that
	// should be visible in the test that made it.
	IgnoredImportKinds extraction.ImportKindSet
	// BuildTags are the build constraints to analyze the project under, becoming build flags for
	// go/packages. Empty means the toolchain's defaults for the host platform.
	//
	// A file excluded by a constraint is absent from the graph, so a rule that selects nothing on one
	// platform and everything on another is usually a tag missing from here.
	BuildTags []string
}

// WithDefaults returns the options a check should actually run with: a copy of the receiver, or the
// defaults when the receiver is nil. Terminals start with this, so that the "nil means defaults"
// contract is honored in one place instead of being re-derived as a nil check per field.
//
// Today every default is a zero value, so the only default this has to supply is for the nil case.
// Going through it anyway is what lets a default that is not a zero value be added here later without
// touching a single terminal.
//
// BuildTags is cloned, for the reason EmptyTestOptions clones its selectors: a struct copy shares the
// slice's backing array, so a terminal appending a tag to its resolved bag would reach into the user's
// own options — which a stored half-built rule shares — and into every other copy.
func (o *CheckOptions) WithDefaults() CheckOptions {
	if o == nil {
		return CheckOptions{}
	}
	resolved := *o
	resolved.BuildTags = slices.Clone(o.BuildTags)
	return resolved
}

// LogWriter is where the check should write its progress log, or nil when it should not log — which
// is the default, and what a nil options bag means.
func (o *CheckOptions) LogWriter() io.Writer {
	if o == nil {
		return nil
	}
	return o.Logging
}

// IgnoresImportKind reports whether an import of this flavor should be left out of the graph. It is
// the question the extractor asks of each import declaration, and the answer is no for a nil options
// bag and for anything that is not a declared extraction.ImportKind.
func (o *CheckOptions) IgnoresImportKind(kind extraction.ImportKind) bool {
	if o == nil {
		return false
	}
	return o.IgnoredImportKinds.Contains(kind)
}

// EmptyTestOptions translates these check options into the empty-test guard's own options, adding
// what the rule was selecting and the selectors it was selecting with. Every terminal wires the
// guard in through this, so AllowEmptyTests is copied across in exactly one place:
//
//	violations := assertion.GatherEmptyTestViolations(matched, options.EmptyTestOptions("files", selectors...))
//
// The guard takes the one flag rather than the whole bag because fluentapi depends on assertion, for
// the violations Check returns, and not the other way round.
//
// The selectors are copied, for the reason assertion.NewEmptyTestViolation copies them: spreading a
// caller's slice into a variadic parameter shares its backing array.
func (o *CheckOptions) EmptyTestOptions(subject string, selectors ...matching.Filter) *assertion.EmptyTestOptions {
	return &assertion.EmptyTestOptions{
		Subject:         subject,
		Selectors:       slices.Clone(selectors),
		AllowEmptyTests: o.WithDefaults().AllowEmptyTests,
	}
}

// SourceOptions translates these check options into the extraction stage's own options, the way
// EmptyTestOptions translates them into the empty-test guard's. It is where the three Go-specific knobs
// cross from the bag a user filled in to the walk and the toolchain that read it, in one place, so that
// a terminal never assembles a second one by hand and finds it disagreeing with the first.
//
// Folder exclusions have no knob on the check options, so the enumeration's own defaults apply —
// vendored dependencies and build output, plus everything the Go toolchain itself ignores.
//
// BuildTags arrives already cloned, from WithDefaults, so the extraction bag does not share an array
// with the user's own options.
func (o *CheckOptions) SourceOptions() *extraction.SourceOptions {
	resolved := o.WithDefaults()
	return &extraction.SourceOptions{
		IncludeTestFiles:   resolved.IncludeTestFiles,
		BuildTags:          resolved.BuildTags,
		IgnoredImportKinds: resolved.IgnoredImportKinds,
	}
}
