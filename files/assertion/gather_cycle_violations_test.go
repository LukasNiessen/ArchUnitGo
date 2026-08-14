package assertion_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

func TestGatherCycleViolationsReportsOneViolationPerCycle(t *testing.T) {
	// Three files in one cyclic region, two elementary cycles inside it: the report is per cycle, because
	// a cycle is what a reader has to break.
	circuits := fileCircuits(t,
		fileEdge("internal/api/handler.go", "internal/db/conn.go"),
		fileEdge("internal/db/conn.go", "internal/api/handler.go"),
		fileEdge("internal/db/conn.go", "internal/db/query.go"),
		fileEdge("internal/db/query.go", "internal/api/handler.go"),
	)

	violations := assertion.GatherCycleViolations(circuits)

	if len(violations) != len(circuits) {
		t.Fatalf("GatherCycleViolations reported %d violations for %d cycles, want one each", len(violations), len(circuits))
	}
	// In the order the enumeration reported them: shortest cycle first, which is the smallest thing to fix.
	want := []string{
		"internal/api/handler.go -> internal/db/conn.go -> internal/api/handler.go",
		"internal/api/handler.go -> internal/db/conn.go -> internal/db/query.go -> internal/api/handler.go",
	}
	rendered := make([]string, 0, len(violations))
	for _, violation := range violations {
		if violation.Kind() != assertion.KindFileCycle {
			t.Errorf("a violation is of kind %q, want %q", violation.Kind(), assertion.KindFileCycle)
		}
		cycle, ok := violation.(assertion.CycleViolation)
		if !ok {
			t.Fatalf("GatherCycleViolations reported a %T, want a CycleViolation", violation)
		}
		rendered = append(rendered, cycle.String())
	}
	if !slices.Equal(rendered, want) {
		t.Errorf("GatherCycleViolations reported %v, want %v", rendered, want)
	}
}

func TestGatherCycleViolationsOfNoCycleIsThePass(t *testing.T) {
	// No cycles is no violations. There is no boolean beside the list: an empty result is what every
	// terminal in the library returns for a rule that holds.
	for _, circuits := range [][]cycles.Circuit{nil, {}} {
		violations := assertion.GatherCycleViolations(circuits)

		if len(violations) != 0 {
			t.Errorf("GatherCycleViolations(%v) reported %v, want the pass", circuits, violations)
		}
	}
}

func TestGatherCycleViolationsCarriesEachCycleUnchanged(t *testing.T) {
	// The judgement adds nothing to the cycle and drops nothing from it: the violation is the circuit the
	// projection found, so a report can still walk its edges.
	circuits := fileCircuits(t,
		fileEdge("internal/api/handler.go", "internal/db/conn.go"),
		fileEdge("internal/db/conn.go", "internal/api/handler.go"),
	)

	violations := assertion.GatherCycleViolations(circuits)

	violation, ok := violations[0].(assertion.CycleViolation)
	if !ok {
		t.Fatalf("GatherCycleViolations reported a %T, want a CycleViolation", violations[0])
	}
	if !slices.Equal(violation.Files(), circuits[0].Labels()) {
		t.Errorf("the violation names %v, want the cycle's own files %v", violation.Files(), circuits[0].Labels())
	}
	if len(violation.Cycle.Edges()) != circuits[0].Length() {
		t.Errorf("the violation carries %d dependencies, want the cycle's %d", len(violation.Cycle.Edges()), circuits[0].Length())
	}
}
