package archtest_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
)

// The two claims the suite helper's interface makes: the standard library's own handle satisfies it, with
// nothing to register and nothing to adapt, and a handle written by hand can stand in for it in a test.
// *testing.B deliberately does not appear here — its Run takes a *testing.B, so it cannot satisfy this
// interface, which is the whole reason AssertPasses asks for one method instead.
var (
	_ archtest.TestingRunner = (*testing.T)(nil)
	_ archtest.TestingRunner = (*runner)(nil)
)

func TestEveryRuleOfASuiteIsAssertedInItsOwnNamedSubtest(t *testing.T) {
	// The shape the suite form exists for: one subtest per rule, named the way its author named it, and the
	// rule checked inside it. The order of the checks is the order of the subtests, which is what says that
	// each subtest asserted its own rule rather than the suite asserting them all in one.
	framework := &runner{}
	suite, checked := suiteOf(map[string][]kernel.Violation{
		"the api does not touch the database":             nil,
		"no file depends on another in a circle":          nil,
		"every file is named the way the layout asks for": nil,
	})

	archtest.AssertAllPass(framework, suite, nil)

	want := slices.Sorted(maps.Keys(suite))
	if names := framework.names(); !slices.Equal(names, want) {
		t.Errorf("the suite ran the subtests %v, want one per rule, named and sorted: %v", names, want)
	}
	if !slices.Equal(*checked, want) {
		t.Errorf("the suite checked %v, want each rule inside its own subtest, in that order: %v", *checked, want)
	}
	if failures := framework.failures(t); len(failures) != 0 {
		t.Errorf("a suite whose rules all hold reported %v against the test itself, want nothing", failures)
	}
	for _, subtest := range framework.subtests {
		if subtest.failed {
			t.Errorf("the subtest %q failed, want the pass a rule that holds reports", subtest.name)
		}
	}
}

func TestTheSubtestsOfASuiteAreRunInTheSameOrderOnEveryRun(t *testing.T) {
	// Go randomizes map iteration deliberately, so a suite that ranged over its rules would report them in a
	// different order every run — and `go test -run` output that reorders itself is output nobody can diff.
	// Eight runs, because one run of a randomized order agrees with a sorted one often enough to pass.
	suite, _ := suiteOf(map[string][]kernel.Violation{
		"d": nil, "c": nil, "b": nil, "a": nil, "e": nil, "f": nil, "g": nil,
	})
	want := []string{"a", "b", "c", "d", "e", "f", "g"}

	for range 8 {
		framework := &runner{}

		archtest.AssertAllPass(framework, suite, nil)

		if names := framework.names(); !slices.Equal(names, want) {
			t.Fatalf("a run of the suite reported %v, want the same sorted order every time: %v", names, want)
		}
	}
}

func TestARuleOfASuiteThatDoesNotHoldFailsItsOwnSubtestAndNoOther(t *testing.T) {
	// A framework counts failures per subtest, which is what the suite form is for: the rule that broke is
	// named by the subtest that failed, and the rules around it are still asserted and still pass. The failure
	// is the subtest's and not the suite's — nothing is reported against the test that called the helper.
	// The failing rule sorts first on purpose: the subtests are counted before their outcomes are read, so a
	// helper that stopped at the first Run that came back false would record one subtest here instead of three.
	framework := &runner{}
	suite, _ := suiteOf(map[string][]kernel.Violation{
		"a rule that holds":         nil,
		"a rule that does not hold": namingViolations(t, "a.go", "b.go"),
		"another rule that holds":   nil,
	})

	archtest.AssertAllPass(framework, suite, nil)

	want := slices.Sorted(maps.Keys(suite))
	if names := framework.names(); !slices.Equal(names, want) {
		t.Fatalf("the suite ran the subtests %v, want one per rule even after one of them failed: %v", names, want)
	}
	if failures := framework.failures(t); len(failures) != 0 {
		t.Errorf("the suite reported %v against the test itself, want each failure inside its own subtest", failures)
	}
	for _, subtest := range framework.subtests {
		if failed := subtest.name == "a rule that does not hold"; failed != subtest.failed {
			t.Errorf("the subtest %q failed: %t, want %t", subtest.name, subtest.failed, failed)
		}
	}
}

func TestASuiteDoesNotLoseANilRuleQuietly(t *testing.T) {
	// A rule left nil in a map — an entry that was never filled in, a field that was never assigned — fails
	// its own subtest, in the words AssertPasses reports it in. There is no branch for it here: the suite
	// hands every entry to the same helper, so the mistake is caught by the layer that already catches it.
	framework := &runner{}

	archtest.AssertAllPass(framework, map[string]fluentapi.Checkable{"a rule that was never built": nil}, nil)

	if len(framework.subtests) != 1 {
		t.Fatalf("the suite ran %d subtests, want the one the nil rule was named by", len(framework.subtests))
	}
	if !framework.subtests[0].failed {
		t.Errorf("the subtest %q passed, want a nil rule reported as the mistake it is", framework.subtests[0].name)
	}
}

func TestASuiteWithNoRulesInItIsAFailureRatherThanASilentPass(t *testing.T) {
	// The empty test one level up. A nil map and an empty one are the same suite, and both are reported
	// against the test itself rather than as a subtest, because there is no rule to name one after.
	for name, suite := range map[string]map[string]fluentapi.Checkable{
		"a nil suite":    nil,
		"an empty suite": {},
	} {
		framework := &runner{}

		archtest.AssertAllPass(framework, suite, nil)

		failures := framework.failures(t)
		if len(failures) != 1 {
			t.Fatalf("%s reported %d failures, want the one:\n%v", name, len(failures), failures)
		}
		if want := "there are no rules to check: AssertAllPass was given no rules"; failures[0] != want {
			t.Errorf("%s reads %q, want %q", name, failures[0], want)
		}
		if len(framework.subtests) != 0 {
			t.Errorf("%s ran the subtests %v, want none", name, framework.names())
		}
	}
}

func TestTheOptionsOfASuiteReachEveryRuleOfIt(t *testing.T) {
	// One bag for the whole suite, and every rule of it runs under the same one: a suite is a policy, and a
	// knob that reached the first rule and not the third would be a policy with a hole in it. The bag arrives
	// resolved, as it does at every other Check in the library, so a nil one is the defaults and not nothing.
	framework := &runner{}
	suite, _ := suiteOf(map[string][]kernel.Violation{"first": nil, "second": nil, "third": nil})

	archtest.AssertAllPass(framework, suite, &archtest.AssertOptions{
		Check: fluentapi.CheckOptions{AllowEmptyTests: true, IncludeTestFiles: true},
	})

	for name, rule := range suite {
		checked, ok := rule.(*suiteRule)
		if !ok {
			t.Fatalf("the suite holds a %T under %q, want the test's own rule", rule, name)
		}
		if checked.options == nil {
			t.Fatalf("%q was checked with a nil options bag, want the resolved one", name)
		}
		if !checked.options.AllowEmptyTests || !checked.options.IncludeTestFiles {
			t.Errorf("%q was checked with %+v, want the suite's own knobs", name, checked.options)
		}
	}
}

func TestTheFailureOfASuiteIsAttributedToTheTestThatAssertedIt(t *testing.T) {
	// Helper() puts the file and line of the user's own AssertAllPass call on the one failure the suite
	// reports itself, and it is called whether or not the suite has rules in it — a helper that only marked
	// itself on the way to a failure would leave a frame behind for the assertion after it. The mark inside
	// each subtest, which carries that same attribution out of a subtest's own failure, is not asserted here:
	// a *testing.T does not expose the frame it filed a failure against, and Run's argument has to be a real
	// one.
	for name, suite := range map[string]map[string]fluentapi.Checkable{
		"a suite with no rules": nil,
		"a suite of one rule":   {"a rule that holds": &suiteRule{log: new([]string)}},
	} {
		framework := &runner{}

		archtest.AssertAllPass(framework, suite, nil)

		if framework.helpers != 1 {
			t.Errorf("%s marked itself as a helper %d times, want exactly once", name, framework.helpers)
		}
	}
}

// suiteOf builds a suite of rules from the violations each of them is to report, plus the log they append
// their own name to as they are checked — which is how a test sees the order the suite asserted them in, and
// that each of them was asserted exactly once.
func suiteOf(violations map[string][]kernel.Violation) (map[string]fluentapi.Checkable, *[]string) {
	checked := new([]string)
	suite := make(map[string]fluentapi.Checkable, len(violations))
	for name, reported := range violations {
		suite[name] = &suiteRule{name: name, violations: reported, log: checked}
	}
	return suite, checked
}

// suiteRule is a rule of a suite as the helper sees one: a Checkable that answers with what the test put in
// it, records the options it was checked with, and appends its own name to the suite's log.
type suiteRule struct {
	name       string
	violations []kernel.Violation
	options    *fluentapi.CheckOptions
	log        *[]string
}

func (r *suiteRule) Check(options *fluentapi.CheckOptions) ([]kernel.Violation, error) {
	r.options = options
	*r.log = append(*r.log, r.name)
	return r.violations, nil
}

func (r *suiteRule) String() string {
	return "project files, should, " + r.name
}

// runner is the standard library's handle as the suite helper sees one: the recorder every other test here
// uses, plus Run. It records the name of each subtest and whether it failed, and it runs the body against a
// handle of its own so that a test can read the outcome instead of failing this test with it.
type runner struct {
	recorder
	subtests []subtest
}

// subtest is one rule's outcome as a framework reports it: the name the suite ran it under, and whether the
// rule asserted inside it held.
type subtest struct {
	name   string
	failed bool
}

func (r *runner) Run(name string, f func(t *testing.T)) bool {
	// A fresh *testing.T with no parent: the assert helper reports through Error, and Failed is what a
	// framework reads that back off — which is all a test of the suite form needs, because what the failure
	// reads as is AssertPasses's promise and is tested against a recorder over there.
	handle := &testing.T{}
	f(handle)
	r.subtests = append(r.subtests, subtest{name: name, failed: handle.Failed()})
	return !handle.Failed()
}

// names are the subtests the suite ran, in the order it ran them.
func (r *runner) names() []string {
	names := make([]string, 0, len(r.subtests))
	for _, subtest := range r.subtests {
		names = append(names, subtest.name)
	}
	return names
}
