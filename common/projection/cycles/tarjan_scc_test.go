package cycles

import (
	"slices"
	"testing"
)

// fixtureAdjacency is a hand-built directed graph with one three-node cycle (a, b, c), one two-node
// cycle (e, f), a node that only points into a cycle (d) and one that touches nothing (g).
func fixtureAdjacency() ([]string, map[string][]string) {
	labels := []string{"a", "b", "c", "d", "e", "f", "g"}
	successors := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a", "e"},
		"d": {"c"},
		"e": {"f"},
		"f": {"e"},
	}
	return labels, successors
}

func componentOf(t *testing.T, components [][]string, label string) []string {
	t.Helper()
	for _, component := range components {
		if slices.Contains(component, label) {
			return component
		}
	}
	t.Fatalf("no component holds %q in %v", label, components)
	return nil
}

func TestTarjanSCCPutsEveryLabelInExactlyOneComponent(t *testing.T) {
	labels, successors := fixtureAdjacency()

	components := tarjanSCC(labels, successors)

	seen := make(map[string]int, len(labels))
	for _, component := range components {
		for _, label := range component {
			seen[label]++
		}
	}
	for _, label := range labels {
		if seen[label] != 1 {
			t.Errorf("%q is in %d components, want 1: %v", label, seen[label], components)
		}
	}
	if len(seen) != len(labels) {
		t.Errorf("components mention %d labels, want %d: %v", len(seen), len(labels), components)
	}
}

func TestTarjanSCCGroupsEveryCycleAndNothingElse(t *testing.T) {
	labels, successors := fixtureAdjacency()

	components := tarjanSCC(labels, successors)

	if got, want := componentOf(t, components, "a"), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("component of a = %v, want %v", got, want)
	}
	if got, want := componentOf(t, components, "e"), []string{"e", "f"}; !slices.Equal(got, want) {
		t.Errorf("component of e = %v, want %v", got, want)
	}
	// d reaches the cycle and g reaches nothing: neither is in one.
	for _, label := range []string{"d", "g"} {
		if got, want := componentOf(t, components, label), []string{label}; !slices.Equal(got, want) {
			t.Errorf("component of %q = %v, want %v", label, got, want)
		}
	}
}

func TestTarjanSCCFindsALabelWithNoSuccessorsAtAll(t *testing.T) {
	components := tarjanSCC([]string{"alone"}, nil)

	if want := [][]string{{"alone"}}; !slices.EqualFunc(components, want, slices.Equal) {
		t.Errorf("components = %v, want %v", components, want)
	}
}

func TestTarjanSCCGroupsTwoCyclesSharingALabelIntoOne(t *testing.T) {
	// a -> b -> a and a -> c -> a are one strongly connected component, not two cycles: every label
	// reaches every other one, and that is what makes the component the thing a report names.
	labels := []string{"a", "b", "c"}
	successors := map[string][]string{"a": {"b", "c"}, "b": {"a"}, "c": {"a"}}

	components := tarjanSCC(labels, successors)

	if want := [][]string{{"a", "b", "c"}}; !slices.EqualFunc(components, want, slices.Equal) {
		t.Errorf("components = %v, want %v", components, want)
	}
}

func TestTarjanSCCIgnoresASuccessorItWasNotGivenAsALabel(t *testing.T) {
	// The adjacency the projection builds always lists both ends of every edge, so this is a
	// defensive case: an unlisted successor is visited through the edge that names it and lands in a
	// component of its own, rather than being silently dropped.
	components := tarjanSCC([]string{"a"}, map[string][]string{"a": {"b"}})

	if got, want := componentOf(t, components, "b"), []string{"b"}; !slices.Equal(got, want) {
		t.Errorf("component of b = %v, want %v", got, want)
	}
}

func TestTarjanSCCIsAFunctionOfTheGraphAlone(t *testing.T) {
	labels, successors := fixtureAdjacency()
	first := tarjanSCC(labels, successors)

	for range 8 {
		again := tarjanSCC(labels, successors)
		if !slices.EqualFunc(first, again, slices.Equal) {
			t.Fatalf("components = %v, want the same answer every run: %v", again, first)
		}
	}
}
