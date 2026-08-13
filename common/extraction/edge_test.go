package extraction

import "testing"

func TestNewEdgeNormalizesBothIdentifiers(t *testing.T) {
	edge := NewEdge(`internal\api\handler.go`, "./internal/db/repo.go", false, ImportKindPlain)

	if edge.Source != "internal/api/handler.go" {
		t.Errorf("Source = %q, want %q", edge.Source, "internal/api/handler.go")
	}
	if edge.Target != "internal/db/repo.go" {
		t.Errorf("Target = %q, want %q", edge.Target, "internal/db/repo.go")
	}
	if edge.External {
		t.Error("External should be false")
	}
	if !edge.ImportKinds.Contains(ImportKindPlain) || edge.ImportKinds.Len() != 1 {
		t.Errorf("ImportKinds = %v, want just plain", edge.ImportKinds)
	}
}

func TestNewEdgeToExternalTarget(t *testing.T) {
	edge := NewEdge("main.go", "github.com/spf13/cobra", true, ImportKindAliased, ImportKindBlank)

	if !edge.External {
		t.Error("External should be true")
	}
	if edge.Target != "github.com/spf13/cobra" {
		t.Errorf("Target = %q, want the import path unchanged", edge.Target)
	}
	if edge.ImportKinds.Len() != 2 {
		t.Errorf("ImportKinds = %v, want aliased and blank", edge.ImportKinds)
	}
}

func TestNewEdgeWithoutImportKinds(t *testing.T) {
	edge := NewEdge("a.go", "b.go", false)

	if !edge.ImportKinds.Empty() {
		t.Errorf("ImportKinds = %v, want empty", edge.ImportKinds)
	}
}

func TestSelfEdge(t *testing.T) {
	edge := SelfEdge(`internal\api\handler.go`)

	if edge.Source != "internal/api/handler.go" || edge.Target != edge.Source {
		t.Errorf("self-edge = %v, want a normalised edge from a node to itself", edge)
	}
	if !edge.IsSelfEdge() {
		t.Error("IsSelfEdge should be true")
	}
	if edge.External {
		t.Error("a self-edge is never external")
	}
	if !edge.ImportKinds.Empty() {
		t.Errorf("ImportKinds = %v, want empty: no import produced a self-edge", edge.ImportKinds)
	}
}

func TestIsSelfEdge(t *testing.T) {
	if NewEdge("a.go", "b.go", false, ImportKindPlain).IsSelfEdge() {
		t.Error("an edge between two nodes is not a self-edge")
	}
	// The two identifiers spell the same node, so this is a self-edge once normalised.
	if !NewEdge("./a.go", `a.go`, false).IsSelfEdge() {
		t.Error("normalisation should make differently spelled identifiers the same node")
	}
}

func TestEdgeIsComparable(t *testing.T) {
	left := NewEdge("a.go", "b.go", false, ImportKindPlain)
	right := NewEdge(`.\a.go`, "b.go", false, ImportKindPlain)
	other := NewEdge("a.go", "b.go", false, ImportKindDot)

	if left != right {
		t.Errorf("%v and %v should be equal", left, right)
	}
	if left == other {
		t.Errorf("%v and %v differ in import kinds and should not be equal", left, other)
	}
	// Comparability is what lets an edge be a map key, which the graph relies on.
	counts := map[Edge]int{left: 1}
	counts[right]++
	if counts[left] != 2 {
		t.Errorf("edges as map keys = %v, want the equal edges to collide", counts)
	}
}

func TestEdgeMergeUnionsImportKinds(t *testing.T) {
	plain := NewEdge("a.go", "b.go", false, ImportKindPlain)
	blank := NewEdge("a.go", "b.go", false, ImportKindBlank)

	merged := plain.merge(blank)

	if merged.Source != "a.go" || merged.Target != "b.go" {
		t.Errorf("merged endpoints = %q -> %q, want a.go -> b.go", merged.Source, merged.Target)
	}
	if !merged.ImportKinds.Contains(ImportKindPlain) || !merged.ImportKinds.Contains(ImportKindBlank) {
		t.Errorf("ImportKinds = %v, want plain and blank", merged.ImportKinds)
	}
	if plain.ImportKinds.Contains(ImportKindBlank) {
		t.Error("merge mutated the receiver")
	}
}

func TestEdgeMergeUnionsExternality(t *testing.T) {
	internal := NewEdge("a.go", "b", false, ImportKindPlain)
	external := NewEdge("a.go", "b", true, ImportKindPlain)

	if !internal.merge(external).External {
		t.Error("merging an external edge in should keep the target external")
	}
	if !external.merge(internal).External {
		t.Error("merging an internal edge in should not un-externalize the target")
	}
}

func TestEdgeString(t *testing.T) {
	tests := []struct {
		name string
		edge Edge
		want string
	}{
		{
			name: "internal edge",
			edge: NewEdge("a.go", "b.go", false, ImportKindPlain, ImportKindDot),
			want: "a.go -> b.go [plain, dot]",
		},
		{
			name: "external edge",
			edge: NewEdge("a.go", "fmt", true, ImportKindPlain),
			want: "a.go -> fmt (external) [plain]",
		},
		{
			name: "self edge",
			edge: SelfEdge("a.go"),
			want: "a.go -> itself",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.edge.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}
