package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// direction is which way a traversal follows a dependency: towards what a node depends on, towards what
// depends on it, or both. It is what makes one traversal serve all three `which nodes` options — `focus on`
// looks both ways, `reachable from` forwards, `dependents of` backwards — instead of three walks of the same
// graph written three times.
type direction uint8

const (
	// directionBoth follows dependencies in either direction, which is a node's neighborhood.
	directionBoth direction = iota
	// directionForward follows a dependency from the depending node to the depended-on one.
	directionForward
	// directionBackward follows it the other way.
	directionBackward
)

// unlimited is the depth of a traversal with no bound on it: the transitive closure `reachable from` and
// `dependents of` mean, as opposed to the counted hops `focus on` is given. Any negative depth means the
// same to reach, and Focus.hops is why a user cannot ask for one by accident.
const unlimited = -1

// nodeSet is a set of a graph's identifiers — the answer to `which nodes`, before anything is drawn.
//
// The query options produce sets and combine them by intersection, so the projection needs a set type rather
// than a slice: order is decided once at the end, when labels are known, and until then it would be a
// promise this type cannot keep.
type nodeSet map[string]struct{}

// newNodeSet is the set holding these nodes.
func newNodeSet(nodes ...string) nodeSet {
	set := make(nodeSet, len(nodes))
	for _, node := range nodes {
		set.add(node)
	}
	return set
}

// add puts a node in the set. It is the one mutating operation in the package, and it is confined to
// building a set that has not been handed to anybody yet.
func (s nodeSet) add(node string) {
	s[node] = struct{}{}
}

// contains reports whether a node is in the set.
func (s nodeSet) contains(node string) bool {
	_, found := s[node]
	return found
}

// intersect is a new set of the nodes in both sets, which is how two `which nodes` options are combined: each
// one narrows the report, so a query holding both means the nodes it can say yes to twice.
func (s nodeSet) intersect(other nodeSet) nodeSet {
	both := make(nodeSet, min(len(s), len(other)))
	for node := range s {
		if other.contains(node) {
			both.add(node)
		}
	}
	return both
}

// adjacency is a graph as a traversal reads it: for every node, the nodes it depends on and the nodes that
// depend on it.
//
// It is built once per snapshot and walked once per `which nodes` option, because a query with three of them
// over a graph of four hundred files would otherwise rescan every edge at every hop.
type adjacency struct {
	// forward maps a node to what it depends on.
	forward map[string][]string
	// backward maps a node to what depends on it.
	backward map[string][]string
}

// newAdjacency indexes these dependencies both ways. Self-dependencies are not in a raw graph, and a
// dependency that is in the graph twice cannot be, so no hop is wasted on either.
func newAdjacency(dependencies extraction.Graph) adjacency {
	links := adjacency{
		forward:  make(map[string][]string, len(dependencies)),
		backward: make(map[string][]string, len(dependencies)),
	}
	for _, dependency := range dependencies {
		links.forward[dependency.Source] = append(links.forward[dependency.Source], dependency.Target)
		links.backward[dependency.Target] = append(links.backward[dependency.Target], dependency.Source)
	}
	return links
}

// neighbors are the nodes one hop from this one, the given way.
func (a adjacency) neighbors(node string, towards direction) []string {
	switch towards {
	case directionForward:
		return a.forward[node]
	case directionBackward:
		return a.backward[node]
	case directionBoth:
		return append(slices.Clone(a.forward[node]), a.backward[node]...)
	default:
		return nil
	}
}

// reach is the set of nodes a traversal from these seeds arrives at, following dependencies the given way,
// stopping after depth hops — or never, when depth is unlimited.
//
// The seeds are in the result, so a focus of depth 0 is the selected nodes themselves and `reachable from`
// includes the code it was asked about. It is a breadth-first walk with the reached set doubling as the
// visited set, which is what makes a cyclic graph terminate: a node is expanded once, at the fewest hops it
// can be reached in.
func (a adjacency) reach(seeds []string, towards direction, depth int) nodeSet {
	reached := newNodeSet(seeds...)
	frontier := slices.Clone(seeds)
	for hop := 0; len(frontier) > 0 && (depth < 0 || hop < depth); hop++ {
		next := make([]string, 0, len(frontier))
		for _, node := range frontier {
			for _, neighbor := range a.neighbors(node, towards) {
				if reached.contains(neighbor) {
					continue
				}
				reached.add(neighbor)
				next = append(next, neighbor)
			}
		}
		frontier = next
	}
	return reached
}
