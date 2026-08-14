package archtest

import (
	"strconv"
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// passMessage is what a rule that holds reports. It says nothing about the rule, because a passing test
// prints nothing a reader has to act on and the rule itself is in the test they are looking at.
const passMessage = "no violations"

// violationIndent is how far a listed violation sits from the count above it. Two spaces, no tab: a tab is
// eight columns in one terminal and four in another, and these lines are long enough already.
const violationIndent = "  "

// Result is a whole rule's outcome as a test framework needs it: whether the rule holds, and the one
// message to print when it does not.
//
// It is the shape of every ArchUnit port's report — a pass flag plus a message — and the seam an adapter
// works against. An adapter reads these two fields and prints; it never assembles a message of its own,
// because then the library would phrase failures in as many ways as it has adapters.
//
// The violations themselves are deliberately not here. A caller who wants the data rather than the prose
// already has it: Check returned it, and this layer is what turns it into words.
type Result struct {
	// Passed is whether the rule holds: true exactly when there were no violations. It is derived from
	// the list rather than tracked beside it, which is why nothing in the library returns a boolean
	// alongside a []Violation.
	Passed bool
	// Message is the report: one line naming how many violations there are and then one numbered line
	// per violation, or the pass message. It is complete and ready to print — already colored if the
	// options asked for that — and it never ends in a newline, because the caller's own t.Error, log line
	// or diff already decides how the last line ends.
	Message string
}

// ResultFactory shapes the violations a rule reported into a Result: the collection of constructors an
// adapter to a test framework goes through.
//
// It is where numbering, ordering and the count at the top are decided, and it phrases each violation
// through a ViolationFactory rather than knowing any violation type itself. That split is the reason a new
// rule family costs this layer one case in one type switch and nothing else.
//
// It is immutable and cheap: the options and the violation factory built from them. Build one per report,
// or keep one for a suite.
type ResultFactory struct {
	options    MessageOptions
	violations ViolationFactory
}

// NewResultFactory returns the factory that shapes results under these options. A nil *MessageOptions
// means the defaults, so NewResultFactory(nil) is the ordinary call: plain text, every violation listed.
func NewResultFactory(options *MessageOptions) ResultFactory {
	resolved := options.WithDefaults()
	return ResultFactory{options: resolved, violations: NewViolationFactory(&resolved)}
}

// Result is what a rule's violations read as: the pass flag, and the report.
//
// An empty or nil list is the pass, because an empty result is what a rule that holds returns — there is
// no separate boolean anywhere in the library to disagree with the list. Anything else is a failure, and
// reads as the count and then the violations, numbered from one in the order the rule found them:
//
//	2 violations:
//	  1. common/matching/filter.go: should, filename matches "regex_factory.go"; it does not
//	  2. common/matching/match_target.go: should, filename matches "regex_factory.go"; it does not
//
// The count comes first because it is the number a reader decides what to do next by, and it is the whole
// count even when MaxViolations lists fewer: a report that cut its list short says so on its own line,
// naming how many it left out and the knob that did it, so a truncated report can never be mistaken for a
// complete one.
func (f ResultFactory) Result(violations []kernel.Violation) Result {
	if len(violations) == 0 {
		return Result{Passed: true, Message: f.options.Palette.Pass.Paint(passMessage)}
	}

	listed := f.listed(len(violations))
	lines := make([]string, 0, listed+2)
	lines = append(lines, f.options.Palette.Failure.Paint(plural(len(violations), "violation")+":"))
	for number, violation := range violations[:listed] {
		lines = append(lines, violationIndent+strconv.Itoa(number+1)+". "+f.violations.Message(violation))
	}
	if left := len(violations) - listed; left > 0 {
		lines = append(lines, violationIndent+f.options.Palette.Hint.Paint(
			"... and "+plural(left, "violation")+" not listed, because MaxViolations is "+
				strconv.Itoa(f.options.MaxViolations),
		))
	}
	return Result{Passed: false, Message: strings.Join(lines, "\n")}
}

// listed is how many of count violations this report lists, and the whole of what MaxViolations does: the
// limit when it is set and it bites, every violation otherwise. What the report says about the ones it left
// out is Result's, above, because a cut that is not reported is not a cut a reader can allow for.
func (f ResultFactory) listed(count int) int {
	if f.options.MaxViolations <= 0 || f.options.MaxViolations >= count {
		return count
	}
	return f.options.MaxViolations
}
