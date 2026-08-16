package cycles

import (
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// DefaultMaxCircuits is how many elementary circuits ProjectCircuits enumerates when its options do not
// say otherwise.
//
// It is a safety limit rather than a preference. A report nobody can read is as good as no report, and a
// projection dense enough to hold a thousand circuits has a structural problem the first dozen of them
// already describe — while enumerating all of them is, in the worst case, exponential work.
const DefaultMaxCircuits = 1000

// Circuit is one elementary cycle of a projection: the chain of projected edges that leaves a label,
// returns to it, and touches no label twice on the way.
//
// It is what a rule reports when the reader needs the cycle itself rather than the region of the graph it
// lives in. `api -> db -> api` names the two dependencies to break; the strongly connected component
// ProjectCycles returns for the same graph names every edge around it, which for a large
// component is a haystack. The two are one fact at two resolutions, and they are both here because only
// the component is cheap enough to always be safe to ask for — see ProjectCircuits.
//
// A Circuit is immutable, and one from ProjectCircuits holds at least two edges: a projection has no
// self-edges by the time it is read, so the shortest circuit is a mutual dependency. The edges are in
// circuit order, and the last one closes back onto the first one's source label. The zero value is the
// empty circuit and is what a label that is in no cycle would have had.
type Circuit struct {
	edges []projection.ProjectedEdge
}

// Edges are the projected edges the circuit runs along, in circuit order. They are what a violation
// names concrete files with, because each one still carries the raw edges it was projected from.
//
// The result is the caller's own copy, because a Circuit that has been reported must not change
// afterwards.
func (c Circuit) Edges() []projection.ProjectedEdge {
	return slices.Clone(c.edges)
}

// Labels are the labels the circuit visits, in circuit order, each exactly once. The label it closes
// onto is the first one and is not repeated at the end.
func (c Circuit) Labels() []string {
	return slices.Clone(c.labels())
}

// Length is how many labels — equivalently, how many dependencies — the circuit is made of. Two is a
// mutual dependency, and a shorter circuit is a smaller thing to fix, which is why ProjectCircuits
// reports the short ones first.
func (c Circuit) Length() int {
	return len(c.edges)
}

// String renders the circuit for logs and test failures, as `api -> db -> api`, closing label included.
// User-facing violation messages are built in the testing layer, not here.
func (c Circuit) String() string {
	labels := c.labels()
	if len(labels) == 0 {
		return ""
	}
	closed := make([]string, 0, len(labels)+1)
	closed = append(closed, labels...)
	closed = append(closed, labels[0])
	return strings.Join(closed, " -> ")
}

// labels is Labels without the copy, for the readers inside this package that only look.
func (c Circuit) labels() []string {
	labels := make([]string, 0, len(c.edges))
	for _, edge := range c.edges {
		labels = append(labels, edge.SourceLabel())
	}
	return labels
}

// CircuitOptions is the options bag of ProjectCircuits. A nil bag, the zero bag and one whose fields are
// all zero are the same enumeration.
type CircuitOptions struct {
	// MaxCircuits is how many circuits may be reported. Zero means DefaultMaxCircuits, and a negative
	// value means no bound at all — which is exponential in the size of a dense component, so ask for
	// it only about a projection you already know is small.
	MaxCircuits int
}

// WithDefaults resolves the bag into one with every default filled in, so that a caller reads the
// effective values rather than the zero ones. A nil receiver resolves to the defaults, which is the
// nil-means-defaults contract every options bag in this library keeps.
func (o *CircuitOptions) WithDefaults() *CircuitOptions {
	resolved := CircuitOptions{}
	if o != nil {
		resolved = *o
	}
	if resolved.MaxCircuits == 0 {
		resolved.MaxCircuits = DefaultMaxCircuits
	}
	return &resolved
}

// ProjectCircuits enumerates the elementary cycles of a projection: one Circuit per closed chain of
// projected edges that returns to where it started without touching a label twice.
//
// It is the detailed half of cycle detection, and ProjectCycles is the summary. This one answers "which
// cycles are there" — the sentence a report needs, `api -> db -> api` — where ProjectCycles answers
// "which parts of the graph are cyclic". Every circuit here lies inside exactly one of the components
// there, which is how the work is divided: the strongly connected components come from Tarjan, and the
// circuits inside each one from Johnson.
//
// The order is by length and then by labels, so the mutual dependencies come first and a report is
// reproducible. Self-edges are ignored, as everywhere else in the PROJECT stage — a node depending on
// itself is not a dependency, so it is not a cycle either — and ProjectEdges has already dropped them, so
// passing a projection that kept them changes nothing. No cycles is an empty result, and that is the pass
// a rule reports.
//
// The second result reports whether the answer is the whole set. Counting elementary circuits is
// exponential in the worst case, so the enumeration stops at options.MaxCircuits and says so rather than
// running for a length of time no test suite can wait out. A false there means "these, and more" —
// enough for a rule, which only needs to know that there is one, and the caller's cue to say so if it is
// building a report. Set MaxCircuits negative to insist on all of them.
func ProjectCircuits(edges []projection.ProjectedEdge, options *CircuitOptions) ([]Circuit, bool) {
	labels, successors := adjacency(edges)
	byPair := edgesByPair(edges)

	remaining := options.WithDefaults().MaxCircuits
	unlimited := remaining < 0

	circuits := make([]Circuit, 0, len(labels))
	complete := true
	for _, component := range cyclicComponents(labels, successors) {
		// The limit is the size of the report, so what is left of it is what the next component may
		// spend. An unbounded run leaves it negative, which is what says "no limit" all the way down.
		found, whole := johnsonCircuits(component, successors, remaining)
		for _, circuit := range found {
			circuits = append(circuits, newCircuit(circuit, byPair))
		}
		if !unlimited {
			remaining -= len(found)
		}
		if !whole {
			complete = false
			break
		}
	}

	// The enumeration order is a function of the projection already, but it is Johnson's order — by
	// starting label, and inside that by the order the search happened to close paths. A report is
	// ordered by what a reader wants first instead: the smallest cycle to fix.
	slices.SortFunc(circuits, compareCircuits)
	return circuits, complete
}

// newCircuit turns one circuit's labels into the projected edges along it, closing the last label back
// onto the first. Every pair it looks up is in the map, because the adjacency the labels came out of was
// built from those same edges.
func newCircuit(labels []string, byPair map[edgePair]projection.ProjectedEdge) Circuit {
	edges := make([]projection.ProjectedEdge, 0, len(labels))
	for index, source := range labels {
		target := labels[(index+1)%len(labels)]
		edges = append(edges, byPair[edgePair{source: source, target: target}])
	}
	return Circuit{edges: edges}
}

// edgePair is the identity of a projected edge while a circuit is being read back out of labels:
// (SourceLabel, TargetLabel) is unique in a projection, so it names one edge.
type edgePair struct {
	source string
	target string
}

// edgesByPair indexes a projection by the pair of labels each edge joins. Self-edges are left out, for
// the same reason the adjacency leaves them out: they are in no circuit.
func edgesByPair(edges []projection.ProjectedEdge) map[edgePair]projection.ProjectedEdge {
	byPair := make(map[edgePair]projection.ProjectedEdge, len(edges))
	for _, edge := range edges {
		if edge.IsSelfEdge() {
			continue
		}
		byPair[edgePair{source: edge.SourceLabel(), target: edge.TargetLabel()}] = edge
	}
	return byPair
}

// compareCircuits orders the shortest circuit first and breaks the tie on the labels. Each circuit is
// rooted at its own least label, so the labels order two circuits of one length totally.
func compareCircuits(left, right Circuit) int {
	if byLength := left.Length() - right.Length(); byLength != 0 {
		return byLength
	}
	return slices.Compare(left.labels(), right.labels())
}
