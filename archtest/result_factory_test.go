package archtest_test

import (
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

func TestNoViolationsIsThePass(t *testing.T) {
	// An empty result is what a rule that holds returns, and the pass flag is read off it rather than
	// tracked beside it: there is no second answer anywhere in the library to disagree with the list.
	for _, violations := range [][]kernel.Violation{nil, {}} {
		result := archtest.NewResultFactory(nil).Result(violations)

		if !result.Passed {
			t.Errorf("Result(%v) failed, want the pass", violations)
		}
		if result.Message != "no violations" {
			t.Errorf("Result(%v) reads %q, want %q", violations, result.Message, "no violations")
		}
	}
}

func TestOneViolationIsCountedInTheSingular(t *testing.T) {
	// A heading that says "1 violations" is the first thing a reader distrusts, and everything under it
	// with it.
	result := archtest.NewResultFactory(nil).Result(namingViolations(t, "internal/db/conn.go"))

	want := "1 violation:\n" +
		`  1. internal/db/conn.go: should, filename matches "*_service.go"; it does not`
	if result.Passed {
		t.Error("a rule with a violation passed, want the failure")
	}
	if result.Message != want {
		t.Errorf("the report reads\n%s\nwant\n%s", result.Message, want)
	}
}

func TestAFailingResultCountsItsViolationsAndThenNumbersThem(t *testing.T) {
	// The count first, because it is the number a reader decides what to do next by; then one line per
	// violation, numbered from one, in the order the rule found them.
	result := archtest.NewResultFactory(nil).Result(namingViolations(t, "a.go", "b.go", "c.go"))

	want := "3 violations:\n" +
		`  1. a.go: should, filename matches "*_service.go"; it does not` + "\n" +
		`  2. b.go: should, filename matches "*_service.go"; it does not` + "\n" +
		`  3. c.go: should, filename matches "*_service.go"; it does not`
	if result.Message != want {
		t.Errorf("the report reads\n%s\nwant\n%s", result.Message, want)
	}
	if strings.HasSuffix(result.Message, "\n") {
		t.Error("the report ends in a newline, want the caller to decide how its last line ends")
	}
}

func TestAReportThatCutItsListShortSaysSo(t *testing.T) {
	// The truncation is never silent: the count at the top is the whole count, and the line at the bottom
	// says how many were left out and which knob did it. A short list that looked complete would be worse
	// than a long one.
	options := &archtest.MessageOptions{MaxViolations: 2}

	result := archtest.NewResultFactory(options).Result(namingViolations(t, "a.go", "b.go", "c.go", "d.go"))

	want := "4 violations:\n" +
		`  1. a.go: should, filename matches "*_service.go"; it does not` + "\n" +
		`  2. b.go: should, filename matches "*_service.go"; it does not` + "\n" +
		"  ... and 2 violations not listed, because MaxViolations is 2"
	if result.Message != want {
		t.Errorf("the report reads\n%s\nwant\n%s", result.Message, want)
	}
	if result.Passed {
		t.Error("a truncated report passed, want the failure")
	}
}

func TestOneViolationLeftOutIsSaidInTheSingularToo(t *testing.T) {
	options := &archtest.MessageOptions{MaxViolations: 1}

	result := archtest.NewResultFactory(options).Result(namingViolations(t, "a.go", "b.go"))

	if !strings.HasSuffix(result.Message, "  ... and 1 violation not listed, because MaxViolations is 1") {
		t.Errorf("the report reads\n%s\nwant the one left-out violation counted in the singular", result.Message)
	}
}

func TestAListThatFitsIsNotCutShort(t *testing.T) {
	// A limit that does not bite says nothing at all, so a suite that sets one does not get a note on every
	// report it reads.
	for _, limit := range []int{-1, 0, 2, 3} {
		options := &archtest.MessageOptions{MaxViolations: limit}

		result := archtest.NewResultFactory(options).Result(namingViolations(t, "a.go", "b.go"))

		if strings.Contains(result.Message, "not listed") {
			t.Errorf("with MaxViolations %d the report reads\n%s\nwant every violation listed and no note", limit, result.Message)
		}
		if lines := strings.Count(result.Message, "\n"); lines != 2 {
			t.Errorf("with MaxViolations %d the report has %d lines under its count, want 2", limit, lines)
		}
	}
}

func TestAColoredReportIsThePlainOneWithItsPartsPainted(t *testing.T) {
	// The same promise the violation messages keep: color is decoration, never content.
	options := &archtest.MessageOptions{Palette: archtest.DefaultPalette(), MaxViolations: 1}
	violations := namingViolations(t, "a.go", "b.go")

	colored := archtest.NewResultFactory(options).Result(violations)
	plain := archtest.NewResultFactory(&archtest.MessageOptions{MaxViolations: 1}).Result(violations)

	if plainText(colored.Message) != plain.Message {
		t.Errorf("the colored report strips to\n%s\nwant\n%s", plainText(colored.Message), plain.Message)
	}
	if !strings.HasPrefix(colored.Message, "\x1b[31m2 violations:\x1b[0m") {
		t.Errorf("the colored report reads\n%s\nwant the count painted in the failure color", colored.Message)
	}
	// The listed violations are painted too, and by the factory's own options: a report whose heading and
	// note are colored but whose violations are plain would still strip to the plain report, so the line
	// itself is what says the options reached the ViolationFactory this factory phrases through.
	line := "  1. \x1b[36ma.go\x1b[0m: \x1b[33mshould, filename matches \"*_service.go\"\x1b[0m; " +
		"\x1b[31mit does not\x1b[0m"
	if !strings.Contains(colored.Message, line) {
		t.Errorf("the colored report reads\n%s\nwant the listed violation painted as\n%s", colored.Message, line)
	}
	if !strings.Contains(colored.Message, "\x1b[90m... and 1 violation not listed") {
		t.Errorf("the colored report reads\n%s\nwant the note painted in the hint color", colored.Message)
	}
	if colored.Passed != plain.Passed {
		t.Error("the colored report and the plain one disagree about the pass, want color to be decoration only")
	}
}

func TestAPassingReportIsPaintedToo(t *testing.T) {
	options := &archtest.MessageOptions{Palette: archtest.DefaultPalette()}

	result := archtest.NewResultFactory(options).Result(nil)

	if result.Message != "\x1b[32mno violations\x1b[0m" {
		t.Errorf("the passing report reads %q, want it painted in the pass color", result.Message)
	}
	if plainText(result.Message) != "no violations" {
		t.Errorf("the passing report strips to %q, want the plain one", plainText(result.Message))
	}
}

func TestTheReportPhrasesEveryKindOfViolationInOneList(t *testing.T) {
	// A suite's rules are of every family, and one report of one rule can already mix them: the empty-test
	// guard reports its own violation beside nothing else, but a reader looking at a list wants the same
	// shape whatever kind each line is.
	violations := []kernel.Violation{
		kernel.NewEmptyTestViolation("files", folderMatcher(t, "internal/renamed/**")),
		filesassertion.NewCycleViolation(cycle(t, "a.go", "b.go")),
		filesassertion.NewAdherenceViolation("c.go", "be short", kernel.ShouldNot),
		nil,
	}

	result := archtest.NewResultFactory(nil).Result(violations)

	want := "4 violations:\n" +
		`  1. no files matched: path without filename matches "internal/renamed/**"; ` + theEmptyTestHint + "\n" +
		"  2. a.go: should, have no cycles; it depends on itself through a.go -> b.go -> a.go\n" +
		`  3. c.go: should not, adhere to "be short"; it does` + "\n" +
		"  4. (no violation)"
	if result.Message != want {
		t.Errorf("the report reads\n%s\nwant\n%s", result.Message, want)
	}
}

// namingViolations are one violation per file, all of the same rule: the fixture a report's shape is tested
// with, because the shape is what these tests are about and one family is enough to see it.
func namingViolations(t *testing.T, files ...string) []kernel.Violation {
	t.Helper()

	required := filenameMatcher(t, "*_service.go")
	violations := make([]kernel.Violation, 0, len(files))
	for _, file := range files {
		violations = append(violations, filesassertion.NewNamingViolation(file, required, kernel.Should))
	}
	return violations
}
