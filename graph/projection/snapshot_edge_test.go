package projection_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestNewEdgeDerivesEverythingButTheLabelsFromTheDependenciesBehindIt(t *testing.T) {
	// An edge cannot claim a count that does not match what it was built from, which is why the count, the
	// external flag and the import kinds are not parameters.
	edge := projection.NewEdge("internal/api", "internal/db",
		plainDependency("internal/api/handler.go", "internal/db/conn.go"),
		extraction.NewEdge("internal/api/router.go", "internal/db/query.go", false, extraction.ImportKindBlank),
	)

	if edge.SourceLabel() != "internal/api" || edge.TargetLabel() != "internal/db" {
		t.Errorf("the edge runs from %q to %q, want internal/api to internal/db", edge.SourceLabel(), edge.TargetLabel())
	}
	if edge.Count() != 2 {
		t.Errorf("the edge stands for %d dependencies, want the 2 it was built from", edge.Count())
	}
	if got := edge.ImportKinds().String(); got != "[plain, blank]" {
		t.Errorf("the edge's import kinds are %s, want the union [plain, blank]", got)
	}
	if edge.IsExternal() {
		t.Error("the edge is external, want it inside the project: neither dependency leaves it")
	}
}

func TestNewEdgeIsExternalWhenAnyDependencyBehindItLeavesTheProject(t *testing.T) {
	// The flag says whether following this arrow takes the reader out of the codebase, and once one of the
	// merged dependencies does, it does. An arrow cannot be partly external.
	edge := projection.NewEdge("internal/db", "third party",
		plainDependency("internal/db/conn.go", "internal/db/query.go"),
		externalDependency("internal/db/conn.go", "database/sql"),
	)

	if !edge.IsExternal() {
		t.Error("the edge is inside the project, want it external: one of the dependencies behind it leaves")
	}
}

func TestNewEdgeCountsOneDependencyGivenTwiceOnce(t *testing.T) {
	// The dependencies go through extraction.NewGraph first, so the invariant that `(source, target)` is unique
	// holds one level up too — and a count is a number a reader is entitled to trust.
	duplicated := plainDependency("internal/api/handler.go", "internal/db/conn.go")

	edge := projection.NewEdge("internal/api", "internal/db", duplicated, duplicated)

	if edge.Count() != 1 {
		t.Errorf("the edge stands for %d dependencies, want the 1 distinct one it was given twice", edge.Count())
	}
}

func TestNewEdgeWithNoDependencyBehindItStandsForNone(t *testing.T) {
	// What a hand-built snapshot in a test that is only about labels wants, and it says so rather than
	// inventing a count.
	edge := projection.NewEdge("internal/api", "internal/db")

	if edge.Count() != 0 {
		t.Errorf("the edge stands for %d dependencies, want none", edge.Count())
	}
	if !edge.ImportKinds().Empty() {
		t.Errorf("the edge's import kinds are %s, want none", edge.ImportKinds())
	}
}

func TestEdgeIsSelfDependencyWhenBothLabelsAreTheSame(t *testing.T) {
	// Never a raw dependency — a file does not depend on itself — but a real one after a collapse, where it
	// says the files inside a folder depend on each other.
	inside := projection.NewEdge("internal/api", "internal/api", plainDependency("internal/api/handler.go", "internal/api/router.go"))
	between := projection.NewEdge("internal/api", "internal/db")

	if !inside.IsSelfDependency() {
		t.Errorf("%v is not a self dependency, want it one", inside)
	}
	if between.IsSelfDependency() {
		t.Errorf("%v is a self dependency, want it a dependency between two nodes", between)
	}
}

func TestEdgeStringRendersTheCountAndTheKinds(t *testing.T) {
	// The count is on the arrow because a collapsed diagram without it invites the reader to think two folders
	// are barely coupled.
	one := projection.NewEdge("internal/api", "internal/db", plainDependency("internal/api/handler.go", "internal/db/conn.go"))
	external := projection.NewEdge("internal/db", "third party",
		externalDependency("internal/db/conn.go", "database/sql"),
		externalDependency("internal/db/query.go", "database/sql"),
	)

	if got, want := one.String(), "internal/api -> internal/db [1 dependency] [plain]"; got != want {
		t.Errorf("the edge renders as %q, want %q", got, want)
	}
	if got, want := external.String(), "internal/db -> third party [2 dependencies] (external) [plain]"; got != want {
		t.Errorf("the external edge renders as %q, want %q", got, want)
	}
	if got, want := projection.NewEdge("a", "b").String(), "a -> b [0 dependencies]"; got != want {
		t.Errorf("the edge standing for nothing renders as %q, want %q", got, want)
	}
}
