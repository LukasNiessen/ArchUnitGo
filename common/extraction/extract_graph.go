package extraction

import (
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// projectPattern is the package pattern the toolchain is asked to resolve: every package of the module
// the project root holds. It stops at a nested module, which is what makes a fixture project inside
// another project the toolchain's problem rather than this library's.
const projectPattern = "./..."

// buildTagFlag is how a build constraint reaches the toolchain. The tags are one comma-separated
// argument, the way `go build -tags` takes them.
const buildTagFlag = "-tags="

// edgesPerNodeEstimate is how many edges a source file is assumed to contribute — its self-edge and a
// few imports — so that the edge slice is allocated once for a project of any size.
const edgesPerNodeEstimate = 8

// ExtractGraph turns a located project into the dependency graph every rule is evaluated against. It is
// the whole EXTRACT stage, and the one function in the library that knows Go: everything downstream sees
// a Graph and never an import declaration, a package path or a file on disk.
//
// root is a host path, normally the one LocateProject returned. The graph it hands back satisfies the
// invariants NewGraph establishes — normalised identifiers, parallel edges merged with their import
// kinds unioned, a reproducible order — plus the one this stage owns: every file that is a node has a
// self-edge, so a file that depends on nothing still appears.
//
// # What becomes a node
//
// A file is a node when the walk enumerates it *and* the Go toolchain puts it in the build. The walk
// decides the first — .go files only, minus test files and excluded folders, as ExtractSourceFiles
// describes — and the toolchain decides the second, so a file excluded by a build constraint is absent
// from the graph and SourceOptions.BuildTags is how a caller asks for it. Nothing else is a node:
// identifiers in the graph are project-relative file paths, never a package or a folder.
//
// # What becomes an edge
//
// An import is a dependency on a package, while a node is a file, so resolving one import means naming
// the files it points at:
//
//   - an import of one of the project's own packages becomes one edge per file that package is built
//     from. That is what the language means: a package is compiled as a whole, so a file importing it
//     depends on every file in it.
//   - an import of anything else — the standard library, a dependency module, a vendored copy, a module
//     nested inside the project — becomes one edge to the import path itself, exactly as the source
//     wrote it, marked External. Rules about external dependencies match against that path.
//
// Which of the two an import is is the toolchain's answer rather than arithmetic on the import string:
// the project's own package paths are the ones `go list` reported for it. The one import the toolchain
// has nothing to say about is a package that is not there, and the project's own module path is what
// settles that one — classify_target.go is where the whole decision lives.
//
// # What is left out, and none of it is an error
//
//   - imports of a flavor SourceOptions.IgnoredImportKinds names;
//   - an import of the project's own code that has no node to point at: a package whose every file the
//     walk excluded — one under `vendor` or `build` — one whose every file a build constraint excluded,
//     or one that does not exist at all because it is half-written, renamed or deleted. None of the three
//     is an external module, so none of them fires a rule about third-party dependencies;
//   - a test file as the *target* of an import, even when SourceOptions.IncludeTestFiles put it in the
//     graph. A package built with its test files is only visible inside its own test binary, so an
//     import of that package points at the files it is built from everywhere else;
//   - a file that fails to parse contributes the imports found before the parser gave up, and no more.
//     One unreadable file must not fail every rule in a suite.
//
// A project with no source files yields an empty graph and no error, without invoking the toolchain at
// all. Whether that emptiness is a problem is a rule's question, and the empty-test guard is where it is
// asked.
//
// Extracting a graph runs the Go toolchain once over the whole project and then parses the header of
// every file. It is the expensive half of a check, and the reason every rule in a suite is meant to
// share one graph — which is what CheckOptions.ClearCache is the escape hatch from.
func ExtractGraph(root string, options *SourceOptions) (Graph, error) {
	resolved := options.WithDefaults()

	// Resolved once, up front, and handed to both halves: the walk and the toolchain each turn the paths
	// they get back into identifiers relative to it, and those two sets of identifiers are then matched
	// against each other. Resolving it twice would be two chances to disagree.
	directory, err := resolveProjectRoot(root)
	if err != nil {
		return nil, err
	}

	files, err := ExtractSourceFiles(directory, &resolved)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return NewGraph(), nil
	}

	build, err := loadProjectBuild(directory, files, &resolved)
	if err != nil {
		return nil, err
	}

	// One self-edge and a handful of imports per node is the shape of real Go source, so this is the
	// right order of magnitude for the whole graph rather than a guess.
	edges := make([]Edge, 0, len(build.nodes)*edgesPerNodeEstimate)
	for _, node := range build.nodes {
		edges = append(edges, SelfEdge(node.Identifier))
		edges = append(edges, build.importEdges(node, &resolved)...)
	}
	return NewGraph(edges...), nil
}

// projectBuild is what the Go toolchain resolved about a project, and the only two things extraction
// asks of it.
type projectBuild struct {
	// nodes are the enumerated files that are also in the build, in the enumeration's order.
	nodes []FileInfo
	// targets is what an import path of this project resolves to, and the only place a dependency on the
	// project's own code is told from a dependency on somebody else's.
	targets targetIndex
}

// loadProjectBuild asks the Go toolchain what the project is made of, and keeps the half of the answer
// the graph is built from. Letting `go list` decide which files are in the build is what makes build
// constraints, vendoring, nested modules and the standard library all somebody else's problem.
//
// directory is the resolved project root, the one ExtractSourceFiles enumerated the files against.
func loadProjectBuild(directory string, files []FileInfo, options *SourceOptions) (projectBuild, error) {
	enumerated := make(map[string]struct{}, len(files))
	for _, file := range files {
		enumerated[file.Identifier] = struct{}{}
	}

	configuration := &packages.Config{
		// NeedName for the package paths an import is resolved against, NeedFiles for the files each
		// package is built from, NeedForTest to tell a package from the same package built with its
		// test files, NeedModule for the project's own module path. Nothing here needs a type, and
		// asking for one would type-check the project and its dependencies to answer a question about
		// its imports.
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedForTest | packages.NeedModule,
		Dir:        directory,
		Tests:      options.IncludeTestFiles,
		BuildFlags: buildFlags(options.BuildTags),
	}
	loaded, err := packages.Load(configuration, projectPattern)
	if err != nil {
		return projectBuild{}, archerror.NewTechnicalError("load the packages of the project", directory, err)
	}

	// A package that does not compile is reported in its own Errors and is deliberately not read here:
	// architecture rules are about the shape of the source, and a project mid-refactor still has one.
	targets := targetIndex{root: directory, packages: make(map[string][]string, len(loaded))}
	inBuild := make(map[string]struct{}, len(files))
	for _, pkg := range loaded {
		if targets.modulePath == "" && pkg.Module != nil && pkg.Module.Main {
			// Every package the project pattern matches belongs to the module under analysis, so the
			// first one carrying module information carries the answer. Asking the toolchain rather than
			// parsing go.mod keeps one source of truth about what the project is, whichever module file
			// the toolchain was really pointed at.
			targets.modulePath = pkg.Module.Path
		}
		// A package built with its own test files is a second reading of a package that is already
		// here, so its files are nodes but they are not what an import of that package points at:
		// nothing outside the test binary sees them. ForTest is what the toolchain calls that.
		importable := pkg.ForTest == ""
		if _, found := targets.packages[pkg.PkgPath]; importable && !found {
			// The key goes in even when no file survives, because its presence is what tells an import
			// of the project's own code from an import of somebody else's.
			targets.packages[pkg.PkgPath] = nil
		}
		for _, path := range pkg.GoFiles {
			identifier, inside := RelativeIdentifier(directory, path)
			if !inside {
				// A generated file: the toolchain's cgo output and test main live outside the project.
				continue
			}
			if _, enumeratedFile := enumerated[identifier]; !enumeratedFile {
				continue
			}
			inBuild[identifier] = struct{}{}
			if importable {
				targets.packages[pkg.PkgPath] = append(targets.packages[pkg.PkgPath], identifier)
			}
		}
	}
	for path, identifiers := range targets.packages {
		// One file reaches a path through more than one package — a package and the same package built
		// with its test files are both reported — and a target list is a set.
		slices.Sort(identifiers)
		targets.packages[path] = slices.Compact(identifiers)
	}

	build := projectBuild{targets: targets}
	// The enumeration is already ordered by identifier, so filtering it keeps that order rather than
	// inheriting the order the toolchain happened to report packages in.
	build.nodes = make([]FileInfo, 0, len(inBuild))
	for _, file := range files {
		if _, found := inBuild[file.Identifier]; found {
			build.nodes = append(build.nodes, file)
		}
	}
	return build, nil
}

// importEdges resolves one node's imports to the nodes they point at. A node with no imports yields no
// edges: its self-edge is what puts it in the graph, and ExtractGraph adds that.
func (b projectBuild) importEdges(node FileInfo, options *SourceOptions) []Edge {
	imports, parseFailure := ExtractImports(node.Path)
	if parseFailure != nil && len(imports) == 0 {
		// Skipped, not fatal: the file is still a node, it just contributes no dependency.
		return nil
	}

	edges := make([]Edge, 0, len(imports))
	for _, imported := range imports {
		if options.IgnoresImportKind(imported.Kind) {
			continue
		}
		targets, external := b.targets.classify(imported.Path)
		if external {
			// The import path itself, exactly as the file wrote it: an external target is a module this
			// library knows nothing else about, and rules about external dependencies match against it.
			edges = append(edges, NewEdge(node.Identifier, imported.Path, true, imported.Kind))
			continue
		}
		// No target at all is the project's own code with no node to point at, and yields no edge.
		for _, target := range targets {
			edges = append(edges, NewEdge(node.Identifier, target, false, imported.Kind))
		}
	}
	return edges
}

// buildFlags turns build tags into the flags the toolchain takes them as. No tags means no flag, so the
// toolchain's own defaults for the host platform apply.
func buildFlags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	return []string{buildTagFlag + strings.Join(tags, ",")}
}
