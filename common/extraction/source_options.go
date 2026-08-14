package extraction

import (
	"slices"
	"strings"
)

// testDataFolder is the directory name the Go toolchain reserves for fixtures. It is never part of a
// project's own source, and a fixture project inside it is a project of its own.
const testDataFolder = "testdata"

// SourceOptions are the knobs on reading a project's source: which files count, which folders the walk
// does not go into, and which imports become edges. A nil *SourceOptions means the defaults —
// production code only, with the default exclusions, under the host platform's build constraints and
// with every import counted — which is what most callers pass.
//
// It is one bag for the whole EXTRACT stage rather than one per function, so that a caller resolves the
// "nil means defaults" contract once and hands the same value to ExtractSourceFiles and ExtractGraph.
// The first two fields bear on the walk, the last two only on the graph.
type SourceOptions struct {
	// IncludeTestFiles adds the project's _test.go files to the enumeration. It is the same knob as
	// CheckOptions.IncludeTestFiles, which is where it comes from, and it is off by default: an
	// architecture rule describes the shape of the production code, and a test reaching across a
	// boundary to build a fixture is rarely the violation the rule meant to catch.
	IncludeTestFiles bool
	// ExcludedFolders are the folder names the walk skips, and it replaces DefaultExcludedFolders
	// rather than adding to it. Nil — the default — means that set; to extend it, start from it:
	//
	//	&SourceOptions{ExcludedFolders: append(DefaultExcludedFolders(), "generated")}
	//
	// A non-nil empty slice excludes nothing, beyond the folders the Go toolchain itself passes over,
	// which are never walked whatever this says. Names are matched whole and without a path, so
	// `vendor` skips a vendor folder anywhere in the project.
	ExcludedFolders []string
	// BuildTags are the build constraints to read the project under, and become build flags for the Go
	// toolchain. Empty — the default — means the toolchain's own answer for the host platform.
	//
	// It is the same knob as CheckOptions.BuildTags, which is where it comes from. A file the
	// constraints exclude is not in the build, so it is not a node in the graph and neither a
	// dependency of it nor a dependency on it exists: a rule that selects nothing on one platform and
	// everything on another is usually a tag missing from here.
	BuildTags []string
	// IgnoredImportKinds drops imports of these flavors before any edge is emitted, so that nothing
	// downstream has to know they existed. It is the same knob as CheckOptions.IgnoredImportKinds.
	//
	// The usual member is ImportKindBlank: `import _ "github.com/lib/pq"` registers a driver and
	// depends on no API of it. Empty by default, because dropping an edge is a decision that should be
	// visible in the test that made it.
	IgnoredImportKinds ImportKindSet
}

// DefaultExcludedFolders lists the folder names a walk skips unless a caller says otherwise: the ones
// holding code that is not the project's own, or output built from it.
//
//   - `vendor` and `node_modules` hold vendored dependencies. They are code the project depends on,
//     not code it is made of, and a rule about the project's layering has nothing to say about them.
//   - `bin`, `dist`, `build`, `out` and `target` are where build output lands. Generated or compiled
//     artifacts are not the source an architecture rule is about, and a project that keeps real Go
//     source in one of them can say so through SourceOptions.ExcludedFolders.
//
// Version-control directories, editor state and caches — `.git`, `.idea`, `.cache`, `__pycache__` —
// are deliberately absent: they are already excluded by the rule the Go toolchain itself applies, and
// no caller gets to switch that off. See SourceOptions.ExcludesFolder.
//
// The slice is freshly built on every call, so a caller may append to it.
func DefaultExcludedFolders() []string {
	return []string{"vendor", "node_modules", "bin", "dist", "build", "out", "target"}
}

// WithDefaults returns the options an enumeration should actually run with: a copy of the receiver
// with the default exclusions filled in, or the defaults when the receiver is nil. ExtractSourceFiles
// starts with this, so that the "nil means defaults" contract is honored in one place instead of being
// re-derived per field.
//
// Both slices are cloned, for the reason CheckOptions clones BuildTags: a struct copy shares the
// slice's backing array, so writing through the resolved copy would reach into the caller's own
// options. The ExcludedFolders clone is kept non-nil even when it is empty, because nil is how this bag
// spells "the defaults" and a caller who excluded nothing on purpose must not be handed them back.
func (o *SourceOptions) WithDefaults() SourceOptions {
	if o == nil {
		return SourceOptions{ExcludedFolders: DefaultExcludedFolders()}
	}
	resolved := *o
	resolved.BuildTags = slices.Clone(o.BuildTags)
	if o.ExcludedFolders == nil {
		resolved.ExcludedFolders = DefaultExcludedFolders()
		return resolved
	}
	resolved.ExcludedFolders = append(make([]string, 0, len(o.ExcludedFolders)), o.ExcludedFolders...)
	return resolved
}

// ExcludesFolder reports whether the walk should skip a directory with this name, and with it
// everything underneath. It is the one place the exclusion policy is stated, and it has two halves.
//
// The first is not configurable: a name beginning with `.` or `_`, and any directory named testdata,
// is invisible to the Go toolchain. A file in there is not part of the build, so it cannot be a node in
// a graph the toolchain would ever produce — which is also why version-control directories, editor
// state and caches need no entry of their own.
//
// The second is this bag's ExcludedFolders, defaulting to DefaultExcludedFolders.
func (o *SourceOptions) ExcludesFolder(name string) bool {
	if ignoredByToolchain(name) {
		return true
	}
	return slices.Contains(o.excludedFolders(), name)
}

// IgnoresImportKind reports whether an import of this flavor should be left out of the graph. It is the
// question the graph extractor asks of every import declaration, and the answer is no for a nil options
// bag and for anything that is not a declared ImportKind.
func (o *SourceOptions) IgnoresImportKind(kind ImportKind) bool {
	if o == nil {
		return false
	}
	return o.IgnoredImportKinds.Contains(kind)
}

// excludedFolders is the effective exclusion list, without the copy WithDefaults makes: the walk asks
// this question once per directory, and it asks it of options it has already resolved.
func (o *SourceOptions) excludedFolders() []string {
	if o == nil || o.ExcludedFolders == nil {
		return DefaultExcludedFolders()
	}
	return o.ExcludedFolders
}

// ignoredByToolchain reports whether the Go toolchain itself passes over a file or directory with this
// name. `go build` ignores anything beginning with `.` or `_`, and any directory named testdata, so
// `.git`, `.cache`, `__pycache__`, `_scratch` and a fixture project all fall out of one rule rather
// than a list this library would have to keep up to date.
func ignoredByToolchain(name string) bool {
	return name == testDataFolder || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
