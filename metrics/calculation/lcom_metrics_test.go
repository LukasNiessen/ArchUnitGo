package calculation_test

import (
	"math"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
)

func TestEveryLCOMOfAPerfectlyCohesiveClassSaysSo(t *testing.T) {
	// Every method reaches every field, which is the one shape the whole family agrees about: 0 for the seven
	// ratios and counts whose 0 is perfect cohesion, and one component for LCOM4.
	class := classWith([]string{"reader", "writer"},
		method{name: "Read", accesses: []string{"reader", "writer"}},
		method{name: "Write", accesses: []string{"writer", "reader"}})

	for _, metric := range everyLCOM() {
		t.Run(metric.name, func(t *testing.T) {
			if got := metric.of(class); !closeTo(got, metric.cohesive) {
				t.Errorf("%s of a class whose methods all share = %v, want %v", metric.name, got, metric.cohesive)
			}
		})
	}
}

func TestEachLCOMScoresAClassThatIsReallyTwoOnItsOwnScale(t *testing.T) {
	// Two methods, two fields, each method reaching one field of its own: the class that should have been split.
	// The eight measures put that at different numbers on purpose — a count of pairs, a fraction of pairings, a
	// number of pieces — and this is the case that pins each of the scales.
	class := classWith([]string{"reader", "writer"},
		method{name: "Read", accesses: []string{"reader"}},
		method{name: "Write", accesses: []string{"writer"}})

	assertEvery(t, class, map[string]float64{
		"LCOM96a": 1,   // ((2/2) - 2) / (1 - 2)
		"LCOM96b": 0.5, // 1 - 2/(2*2)
		"LCOM1":   1,   // one disjoint pair, no sharing pair
		"LCOM2":   0.5, // LCOM96b's twin
		"LCOM3":   1,   // LCOM96a's twin
		"LCOM4":   2,   // the two classes it is
		"LCOM5":   1,   // (2 - 2/2) / (2 - 1)
		"LCOM*":   1,   // the only pair reaches nothing in common
	})
}

func TestEachLCOMScoresAPartlySharedClassOnItsOwnScale(t *testing.T) {
	// The ordinary case: three methods over two fields, with a middle method reaching both, so the class holds
	// together through it. LCOM1 is 0 because the sharing pairs outnumber the disjoint one, while the ratios all
	// report the share of the class that does not overlap.
	class := classWith([]string{"reader", "writer"},
		method{name: "Read", accesses: []string{"reader"}},
		method{name: "Copy", accesses: []string{"reader", "writer"}},
		method{name: "Write", accesses: []string{"writer"}})

	assertEvery(t, class, map[string]float64{
		"LCOM96a": 0.5,     // ((4/2) - 3) / (1 - 3)
		"LCOM96b": 1.0 / 3, // 1 - 4/(2*3)
		"LCOM1":   0,       // two sharing pairs against one disjoint pair, floored at 0
		"LCOM2":   1.0 / 3,
		"LCOM3":   0.5,
		"LCOM4":   1,       // Read and Write hold together through Copy
		"LCOM5":   2.0 / 3, // (2 - 4/3) / (2 - 1)
		"LCOM*":   1.0 / 3, // one of the three pairs reaches nothing in common
	})
}

func TestLCOM4CountsThePiecesAClassWouldFallInto(t *testing.T) {
	// The connected components of the graph whose nodes are the methods and whose edges are the fields two of
	// them share. A method that reaches no field is joined to nothing and is a piece of its own, which is what
	// makes the number a count of pieces rather than a count of fields.
	tests := []struct {
		name  string
		class extraction.ClassInfo
		want  int
	}{
		{
			name: "two methods per field are two pieces",
			class: classWith([]string{"reader", "writer"},
				method{name: "Read", accesses: []string{"reader"}},
				method{name: "Peek", accesses: []string{"reader"}},
				method{name: "Write", accesses: []string{"writer"}},
				method{name: "Flush", accesses: []string{"writer"}}),
			want: 2,
		},
		{
			name: "a chain through a shared field is one piece",
			class: classWith([]string{"reader", "buffer", "writer"},
				method{name: "Read", accesses: []string{"reader"}},
				method{name: "Fill", accesses: []string{"reader", "buffer"}},
				method{name: "Drain", accesses: []string{"buffer", "writer"}},
				method{name: "Write", accesses: []string{"writer"}}),
			want: 1,
		},
		{
			name: "a method that reaches no field is a piece of its own",
			class: classWith([]string{"reader"},
				method{name: "Read", accesses: []string{"reader"}},
				method{name: "Peek", accesses: []string{"reader"}},
				method{name: "String"}),
			want: 2,
		},
		{
			name: "three fields nobody shares are three pieces",
			class: classWith([]string{"reader", "buffer", "writer"},
				method{name: "Read", accesses: []string{"reader"}},
				method{name: "Fill", accesses: []string{"buffer"}},
				method{name: "Write", accesses: []string{"writer"}}),
			want: 3,
		},
		{
			// Four methods on one field join the same group one after the other, so each join has to be made
			// between the two groups' roots rather than between the two methods: the method a field was first
			// reached by has stopped being a root by the third join, and a join that overwrote it, or a walk
			// that stopped one step short of the root, would count this class as three pieces or as two.
			name: "four methods on one field are one piece",
			class: classWith([]string{"reader"},
				method{name: "Read", accesses: []string{"reader"}},
				method{name: "Peek", accesses: []string{"reader"}},
				method{name: "Scan", accesses: []string{"reader"}},
				method{name: "Close", accesses: []string{"reader"}}),
			want: 1,
		},
		{
			// The same depth, joined to a second group through the method that reaches both fields: the group
			// Read and Peek are already in is what `writer`'s piece is joined to, not the method that happened
			// to reach `reader` first.
			name: "a deeper group joined to a second one is one piece",
			class: classWith([]string{"reader", "writer"},
				method{name: "Read", accesses: []string{"reader"}},
				method{name: "Peek", accesses: []string{"reader"}},
				method{name: "Copy", accesses: []string{"reader", "writer"}},
				method{name: "Write", accesses: []string{"writer"}}),
			want: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := calculation.LCOM4(test.class); got != test.want {
				t.Errorf("LCOM4 = %d, want %d pieces", got, test.want)
			}
		})
	}
}

func TestEveryLCOMOfAClassTheQuestionCannotBeAskedOfReportsNoEvidence(t *testing.T) {
	// It takes a field for methods to share and two methods to share it or fail to. Without both there is no
	// evidence of a lack of cohesion, and reporting one would fail a rule about every interface in a project —
	// none of which has a field — for something nobody could fix.
	//
	// A class with no methods at all is 0 on every scale, LCOM4 included: there is nothing there to fall apart.
	tests := []struct {
		name  string
		class extraction.ClassInfo
	}{
		{
			name: "an interface has no fields",
			class: extraction.ClassInfo{
				Name: "Reader", Identifier: "internal/api.Reader", Interface: true, MethodCount: 3,
			},
		},
		{
			name:  "a single method shares with nobody",
			class: classWith([]string{"reader", "writer"}, method{name: "Read", accesses: []string{"reader"}}),
		},
		{
			name:  "a class with fields and no methods",
			class: classWith([]string{"reader", "writer"}),
		},
		{
			name:  "a class whose methods reach no field of it",
			class: classWith(nil, method{name: "Read"}, method{name: "Write"}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, metric := range everyLCOM() {
				want := metric.cohesive
				if len(test.class.Methods) == 0 {
					want = 0
				}
				if got := metric.of(test.class); !closeTo(got, want) {
					t.Errorf("%s of %s = %v, want %v", metric.name, test.name, got, want)
				}
			}
		})
	}
}

func TestALackOfCohesionIsReadOffTheRelationAndNotTheCounts(t *testing.T) {
	// FieldCount and MethodCount are what `count, field count` and `count, method count` report. Cohesion is a
	// question about which method reaches which field, so a class whose relation was never extracted has no
	// answer, however many fields and methods it was counted as having.
	counted := extraction.ClassInfo{
		Name: "Handler", Identifier: "internal/api.Handler", FieldCount: 9, MethodCount: 9,
	}

	for _, metric := range everyLCOM() {
		if got := metric.of(counted); !closeTo(got, 0) {
			t.Errorf("%s of a class with counts but no relation = %v, want no evidence at all", metric.name, got)
		}
	}
}

func TestTheTwoPairsOfNamesThatAreOneNumberStayInStep(t *testing.T) {
	// LCOM3 is LCOM96a's exact twin and LCOM2 is LCOM96b's — the same expression from different papers, kept
	// under both names because that is what a user arriving from either looks for. They are one function each,
	// and this is the test that says so, so that a change to one cannot quietly move only half of the family.
	classes := []extraction.ClassInfo{
		classWith([]string{"reader", "writer"},
			method{name: "Read", accesses: []string{"reader"}},
			method{name: "Write", accesses: []string{"writer"}}),
		classWith([]string{"reader", "writer"},
			method{name: "Read", accesses: []string{"reader"}},
			method{name: "Copy", accesses: []string{"reader", "writer"}},
			method{name: "Write", accesses: []string{"writer"}}),
		classWith([]string{"reader"},
			method{name: "Read", accesses: []string{"reader"}},
			method{name: "String"}),
		{Name: "Reader", Interface: true, MethodCount: 2},
	}

	for _, class := range classes {
		if got, want := calculation.LCOM3(class), calculation.LCOM96a(class); !closeTo(got, want) {
			t.Errorf("LCOM3 of %s = %v, want LCOM96a's %v", class.Name, got, want)
		}
		if got, want := calculation.LCOM2(class), calculation.LCOM96b(class); !closeTo(got, want) {
			t.Errorf("LCOM2 of %s = %v, want LCOM96b's %v", class.Name, got, want)
		}
	}
}

func TestLCOM5NormalisesOverTheFieldsWhereLCOM96aNormalisesOverTheMethods(t *testing.T) {
	// The two are duals rather than twins: LCOM96a averages the methods that reach a field, LCOM5 averages the
	// fields a method reaches. Three methods over two fields is where that shows, and a class with one field is
	// where LCOM5 has nothing left to normalise and says so with a 0.
	spread := classWith([]string{"reader", "writer"},
		method{name: "Read", accesses: []string{"reader"}},
		method{name: "Peek", accesses: []string{"reader"}},
		method{name: "Copy", accesses: []string{"reader", "writer"}})

	if got := calculation.LCOM96a(spread); !closeTo(got, 0.5) {
		t.Errorf("LCOM96a = %v, want ((4/2) - 3) / (1 - 3) = 0.5", got)
	}
	if got := calculation.LCOM5(spread); !closeTo(got, 2.0/3) {
		t.Errorf("LCOM5 = %v, want (2 - 4/3) / (2 - 1) = 0.667", got)
	}

	single := classWith([]string{"reader"},
		method{name: "Read", accesses: []string{"reader"}},
		method{name: "String"})

	if got := calculation.LCOM5(single); !closeTo(got, 0) {
		t.Errorf("LCOM5 of a class with one field = %v, want 0: there is no spread over fields to normalise", got)
	}
	if got := calculation.LCOM96a(single); !closeTo(got, 1) {
		t.Errorf("LCOM96a of the same class = %v, want ((1/1) - 2) / (1 - 2) = 1", got)
	}
}

func TestALCOMOfAClassWhoseFieldsNobodyReachesIsAboveOne(t *testing.T) {
	// Two of the ratios are deliberately unbounded above, because a class whose methods reach no field at all is
	// worse than one whose methods each keep a field to themselves, and a measure that stopped at 1 could not
	// say so.
	unreached := classWith([]string{"reader", "writer", "buffer"},
		method{name: "Read"},
		method{name: "Write"})

	if got := calculation.LCOM96a(unreached); !closeTo(got, 2) {
		t.Errorf("LCOM96a = %v, want ((0/3) - 2) / (1 - 2) = 2", got)
	}
	if got := calculation.LCOM5(unreached); !closeTo(got, 1.5) {
		t.Errorf("LCOM5 = %v, want (3 - 0/2) / (3 - 1) = 1.5", got)
	}
	if got := calculation.LCOM96b(unreached); !closeTo(got, 1) {
		t.Errorf("LCOM96b = %v, want the 1 it is bounded by", got)
	}
}

// cohesion is one of the eight measures as the tests read it: the name the family spells it with, the number it
// takes of a class, and what it answers about a class that holds together.
//
// Every measure is read as a float64 so that one table can hold the family — LCOM1 and LCOM4 are counts and the
// other six are ratios — and the two scales are why `cohesive` is a field rather than a 0 written everywhere:
// LCOM4 counts pieces, and one piece is the cohesive answer.
type cohesion struct {
	name     string
	of       func(extraction.ClassInfo) float64
	cohesive float64
}

// everyLCOM is the eight, in the order the issue lists them.
func everyLCOM() []cohesion {
	return []cohesion{
		{name: "LCOM96a", of: calculation.LCOM96a},
		{name: "LCOM96b", of: calculation.LCOM96b},
		{name: "LCOM1", of: func(class extraction.ClassInfo) float64 { return float64(calculation.LCOM1(class)) }},
		{name: "LCOM2", of: calculation.LCOM2},
		{name: "LCOM3", of: calculation.LCOM3},
		{name: "LCOM4", of: func(class extraction.ClassInfo) float64 { return float64(calculation.LCOM4(class)) }, cohesive: 1},
		{name: "LCOM5", of: calculation.LCOM5},
		{name: "LCOM*", of: calculation.LCOMStar},
	}
}

// assertEvery checks all eight measures of one class against the numbers the table names, and fails if the
// table names fewer than eight: a measure nobody wrote an expectation for is a measure this file does not test.
func assertEvery(t *testing.T, class extraction.ClassInfo, want map[string]float64) {
	t.Helper()
	metrics := everyLCOM()
	if len(want) != len(metrics) {
		t.Fatalf("the table names %d of the %d measures", len(want), len(metrics))
	}
	for _, metric := range metrics {
		expected, named := want[metric.name]
		if !named {
			t.Errorf("the table names no number for %s", metric.name)
			continue
		}
		if got := metric.of(class); !closeTo(got, expected) {
			t.Errorf("%s = %v, want %v", metric.name, got, expected)
		}
	}
}

// method is one method of a fixture class: what it is called, and the fields it reaches.
type method struct {
	name     string
	accesses []string
}

// classWith builds the class information a cohesion measure is a formula over, filling both directions of the
// relation the way metrics/extraction does: every method keeps the fields it reaches, and every field keeps the
// methods that reach it. Building it by hand in one direction only would test a class no extractor can produce.
//
// The counts are filled too, so that a fixture is a class as the extractor would describe it rather than one
// whose counts contradict its relation.
func classWith(fields []string, methods ...method) extraction.ClassInfo {
	class := extraction.ClassInfo{
		Name: "Handler", Identifier: "internal/api.Handler", Path: "internal/api/handler.go",
	}
	position := make(map[string]int, len(fields))
	for _, name := range fields {
		position[name] = len(class.Fields)
		class.Fields = append(class.Fields, extraction.FieldInfo{Name: name})
	}
	for _, declared := range methods {
		class.Methods = append(class.Methods, extraction.MethodInfo{Name: declared.name, AccessedFields: declared.accesses})
		for _, field := range declared.accesses {
			class.Fields[position[field]].AccessedBy = append(class.Fields[position[field]].AccessedBy, declared.name)
		}
	}
	class.FieldCount = len(class.Fields)
	class.MethodCount = len(class.Methods)
	return class
}

// closeTo compares two of the family's ratios. Six of the eight are divisions, so a third and a sixth are
// exactly the numbers a formula produces and never the ones a literal spells.
func closeTo(got, want float64) bool {
	return math.Abs(got-want) <= 1e-9
}
