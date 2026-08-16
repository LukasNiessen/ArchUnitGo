package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestProjectSnapshotFocusedOnZeroHopsKeepsTheSelectedFilesAlone(t *testing.T) {
	// The `show me only these` report. The seeds are always in the result, so depth 0 is the selection itself.
	query := projection.SnapshotOptions{Focus: []projection.Focus{{Selector: pathMatcher(t, "internal/db/**")}}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{"internal/db/conn.go", "internal/db/query.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("focusing on the db folder kept %v, want %v", labels, wantNodes)
	}
	wantEdges := []string{"internal/db/conn.go -> internal/db/query.go [1 dependency] [plain]"}
	if edges := edgeDescriptions(snapshot.Edges()); !slices.Equal(edges, wantEdges) {
		t.Errorf("the edges are %v, want %v: a dependency with an end off the diagram is an arrow to nowhere", edges, wantEdges)
	}
}

func TestProjectSnapshotFocusedOnOneHopKeepsTheNeighborhoodInBothDirections(t *testing.T) {
	// `What is around this code` is not `what does it depend on`: a reader zooming in on one folder wants its
	// collaborators on both sides of it. The isolated file is a hop away from nothing, so it goes.
	query := projection.SnapshotOptions{Focus: []projection.Focus{{Selector: pathMatcher(t, "internal/db/**"), Depth: 1}}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"internal/db/query.go",
		"main.go",
	}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("focusing one hop out kept %v, want %v", labels, wantNodes)
	}
}

func TestProjectSnapshotFocusedOnANegativeDepthKeepsTheSelectedFilesAlone(t *testing.T) {
	// A mistyped depth should narrow a report, never blow it up into the whole graph.
	query := projection.SnapshotOptions{Focus: []projection.Focus{{Selector: pathMatcher(t, "main.go"), Depth: -3}}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, []string{"main.go"}) {
		t.Errorf("focusing with a negative depth kept %v, want main.go alone", labels)
	}
}

func TestProjectSnapshotFocusedOnAPatternThatNamesNothingKeepsNothing(t *testing.T) {
	// The stale glob. An empty report is not quietly rendered: the fluent terminal turns it into
	// ErrEmptySnapshot, and this is the projection half of that guard.
	query := projection.SnapshotOptions{Focus: []projection.Focus{{Selector: pathMatcher(t, "internal/transport/**"), Depth: 2}}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	if !snapshot.Empty() {
		t.Errorf("focusing on a folder nothing is in kept %v, want nothing at all", nodeLabels(snapshot.Nodes()))
	}
}

func TestProjectSnapshotReachableFromFollowsTheArrowsForwardsAllTheWay(t *testing.T) {
	// The `what does this pull in` report: what a binary actually reaches, however far away — the seeds
	// included, because a report about what code depends on should show the code.
	query := projection.SnapshotOptions{ReachableFrom: []matching.Filter{pathMatcher(t, "main.go")}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

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

func TestProjectSnapshotReachableFromIncludesTheExternalModulesTheCodePullsIn(t *testing.T) {
	// The two options compose: what this code reaches, including the parts of it that are not this project's.
	query := projection.SnapshotOptions{
		IncludeExternalDependencies: true,
		ReachableFrom:               []matching.Filter{pathMatcher(t, "internal/db/conn.go")},
	}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{"database/sql", "internal/db/conn.go", "internal/db/query.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("what conn.go reaches is %v, want %v", labels, wantNodes)
	}
}

func TestProjectSnapshotDependentsOfFollowsTheArrowsBackwardsAllTheWay(t *testing.T) {
	// The impact-analysis report: who would notice if this code changed.
	query := projection.SnapshotOptions{DependentsOf: []matching.Filter{pathMatcher(t, "internal/db/query.go")}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"internal/db/query.go",
		"main.go",
	}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("what depends on query.go is %v, want %v", labels, wantNodes)
	}
}

func TestProjectSnapshotDependentsOfSomethingNothingDependsOnKeepsItAlone(t *testing.T) {
	// The answer worth knowing before deleting a folder.
	query := projection.SnapshotOptions{DependentsOf: []matching.Filter{pathMatcher(t, "internal/util/**")}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, []string{"internal/util/orphan.go"}) {
		t.Errorf("what depends on the isolated file is %v, want the file alone", labels)
	}
}

func TestProjectSnapshotKeepsOnlyTheNodesEveryQueryOptionNames(t *testing.T) {
	// Every modifier narrows the report, so a query holding two of them means the nodes both describe — which
	// is what makes `reachable from` plus `dependents of` the `everything between these two` report.
	query := projection.SnapshotOptions{
		ReachableFrom: []matching.Filter{pathMatcher(t, "internal/api/handler.go")},
		DependentsOf:  []matching.Filter{pathMatcher(t, "internal/db/conn.go")},
	}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{"internal/api/handler.go", "internal/db/conn.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("what is between handler.go and conn.go is %v, want %v", labels, wantNodes)
	}
}

func TestProjectSnapshotIsTheSameWhicheverOrderTheQueryOptionsWereWrittenIn(t *testing.T) {
	// The modifiers are order-independent, and that is only true because each one is resolved against the
	// whole graph rather than against what an earlier one left: a focus applied to what another focus already
	// narrowed would reach a hop into a set the first one had removed, and the two orders would disagree.
	around := projection.Focus{Selector: pathMatcher(t, "internal/api/**"), Depth: 1}
	alone := projection.Focus{Selector: pathMatcher(t, "internal/db/**")}
	first := projection.SnapshotOptions{Focus: []projection.Focus{around, alone}}
	second := projection.SnapshotOptions{Focus: []projection.Focus{alone, around}}

	one := projection.ProjectSnapshot(fixtureGraph(), &first)
	other := projection.ProjectSnapshot(fixtureGraph(), &second)

	wantNodes := []string{"internal/db/conn.go", "internal/db/query.go"}
	if labels := nodeLabels(one.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the two focuses kept %v, want the %v both of them name", labels, wantNodes)
	}
	if one.String() != other.String() {
		t.Errorf("the same two focuses in two orders rendered\n%s\nand\n%s", one, other)
	}
}

func TestProjectSnapshotWalksACycleWithoutRunningForever(t *testing.T) {
	// A dependency graph is not a tree and this library's own has cycles in it. The reached set doubles as the
	// visited set, so every node is expanded once.
	graph := extraction.NewGraph(
		extraction.SelfEdge("a.go"),
		extraction.SelfEdge("b.go"),
		extraction.SelfEdge("c.go"),
		extraction.NewEdge("a.go", "b.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("b.go", "c.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("c.go", "a.go", false, extraction.ImportKindPlain),
	)
	query := projection.SnapshotOptions{ReachableFrom: []matching.Filter{pathMatcher(t, "a.go")}}

	snapshot := projection.ProjectSnapshot(graph, &query)

	wantNodes := []string{"a.go", "b.go", "c.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("what a.go reaches around the cycle is %v, want %v", labels, wantNodes)
	}
}
