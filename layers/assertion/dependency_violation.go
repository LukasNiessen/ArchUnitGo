package assertion

import (
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// KindLayerDependency is the kind of DependencyViolation: a layer that depends on a layer the policy does
// not allow it to.
//
// It names the vocabulary as well as the failure, the way KindFileDependency does in the files module,
// because every vocabulary the library grows has a rule about what may depend on what and each of them
// reports its own type over its own labels. The kind is what the testing layer picks a phrasing by, so two
// families sharing one name would be two shapes of data under one key.
const KindLayerDependency kernel.ViolationKind = "layer-dependency"

// DependencyViolation says that one of a policy's layers depends on another layer, and that the clause
// judging it does not allow that: the dependency is one the blocklist forbade, or one the allowlist did
// not name.
//
// It is what `project layers, ..., where layer "api", may not depend on layers "db"` reports, one violation
// per pair of layers rather than one per offending import, and it carries the two layers, the clause it
// broke and the concrete file dependencies that produced it — not a sentence about them.
//
// One violation per pair of layers is the shape a layer policy fails in. The offense is that these two
// layers are connected at all, so the reader's question is which files connect them, and every one of them
// is here: repeating the pair once per import would bury the finding under the size of the layers. Both
// clauses report the same shape, because both are broken by a dependency that exists — the allowlist by
// one it did not name and the blocklist by one it did — which is why, unlike a rule about files, neither
// mood of this rule ever reports the absence of a dependency. `may only depend on layers "domain"` does not
// require anything to depend on the domain.
type DependencyViolation struct {
	// Layer is the name of the depending layer, as the policy declared it — `api`. It is always the layer
	// the broken clause was about.
	Layer string
	// DependsOn is the name of the layer it depends on, which is the layer the clause does not allow it to
	// reach. It is never Layer itself: a dependency inside one layer is always allowed, and the projection
	// does not even carry it.
	DependsOn string
	// Named are the layers the broken clause named, in the order the user typed them: the ones allowed
	// under `may only depend on layers` and the ones forbidden under `may not depend on layers`. It is
	// empty for a sealed layer, which allowed nothing.
	Named []string
	// Mood is which way round the broken clause was written — Should for `may only depend on layers`,
	// ShouldNot for `may not depend on layers`. Without it a report could not tell the dependency an
	// allowlist failed to name from one a blocklist forbade, which are the same pair of layers under two
	// different rules.
	Mood kernel.Mood
	// Dependencies are the concrete dependencies this pair of layers stands for: every extracted edge from
	// a file of Layer to a file of DependsOn, in the graph's own order.
	//
	// They are what a reader has to go and unpick, and they are the reason a projected edge cumulates the
	// raw edges it was built from — after relabelling, the files are nowhere else. A violation built by
	// hand may carry none, and a report then names the layers alone.
	Dependencies extraction.Graph
}

// NewDependencyViolation records that the layer this clause is about depends on dependsOn, which the clause
// does not allow, through these extracted edges.
//
// It is the only way a violation of this family is made, and it takes the whole clause rather than its
// parts so that what a report says the rule was cannot drift from what was judged. The clause's own layer
// is the depending one, because a clause is only ever asked about the dependencies of the layer it is
// about.
//
// Both the named layers and the edges are copied, for the reason assertion.NewEmptyTestViolation copies its
// selectors: a violation that has been reported must not change when the projection it was found in is
// walked on. The edges go through extraction.NewGraph, which gives them the library's one edge order, so
// that a violation built from a hand-written list reads exactly like one built from a projection.
func NewDependencyViolation(clause Clause, dependsOn string, dependencies ...extraction.Edge) DependencyViolation {
	return DependencyViolation{
		Layer:        clause.Layer(),
		DependsOn:    dependsOn,
		Named:        clause.Named(),
		Mood:         clause.Mood(),
		Dependencies: extraction.NewGraph(dependencies...),
	}
}

// Kind is KindLayerDependency.
func (DependencyViolation) Kind() kernel.ViolationKind {
	return KindLayerDependency
}

// String renders the violation as the offending layer, the clause it broke in the words the clause was
// written in, and the dependency it was broken by — `api: may not depend on layers "db" -> db
// (internal/api/handler.go -> internal/db/conn.go)` — for a log line or a test failure.
//
// The clause is rendered as the policy stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. The user-facing message is
// still the testing layer's to build, from Layer, DependsOn, Named, Mood and Dependencies.
func (v DependencyViolation) String() string {
	// The clause is rebuilt out of what the violation carries, so that the words a clause is written in
	// live in exactly one place: this is the clause NewDependencyViolation was given, read back.
	clause := NewClause(v.Layer, v.Named, v.Mood)
	return v.Layer + ": " + clause.verb() + " " + clause.namedLayers() + " -> " + v.DependsOn + v.found()
}

// found renders the concrete dependencies the clause was broken by, in parentheses, and the empty string
// when a hand-built violation carries none. It is the one place that list is spelled, because a report of
// either mood reads it.
func (v DependencyViolation) found() string {
	if len(v.Dependencies) == 0 {
		return ""
	}
	rendered := make([]string, 0, len(v.Dependencies))
	for _, edge := range v.Dependencies {
		rendered = append(rendered, edge.Source+" -> "+edge.Target)
	}
	return " (" + strings.Join(rendered, ", ") + ")"
}
