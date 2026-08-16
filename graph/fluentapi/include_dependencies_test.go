package fluentapi_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestIncludingExternalDependenciesDrawsWhatTheProjectDependsOn(t *testing.T) {
	// The report that asks what a project pulls in rather than how it is arranged. The fixture imports net/http
	// and, blankly, database/sql, and both are nodes only when the chain asks for them.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).IncludingExternalDependencies().Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	labels := nodeLabels(snapshot.Nodes())
	if !slices.Contains(labels, "net/http") || !slices.Contains(labels, "database/sql") {
		t.Errorf("the report's nodes are %v, want the two import paths among them", labels)
	}
	want := projection.Summary{Nodes: 7, Edges: 6, Dependencies: 6, ExternalNodes: 2, ExternalEdges: 2}
	if summary := snapshot.Summary(); summary != want {
		t.Errorf("the summary is %+v, want %+v", summary, want)
	}
}

func TestIncludingExternalDependenciesKeepsTheBlankImportsKindOnTheEdge(t *testing.T) {
	// A blank import registers a driver and depends on no API, so a reader looking at an arrow to database/sql
	// needs to be told which kind it is. It is the union of the kinds behind the edge, and here there is one.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		IncludingExternalDependencies().
		CollapseToFolderDepth(2).
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	want := "internal/db -> database/sql [1 dependency] (external) [blank]"
	if edges := edgeDescriptions(snapshot.Edges()); !slices.Contains(edges, want) {
		t.Errorf("the report's edges are %v, want %q among them", edges, want)
	}
}

func TestTheDefaultReportIsAboutTheProjectsOwnCode(t *testing.T) {
	// A diagram in which fmt and net/http are nodes mostly draws somebody else's code, so the modifier is off
	// unless it is asked for — and then no edge of the report leaves the project either.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	for _, node := range snapshot.Nodes() {
		if node.IsExternal() {
			t.Errorf("the default report draws %v, want the project's own files only", node)
		}
	}
	if summary := snapshot.Summary(); summary.ExternalEdges != 0 {
		t.Errorf("the summary is %v, want no edge leaving the project", summary)
	}
}

func TestIncludingSelfDependenciesDrawsTheDependenciesInsideACollapsedFolder(t *testing.T) {
	// The cohesion report: a folder with a loud self-dependency is one whose files belong together. Collapsed to
	// one segment, the fixture is `.` and `internal`, and the two dependencies from api to db are both inside
	// `internal` — which is the arrow this modifier draws and the default report leaves out.
	root := writeFixtureProject(t)
	locator := fixtureLocator(t, root)

	with, err := fluentapi.ProjectGraph(locator).CollapseToFolderDepth(1).IncludingSelfDependencies().Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	without, err := fluentapi.ProjectGraph(locator).CollapseToFolderDepth(1).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	want := "internal -> internal [2 dependencies] [plain]"
	if edges := edgeDescriptions(with.Edges()); !slices.Contains(edges, want) {
		t.Errorf("the report's edges are %v, want %q among them", edges, want)
	}
	for _, edge := range without.Edges() {
		if edge.IsSelfDependency() {
			t.Errorf("the default report draws %v, want no dependency from a node to itself", edge)
		}
	}
	if with.Summary().Nodes != without.Summary().Nodes {
		t.Errorf("including self dependencies drew %d nodes, want the same %d: it adds arrows, not boxes",
			with.Summary().Nodes, without.Summary().Nodes)
	}
}

func TestIncludingSelfDependenciesDrawsNoneWithoutACollapse(t *testing.T) {
	// A file does not depend on itself, so the modifier only ever has something to say after a collapse.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).IncludingSelfDependencies().Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	for _, edge := range snapshot.Edges() {
		if edge.IsSelfDependency() {
			t.Errorf("the uncollapsed report draws %v, want no dependency from a file to itself", edge)
		}
	}
}

func TestTheIncludingModifiersAreIdempotentAndOrderIndependent(t *testing.T) {
	// Participles, chainable, and saying the same thing twice says it once.
	twice := fluentapi.ProjectGraph(nil).IncludingSelfDependencies().IncludingExternalDependencies().IncludingSelfDependencies()

	want := "project graph, including external dependencies, including self dependencies"
	if got := twice.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if strings.Count(twice.String(), "including self dependencies") != 1 {
		t.Errorf("String() = %q, want the modifier in the sentence once", twice)
	}
}
