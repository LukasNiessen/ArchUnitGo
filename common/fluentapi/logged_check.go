package fluentapi

import (
	"fmt"

	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
)

// LoggedCheck runs a rule's own Check under this bag's logging options: the log opened, `start check`
// written, the rule run, one `violation` record per violation it reported, `end check` written, the log
// closed. It is how every terminal in the library implements Check:
//
//	func (c FilesDependencyCondition) Check(options *kernel.CheckOptions) ([]assertion.Violation, error) {
//		return options.LoggedCheck(c, func(log *logging.Logger) ([]assertion.Violation, error) {
//			// the check itself, logging its own progress through log
//		})
//	}
//
// It is a door in the kernel for the same reason GatherEmptyTestViolations is one: the three records that
// every rule family writes identically — the rule, its violations, its outcome — are written in one place,
// so no family gets to log a little differently and none can forget to log at all. What is left to the
// terminal is the part only it knows, which is the progress of its own pipeline and the numbers a metrics
// rule measured; the logger it is handed is how it says those.
//
// The logger is never nil, so the closure has no branch to write, and it writes nothing at all when no
// destination was asked for — which is the default. Nothing about a check changes when logging is off
// beyond the writing: the violations, the error and the order of the steps are the same either way.
//
// The rule is asked for its own sentence, and a nil one is logged as `a rule` rather than as an empty
// line. The error is the check's own, or — when the check itself succeeded — the log's: a record that
// could not be written or a log file that would not close is a TechnicalError, and it comes back with no
// violations beside it, because the contract Checkable states is that the two are mutually exclusive.
func (o *CheckOptions) LoggedCheck(
	rule fmt.Stringer,
	check func(log *logging.Logger) ([]assertion.Violation, error),
) ([]assertion.Violation, error) {
	log, err := o.Logger()
	if err != nil {
		return nil, err
	}

	sentence := ruleSentence(rule)
	log.StartCheck(sentence)
	violations, err := check(log)
	for _, violation := range violations {
		log.LogViolation(violation)
	}
	log.EndCheck(sentence, len(violations), err)

	if closed := log.Close(); closed != nil && err == nil {
		// The log is an artifact of the run and a truncated one nobody was told about is the failure this
		// reports. It loses to the check's own error, because a rule that could not be run is the more
		// useful of the two things to say.
		return nil, closed
	}
	return violations, err
}

// ruleSentence is the rule as the start and end records name it: whatever it renders itself as, and `a
// rule` when there is nothing to render.
//
// A terminal always passes itself, so the fallback is for a caller of LoggedCheck that has no sentence to
// give — and it says `a rule` rather than nothing at all so that a log's start and end records are still a
// pair a reader can match up.
func ruleSentence(rule fmt.Stringer) string {
	if rule == nil {
		return "a rule"
	}
	if sentence := rule.String(); sentence != "" {
		return sentence
	}
	return "a rule"
}
