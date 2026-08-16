package fluentapi_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

func TestGatherEmptyTestViolationsReportsEveryPopulationThatMatchedNothing(t *testing.T) {
	// The guard as a terminal asks it, over the two populations of a relational rule. Either half can be
	// the stale pattern, so either half being empty is a violation — and when both are, both are reported,
	// because a reader who fixed one of two wrong patterns would come straight back for the other.
	subject := fluentapi.EmptyTestPopulation{
		Subject:   "files",
		Selectors: []matching.Filter{matching.FolderMatcher(mustGlob(t, "internal/api/**"))},
	}
	object := fluentapi.EmptyTestPopulation{
		Subject:   "files to depend on",
		Selectors: []matching.Filter{matching.FolderMatcher(mustGlob(t, "internal/db/**"))},
	}

	tests := []struct {
		name        string
		options     *fluentapi.CheckOptions
		populations []fluentapi.EmptyTestPopulation
		want        []string
	}{
		{
			name:        "both halves matched something, which is nothing for the guard to say",
			options:     nil,
			populations: []fluentapi.EmptyTestPopulation{withMatched(subject, 3), withMatched(object, 2)},
			want:        nil,
		},
		{
			name:        "the scope selected nothing",
			options:     nil,
			populations: []fluentapi.EmptyTestPopulation{withMatched(subject, 0), withMatched(object, 2)},
			want:        []string{"files"},
		},
		{
			name:        "the object named nothing, which is the half that goes stale unnoticed",
			options:     &fluentapi.CheckOptions{},
			populations: []fluentapi.EmptyTestPopulation{withMatched(subject, 3), withMatched(object, 0)},
			want:        []string{"files to depend on"},
		},
		{
			name:        "both are empty, so both are reported, in the order the sentence names them",
			options:     nil,
			populations: []fluentapi.EmptyTestPopulation{withMatched(subject, 0), withMatched(object, 0)},
			want:        []string{"files", "files to depend on"},
		},
		{
			name:        "allowEmptyTests is how a user opts out, for every population at once",
			options:     &fluentapi.CheckOptions{AllowEmptyTests: true},
			populations: []fluentapi.EmptyTestPopulation{withMatched(subject, 0), withMatched(object, 0)},
			want:        nil,
		},
		{
			name:        "a rule about one vocabulary has one population",
			options:     nil,
			populations: []fluentapi.EmptyTestPopulation{withMatched(subject, 0)},
			want:        []string{"files"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := test.options.GatherEmptyTestViolations(test.populations...)

			if len(violations) != len(test.want) {
				t.Fatalf("GatherEmptyTestViolations(%+v) = %v, want %d violations", test.populations, violations, len(test.want))
			}
			for index, subject := range test.want {
				if kind := violations[index].Kind(); kind != assertion.KindEmptyTest {
					t.Errorf("violation %d is of kind %q, want %q", index, kind, assertion.KindEmptyTest)
					continue
				}
				empty, ok := violations[index].(assertion.EmptyTestViolation)
				if !ok {
					t.Fatalf("violation %d is a %T, want an EmptyTestViolation", index, violations[index])
				}
				if empty.Subject != subject {
					t.Errorf("violation %d says the rule selected no %q, want %q", index, empty.Subject, subject)
				}
				if len(empty.Selectors) != 1 {
					t.Errorf("violation %d carries %v, want the selectors that described the empty set", index, empty.Selectors)
				}
			}
		})
	}
}

func TestGatherEmptyTestViolationsOfNoPopulationSaysNothing(t *testing.T) {
	// The guard cannot know what a rule was about, so a terminal that hands it no population is a terminal
	// that is not guarded. It is stated here because it is the failure mode the library holds its own
	// terminals to not having: there is no violation to invent from an argument nobody passed.
	if violations := (&fluentapi.CheckOptions{}).GatherEmptyTestViolations(); len(violations) != 0 {
		t.Errorf("GatherEmptyTestViolations() = %v, want nothing: the guard was given no population", violations)
	}
}

func TestGatherEmptyTestViolationsCopiesEachPopulationsSelectors(t *testing.T) {
	// A population's selectors are the caller's slice, and a violation that has been reported must not
	// change afterwards. The copy happens on the way to the violation, which is what this holds.
	selectors := []matching.Filter{matching.FolderMatcher(mustGlob(t, "internal/apis/**"))}

	violations := (&fluentapi.CheckOptions{}).GatherEmptyTestViolations(fluentapi.EmptyTestPopulation{
		Subject:   "files",
		Selectors: selectors,
	})
	selectors[0] = matching.FilenameMatcher(mustGlob(t, "*.go"))

	empty, ok := violations[0].(assertion.EmptyTestViolation)
	if !ok {
		t.Fatalf("got a %T, want an EmptyTestViolation", violations[0])
	}
	if source := empty.Selectors[0].Pattern().Source(); source != "internal/apis/**" {
		t.Errorf("the violation's selector changed with the caller's slice: %q", source)
	}
}

// TestTheGuardOnEveryPopulationOfARelationalRule is the level above the unit tests: the guard where a
// relational terminal wires it in, over the two halves of one sentence resolved against a hand-built graph.
// A pattern that has gone stale on either side is the mistake it exists to catch, and it is caught before
// anything is judged.
func TestTheGuardOnEveryPopulationOfARelationalRule(t *testing.T) {
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
	)

	tests := []struct {
		name   string
		scope  string
		object string
		want   []string
	}{
		{name: "both halves name folders of the project", scope: "internal/api/**", object: "internal/db/**", want: nil},
		{name: "the scope is a typo", scope: "internal/apis/**", object: "internal/db/**", want: []string{"files"}},
		{name: "the folder the object names was renamed", scope: "internal/api/**", object: "internal/database/**", want: []string{"files to depend on"}},
		{
			name:   "both patterns are stale",
			scope:  "internal/apis/**",
			object: "internal/database/**",
			want:   []string{"files", "files to depend on"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scope := matching.FolderMatcher(mustGlob(t, test.scope))
			object := matching.FolderMatcher(mustGlob(t, test.object))

			// What a relational terminal does: resolve both populations against the one graph it extracted,
			// count each of them, and ask the guard before projecting a single dependency.
			violations := (*fluentapi.CheckOptions)(nil).GatherEmptyTestViolations(
				fluentapi.EmptyTestPopulation{Subject: "files", Matched: countMatches(graph, scope), Selectors: []matching.Filter{scope}},
				fluentapi.EmptyTestPopulation{Subject: "files to depend on", Matched: countMatches(graph, object), Selectors: []matching.Filter{object}},
			)

			if len(violations) != len(test.want) {
				t.Fatalf("`in folder %q, should not depend on files in folder %q` reports %v, want %d empty tests", test.scope, test.object, violations, len(test.want))
			}
			for index, subject := range test.want {
				empty, ok := violations[index].(assertion.EmptyTestViolation)
				if !ok {
					t.Fatalf("violation %d is a %T, want an EmptyTestViolation", index, violations[index])
				}
				if empty.Subject != subject {
					t.Errorf("violation %d says the rule selected no %q, want %q", index, empty.Subject, subject)
				}
			}
		})
	}
}

// withMatched is one population with a match count on it, so that a table can vary the count without
// restating what the population is.
func withMatched(population fluentapi.EmptyTestPopulation, matched int) fluentapi.EmptyTestPopulation {
	population.Matched = matched
	return population
}

// countMatches is what a projection does for a terminal, in the one line a guard test needs of it: how many
// of the project's own files a selector accepts.
func countMatches(graph extraction.Graph, selector matching.Filter) int {
	matched := 0
	for _, edge := range graph.SelfEdges() {
		if selector.Matches(edge.Source) {
			matched++
		}
	}
	return matched
}
