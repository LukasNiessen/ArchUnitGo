// Package assertion is the slices module's half of the ASSERT stage: it judges the projected
// dependencies between a project's slices and reports one violation per dependency a rule does not allow
// — or per dependency it required and did not find.
//
// One function judges, GatherDependencyViolations, and one type is reported, DependencyViolation. A
// violation is data: it says which slice depended on which, in which mood the rule was written, and it
// carries the concrete file dependencies it was broken by, but not a word about them. Message
// construction belongs to the testing layer, where one place controls phrasing, numbering and color.
//
// The mood is what makes `should contain dependency` and `should not contain dependency` one piece of
// logic. The predicate is the same question either way — does this projection have a dependency from this
// slice to that one — and assertion.Mood.Holds is the single comparison that inverts it, so the forbidden
// dependency and the required one are one code path rather than two.
//
// Slices differ from layers in what a rule is written over, and it shows here: a layer policy is a list of
// clauses judged against a projection whose labels were declared, while a slicing rule is one question
// about two slice names that were cut out of the identifiers by the pattern. There is no Clause type,
// because there is no list — `contain dependency(from, to)` is the whole predicate.
//
// The package is pure, like every assertion package in the library: no filesystem, no clock, no globals,
// and nothing in it knows Go. It takes the projected structure common/projection produced and hands back
// a []assertion.Violation, so what a rule judges can be tested against a hand-built projection before any
// project is extracted at all.
package assertion

import (
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// KindSliceDependency is the kind of DependencyViolation: a dependency between two slices that the rule
// forbade, or one it required and the project does not have.
//
// It names the vocabulary as well as the failure, the way KindFileDependency and KindLayerDependency do in
// their modules, because every vocabulary the library grows has a rule about what may depend on what and
// each of them reports its own type over its own labels. The kind is what the testing layer picks a
// phrasing by, so two families sharing one name would be two shapes of data under one key.
const KindSliceDependency kernel.ViolationKind = "slice-dependency"

// DependencyViolation says that one slice of a project depends on another and the rule does not allow it,
// or that the rule required that dependency and the project does not have it.
//
// It is what `project slices, defined by "internal/(**)/**", should not, contain dependency "api", "db"`
// reports, and there is at most one of them per rule: the rule asks one question about one pair of slices,
// so a second violation could only repeat it. Which files connect the two slices is the reader's next
// question, and every one of them is carried here rather than reported separately — one violation per
// offending import would bury the finding under the size of the slices.
//
// The absent direction is real for this family, unlike for layers: `should contain dependency` is broken
// by a dependency that is not there, and such a violation carries no Dependencies at all. That is not the
// empty-test guard's case — the slices were found and the projection was judged; what is missing is one
// edge between them.
type DependencyViolation struct {
	// Slice is the name of the depending slice, as the rule named it — `api`. It is the subject of the
	// rule, so it is always the `from` of the dependency the rule was about.
	Slice string
	// DependsOn is the name of the slice it depends on, or was required to depend on — `db`. It is never
	// Slice itself: a slice may always depend on itself, and the projection does not even carry that
	// dependency.
	DependsOn string
	// Mood is which way round the rule was written — Should for `should contain dependency`, ShouldNot for
	// `should not contain dependency`. Without it a report could not tell a forbidden dependency that
	// exists from a required one that does not, which are the same pair of slices under two rules that
	// fail in opposite ways.
	Mood kernel.Mood
	// Dependencies are the concrete dependencies this pair of slices stands for: every extracted edge from
	// a file of Slice to a file of DependsOn, in the graph's own order.
	//
	// They are what a reader has to go and unpick, and they are the reason a projected edge cumulates the
	// raw edges it was built from — after relabelling, the files are nowhere else. A violation of the
	// positive mood carries none, because the dependency it is about was not found; so does one built by
	// hand, and a report then names the slices alone.
	Dependencies extraction.Graph
}

// NewDependencyViolation records that the dependency from slice to dependsOn breaks a rule written in this
// mood, through these extracted edges.
//
// The edges are copied, for the reason assertion.NewEmptyTestViolation copies its selectors: a violation
// that has been reported must not change when the projection it was found in is walked on. They go through
// extraction.NewGraph, which gives them the library's one edge order, so that a violation built from a
// hand-written list reads exactly like one built from a projection. A violation of the positive mood is
// built with no edges, because the dependency it reports is the one that is missing.
func NewDependencyViolation(slice, dependsOn string, mood kernel.Mood, dependencies ...extraction.Edge) DependencyViolation {
	return DependencyViolation{
		Slice:        slice,
		DependsOn:    dependsOn,
		Mood:         mood,
		Dependencies: extraction.NewGraph(dependencies...),
	}
}

// Kind is KindSliceDependency.
func (DependencyViolation) Kind() kernel.ViolationKind {
	return KindSliceDependency
}

// String renders the violation as the pair of slices, the rule in the words it was written in, and the
// dependency it was broken by — `api -> db: should not contain dependency (internal/api/handler.go ->
// internal/db/conn.go)` — for a log line or a test failure.
//
// The rule is rendered as the user stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. The user-facing message is
// still the testing layer's to build, from Slice, DependsOn, Mood and Dependencies.
func (v DependencyViolation) String() string {
	return v.Slice + " -> " + v.DependsOn + ": " + v.Mood.String() + " contain dependency" + v.found()
}

// found renders the concrete dependencies the rule was broken by, in parentheses, and the empty string
// when there are none — a required dependency that is missing, or a violation built by hand.
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
