package projection_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestNewNodeIsTheProjectsOwnCodeUnderThisLabel(t *testing.T) {
	node := projection.NewNode("internal/api")

	if node.Label() != "internal/api" {
		t.Errorf("the node is labeled %q, want internal/api", node.Label())
	}
	if node.IsExternal() {
		t.Error("the node is external, want it the project's own code")
	}
}

func TestNewExternalNodeIsSomebodyElsesCodeUnderThisLabel(t *testing.T) {
	// Two constructors rather than a boolean argument: `NewNode("net/http", true)` at a call site says nothing
	// about what the flag means, and a report where internal and external have been swapped is one that lies.
	node := projection.NewExternalNode("net/http")

	if node.Label() != "net/http" {
		t.Errorf("the node is labeled %q, want net/http", node.Label())
	}
	if !node.IsExternal() {
		t.Error("the node is the project's own code, want it external")
	}
}

func TestNodeStringSaysWhenANodeIsOutsideTheProject(t *testing.T) {
	// What a reader of a failing test needs: the difference between a folder they can go and edit and a module
	// they cannot.
	if got := projection.NewNode("internal/api").String(); got != "internal/api" {
		t.Errorf("the node renders as %q, want internal/api", got)
	}
	if got := projection.NewExternalNode("net/http").String(); got != "net/http (external)" {
		t.Errorf("the external node renders as %q, want net/http (external)", got)
	}
}
