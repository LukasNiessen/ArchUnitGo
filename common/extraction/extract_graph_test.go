package extraction

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// thisModulePath is the path of the module the tests dogfooding this repository are about. Nothing in the
// library holds it — the toolchain reports it — so a test asserting how this repository is classified has
// to write it out.
const thisModulePath = "github.com/LukasNiessen/ArchUnitGo"

// writeSourceProject writes a project whose files have the content the test gave them, which is what
// resolving imports needs and what writeProject — every file a bare `package fixture` — cannot give.
// The module is `example.com/fixture`, so a project package's import path starts with that.
func writeSourceProject(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	writeProjectFile(t, root, moduleFileName, "module example.com/fixture\n\ngo 1.26\n")
	for identifier, content := range files {
		writeProjectFile(t, root, identifier, content)
	}
	return root
}

// fixtureSourceProject is a project with one of everything the resolution has to have an opinion about:
// an import of the project's own code, an import of the standard library, all four flavors of import
// declaration, a package built from two files, a file that imports nothing at all, and a test file.
//
// It compiles, so that nothing in a test that uses it depends on how a broken package is reported.
func fixtureSourceProject() map[string]string {
	return map[string]string{
		"main.go": `package main

import (
	"fmt"

	"example.com/fixture/internal/api"
)

func main() { fmt.Println(api.Handle()) }
`,
		"internal/api/handler.go": `package api

import (
	"strings"

	"example.com/fixture/internal/db"
)

func Handle() string { return strings.TrimSpace(db.Query()) }
`,
		"internal/api/handler_test.go": `package api

import (
	"testing"

	"example.com/fixture/internal/db"
)

func TestHandle(t *testing.T) {
	if Handle() != db.Query() {
		t.Error("the fixture disagrees with itself")
	}
}
`,
		"internal/api/router.go": `package api

func Route() string { return "/" }
`,
		"internal/db/conn.go": `package db

import (
	quoted "strings"

	_ "example.com/fixture/internal/db/driver"
)

func Query() string { return quoted.TrimSpace(" row ") }
`,
		"internal/db/query.go": `package db

const statement = "select 1"
`,
		"internal/db/driver/pq.go": `package driver

import . "errors"

var errNotRegistered = New("no driver")
`,
	}
}

// fixtureSourceGraph is the whole graph of fixtureSourceProject under the default options: a self-edge
// per file, one edge per import of the standard library, and one edge per *file* of an imported package
// of the project's own — importing a package is depending on every file it is built from.
//
// The test file is absent, from both ends: the defaults leave it out.
func fixtureSourceGraph() Graph {
	return NewGraph(
		SelfEdge("main.go"),
		SelfEdge("internal/api/handler.go"),
		SelfEdge("internal/api/router.go"),
		SelfEdge("internal/db/conn.go"),
		SelfEdge("internal/db/query.go"),
		SelfEdge("internal/db/driver/pq.go"),

		NewEdge("main.go", "fmt", true, ImportKindPlain),
		NewEdge("main.go", "internal/api/handler.go", false, ImportKindPlain),
		NewEdge("main.go", "internal/api/router.go", false, ImportKindPlain),

		NewEdge("internal/api/handler.go", "strings", true, ImportKindPlain),
		NewEdge("internal/api/handler.go", "internal/db/conn.go", false, ImportKindPlain),
		NewEdge("internal/api/handler.go", "internal/db/query.go", false, ImportKindPlain),

		NewEdge("internal/db/conn.go", "strings", true, ImportKindAliased),
		NewEdge("internal/db/conn.go", "internal/db/driver/pq.go", false, ImportKindBlank),

		NewEdge("internal/db/driver/pq.go", "errors", true, ImportKindDot),
	)
}

// extractGraph runs the whole EXTRACT stage over a project and fails the test if it could not.
func extractGraph(t *testing.T, root string, options *SourceOptions) Graph {
	t.Helper()

	graph, err := ExtractGraph(root, options)
	if err != nil {
		t.Fatalf("ExtractGraph failed: %v", err)
	}
	return graph
}

func TestExtractGraphResolvesEveryImportToItsTargets(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	graph := extractGraph(t, root, nil)

	if want := fixtureSourceGraph(); !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
}

func TestExtractGraphGivesEveryFileASelfEdge(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	graph := extractGraph(t, root, nil)

	// Every file of the project has one, whether or not any import mentions it: internal/db/query.go
	// imports nothing and nothing points at it by name, so its self-edge is the only reason it is a node.
	files, err := ExtractSourceFiles(root, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}
	for _, file := range files {
		if _, found := graph.Find(file.Identifier, file.Identifier); !found {
			t.Errorf("graph =\n%s\n\nwant a self-edge for %q", graph, file.Identifier)
		}
	}

	selfEdges := 0
	for _, edge := range graph {
		if edge.IsSelfEdge() {
			selfEdges++
		}
	}
	if selfEdges != len(files) {
		t.Errorf("graph =\n%s\n\nhas %d self-edges, want the %d files of the project", graph, selfEdges, len(files))
	}
}

func TestExtractGraphGivesEveryNodeOfEveryEdgeASelfEdge(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	graph := extractGraph(t, root, nil)

	// The invariant a node projection rests on: every node an edge mentions is in the self-edges, so
	// the two halves of a graph name the same population. Only an external target is exempt — an import
	// path is not a file of the project, and no self-edge carries one.
	for _, edge := range graph {
		if _, found := graph.Find(edge.Source, edge.Source); !found {
			t.Errorf("edge %s has a source with no self-edge", edge)
		}
		if edge.External {
			if _, found := graph.Find(edge.Target, edge.Target); found {
				t.Errorf("edge %s has a self-edge for its external target", edge)
			}
			continue
		}
		if _, found := graph.Find(edge.Target, edge.Target); !found {
			t.Errorf("edge %s points at a node with no self-edge", edge)
		}
	}
}

func TestExtractGraphMergesAFilesImportOfItsOwnPackageIntoItsSelfEdge(t *testing.T) {
	// Importing your own package is illegal Go, but it is a string a file can write, and the toolchain
	// resolves it to every file the package is built from — the importing file among them. The edge to
	// itself is therefore emitted, and it must arrive as the one self-edge shape rather than as a
	// self-edge claiming a plain import: projections drop self-edges without reading their kinds.
	root := writeSourceProject(t, map[string]string{
		"main.go": `package main

import "example.com/fixture/internal/api"

func main() { _ = api.Handle() }
`,
		"internal/api/handler.go": `package api

import "example.com/fixture/internal/api"

func Handle() string { return api.Route() }
`,
		"internal/api/router.go": `package api

func Route() string { return "/" }
`,
	})

	graph := extractGraph(t, root, nil)

	want := NewGraph(
		SelfEdge("main.go"),
		SelfEdge("internal/api/handler.go"),
		SelfEdge("internal/api/router.go"),
		NewEdge("main.go", "internal/api/handler.go", false, ImportKindPlain),
		NewEdge("main.go", "internal/api/router.go", false, ImportKindPlain),
		// The other file of its own package is a dependency the file really has.
		NewEdge("internal/api/handler.go", "internal/api/router.go", false, ImportKindPlain),
	)
	if !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
	for _, edge := range graph.SelfEdges() {
		if !edge.ImportKinds.Empty() || edge.External {
			t.Errorf("self-edge %+v, want no import kinds and not external", edge)
		}
	}
}

func TestExtractGraphMarksATargetOutsideTheProjectExternal(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	graph := extractGraph(t, root, nil)

	for _, edge := range graph {
		outside := !slices.Contains([]string{
			"main.go",
			"internal/api/handler.go",
			"internal/api/router.go",
			"internal/db/conn.go",
			"internal/db/query.go",
			"internal/db/driver/pq.go",
		}, edge.Target)
		if edge.External != outside {
			t.Errorf("edge %s has External = %v, want %v", edge, edge.External, outside)
		}
	}
}

func TestExtractGraphMergesParallelImportsOfOnePackage(t *testing.T) {
	// Two imports of one package in one file is legal Go as long as they are named differently, and it
	// is the one way a file produces parallel edges. Downstream code may assume (source, target) is
	// unique, so the two arrive as one edge carrying both flavors.
	root := writeSourceProject(t, map[string]string{
		"main.go": `package main

import (
	"strings"
	quoted "strings"
)

func main() { _ = strings.TrimSpace(quoted.ToUpper("")) }
`,
	})

	graph := extractGraph(t, root, nil)

	edge, found := graph.Find("main.go", "strings")
	if !found {
		t.Fatalf("graph =\n%s\n\nwant an edge to strings", graph)
	}
	want := NewImportKindSet(ImportKindPlain, ImportKindAliased)
	if edge.ImportKinds != want {
		t.Errorf("edge %s has ImportKinds %s, want %s", edge, edge.ImportKinds, want)
	}
}

func TestExtractGraphMergesParallelImportsOfOneOfItsOwnPackages(t *testing.T) {
	// The internal half of the same thing, and the one that really produces parallel edges: two imports
	// of a package built from two files are four edges before the merge and two after it, each carrying
	// both flavors.
	root := writeSourceProject(t, map[string]string{
		"main.go": `package main

import (
	"example.com/fixture/internal/db"
	stored "example.com/fixture/internal/db"
)

func main() { _ = db.Query() + stored.Statement }
`,
		"internal/db/conn.go": `package db

func Query() string { return "row" }
`,
		"internal/db/query.go": `package db

const Statement = "select 1"
`,
	})

	graph := extractGraph(t, root, nil)

	want := NewImportKindSet(ImportKindPlain, ImportKindAliased)
	for _, target := range []string{"internal/db/conn.go", "internal/db/query.go"} {
		edge, found := graph.Find("main.go", target)
		if !found {
			t.Errorf("graph =\n%s\n\nwant an edge to %s", graph, target)
			continue
		}
		if edge.ImportKinds != want {
			t.Errorf("edge %s has ImportKinds %s, want %s", edge, edge.ImportKinds, want)
		}
	}
	if dependencies := graph.Dependencies(); len(dependencies) != 2 {
		t.Errorf("dependencies =\n%s\n\nwant the four resolved edges merged into two", dependencies)
	}
}

func TestExtractGraphSplitsIntoOneNodePerFileAndTheDependenciesBetweenThem(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	graph := extractGraph(t, root, nil)

	files, err := ExtractSourceFiles(root, nil)
	if err != nil {
		t.Fatalf("ExtractSourceFiles failed: %v", err)
	}
	nodes := make([]string, 0, len(files))
	for _, file := range files {
		nodes = append(nodes, file.Identifier)
	}

	// What a node projection sees: exactly the files of the project, each once, whether or not any
	// import mentions it.
	got := make([]string, 0, len(graph))
	for _, edge := range graph.SelfEdges() {
		got = append(got, edge.Source)
	}
	if !slices.Equal(got, nodes) {
		t.Errorf("SelfEdges() name %v, want the %d files of the project %v", got, len(nodes), nodes)
	}
	// And what an edge projection sees: the dependencies, with none of the nodes' own edges among them.
	if len(graph.Dependencies())+len(nodes) != len(graph) {
		t.Errorf("graph =\n%s\n\ndoes not split into %d nodes and its dependencies", graph, len(nodes))
	}
}

func TestExtractGraphDropsTheImportKindsTheOptionsIgnore(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	// The usual reason to ignore blank imports: they register a driver and depend on no API of it.
	graph := extractGraph(t, root, &SourceOptions{IgnoredImportKinds: NewImportKindSet(ImportKindBlank)})

	if _, found := graph.Find("internal/db/conn.go", "internal/db/driver/pq.go"); found {
		t.Errorf("graph =\n%s\n\nwant the blank import of the driver left out", graph)
	}
	// Dropping an import does not drop a file: the driver is still a node, with its own dependency.
	if _, found := graph.Find("internal/db/driver/pq.go", "internal/db/driver/pq.go"); !found {
		t.Errorf("graph =\n%s\n\nwant the driver to still be a node", graph)
	}
	if _, found := graph.Find("internal/db/conn.go", "strings"); !found {
		t.Errorf("graph =\n%s\n\nwant the other imports of the same file kept", graph)
	}
}

// ignoreDirectiveProject is a project whose main file asks for two of its own imports to be left out of
// the graph: one with a bare directive, which holds for every analysis, and one scoped to `layers`, which
// holds only where that scope is answered to. The third import carries nothing and is the control.
//
// It compiles, for the reason fixtureSourceProject does: a directive is a comment, so leaving an import
// out of the graph leaves it in the build.
func ignoreDirectiveProject() map[string]string {
	return map[string]string{
		"main.go": `package main

import (
	"fmt" //archunit:ignore

	//archunit:ignore layers
	"example.com/fixture/internal/api"
	"example.com/fixture/internal/db"
)

func main() { fmt.Println(api.Handle(), db.Query()) }
`,
		"internal/api/handler.go": `package api

func Handle() string { return "handled" }
`,
		"internal/db/conn.go": `package db

func Query() string { return "row" }
`,
	}
}

func TestExtractGraphDropsTheImportsTheFileItselfIgnores(t *testing.T) {
	root := writeSourceProject(t, ignoreDirectiveProject())

	// Under the defaults: an unscoped directive is honored, because a file saying "not this import" needs
	// no configuration to be believed, and a scoped one is not, because no analysis answers to a scope.
	graph := extractGraph(t, root, nil)

	want := NewGraph(
		SelfEdge("main.go"),
		SelfEdge("internal/api/handler.go"),
		SelfEdge("internal/db/conn.go"),

		NewEdge("main.go", "internal/api/handler.go", false, ImportKindPlain),
		NewEdge("main.go", "internal/db/conn.go", false, ImportKindPlain),
	)
	if !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
}

func TestExtractGraphHonorsAScopedIgnoreDirectiveOnlyWhereItsScopeIsAnsweredTo(t *testing.T) {
	root := writeSourceProject(t, ignoreDirectiveProject())

	graph := extractGraph(t, root, &SourceOptions{IgnoreScopes: []string{"layers", "slices"}})

	if _, found := graph.Find("main.go", "internal/api/handler.go"); found {
		t.Errorf("graph =\n%s\n\nwant the import scoped to layers left out", graph)
	}
	// Dropping an import does not drop a file: the ignored package is still a node.
	if _, found := graph.Find("internal/api/handler.go", "internal/api/handler.go"); !found {
		t.Errorf("graph =\n%s\n\nwant the ignored package to still be a node", graph)
	}
	if _, found := graph.Find("main.go", "internal/db/conn.go"); !found {
		t.Errorf("graph =\n%s\n\nwant the other imports of the same file kept", graph)
	}

	// And nowhere else: the same source read by an analysis answering to another name has the dependency.
	elsewhere := extractGraph(t, root, &SourceOptions{IgnoreScopes: []string{"slices"}})
	if _, found := elsewhere.Find("main.go", "internal/api/handler.go"); !found {
		t.Errorf("graph =\n%s\n\nwant the import kept where its scope is not answered to", elsewhere)
	}
}

func TestExtractGraphIncludesTestFilesWhenAsked(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	graph := extractGraph(t, root, &SourceOptions{IncludeTestFiles: true})

	// A test file is a node like any other, with the dependencies it declares.
	want := fixtureSourceGraph().Add(
		SelfEdge("internal/api/handler_test.go"),
		NewEdge("internal/api/handler_test.go", "testing", true, ImportKindPlain),
		NewEdge("internal/api/handler_test.go", "internal/db/conn.go", false, ImportKindPlain),
		NewEdge("internal/api/handler_test.go", "internal/db/query.go", false, ImportKindPlain),
	)
	if !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
	// And it is never a target: main.go imports the api package, which outside its own test binary is
	// built from its production files alone.
	if _, found := graph.Find("main.go", "internal/api/handler_test.go"); found {
		t.Errorf("graph =\n%s\n\nwant no import pointing at a test file", graph)
	}
}

func TestExtractGraphReadsTheProjectUnderTheBuildTagsItIsGiven(t *testing.T) {
	root := writeSourceProject(t, map[string]string{
		"main.go": `package main

import "example.com/fixture/internal/platform"

func main() { _ = platform.Name() }
`,
		"internal/platform/portable.go": `//go:build !fixture

package platform

func Name() string { return "portable" }
`,
		"internal/platform/tagged.go": `//go:build fixture

package platform

import "strings"

func Name() string { return strings.ToUpper("tagged") }
`,
	})

	// A file the constraints exclude is not in the build, so it is not a node and no import points at it.
	byDefault := extractGraph(t, root, nil)
	want := NewGraph(
		SelfEdge("main.go"),
		SelfEdge("internal/platform/portable.go"),
		NewEdge("main.go", "internal/platform/portable.go", false, ImportKindPlain),
	)
	if !slices.Equal(byDefault, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", byDefault, want)
	}

	tagged := extractGraph(t, root, &SourceOptions{BuildTags: []string{"fixture"}})
	want = NewGraph(
		SelfEdge("main.go"),
		SelfEdge("internal/platform/tagged.go"),
		NewEdge("main.go", "internal/platform/tagged.go", false, ImportKindPlain),
		NewEdge("internal/platform/tagged.go", "strings", true, ImportKindPlain),
	)
	if !slices.Equal(tagged, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", tagged, want)
	}
}

func TestExtractGraphSkipsAFileThatDoesNotParse(t *testing.T) {
	// One file the parser cannot finish must not fail every rule in a suite: the rest of the project is
	// extracted as usual, and the broken file keeps the dependencies declared above the break.
	root := writeSourceProject(t, map[string]string{
		"main.go": `package main

import "example.com/fixture/internal/api"

func main() { api.Handle() }
`,
		"internal/api/handler.go": `package api

import (
	"fmt"
	"strings
)
`,
	})

	graph := extractGraph(t, root, nil)

	want := NewGraph(
		SelfEdge("main.go"),
		SelfEdge("internal/api/handler.go"),
		NewEdge("main.go", "internal/api/handler.go", false, ImportKindPlain),
		NewEdge("internal/api/handler.go", "fmt", true, ImportKindPlain),
	)
	if !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
}

func TestExtractGraphDropsAnImportOfAPackageTheWalkExcluded(t *testing.T) {
	root := writeSourceProject(t, map[string]string{
		"main.go": `package main

import "example.com/fixture/build/tool"

func main() { tool.Run() }
`,
		"build/tool/tool.go": `package tool

func Run() {}
`,
	})

	graph := extractGraph(t, root, nil)

	// `build` is excluded by default, so the package it holds has no node for an edge to point at. It is
	// still the project's own code, so it must not turn up as an external module either.
	if want := NewGraph(SelfEdge("main.go")); !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
}

func TestExtractGraphDoesNotCallAMissingPackageOfTheProjectsOwnExternal(t *testing.T) {
	root := writeSourceProject(t, map[string]string{
		"main.go": `package main

import (
	"example.com/fixture/internal/nope"
	"example.com/fixture/docs"
)

func main() { nope.Run(docs.Text) }
`,
		// A folder of the project holding no Go source is not a package either, so an import of it is the
		// same case as one of a package that was never written.
		"docs/README.md": "The toolchain reports no package for this folder.\n",
	})

	graph := extractGraph(t, root, nil)

	// The project does not compile, and a project mid-refactor still has a shape. What must not happen is
	// the project's own path turning up as an external module: every rule about third-party dependencies
	// would fire on it.
	if want := NewGraph(SelfEdge("main.go")); !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
}

func TestExtractGraphMarksAModuleNestedInTheProjectExternal(t *testing.T) {
	// A module inside the project is a module of its own however its path reads: separately versioned,
	// resolved through the module graph, and none of its files in this project's build. So a dependency on
	// it is external and keeps the import path as its target — the opposite answer from the missing
	// package above, and a go.mod is the whole difference.
	root := writeSourceProject(t, map[string]string{
		moduleFileName: `module example.com/fixture

go 1.26

require example.com/fixture/tools v0.0.0

replace example.com/fixture/tools => ./tools
`,
		"main.go": `package main

import "example.com/fixture/tools/generate"

func main() { generate.Run() }
`,
		"tools/" + moduleFileName: "module example.com/fixture/tools\n\ngo 1.26\n",
		"tools/generate/generate.go": `package generate

func Run() {}
`,
	})

	graph := extractGraph(t, root, nil)

	want := NewGraph(
		SelfEdge("main.go"),
		NewEdge("main.go", "example.com/fixture/tools/generate", true, ImportKindPlain),
	)
	if !slices.Equal(graph, want) {
		t.Errorf("graph =\n%s\n\nwant\n%s", graph, want)
	}
}

func TestExtractGraphIsReproducible(t *testing.T) {
	root := writeSourceProject(t, fixtureSourceProject())

	first := extractGraph(t, root, nil)
	second := extractGraph(t, root, nil)

	// The toolchain reports packages in whatever order it likes and the merge runs over a map, so the
	// order edges come out in has to be established rather than inherited.
	if !slices.IsSortedFunc(first, compareEdges) {
		t.Errorf("graph =\n%s\n\nwant the edges ordered by source then target", first)
	}
	if !slices.Equal(first, second) {
		t.Errorf("two extractions of one project disagree:\n%s\n\nand\n%s", first, second)
	}
}

func TestExtractGraphFindsNothingRatherThanFailingInAnEmptyProject(t *testing.T) {
	root := writeSourceProject(t, nil)

	graph := extractGraph(t, root, nil)

	// Whether an empty selection is a problem is a rule's question, answered by the empty-test guard.
	if len(graph) != 0 {
		t.Errorf("graph =\n%s\n\nwant no edges", graph)
	}
}

func TestExtractGraphRejectsARootThatIsNotAProject(t *testing.T) {
	root := writeSourceProject(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})

	_, err := ExtractGraph(filepath.Join(root, "main.go"), nil)

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("ExtractGraph error = %v, want a *archerror.TechnicalError", err)
	}
	if !errors.Is(err, ErrNotADirectory) {
		t.Errorf("ExtractGraph error = %v, want it to wrap ErrNotADirectory", err)
	}
}

func TestExtractGraphExtractsThisRepository(t *testing.T) {
	// The level above the hand-written fixtures: this repository, located and extracted the way a check
	// will do it, with nothing hand-built about any step.
	root, err := LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}

	graph := extractGraph(t, root, nil)

	// An import of the project's own code, resolved to two files of the imported package — the library
	// imports packages, and a package is every file it is built from; an import of the standard library
	// and one of the analysis toolchain, both external; and a file that is a node.
	wanted := []Edge{
		NewEdge("common/fluentapi/check_options.go", "common/extraction/edge.go", false, ImportKindPlain),
		NewEdge("common/fluentapi/check_options.go", "common/extraction/import_kind.go", false, ImportKindPlain),
		NewEdge("common/extraction/extract_graph.go", "golang.org/x/tools/go/packages", true, ImportKindPlain),
		NewEdge("common/extraction/extract_imports.go", "go/ast", true, ImportKindPlain),
		SelfEdge("common/matching/regex_factory.go"),
	}
	for _, want := range wanted {
		found, ok := graph.Find(want.Source, want.Target)
		if !ok {
			t.Errorf("graph has no edge %s -> %s", want.Source, want.Target)
			continue
		}
		if found != want {
			t.Errorf("edge = %s, want %s", found, want)
		}
	}
	for _, edge := range graph {
		if edge.External && !edge.IsSelfEdge() {
			continue
		}
		// Every internal identifier is a project-relative Go file, never a package or a folder.
		for _, identifier := range []string{edge.Source, edge.Target} {
			if filepath.Ext(identifier) != goFileExtension {
				t.Errorf("edge %s has %q as a node, want a Go file of the project", edge, identifier)
			}
		}
	}
}

func TestExtractGraphClassifiesThisRepositoriesDependencies(t *testing.T) {
	root, err := LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}

	graph := extractGraph(t, root, nil)

	externals := 0
	for _, edge := range graph {
		if !edge.External {
			continue
		}
		externals++
		// The target of an external edge is the import path as the file wrote it, so it is the standard
		// library or a dependency module — never this library's own code under another name.
		if strings.HasPrefix(edge.Target, thisModulePath) {
			t.Errorf("edge %s calls this repository's own code an external module", edge)
		}
	}
	if externals == 0 {
		// This library imports the standard library everywhere, so nothing being external means the
		// classification collapsed rather than that there was nothing to classify.
		t.Errorf("graph =\n%s\n\nwant some external edges", graph)
	}
}
