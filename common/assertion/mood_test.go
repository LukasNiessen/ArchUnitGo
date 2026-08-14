package assertion_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

func TestAMoodIsOneComparison(t *testing.T) {
	tests := []struct {
		name      string
		mood      assertion.Mood
		satisfied bool
		want      bool
	}{
		{name: "should, and the predicate is satisfied", mood: assertion.Should, satisfied: true, want: true},
		{name: "should, and the predicate is not", mood: assertion.Should, satisfied: false, want: false},
		{name: "should not, and the predicate is satisfied", mood: assertion.ShouldNot, satisfied: true, want: false},
		{name: "should not, and the predicate is not", mood: assertion.ShouldNot, satisfied: false, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if holds := test.mood.Holds(test.satisfied); holds != test.want {
				t.Errorf("%s.Holds(%t) = %t, want %t", test.mood, test.satisfied, holds, test.want)
			}
		})
	}
}

func TestAMoodIsTheFlagAndTheTwoWordsOfTheGrammar(t *testing.T) {
	tests := []struct {
		mood        assertion.Mood
		wantNegated bool
		wantString  string
	}{
		{mood: assertion.Should, wantNegated: false, wantString: "should"},
		{mood: assertion.ShouldNot, wantNegated: true, wantString: "should not"},
	}

	for _, test := range tests {
		t.Run(test.wantString, func(t *testing.T) {
			if negated := test.mood.Negated(); negated != test.wantNegated {
				t.Errorf("Negated() = %t, want %t", negated, test.wantNegated)
			}
			if rendered := test.mood.String(); rendered != test.wantString {
				t.Errorf("String() = %q, want %q", rendered, test.wantString)
			}
		})
	}
}

func TestTheZeroMoodIsThePositiveOne(t *testing.T) {
	// A rule with no mood cannot be built through the fluent API, but a Mood is a field of every rule
	// value, so its zero value has to be the harmless one: a rule that judges what its predicate says
	// rather than the opposite of it.
	var zero assertion.Mood

	if zero != assertion.Should {
		t.Errorf("the zero Mood is %s, want %s", zero, assertion.Should)
	}
}

func TestOneAssertionServesBothMoods(t *testing.T) {
	// The reason Mood exists: `should` and `should not` are one walk over one structure, told apart by
	// one flag. Both moods are gathered here by the same function, and what they report has to be
	// exactly complementary — every file judged by one mood or the other, and none by both.
	graph := fixtureGraph()
	files := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go", "main.go"}

	satisfying := offendersOf(t, gatherFixtureViolations(graph, "internal/db/conn.go", assertion.ShouldNot))
	failing := offendersOf(t, gatherFixtureViolations(graph, "internal/db/conn.go", assertion.Should))

	if want := []string{"internal/api/handler.go"}; !slices.Equal(satisfying, want) {
		t.Errorf("`should not depend on internal/db/conn.go` reports %v, want the files that do, %v", satisfying, want)
	}
	if want := []string{"internal/api/router.go", "internal/db/conn.go", "main.go"}; !slices.Equal(failing, want) {
		t.Errorf("`should depend on internal/db/conn.go` reports %v, want the files that do not, %v", failing, want)
	}
	both := slices.Concat(satisfying, failing)
	slices.Sort(both)
	if !slices.Equal(both, files) {
		t.Errorf("the two moods report %v between them, want every file exactly once, %v", both, files)
	}
}

func TestAViolationOfEitherMoodIsStillJustAViolation(t *testing.T) {
	// The mood decides which subjects are reported, and nothing else. It reaches neither the violation
	// type nor its kind, so nothing downstream has to ask which mood a rule was written in.
	graph := fixtureGraph()

	for _, mood := range []assertion.Mood{assertion.Should, assertion.ShouldNot} {
		t.Run(mood.String(), func(t *testing.T) {
			violations := gatherFixtureViolations(graph, "internal/db/conn.go", mood)

			if len(violations) == 0 {
				t.Fatalf("`%s depend on internal/db/conn.go` reported nothing, want the fixture's offenders", mood)
			}
			for _, violation := range violations {
				if kind := violation.Kind(); kind != fixtureKind {
					t.Errorf("Kind() = %q, want %q whichever mood was gathered", kind, fixtureKind)
				}
			}
		})
	}
}

// fixtureKind is the kind of the violation below: a rule family's own, as every module declares one.
const fixtureKind assertion.ViolationKind = "fixture-dependency"

// fixtureViolation is the smallest violation a rule family can have — the two identifiers that
// disagreed with the rule, and not a word about them.
type fixtureViolation struct {
	source string
	target string
}

// Kind is fixtureKind.
func (fixtureViolation) Kind() assertion.ViolationKind {
	return fixtureKind
}

// gatherFixtureViolations is the shape of every `gather <thing> violations` function in the library,
// written out here because no rule family has landed one yet: one walk over the structure, one
// predicate asked the positive question, and the mood as a parameter.
//
// Nothing in it branches on the mood — Holds is the whole of the difference between the two — which is
// what the assertion is being held to.
func gatherFixtureViolations(graph extraction.Graph, target string, mood assertion.Mood) []assertion.Violation {
	violations := make([]assertion.Violation, 0, len(graph))
	for _, node := range graph.SelfEdges() {
		// Read off the dependencies rather than the whole graph: a file's self-edge is how it appears
		// as a node and is not a dependency on itself.
		_, depends := graph.Dependencies().Find(node.Source, target)
		if mood.Holds(depends) {
			continue
		}
		violations = append(violations, fixtureViolation{source: node.Source, target: target})
	}
	return violations
}

// offendersOf reads the files a gathered result names, sorted, so that a test can compare a whole
// result against the fixture it was gathered from.
func offendersOf(t *testing.T, violations []assertion.Violation) []string {
	t.Helper()

	offenders := make([]string, 0, len(violations))
	for _, violation := range violations {
		dependency, ok := violation.(fixtureViolation)
		if !ok {
			t.Fatalf("gathered %T, want a fixtureViolation", violation)
		}
		offenders = append(offenders, dependency.source)
	}
	slices.Sort(offenders)
	return offenders
}

// fixtureGraph is a project hand-built in the shape the extractor produces one: a self-edge per file,
// and the dependencies between them. One file of it depends on the database package and three do not,
// which is what makes the two moods' answers tell each other apart.
func fixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
	)
}
