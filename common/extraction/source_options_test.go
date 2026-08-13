package extraction

import (
	"slices"
	"testing"
)

func TestNilSourceOptionsMeansTheDefaults(t *testing.T) {
	var options *SourceOptions

	defaults := options.WithDefaults()

	// The defaults spelled out, because they are a promise: the production code of the project itself,
	// with vendored dependencies and build output left out. Spelled out as a literal, and not as a call
	// to DefaultExcludedFolders(), which is the function WithDefaults assigns this field from —
	// comparing against that call holds for any list it returns, so deleting `out` from the defaults
	// would change both sides of the comparison together and no test would notice.
	wantExcludedFolders := []string{"vendor", "node_modules", "bin", "dist", "build", "out", "target"}
	if defaults.IncludeTestFiles {
		t.Error("IncludeTestFiles defaults to true; a rule is about the production code")
	}
	if !slices.Equal(defaults.ExcludedFolders, wantExcludedFolders) {
		t.Errorf("ExcludedFolders defaults to %v, want %v", defaults.ExcludedFolders, wantExcludedFolders)
	}
	// A nil bag has to answer the question the walk actually asks it, without being resolved first.
	if !options.ExcludesFolder("vendor") {
		t.Error("a nil options bag walks into vendor")
	}
	if options.ExcludesFolder("internal") {
		t.Error("a nil options bag skips internal")
	}
}

func TestSourceOptionsWithDefaultsIsACopy(t *testing.T) {
	options := &SourceOptions{
		IncludeTestFiles: true,
		ExcludedFolders:  []string{"generated"},
	}

	resolved := options.WithDefaults()

	if !resolved.IncludeTestFiles {
		t.Error("WithDefaults dropped IncludeTestFiles")
	}
	if !slices.Equal(resolved.ExcludedFolders, []string{"generated"}) {
		t.Errorf("ExcludedFolders = %v, want the caller's own list", resolved.ExcludedFolders)
	}

	resolved.ExcludedFolders[0] = "vendor"

	// The resolved bag is passed around by whatever is walking; the user's own options, which a stored
	// half-built rule shares, must not move underneath them.
	if options.ExcludedFolders[0] != "generated" {
		t.Errorf("the caller's exclusions changed with the resolved copy: %v", options.ExcludedFolders)
	}
}

func TestSourceOptionsWithDefaultsKeepsAnEmptyExclusionListEmpty(t *testing.T) {
	// Nil is how this bag spells "the defaults", so an empty list has to survive resolution as an empty
	// list — otherwise a caller who excluded nothing on purpose is handed the defaults back.
	resolved := (&SourceOptions{ExcludedFolders: []string{}}).WithDefaults()

	if len(resolved.ExcludedFolders) != 0 {
		t.Fatalf("ExcludedFolders = %v, want the empty list the caller asked for", resolved.ExcludedFolders)
	}
	if resolved.ExcludedFolders == nil {
		t.Error("ExcludedFolders resolved to nil, which the bag reads back as the defaults")
	}
	if resolved.ExcludesFolder("vendor") {
		t.Error("vendor is still excluded after the caller emptied the list")
	}
}

func TestDefaultExcludedFoldersIsFreshEachTime(t *testing.T) {
	// It is documented as appendable — `append(DefaultExcludedFolders(), "generated")` — so it must not
	// be a shared table a caller can write through.
	first := DefaultExcludedFolders()
	first[0] = "somewhere else"

	if slices.Contains(DefaultExcludedFolders(), "somewhere else") {
		t.Errorf("DefaultExcludedFolders() = %v, want a fresh list", DefaultExcludedFolders())
	}
}

func TestSourceOptionsExcludesFolder(t *testing.T) {
	options := &SourceOptions{ExcludedFolders: append(DefaultExcludedFolders(), "generated")}

	tests := []struct {
		name   string
		folder string
		want   bool
	}{
		{name: "vendored dependencies are not the project's own code", folder: "vendor", want: true},
		{name: "nor are vendored JavaScript ones", folder: "node_modules", want: true},
		{name: "build output is not source", folder: "dist", want: true},
		{name: "the caller's own addition", folder: "generated", want: true},
		{name: "version control is invisible to the toolchain", folder: ".git", want: true},
		{name: "and so is a cache", folder: ".cache", want: true},
		{name: "and so is an underscore folder", folder: "_scratch", want: true},
		{name: "and so is a fixture folder", folder: testDataFolder, want: true},
		{name: "the project's own code is not excluded", folder: "internal", want: false},
		{name: "nor is a folder whose name merely contains one", folder: "vendors", want: false},
		{name: "nor is a command", folder: "cmd", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := options.ExcludesFolder(test.folder); got != test.want {
				t.Errorf("ExcludesFolder(%q) = %v, want %v", test.folder, got, test.want)
			}
		})
	}
}

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		name             string
		file             string
		includeTestFiles bool
		want             bool
	}{
		{name: "ordinary Go source", file: "handler.go", want: true},
		{name: "a test file is left out by default", file: "handler_test.go", want: false},
		{name: "and included when asked for", file: "handler_test.go", includeTestFiles: true, want: true},
		{name: "a file the toolchain ignores", file: "_ignored.go", includeTestFiles: true, want: false},
		{name: "a hidden file the toolchain ignores", file: ".hidden.go", want: false},
		{name: "cgo C source carries no import declaration", file: "bridge.c", want: false},
		{name: "assembly carries none either", file: "asm.s", want: false},
		{name: "documentation is not source", file: "README.md", want: false},
		{name: "a file merely mentioning the extension", file: "notes.go.txt", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isSourceFile(test.file, test.includeTestFiles); got != test.want {
				t.Errorf("isSourceFile(%q, %v) = %v, want %v", test.file, test.includeTestFiles, got, test.want)
			}
		})
	}
}
