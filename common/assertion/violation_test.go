package assertion_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// These tests are an external test package on purpose. The property under test is that a rule family
// living outside common can implement Violation at all, and an in-package test cannot tell an open
// interface from a sealed one: a type declared beside the interface can satisfy an unexported method
// too. Here it cannot, so adding one to Violation stops this file compiling — which is the point.

// kindCycle is what a domain module declares beside its own violation type.
const kindCycle assertion.ViolationKind = "cycle"

// cycleViolation stands in for a rule family's violation: the offending data, and nothing that reads
// like a message.
type cycleViolation struct {
	Cycle []string
}

func (cycleViolation) Kind() assertion.ViolationKind {
	return kindCycle
}

func TestViolationCanBeImplementedOutsideCommon(t *testing.T) {
	var violation assertion.Violation = cycleViolation{Cycle: []string{"a.go", "b.go", "a.go"}}

	if violation.Kind() != kindCycle {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), kindCycle)
	}
}

func TestViolationsOfDifferentFamiliesAreToldApartByKind(t *testing.T) {
	// One result list, two rule families: the testing layer picks a phrasing per violation from the
	// kind alone, without knowing what concrete types are in the list.
	violations := []assertion.Violation{
		assertion.NewEmptyTestViolation("files"),
		cycleViolation{Cycle: []string{"a.go", "b.go", "a.go"}},
	}

	counted := map[assertion.ViolationKind]int{}
	for _, violation := range violations {
		counted[violation.Kind()]++
	}

	want := map[assertion.ViolationKind]int{assertion.KindEmptyTest: 1, kindCycle: 1}
	for kind, wanted := range want {
		if counted[kind] != wanted {
			t.Errorf("counted %d violations of kind %q, want %d", counted[kind], kind, wanted)
		}
	}
}

func TestEmptyTestKindSpelling(t *testing.T) {
	// The kind is what the testing layer keys its phrasing off, and it is spelled the same way in
	// every port. Changing it is a breaking change, so it is pinned here.
	if assertion.KindEmptyTest != "empty-test" {
		t.Errorf("KindEmptyTest = %q, want %q", assertion.KindEmptyTest, "empty-test")
	}
}
