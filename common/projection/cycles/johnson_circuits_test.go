package cycles

import (
	"slices"
	"strings"
	"testing"
)

// adjacencyOf builds the sorted adjacency johnsonCircuits takes out of `source -> target` pairs, through
// the same reader ProjectCircuits uses — so a fixture here is a projection, spelled shortly.
func adjacencyOf(pairs ...string) ([]string, map[string][]string) {
	return adjacency(projectedEdges(pairs...))
}

// renderedCircuits turns label sequences into `a -> b -> a` strings, closing label included, so a
// failure reads as the cycle it is about.
func renderedCircuits(circuits [][]string) []string {
	rendered := make([]string, 0, len(circuits))
	for _, circuit := range circuits {
		closed := make([]string, 0, len(circuit)+1)
		closed = append(closed, circuit...)
		closed = append(closed, circuit[0])
		rendered = append(rendered, strings.Join(closed, " -> "))
	}
	return rendered
}

// sortedCircuits is renderedCircuits with the order taken out. The order Johnson finds circuits in is
// reproducible but it is the search's, not the report's — ProjectCircuits owns the order a reader sees,
// and its own tests are where that is pinned.
func sortedCircuits(circuits [][]string) []string {
	rendered := renderedCircuits(circuits)
	slices.Sort(rendered)
	return rendered
}

// completeAdjacency is every edge between every pair of labels: the worst case for an enumeration, and
// the graph whose exact circuit count is known — sum over k of C(n,k)*(k-1)! elementary circuits.
func completeAdjacency(labels ...string) ([]string, map[string][]string) {
	pairs := make([]string, 0, len(labels)*(len(labels)-1))
	for _, source := range labels {
		for _, target := range labels {
			if source != target {
				pairs = append(pairs, source+" -> "+target)
			}
		}
	}
	return adjacencyOf(pairs...)
}

func TestJohnsonCircuitsFindsAMutualDependency(t *testing.T) {
	_, successors := adjacencyOf("a -> b", "b -> a")

	circuits, complete := johnsonCircuits([]string{"a", "b"}, successors, -1)

	if want := []string{"a -> b -> a"}; !slices.Equal(sortedCircuits(circuits), want) {
		t.Errorf("johnsonCircuits = %v, want %v", sortedCircuits(circuits), want)
	}
	if !complete {
		t.Error("johnsonCircuits reported an incomplete enumeration, want the whole set")
	}
}

func TestJohnsonCircuitsReportsATriangleOnceAndNoRotationOfIt(t *testing.T) {
	// a -> b -> c -> a is one circuit, not three: b -> c -> a -> b and c -> a -> b -> c are the same
	// cycle read from another label. Rooting each circuit at its least label is what rules them out.
	_, successors := adjacencyOf("a -> b", "b -> c", "c -> a")

	circuits, complete := johnsonCircuits([]string{"a", "b", "c"}, successors, -1)

	if want := []string{"a -> b -> c -> a"}; !slices.Equal(sortedCircuits(circuits), want) {
		t.Errorf("johnsonCircuits = %v, want %v", sortedCircuits(circuits), want)
	}
	if !complete {
		t.Error("johnsonCircuits reported an incomplete enumeration, want the whole set")
	}
}

func TestJohnsonCircuitsSeparatesTwoCyclesSharingALabel(t *testing.T) {
	// The pair with tarjan_scc_test.go's TestTarjanSCCGroupsTwoCyclesSharingALabelIntoOne, and the whole
	// reason this file exists: {a, b, c} is one strongly connected component and two cycles, and only
	// the second answer tells a reader which two dependencies to break.
	_, successors := adjacencyOf("a -> b", "b -> a", "a -> c", "c -> a")

	circuits, _ := johnsonCircuits([]string{"a", "b", "c"}, successors, -1)

	want := []string{"a -> b -> a", "a -> c -> a"}
	if got := sortedCircuits(circuits); !slices.Equal(got, want) {
		t.Errorf("johnsonCircuits = %v, want %v", got, want)
	}
}

func TestJohnsonCircuitsFindsTheNestedCyclesOfACompleteTriangle(t *testing.T) {
	labels, successors := completeAdjacency("a", "b", "c")

	circuits, _ := johnsonCircuits(labels, successors, -1)

	want := []string{
		"a -> b -> a",
		"a -> b -> c -> a",
		"a -> c -> a",
		"a -> c -> b -> a",
		"b -> c -> b",
	}
	if got := sortedCircuits(circuits); !slices.Equal(got, want) {
		t.Errorf("johnsonCircuits = %v, want %v", got, want)
	}
}

func TestJohnsonCircuitsCountsEveryCircuitOfACompleteGraphExactlyOnce(t *testing.T) {
	// Four labels all depending on each other hold 6 + 8 + 6 = 20 elementary circuits — C(4,k)*(k-1)! of
	// length k. Both halves matter: 20 is what "finds every one" means, and no duplicate among them is
	// what "exactly once" means.
	labels, successors := completeAdjacency("a", "b", "c", "d")

	circuits, complete := johnsonCircuits(labels, successors, -1)

	rendered := sortedCircuits(circuits)
	if len(rendered) != 20 {
		t.Errorf("johnsonCircuits found %d circuits, want 20: %v", len(rendered), rendered)
	}
	if unique := slices.Compact(slices.Clone(rendered)); len(unique) != len(rendered) {
		t.Errorf("johnsonCircuits repeated a circuit: %v", rendered)
	}
	if !complete {
		t.Error("johnsonCircuits reported an incomplete enumeration, want the whole set")
	}
}

// deadEndAdjacency is the fixture that exercises the blocking rule: one triangle a -> b -> c -> a with
// two mutual dependencies hanging off b in a chain, b <-> d <-> e.
//
// Searching for the circuits through a walks into d and then e, and neither of them can reach a without
// going back through b, which the path is already standing on. Both are dead ends for that search and are
// blocked rather than released — and both are released again once c closes a circuit through b, e through
// d. A search without the rule would rediscover those dead ends once per path that reaches them.
func deadEndAdjacency() ([]string, map[string][]string) {
	return adjacencyOf("a -> b", "b -> c", "c -> a", "b -> d", "d -> b", "d -> e", "e -> d")
}

func TestJohnsonCircuitsWalksPastADeadEndInsideAComponent(t *testing.T) {
	labels, successors := deadEndAdjacency()

	circuits, complete := johnsonCircuits(labels, successors, -1)

	want := []string{"a -> b -> c -> a", "b -> d -> b", "d -> e -> d"}
	if got := sortedCircuits(circuits); !slices.Equal(got, want) {
		t.Errorf("johnsonCircuits = %v, want %v", got, want)
	}
	if !complete {
		t.Error("johnsonCircuits reported an incomplete enumeration, want the whole set")
	}
}

// releasedDeadEndPairs is the fixture that exercises the *transitive* half of the blocking rule, which
// the chain above does not: the triangle a -> c -> d -> b -> a, with a <-> b and b <-> d inside it.
//
// Searching for the circuits through a takes the b branch first, and there d is a dead end — every way on
// from it goes back through b, which the path is standing on — so d is blocked and records itself against
// b. Then b closes a -> b -> a, and releasing b has to release d with it, because the second branch of the
// same root walks a -> c -> d and needs d released to find a -> c -> d -> b -> a. Without blockOn's record
// or without unblock's recursion over it, d stays blocked from the first branch and that circuit is lost.
func releasedDeadEndPairs() []string {
	return []string{"a -> b", "b -> a", "b -> d", "d -> b", "a -> c", "c -> d"}
}

func TestJohnsonCircuitsReleasesADeadEndForALaterBranchOfTheSameRoot(t *testing.T) {
	labels, successors := adjacencyOf(releasedDeadEndPairs()...)

	circuits, complete := johnsonCircuits(labels, successors, -1)

	want := []string{"a -> b -> a", "a -> c -> d -> b -> a", "b -> d -> b"}
	if got := sortedCircuits(circuits); !slices.Equal(got, want) {
		t.Errorf("johnsonCircuits = %v, want %v", got, want)
	}
	if !complete {
		t.Error("johnsonCircuits reported an incomplete enumeration, want the whole set")
	}
}

func TestJohnsonCircuitsIsAFunctionOfTheGraphAlonePastADeadEnd(t *testing.T) {
	// The blocking state is the one part of the search that is not read straight off the sorted
	// adjacency, so it is the one that could leak a map's iteration order into the answer. Both dead-end
	// fixtures are run, because only the second one has a blockedOn list whose contents reach the answer:
	// the order it is read back in is what decides which branch is explored next.
	for name, fixture := range map[string]func() ([]string, map[string][]string){
		"a dead end blocked and released in turn": deadEndAdjacency,
		"a dead end released through another label": func() ([]string, map[string][]string) {
			return adjacencyOf(releasedDeadEndPairs()...)
		},
	} {
		t.Run(name, func(t *testing.T) {
			labels, successors := fixture()
			first, _ := johnsonCircuits(labels, successors, -1)

			for range 8 {
				again, _ := johnsonCircuits(labels, successors, -1)
				if !slices.Equal(renderedCircuits(first), renderedCircuits(again)) {
					t.Fatalf("johnsonCircuits = %v, want the same answer every run: %v",
						renderedCircuits(again), renderedCircuits(first))
				}
			}
		})
	}
}

// naiveCircuits is the answer johnsonCircuits exists to produce faster: every simple path that returns to
// where it started, kept only when it starts at the least of its own labels so that the rotations naming
// one circuit are counted once.
//
// It has no blocking rule and it walks every path in the graph, which is the point — it is slow enough to
// be obviously right, and it is the ground truth the fixtures below are checked against instead of a
// hand-counted expectation.
func naiveCircuits(labels []string, successors map[string][]string) [][]string {
	found := make([][]string, 0, len(labels))
	var walk func(path []string)
	walk = func(path []string) {
		start, last := path[0], path[len(path)-1]
		for _, successor := range successors[last] {
			switch {
			case successor == start:
				found = append(found, slices.Clone(path))
			case successor > start && !slices.Contains(path, successor):
				walk(append(slices.Clone(path), successor))
			}
		}
	}
	for _, label := range labels {
		walk([]string{label})
	}
	return found
}

func TestJohnsonCircuitsAgreesWithTheNaiveEnumeration(t *testing.T) {
	for name, pairs := range map[string][]string{
		"acyclic":                    {"a -> b", "b -> c", "a -> c", "d -> c"},
		"a triangle with a shortcut": {"a -> b", "b -> c", "c -> a", "a -> c"},
		"two independent cycles":     {"a -> b", "b -> a", "a -> y", "y -> z", "z -> y"},
		"a dead end inside a component": {
			"a -> b", "b -> c", "c -> a", "b -> d", "d -> b", "d -> e", "e -> d",
		},
		// The fixture that makes a release transitive, which is the half of the blocking rule the chain
		// above never needs: see releasedDeadEndPairs.
		"a dead end released through another label": releasedDeadEndPairs(),
		// The fixture that makes the search record one dead end against the same blocked label twice,
		// which is why blockOn keeps its records a set. Found by enumerating the graphs of five labels.
		"a label blocked on twice": {
			"a -> c", "b -> a", "b -> d", "c -> b", "c -> e", "d -> b", "d -> c", "e -> b",
		},
		"five labels in a ring with every shortcut back": {
			"a -> b", "b -> c", "c -> d", "d -> e", "e -> a",
			"b -> a", "c -> a", "d -> a", "e -> b", "e -> c",
		},
	} {
		t.Run(name, func(t *testing.T) {
			labels, successors := adjacencyOf(pairs...)

			circuits, complete := johnsonCircuits(labels, successors, -1)

			if want := sortedCircuits(naiveCircuits(labels, successors)); !slices.Equal(sortedCircuits(circuits), want) {
				t.Errorf("johnsonCircuits = %v, want %v", sortedCircuits(circuits), want)
			}
			if !complete {
				t.Error("johnsonCircuits reported an incomplete enumeration, want the whole set")
			}
		})
	}
}

func TestJohnsonCircuitsOnlyLooksInsideTheComponentItWasGiven(t *testing.T) {
	// The adjacency is the whole projection's, so the restriction to the component is what keeps the
	// search off the labels that only touch it — x reaches the cycle and y is reached by it.
	_, successors := adjacencyOf("x -> a", "a -> b", "b -> a", "b -> y", "y -> x")

	circuits, _ := johnsonCircuits([]string{"a", "b"}, successors, -1)

	if want := []string{"a -> b -> a"}; !slices.Equal(sortedCircuits(circuits), want) {
		t.Errorf("johnsonCircuits = %v, want %v", sortedCircuits(circuits), want)
	}
}

func TestJohnsonCircuitsFindsNothingInAnAcyclicComponent(t *testing.T) {
	// A component of one label, handed in as one: nothing to close a circuit with, and no self-edge in a
	// projection to pretend otherwise.
	_, successors := adjacencyOf("a -> b", "b -> c")

	circuits, complete := johnsonCircuits([]string{"a", "b", "c"}, successors, -1)

	if len(circuits) != 0 {
		t.Errorf("johnsonCircuits = %v, want no circuit", renderedCircuits(circuits))
	}
	if !complete {
		t.Error("johnsonCircuits reported an incomplete enumeration, want the whole set")
	}
}

func TestJohnsonCircuitsStopsAtItsLimitAndSaysSo(t *testing.T) {
	labels, successors := completeAdjacency("a", "b", "c", "d")

	for _, limit := range []int{0, 1, 7, 19} {
		circuits, complete := johnsonCircuits(labels, successors, limit)

		if len(circuits) != limit {
			t.Errorf("johnsonCircuits with limit %d found %d circuits, want %d", limit, len(circuits), limit)
		}
		if complete {
			t.Errorf("johnsonCircuits with limit %d reported the whole set, want a truncated one", limit)
		}
	}
}

func TestJohnsonCircuitsIsCompleteWhenTheLimitIsExactlyEnough(t *testing.T) {
	labels, successors := completeAdjacency("a", "b", "c", "d")

	circuits, complete := johnsonCircuits(labels, successors, 20)

	if len(circuits) != 20 || !complete {
		t.Errorf("johnsonCircuits with limit 20 = %d circuits, complete %t, want 20 and true",
			len(circuits), complete)
	}
}

func TestJohnsonCircuitsIsAFunctionOfTheGraphAlone(t *testing.T) {
	// Including the truncated run: which circuits a limit keeps is the order the search found them in, so
	// an unstable order would make a truncated report unreproducible rather than merely oddly sorted.
	labels, successors := completeAdjacency("a", "b", "c", "d")

	for _, limit := range []int{5, -1} {
		first, _ := johnsonCircuits(labels, successors, limit)
		for range 8 {
			again, _ := johnsonCircuits(labels, successors, limit)
			if !slices.Equal(renderedCircuits(first), renderedCircuits(again)) {
				t.Fatalf("johnsonCircuits with limit %d = %v, want the same answer every run: %v",
					limit, renderedCircuits(again), renderedCircuits(first))
			}
		}
	}
}

func TestStrongComponentOfShrinksWithTheSubgraphItIsGiven(t *testing.T) {
	// Johnson's restriction, on its own: in the whole triangle b is in a cycle, and in the subgraph that
	// has dropped a it is in none — which is what makes the enumeration report each circuit once.
	labels, successors := adjacencyOf("a -> b", "b -> c", "c -> a")

	if got, want := strongComponentOf("b", labels, successors), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("strongComponentOf(b) in %v = %v, want %v", labels, got, want)
	}
	if got, want := strongComponentOf("b", []string{"b", "c"}, successors), []string{"b"}; !slices.Equal(got, want) {
		t.Errorf("strongComponentOf(b) in [b c] = %v, want %v", got, want)
	}
}

func TestInducedSuccessorsKeepsOnlyTheEdgesWithBothEndsInTheSubgraph(t *testing.T) {
	_, successors := adjacencyOf("a -> b", "a -> x", "b -> a", "b -> y")

	induced := inducedSuccessors([]string{"a", "b"}, successors)

	if got, want := induced["a"], []string{"b"}; !slices.Equal(got, want) {
		t.Errorf("induced successors of a = %v, want %v", got, want)
	}
	if got, want := induced["b"], []string{"a"}; !slices.Equal(got, want) {
		t.Errorf("induced successors of b = %v, want %v", got, want)
	}
	if got := induced["x"]; len(got) != 0 {
		t.Errorf("induced successors of x = %v, want none — x is not in the subgraph", got)
	}
}
