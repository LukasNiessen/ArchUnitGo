package projection

import (
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// mapFunction is one of the four factories of map_function.go together with the answer it is supposed
// to give for an edge of the graph.
type mapFunction struct {
	name   string
	mapper func() MapFunction
	keeps  func(edge extraction.Edge) bool
}

// mapFunctions is all four of them. Keeping them in one table is what makes the family's one rule
// checkable in a loop: every `per <thing> edge` factory is about dependencies, so Identity is the only
// one that keeps a self-edge.
func mapFunctions() []mapFunction {
	return []mapFunction{
		{
			name:   "Identity",
			mapper: Identity,
			keeps:  func(extraction.Edge) bool { return true },
		},
		{
			name:   "PerEdge",
			mapper: PerEdge,
			keeps:  func(edge extraction.Edge) bool { return !edge.IsSelfEdge() },
		},
		{
			name:   "PerInternalEdge",
			mapper: PerInternalEdge,
			keeps:  func(edge extraction.Edge) bool { return !edge.IsSelfEdge() && !edge.External },
		},
		{
			name:   "PerExternalEdge",
			mapper: PerExternalEdge,
			keeps:  func(edge extraction.Edge) bool { return !edge.IsSelfEdge() && edge.External },
		},
	}
}

func TestTheMapFunctionsKeepExactlyTheEdgesTheyName(t *testing.T) {
	for _, factory := range mapFunctions() {
		mapper := factory.mapper()
		for _, edge := range fixtureGraph() {
			_, kept := mapper(edge)
			if want := factory.keeps(edge); kept != want {
				t.Errorf("%s()(%s) kept = %t, want %t", factory.name, edge, kept, want)
			}
		}
	}
}

func TestTheMapFunctionsRelabelNothing(t *testing.T) {
	// All four are the identity labeling — the labels of a file-level projection are the identifiers the
	// extractor minted, which is what the rule's own patterns were written against. Only which edges
	// survive tells them apart.
	for _, factory := range mapFunctions() {
		mapper := factory.mapper()
		for _, edge := range fixtureGraph() {
			mapped, kept := mapper(edge)
			if !kept {
				continue
			}
			if mapped.SourceLabel != edge.Source || mapped.TargetLabel != edge.Target {
				t.Errorf("%s()(%s) = %+v, want the identifiers unchanged", factory.name, edge, mapped)
			}
		}
	}
}

func TestTheMapFunctionsAnswerTheSameEdgeTheSameWayEveryTime(t *testing.T) {
	// The purity a MapFunction promises: nothing is remembered between calls, so a projection is a
	// function of the graph alone and two rules sharing one factory cannot disagree.
	for _, factory := range mapFunctions() {
		first, second := factory.mapper(), factory.mapper()
		for _, edge := range slices.Concat(fixtureGraph(), fixtureGraph()) {
			mappedFirst, keptFirst := first(edge)
			mappedSecond, keptSecond := second(edge)
			if keptFirst != keptSecond || mappedFirst != mappedSecond {
				t.Errorf("%s() answered %s as (%+v, %t) and (%+v, %t), want one answer",
					factory.name, edge, mappedFirst, keptFirst, mappedSecond, keptSecond)
			}
		}
	}
}

func TestIdentityIsTheOnlyMapFunctionThatKeepsASelfEdge(t *testing.T) {
	selfEdge := extraction.SelfEdge("internal/util/noop.go")

	for _, factory := range mapFunctions() {
		mapped, kept := factory.mapper()(selfEdge)
		if kept != (factory.name == "Identity") {
			t.Errorf("%s()(%s) kept = %t, want %t: only Identity carries the edge that names a node",
				factory.name, selfEdge, kept, factory.name == "Identity")
		}
		if kept && mapped.SourceLabel != mapped.TargetLabel {
			t.Errorf("%s()(%s) = %+v, want both labels to be the node", factory.name, selfEdge, mapped)
		}
	}
}

func TestPerEdgeProjectsTheSameDependenciesAsIdentity(t *testing.T) {
	// Which is why dropping the self-edges in PerEdge costs a rule about dependencies nothing:
	// ProjectEdges drops an edge whose two labels are equal in any case.
	graph := fixtureGraph()

	perEdge := ProjectEdges(graph, PerEdge())
	identity := ProjectEdges(graph, Identity())

	if got, want := labelsOf(perEdge), labelsOf(identity); !slices.Equal(got, want) {
		t.Errorf("ProjectEdges(graph, PerEdge()) = %v, want ProjectEdges(graph, Identity()) = %v", got, want)
	}
}

func TestOnlyIdentityGivesAFileThatDependsOnNothingANode(t *testing.T) {
	// The difference the two do make, and the reason both exist: node projection is written over the
	// self-edges, so a rule about naming or placement is projected through Identity and a rule about
	// dependencies through PerEdge.
	if got := nodeLabels(ProjectToNodes(fixtureGraph(), Identity())); !slices.Contains(got, "internal/util/noop.go") {
		t.Errorf("ProjectToNodes(graph, Identity()) = %v, want the file that depends on nothing among them", got)
	}
	if got := nodeLabels(ProjectToNodes(fixtureGraph(), PerEdge())); slices.Contains(got, "internal/util/noop.go") {
		t.Errorf("ProjectToNodes(graph, PerEdge()) = %v, want no node for the file that depends on nothing", got)
	}
}

func TestPerInternalEdgeAndPerExternalEdgePartitionWhatPerEdgeKeeps(t *testing.T) {
	graph := fixtureGraph()

	internal := ProjectEdges(graph, PerInternalEdge())
	external := ProjectEdges(graph, PerExternalEdge())

	if got, want := labelsOf(internal), []string{
		"internal/api/handler.go -> internal/db/query.go",
		"internal/api/handler.go -> internal/db/repo.go",
		"internal/api/router.go -> internal/api/handler.go",
	}; !slices.Equal(got, want) {
		t.Errorf("PerInternalEdge() projected %v, want %v", got, want)
	}
	if got, want := labelsOf(external), []string{"internal/db/repo.go -> database/sql"}; !slices.Equal(got, want) {
		t.Errorf("PerExternalEdge() projected %v, want %v", got, want)
	}

	// Together they are exactly the dependencies PerEdge keeps: neither half loses an edge and neither
	// claims one twice, which is what makes "external" a decision taken in one place.
	both := slices.Concat(labelsOf(internal), labelsOf(external))
	slices.Sort(both)
	if want := labelsOf(ProjectEdges(graph, PerEdge())); !slices.Equal(both, want) {
		t.Errorf("the two halves projected %v, want the whole of %v", both, want)
	}
}

func TestPerExternalEdgeCarriesTheImportPathTheFileWrote(t *testing.T) {
	projected := ProjectEdges(fixtureGraph(), PerExternalEdge())

	edge := findProjectedEdge(t, projected, "internal/db/repo.go", "database/sql")
	want := extraction.NewGraph(
		extraction.NewEdge("internal/db/repo.go", "database/sql", true, extraction.ImportKindPlain),
	)
	// The raw edge is still under the projected one, so a violation about a forbidden module can name the
	// file that imported it and the kind of import it was.
	if got := edge.CumulatedEdges(); !slices.Equal(got, want) {
		t.Errorf("CumulatedEdges() = %v, want %v", got, want)
	}
}

func TestPerExternalEdgeGivesEveryExternalModuleANode(t *testing.T) {
	nodes := ProjectToNodes(fixtureGraph(), PerExternalEdge())

	// A rule about third-party modules is a rule about those nodes, so the projection holds the module
	// and the files that reach it, and nothing else.
	if got, want := nodeLabels(nodes), []string{"database/sql", "internal/db/repo.go"}; !slices.Equal(got, want) {
		t.Errorf("nodes = %v, want %v", got, want)
	}
	module := findProjectedNode(t, nodes, "database/sql")
	if got, want := labelsOf(module.Incoming()), []string{"internal/db/repo.go -> database/sql"}; !slices.Equal(got, want) {
		t.Errorf("database/sql.Incoming() = %v, want %v", got, want)
	}
	if len(module.Outgoing()) != 0 {
		t.Errorf("database/sql depends on %v, want nothing: the projection stops where the project does", labelsOf(module.Outgoing()))
	}
}

func TestTheMapFunctionsSplitThisRepositoriesDependencies(t *testing.T) {
	// The level above the hand-written fixture, as in project_edges_test.go: this repository, extracted the
	// way a check will do it and projected through each of the factories in turn.
	root, err := extraction.LocateProject(nil)
	if err != nil {
		t.Fatalf("LocateProject(nil) failed: %v", err)
	}
	graph, err := extraction.CachedGraph(root, nil)
	if err != nil {
		t.Fatalf("CachedGraph(%q, nil) failed: %v", root, err)
	}

	// The library's one third-party dependency is the analysis toolchain, imported by the extractor and by
	// nothing else — which is a rule this repository will one day state about itself.
	external := ProjectEdges(graph, PerExternalEdge())
	toolchain := findProjectedEdge(t, external, "common/extraction/extract_graph.go", "golang.org/x/tools/go/packages")
	if len(toolchain.CumulatedEdges()) != 1 {
		t.Errorf("%s cumulates %v, want the one import", toolchain, toolchain.CumulatedEdges())
	}
	for _, edge := range external {
		for _, raw := range edge.CumulatedEdges() {
			if !raw.External {
				t.Errorf("PerExternalEdge() projected %s, which cumulates the internal edge %s", edge, raw)
			}
		}
	}

	// The project's own dependencies are the other half, and this package's on the extractor is one of them.
	internal := ProjectEdges(graph, PerInternalEdge())
	findProjectedEdge(t, internal, "common/projection/map_function.go", "common/extraction/edge.go")
	for _, edge := range internal {
		if strings.HasPrefix(edge.TargetLabel(), "golang.org/") {
			t.Errorf("PerInternalEdge() projected %s, want no dependency that leaves the project", edge)
		}
	}

	// Every file of a real project is a node of the identity projection, the ones that depend on nothing
	// outside themselves included; only the self-edges say so.
	nodes := nodeLabels(ProjectToNodes(graph, Identity()))
	for _, edge := range graph.SelfEdges() {
		if !slices.Contains(nodes, edge.Source) {
			t.Errorf("ProjectToNodes(graph, Identity()) has no node for %q, want every file of the project", edge.Source)
		}
	}
}
