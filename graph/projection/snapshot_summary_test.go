package projection_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestSummaryStringIsTheLineAReportPrintsAboveADiagram(t *testing.T) {
	summary := projection.Summary{Nodes: 9, Edges: 4, Dependencies: 312, ExternalNodes: 2, ExternalEdges: 3}

	want := "9 nodes, 4 edges, 312 dependencies, 2 external nodes, 3 external edges"
	if got := summary.String(); got != want {
		t.Errorf("the summary renders as %q, want %q", got, want)
	}
}

func TestSummaryStringLeavesOutTheExternalCountsWhenThereAreNone(t *testing.T) {
	// A report about a project with no external dependency should not spend a third of its headline saying so,
	// and the default report has none of them at all.
	summary := projection.Summary{Nodes: 3, Edges: 2, Dependencies: 2}

	want := "3 nodes, 2 edges, 2 dependencies"
	if got := summary.String(); got != want {
		t.Errorf("the summary renders as %q, want %q", got, want)
	}
}

func TestSummaryStringCountsOneOfSomethingInTheSingular(t *testing.T) {
	summary := projection.Summary{Nodes: 1, Edges: 1, Dependencies: 1, ExternalNodes: 1, ExternalEdges: 1}

	want := "1 node, 1 edge, 1 dependency, 1 external node, 1 external edge"
	if got := summary.String(); got != want {
		t.Errorf("the summary renders as %q, want %q", got, want)
	}
}

func TestSummaryOfAnEmptyReportIsZeros(t *testing.T) {
	// The zero Snapshot is the empty graph, and its summary has to agree with that.
	if got := (projection.Snapshot{}).Summary(); got != (projection.Summary{}) {
		t.Errorf("the zero snapshot's summary is %+v, want zeros", got)
	}
	if got := projection.NewSnapshot("", nil, nil).Summary().String(); got != "0 nodes, 0 edges, 0 dependencies" {
		t.Errorf("the empty snapshot's summary renders as %q, want zeros", got)
	}
}

func TestSummaryCountsRawDependenciesSeparatelyFromDrawnEdges(t *testing.T) {
	// The pair `1 edge, 3 dependencies` is the coupling a collapsed diagram alone cannot show, so the two
	// numbers are both in the headline and the raw one never shrinks when the arrows merge.
	edges := []projection.Edge{
		projection.NewEdge("internal/api", "internal/db",
			plainDependency("internal/api/handler.go", "internal/db/conn.go"),
			plainDependency("internal/api/router.go", "internal/db/conn.go"),
			plainDependency("internal/api/router.go", "internal/db/query.go"),
		),
	}
	nodes := []projection.Node{projection.NewNode("internal/api"), projection.NewNode("internal/db")}

	summary := projection.NewSnapshot("", nodes, edges).Summary()

	if summary.Edges != 1 || summary.Dependencies != 3 {
		t.Errorf("the summary is %+v, want 1 edge standing for 3 dependencies", summary)
	}
}
