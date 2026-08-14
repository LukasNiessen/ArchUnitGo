package archtest

import (
	"fmt"

	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
)

// checkFailedPrefix introduces the one failure AssertPasses reports that is not a violation: the check did
// not run at all, so nothing is known about the rule. A project that will not load and a rule that does not
// hold are different problems for the reader, and a report that did not distinguish them would send someone
// looking for an architecture mistake that is not there.
const checkFailedPrefix = "the rule could not be checked: "

// nilRuleMessage is what an assertion about no rule at all reports. It is a mistake in the test rather than
// in the code under it — and it is reported as a failure, not skipped, because an assertion that quietly
// asserted nothing is the one outcome this library treats as worse than any noisy one.
const nilRuleMessage = "there is no rule to check: AssertPasses was given a nil rule"

// TestingT is the part of a test framework's handle AssertPasses needs: one method, the one that records a
// failure and lets the test carry on. TestingRunner is this interface plus what a suite of rules asks for on
// top, and nothing else in the library asks a framework for anything.
//
// It is the smallest interface that can report anything, and that is the whole point of it. *testing.T and
// *testing.B satisfy it, so does stdlib testing.TB, and so does every third-party framework's handle that
// has ever called itself a drop-in for them — Ginkgo's GinkgoT(), a gocheck *check.C, a mock a user writes
// to test their own suite helper. There is nothing to register and nothing to configure: a framework works
// here by already having the method every framework has.
//
// Helper() is deliberately not part of it. AssertPasses calls it when the handle has it, so a failure is
// reported against the user's own line, and a framework without it gets the report all the same rather than
// being locked out over a convenience.
type TestingT interface {
	// Error records a failure and lets the test continue, which is Go's non-fatal assertion and what an
	// architecture rule wants: a suite that checks several rules should report all of them in one run, not
	// stop at the first. Its signature is the stdlib's, variadic and untyped, so that *testing.T satisfies
	// this interface without an adapter — AssertPasses always passes it exactly one string.
	Error(args ...any)
}

// helper is the optional other half of a test handle: the method that tells the framework the calling frame
// belongs to a helper, so that a failure is attributed to the test that asserted rather than to a line
// inside this library.
type helper interface {
	Helper()
}

// AssertPasses checks the rule and fails the test with the formatted violations when it does not hold. It is
// the library's test-framework glue, and the call an architecture test about one rule ends in — AssertAllPass
// is the same assertion over a suite of them, and is written over this one:
//
//	func TestTheApiDoesNotTouchTheDatabase(t *testing.T) {
//		rule := archunit.ProjectFiles(nil).
//			InFolder("internal/api/**").
//			ShouldNot().
//			DependOnFiles().
//			InFolder("internal/db/**")
//
//		archunit.AssertPasses(t, rule, nil)
//	}
//
// A rule that holds reports nothing at all: a passing test that printed its own success would bury the one
// that did not. A rule that does not hold is one t.Error, carrying the rule as the user wrote it and then
// the whole report — the count, and the violations numbered from one:
//
//	project files, in folder "internal/api/**", should not, depend on files, in folder "internal/db/**"
//	1 violation:
//	  1. internal/api/handler.go: should not, depend on files, path without filename matches "internal/db"; it depends on internal/db/conn.go
//
// The rule's own sentence is the first line because a test that asserts several rules, or asserts one in a
// loop, otherwise reports a list of files with nothing saying which sentence they broke. It is left out when
// the rule cannot describe itself, which no rule the library builds does.
//
// A nil *AssertOptions means the defaults, which is what makes this helper the documented fallback: it needs
// no configuration, no registration and no framework of its own. AssertOptions is how a suite asks for
// colored output, a violation limit, or a check that allows an empty selection.
//
// Three things it does not do. It does not stop the test — Error and not Fatal, so that a suite reports every
// rule it checked rather than the first that broke. It does not return the violations: a caller who wants the
// data rather than the failure calls Check, which is the layer below and is public for exactly that. And it
// never raises for a rule failure, because a failing rule is a Violation in a list; the only failure it
// reports outside the report is a technical one, and it says so in those words.
//
// t is required: with nowhere to report to there is nothing this helper can do, and a nil handle is a mistake
// in the test rather than something to pass over in silence. A nil rule, on the other hand, is reported as an
// ordinary failure — the test has somewhere to say so.
func AssertPasses(t TestingT, rule fluentapi.Checkable, options *AssertOptions) {
	if marked, ok := t.(helper); ok {
		marked.Helper()
	}
	resolved := options.WithDefaults()

	if rule == nil {
		t.Error(resolved.Message.Palette.Failure.Paint(nilRuleMessage))
		return
	}
	violations, err := rule.Check(&resolved.Check)
	if err != nil {
		// The violations say nothing when the error is non-nil, so they are not reported: the check is what
		// did not happen, and naming an architecture mistake here would be inventing one.
		t.Error(resolved.Message.Palette.Failure.Paint(checkFailedPrefix + err.Error()))
		return
	}

	result := NewResultFactory(&resolved.Message).Result(violations)
	if result.Passed {
		return
	}
	t.Error(headed(rule, result.Message, resolved.Message.Palette))
}

// headed puts the rule the report is about above the report itself, in the words the user wrote it in.
//
// The sentence is painted in the requirement color, because that is the role it plays — the rule that was
// broken, one line up from the files that broke it. A rule that cannot say what it is, which is only ever a
// Checkable of a caller's own, gets the report unheaded rather than a heading saying so.
func headed(rule fluentapi.Checkable, report string, palette Palette) string {
	described, ok := rule.(fmt.Stringer)
	if !ok {
		return report
	}
	sentence := described.String()
	if sentence == "" {
		return report
	}
	return palette.Requirement.Paint(sentence) + "\n" + report
}
