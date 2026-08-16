package archtest

import (
	"maps"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
)

// noRulesMessage is what an assertion about a suite with no rules in it reports. It is the empty test one
// level up — a map that lost its entries to a refactor, or that was filled in by a loop over a list which
// turned out to be empty — and it is reported as a failure for the same reason a rule that selected no file
// is: a green run that checked nothing is the one outcome this library treats as worse than any noisy one.
const noRulesMessage = "there are no rules to check: AssertAllPass was given no rules"

// TestingRunner is the standard library's own test handle, as the suite helper needs it: the method that
// records a failure, the mark that says a frame is a helper, and Run — the subtest, which is how Go's
// testing package gives a result its shape.
//
// It is deliberately the one interface in this layer that is not framework-agnostic. Run's argument is a
// func(*testing.T), so nothing but *testing.T satisfies it — not *testing.B, whose Run takes a *testing.B,
// and not a third-party handle unless it is built on the stdlib's. That is the trade the two assert helpers
// split between them: AssertPasses asks for the one method every framework has and works anywhere, and this
// one asks for the standard library so that it can hand back what only the standard library can do.
//
// Helper() is required here rather than asked for at the call, which is how AssertPasses treats it. A handle
// with Run(string, func(*testing.T)) is the stdlib's handle, and the stdlib's handle has Helper — an
// optional interface for a method that cannot be missing would be a branch no test could reach.
type TestingRunner interface {
	TestingT
	// Helper marks the calling frame as a helper, so that a failure is filed against the user's own
	// AssertAllPass line rather than against a line of this file — both the one failure this helper reports
	// itself, a suite with no rules in it, and the ones its subtests report through AssertPasses.
	Helper()
	// Run runs f as a named subtest and reports whether it passed. The signature is the stdlib's, so that
	// *testing.T satisfies this interface without an adapter. The boolean is deliberately not read: a rule
	// that does not hold has already reported itself, and the rules after it are asserted either way, the
	// same way AssertPasses reports through Error rather than a fatal call.
	Run(name string, f func(t *testing.T)) bool
}

// AssertAllPass asserts a whole suite of rules at once, each in its own named subtest. It is the path a Go
// test suite should reach for as soon as it has more than one rule to keep:
//
//	func TestTheArchitectureHolds(t *testing.T) {
//		archunit.AssertAllPass(t, map[string]archunit.Checkable{
//			"the api does not touch the database": archunit.ProjectFiles(nil).
//				InFolder("internal/api/**").
//				ShouldNot().
//				DependOnFiles().
//				InFolder("internal/db/**"),
//			"no file depends on another in a circle": archunit.ProjectFiles(nil).Should().HaveNoCycles(),
//		}, nil)
//	}
//
// Every rule is asserted through AssertPasses, so there is no rule logic here and no second idea of how a
// failure reads: the check is the rule's, the report is the report layer's, and what this adds is the shape
// Go's own testing package gives a result. One pass or fail line per rule, each rule selectable on its own
// with `go test -run 'TestTheArchitectureHolds/the_api_does_not_touch_the_database'`, and a failure filed
// under the name its author gave it:
//
//	--- FAIL: TestTheArchitectureHolds (0.62s)
//	    --- FAIL: TestTheArchitectureHolds/the_api_does_not_touch_the_database (0.59s)
//	        project files, path without filename matches "internal/api/**", should not, depend on files, path without filename matches "internal/db/**"
//	        1 violation:
//	          1. internal/api/handler.go: should not, depend on files, path without filename matches "internal/db/**"; it depends on internal/db/conn.go
//
// The suite is a map from the name to the rule, and its rules are asserted in the sorted order of their
// names. A map is what keeps a name and the rule it belongs to together at the call site, and what makes two
// rules under one name impossible; sorting is what makes a suite's output the same on every run, since Go
// randomizes map iteration on purpose.
//
// A failure inside a subtest is still located at the user's own AssertAllPass line, exactly as a single
// assertion's is, rather than at a line of this file: the standard library, asked to blame a frame inside a
// subtest and finding every frame of it marked as a helper, walks out into the stack the subtest was created
// from, where this helper's own frames are marked too. The rule's own sentence heads the report underneath, so
// the name its author gave it and the rule it stands for are both on screen.
//
// A suite with no rules in it is one failure and no subtests, in those words, rather than a run that quietly
// asserted nothing. A nil rule under a name is reported by AssertPasses inside that name's subtest, so a
// suite cannot lose a rule in silence either.
//
// The options bag is the whole suite's: how every rule is run, and how each failure is written. A nil
// *AssertOptions means the defaults, so AssertAllPass(t, rules, nil) is the ordinary call, and the one rule
// that needs knobs of its own — a selection that is legitimately empty, a report that has to be cut short —
// is asserted beside the suite with its own AssertPasses call.
func AssertAllPass(t TestingRunner, rules map[string]fluentapi.Checkable, options *AssertOptions) {
	t.Helper()

	if len(rules) == 0 {
		t.Error(options.WithDefaults().Message.Palette.Failure.Paint(noRulesMessage))
		return
	}
	for _, name := range slices.Sorted(maps.Keys(rules)) {
		t.Run(name, func(t *testing.T) {
			// Marked, so that the failure is filed against the user's own AssertAllPass line: the stdlib walks
			// out of a subtest into the stack that created it, and every frame between the two is a helper.
			t.Helper()
			AssertPasses(t, rules[name], options)
		})
	}
}
