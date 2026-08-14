package cycles

import "slices"

// johnsonCircuits enumerates the elementary circuits of one strongly connected component: every closed
// path through it that visits no label twice, each one reported exactly once and never again under one
// of the rotations that name the same circuit from a different starting label.
//
// Johnson is the algorithm the sibling ports use for this half, and the reason it is the right one is
// that the naive answer — a depth-first search that reports every closed path it walks into — reports a
// circuit of k labels k times and revisits parts of the graph that provably hold no new circuit. Johnson
// fixes both with the blocking rule below and runs in time proportional to the answer it produces rather
// than to the graph's size raised to anything, which is what makes it usable on a real projection.
//
// Two mechanisms do all the work:
//
//   - The circuits are enumerated per starting label, in the component's own sorted order, and the
//     search for the circuits through one label only looks at that label and the ones after it. A
//     circuit is therefore reported once, rooted at its least label, and every rotation of it is
//     unreachable by construction.
//   - A label the search leaves without having closed a circuit through it is *blocked* rather than
//     released, and is only unblocked when a label it points at closes one. That is what keeps the
//     search from walking a dead end once per path that reaches it, and blockedOn is the record of who
//     to release when that happens.
//
// component is sorted, and successors is the adjacency of the whole projection: only the edges between
// labels of the component are read, so one component's enumeration is independent of every other. A
// circuit comes out as its labels in order, without repeating the one it closes onto — [a b c] is
// a -> b -> c -> a.
//
// limit is how many circuits may be reported, and a negative limit means no bound. The second result is
// false when the limit stopped the enumeration, which is the caller's cue that it holds a prefix of the
// answer rather than the whole of it: the number of elementary circuits in a component is exponential in
// its size in the worst case — a component of twenty labels that all depend on each other has more than
// 10^17 of them — so an unbounded enumeration is not something a report can promise.
func johnsonCircuits(component []string, successors map[string][]string, limit int) ([][]string, bool) {
	search := &johnsonSearch{limit: limit}
	for index, start := range component {
		// The restriction to the labels from start onwards can leave start in no cycle at all, and
		// then there is nothing to search: the labels that are still strongly connected to it are the
		// only ones a circuit through it can use, and if there are none it has none.
		strong := strongComponentOf(start, component[index:], successors)
		if len(strong) < 2 {
			continue
		}
		search.begin(start, inducedSuccessors(strong, successors))
		search.circuit(start)
		if search.truncated {
			break
		}
	}
	return search.circuits, !search.truncated
}

// strongComponentOf is the labels of the strongly connected component holding label in the subgraph
// induced by labels — Tarjan again, on a shrinking graph. A component of one label is a label no circuit
// of that subgraph runs through, because a projection has no self-edges by the time it reaches here.
func strongComponentOf(label string, labels []string, successors map[string][]string) []string {
	for _, component := range tarjanSCC(labels, inducedSuccessors(labels, successors)) {
		if slices.Contains(component, label) {
			return component
		}
	}
	// Unreachable while label is one of labels, which every caller here guarantees: Tarjan puts every
	// label it is given in exactly one component. No component is the honest answer for one it was not.
	return nil
}

// inducedSuccessors is the adjacency of the subgraph induced by labels: every edge of successors with
// both ends in labels, keeping the order successors had them in. Restricting the adjacency once is what
// lets the search below never test membership, and it is what keeps its answer a function of the sorted
// input alone.
func inducedSuccessors(labels []string, successors map[string][]string) map[string][]string {
	members := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		members[label] = struct{}{}
	}

	induced := make(map[string][]string, len(labels))
	for _, label := range labels {
		for _, successor := range successors[label] {
			if _, member := members[successor]; member {
				induced[label] = append(induced[label], successor)
			}
		}
	}
	return induced
}

// johnsonSearch is one run of the algorithm: the state the recursion carries, for the same reason
// tarjanSearch is a value with methods rather than a closure over locals — circuit and unblock recurse
// by name.
//
// The recursion goes as deep as the component is wide, which is bounded by the number of labels in the
// projection and so is well inside Go's growable goroutine stacks.
type johnsonSearch struct {
	successors map[string][]string
	start      string
	blocked    map[string]bool
	blockedOn  map[string][]string
	stack      []string
	circuits   [][]string
	limit      int
	truncated  bool
}

// begin points the search at one starting label and the subgraph its circuits live in. The blocking
// state is per starting label: a label that was a dead end for one root can be on a circuit through the
// next one.
func (s *johnsonSearch) begin(start string, successors map[string][]string) {
	s.successors = successors
	s.start = start
	s.blocked = make(map[string]bool, len(successors))
	s.blockedOn = make(map[string][]string, len(successors))
	s.stack = s.stack[:0]
}

// circuit is the depth-first step: it extends the current path by label and reports a circuit for every
// successor that is the starting label. It answers whether the path through label closed a circuit,
// which is what decides between releasing label and blocking it.
func (s *johnsonSearch) circuit(label string) bool {
	closed := false
	s.stack = append(s.stack, label)
	s.blocked[label] = true

	for _, successor := range s.successors[label] {
		switch {
		case successor == s.start:
			s.emit()
			closed = true
		case !s.blocked[successor]:
			if s.circuit(successor) {
				closed = true
			}
		}
		if s.truncated {
			break
		}
	}

	if closed {
		// Every label that gave up on this one because it was blocked deserves another look now that
		// a circuit runs through it.
		s.unblock(label)
	} else {
		for _, successor := range s.successors[label] {
			s.blockOn(successor, label)
		}
	}

	s.stack = s.stack[:len(s.stack)-1]
	return closed
}

// emit records the path on the stack as a circuit, or stops the enumeration when the limit is reached.
// The path is copied, because the stack is about to be popped back down.
func (s *johnsonSearch) emit() {
	if s.limit >= 0 && len(s.circuits) >= s.limit {
		s.truncated = true
		return
	}
	s.circuits = append(s.circuits, slices.Clone(s.stack))
}

// blockOn records that label stopped searching because successor was blocked, so that unblocking
// successor releases label too.
//
// The record is a slice rather than a set so that it is read back in the order it was written: the
// unblocking order decides which branches the search explores next, and with it the order the circuits
// come out in, which a truncated enumeration has to be able to reproduce.
func (s *johnsonSearch) blockOn(successor, label string) {
	if slices.Contains(s.blockedOn[successor], label) {
		return
	}
	s.blockedOn[successor] = append(s.blockedOn[successor], label)
}

// unblock releases a label and, transitively, everything that had blocked on it. Those are exactly the
// labels whose dead end was a dead end only because this one was on the path.
func (s *johnsonSearch) unblock(label string) {
	s.blocked[label] = false
	// Taken and dropped before recursing: nothing adds to a label's list while it is being released,
	// and reading it after the map entry is gone keeps the recursion from seeing a half-cleared list.
	blockedHere := s.blockedOn[label]
	delete(s.blockedOn, label)

	for _, blocked := range blockedHere {
		if s.blocked[blocked] {
			s.unblock(blocked)
		}
	}
}
