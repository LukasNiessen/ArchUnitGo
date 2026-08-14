package cycles

import "slices"

// tarjanSCC returns the strongly connected components of a directed graph given as sorted labels and
// their sorted successors. Every label is in exactly one component, and a component of two or more
// labels is a cycle: each of its labels reaches every other one and comes back.
//
// Tarjan is the algorithm the sibling ports use, and it is the right one here for two reasons beyond
// being linear: it needs one pass rather than the reversed graph a Kosaraju pass wants, and it finds
// the components in an order that is a function of the order it visits nodes in — so sorted input is
// all it takes to make the answer reproducible, which a report has to be. The components come out in
// reverse topological order for that reason, and ProjectCycles sorts them for its own output.
//
// Each component's labels are sorted. A label with no entry in successors is a component of its own.
func tarjanSCC(labels []string, successors map[string][]string) [][]string {
	search := &tarjanSearch{
		successors: successors,
		index:      make(map[string]int, len(labels)),
		lowLink:    make(map[string]int, len(labels)),
		onStack:    make(map[string]bool, len(labels)),
	}
	for _, label := range labels {
		if _, visited := search.index[label]; !visited {
			search.visit(label)
		}
	}
	return search.components
}

// tarjanSearch is one run of the algorithm: the state that a depth-first search carries and that is
// pointless to hand around as eight parameters.
//
// It is a value with methods rather than a closure over local variables so that visit can recurse by
// name. The recursion goes as deep as the longest path in the projection, which for a real project is
// far inside what Go's growable goroutine stacks take.
type tarjanSearch struct {
	successors map[string][]string
	index      map[string]int
	lowLink    map[string]int
	onStack    map[string]bool
	stack      []string
	next       int
	components [][]string
}

// visit is the depth-first step: it numbers a label, walks its successors, and pops a component off the
// stack when the label turns out to be the root of one — the label whose lowest reachable number is its
// own.
func (s *tarjanSearch) visit(label string) {
	s.index[label] = s.next
	s.lowLink[label] = s.next
	s.next++
	s.stack = append(s.stack, label)
	s.onStack[label] = true

	for _, successor := range s.successors[label] {
		if _, visited := s.index[successor]; !visited {
			s.visit(successor)
			s.lowLink[label] = min(s.lowLink[label], s.lowLink[successor])
			continue
		}
		// A successor that is not on the stack belongs to a component already closed, so it says
		// nothing about this one.
		if s.onStack[successor] {
			s.lowLink[label] = min(s.lowLink[label], s.index[successor])
		}
	}

	if s.lowLink[label] != s.index[label] {
		return
	}
	s.components = append(s.components, s.popComponent(label))
}

// popComponent takes everything above the component's root off the stack, root included. Those labels
// are exactly the ones that reach the root and are reached by it.
func (s *tarjanSearch) popComponent(root string) []string {
	component := make([]string, 0, len(s.stack))
	for {
		last := len(s.stack) - 1
		popped := s.stack[last]
		s.stack = s.stack[:last]
		s.onStack[popped] = false
		component = append(component, popped)
		if popped == root {
			break
		}
	}
	slices.Sort(component)
	return component
}
