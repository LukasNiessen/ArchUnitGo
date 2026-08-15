package fluentapi_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
)

func TestFocusOnKeepsTheNamedFilesAndTheirNeighborhood(t *testing.T) {
	// Zooming in: depth 0 is the named files alone, one hop adds everything a dependency away on either side.
	root := writeFixtureProject(t)
	locator := fixtureLocator(t, root)

	alone, err := fluentapi.ProjectGraph(locator).FocusOn("internal/db/**", 0).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	around, err := fluentapi.ProjectGraph(locator).FocusOn("internal/db/**", 1).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	wantAlone := []string{"internal/db/conn.go", "internal/db/query.go"}
	if labels := nodeLabels(alone.Nodes()); !slices.Equal(labels, wantAlone) {
		t.Errorf("focusing on the db folder kept %v, want %v", labels, wantAlone)
	}
	wantAround := []string{"internal/api/handler.go", "internal/db/conn.go", "internal/db/query.go"}
	if labels := nodeLabels(around.Nodes()); !slices.Equal(labels, wantAround) {
		t.Errorf("focusing one hop out kept %v, want %v: handler.go is what touches that folder", labels, wantAround)
	}
}

func TestFocusOnLooksBothWaysAlongADependency(t *testing.T) {
	// `What is around this code` is not `what does it depend on`: a reader zooming in on one file wants its
	// collaborators on both sides. main.go depends on handler.go and handler.go depends on the db folder, so one
	// hop from handler.go is both.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).FocusOn("internal/api/handler.go", 1).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{"internal/api/handler.go", "internal/db/conn.go", "internal/db/query.go", "main.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("focusing on handler.go kept %v, want %v", labels, wantNodes)
	}
}

func TestReachableFromKeepsWhatTheNamedFilesPullIn(t *testing.T) {
	// The `what does this binary actually reach` report, followed all the way: main.go reaches the db folder
	// through the api folder, and the two files nothing leads to are not in it.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).ReachableFrom("main.go").Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"internal/db/query.go",
		"main.go",
	}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("what main.go reaches is %v, want %v", labels, wantNodes)
	}
}

func TestDependentsOfKeepsWhatWouldNoticeIfTheNamedFilesChanged(t *testing.T) {
	// The impact-analysis report, the arrows followed backwards: the db folder is reached from handler.go, which
	// is reached from main.go, and router.go is in none of that.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).DependentsOf("internal/db/**").Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{"internal/api/handler.go", "internal/db/conn.go", "internal/db/query.go", "main.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("what depends on the db folder is %v, want %v", labels, wantNodes)
	}
	if slices.Contains(nodeLabels(snapshot.Nodes()), "internal/api/router.go") {
		t.Error("router.go is in the report, want only what leads to the db folder")
	}
}

func TestTheWhichNodesModifiersNarrowEachOther(t *testing.T) {
	// Every modifier of this library narrows rather than widens, so a chain with two of them means the nodes
	// both describe — which is what makes `reachable from` plus `dependents of` the `everything between these
	// two` report.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		ReachableFrom("main.go").
		DependentsOf("internal/db/conn.go").
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{"internal/api/handler.go", "internal/db/conn.go", "main.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("what is between main.go and conn.go is %v, want %v", labels, wantNodes)
	}
}

func TestTheWhichNodesModifiersMatchIdentifiersRatherThanTheLabelsACollapseDraws(t *testing.T) {
	// The only reading in which focusing and collapsing stay order-independent: the query names files, the
	// collapse decides how they are drawn. So this is the neighborhood of handler.go, drawn as folders.
	root := writeFixtureProject(t)
	locator := fixtureLocator(t, root)

	first, err := fluentapi.ProjectGraph(locator).FocusOn("internal/api/handler.go", 1).CollapseToFolderDepth(2).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	second, err := fluentapi.ProjectGraph(locator).CollapseToFolderDepth(2).FocusOn("internal/api/handler.go", 1).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	wantNodes := []string{".", "internal/api", "internal/db"}
	if labels := nodeLabels(first.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the focused and collapsed report drew %v, want %v", labels, wantNodes)
	}
	if first.String() != second.String() {
		t.Errorf("chaining the two modifiers the other way round rendered\n%s\nand\n%s", first, second)
	}
}

func TestFocusOnANegativeDepthKeepsTheNamedFilesAlone(t *testing.T) {
	// A mistyped depth should narrow a report, never blow it up into the whole graph — and the sentence the
	// report renders as says what it will actually draw.
	root := writeFixtureProject(t)
	report := fluentapi.ProjectGraph(fixtureLocator(t, root)).FocusOn("main.go", -1)

	snapshot, err := report.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, []string{"main.go"}) {
		t.Errorf("focusing with a negative depth kept %v, want main.go alone", labels)
	}
	want := `project graph, focus on path matches "main.go" within 0 hops`
	if got := report.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTheWhichNodesModifiersAreChainableAndAllInTheSentence(t *testing.T) {
	// Two of a kind are two modifiers, not one overwriting the other, so both are in the report a reader sees.
	report := fluentapi.ProjectGraph(nil).
		FocusOn("internal/api/**", 1).
		FocusOn("internal/db/**", 0).
		ReachableFrom("main.go").
		DependentsOf("internal/db/**")

	want := `project graph, ` +
		`focus on path matches "internal/api/**" within 1 hop, ` +
		`focus on path matches "internal/db/**" within 0 hops, ` +
		`reachable from path matches "main.go", ` +
		`dependents of path matches "internal/db/**"`
	if got := report.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
