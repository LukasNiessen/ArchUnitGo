package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// CycleViolation is one of the violations every consumer of a rule programs against, so the interface is
// asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.CycleViolation{}

func TestCycleViolationIsOfTheFileCycleKind(t *testing.T) {
	// The kind names the vocabulary as well as the failure, because every vocabulary the library grows
	// reports its own cycles and the testing layer picks a phrasing by this key.
	violation := assertion.NewCycleViolation(cycles.Circuit{})

	if violation.Kind() != assertion.KindFileCycle {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindFileCycle)
	}
	if assertion.KindFileCycle != "file-cycle" {
		t.Errorf("KindFileCycle = %q, want the name every ArchUnit port spells it with", assertion.KindFileCycle)
	}
}

func TestCycleViolationNamesTheFilesInCycleOrder(t *testing.T) {
	circuits := fileCircuits(t,
		fileEdge("internal/api/handler.go", "internal/db/conn.go"),
		fileEdge("internal/db/conn.go", "internal/api/handler.go"),
	)
	if len(circuits) != 1 {
		t.Fatalf("the fixture holds %d cycles, want the one mutual dependency", len(circuits))
	}

	violation := assertion.NewCycleViolation(circuits[0])

	// The file it closes onto is the first one and is not repeated at the end: these are the offenders,
	// not the path.
	want := []string{"internal/api/handler.go", "internal/db/conn.go"}
	if files := violation.Files(); !slices.Equal(files, want) {
		t.Errorf("Files() = %v, want %v", files, want)
	}
}

func TestCycleViolationPrintsTheCycleAsAReadablePath(t *testing.T) {
	// What the issue asks of the message: the cycle, closed, as a path a reader can follow.
	circuits := fileCircuits(t,
		fileEdge("internal/api/handler.go", "internal/db/conn.go"),
		fileEdge("internal/db/conn.go", "internal/db/query.go"),
		fileEdge("internal/db/query.go", "internal/api/handler.go"),
	)
	if len(circuits) != 1 {
		t.Fatalf("the fixture holds %d cycles, want the one three-file cycle", len(circuits))
	}

	violation := assertion.NewCycleViolation(circuits[0])

	want := "internal/api/handler.go -> internal/db/conn.go -> internal/db/query.go -> internal/api/handler.go"
	if rendered := violation.String(); rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
}

func TestCycleViolationKeepsTheDependenciesTheCycleRunsAlong(t *testing.T) {
	// The violation carries the cycle rather than a sentence about it, which is what lets a report name
	// the import behind each step instead of only the files.
	circuits := fileCircuits(t,
		fileEdge("internal/api/handler.go", "internal/db/conn.go"),
		fileEdge("internal/db/conn.go", "internal/api/handler.go"),
	)

	violation := assertion.NewCycleViolation(circuits[0])

	edges := violation.Cycle.Edges()
	if len(edges) != 2 {
		t.Fatalf("the cycle runs along %d dependencies, want the two of a mutual dependency", len(edges))
	}
	for _, edge := range edges {
		raw := edge.CumulatedEdges()
		if len(raw) != 1 {
			t.Fatalf("%s cumulates %v, want the one raw edge behind it", edge, raw)
		}
		if !raw[0].ImportKinds.Contains(extraction.ImportKindPlain) {
			t.Errorf("%s cumulates %v, want the import kind the extractor read", edge, raw)
		}
	}
}

func TestCycleViolationSharesNothingWithTheCallerThatReadsIt(t *testing.T) {
	// A violation that has been reported must not change afterwards, and Cycle.Edges is the accessor with
	// the hazard: it hands out the chain the circuit reads its labels off, so a reader that writes into it
	// would rewrite the violation. Files builds its slice fresh either way.
	circuits := fileCircuits(t,
		fileEdge("internal/api/handler.go", "internal/db/conn.go"),
		fileEdge("internal/db/conn.go", "internal/api/handler.go"),
	)
	violation := assertion.NewCycleViolation(circuits[0])

	edges := violation.Cycle.Edges()
	edges[0] = fileEdge("somewhere/else.go", "elsewhere.go")
	violation.Files()[0] = "somewhere/else.go"

	wantPath := "internal/api/handler.go -> internal/db/conn.go -> internal/api/handler.go"
	if rendered := violation.String(); rendered != wantPath {
		t.Errorf("String() = %q after a caller rewrote the edges it was handed, want %q", rendered, wantPath)
	}
	wantFiles := []string{"internal/api/handler.go", "internal/db/conn.go"}
	if files := violation.Files(); !slices.Equal(files, wantFiles) {
		t.Errorf("Files() = %v after a caller rewrote what it was handed, want %v", files, wantFiles)
	}
}

func TestCycleViolationOfNoCycleSaysNothing(t *testing.T) {
	// The zero Circuit is not a cycle, and GatherCycleViolations never makes a violation of one. It is
	// readable anyway rather than panicking, because a violation is data a report walks over.
	violation := assertion.NewCycleViolation(cycles.Circuit{})

	if files := violation.Files(); len(files) != 0 {
		t.Errorf("Files() = %v, want no file", files)
	}
	if rendered := violation.String(); rendered != "" {
		t.Errorf("String() = %q, want the empty string", rendered)
	}
}

// fileEdge is one dependency between two files as the files module projects it: the identifiers as
// labels, and the raw edge behind them.
func fileEdge(source, target string) projection.ProjectedEdge {
	return projection.NewProjectedEdge(source, target, extraction.NewEdge(source, target, false, extraction.ImportKindPlain))
}

// fileCircuits are the cycles between such dependencies, which is the input a violation is made from and
// the only way to build a Circuit — the type has no exported constructor, on purpose.
func fileCircuits(t *testing.T, edges ...projection.ProjectedEdge) []cycles.Circuit {
	t.Helper()

	circuits, complete := cycles.ProjectCircuits(edges, nil)
	if !complete {
		t.Fatalf("the enumeration of %d cycles is truncated, want a fixture small enough to enumerate whole", len(circuits))
	}
	return circuits
}
