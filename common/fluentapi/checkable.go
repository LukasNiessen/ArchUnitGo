// Package fluentapi holds the contract every rule in the library ends in — Checkable — and the one
// options bag its terminal takes, CheckOptions.
//
// A rule is a value, not an action. The chain a user types — `project files, in folder
// "internal/api/**", should not, depend on files, in folder "internal/db/**"` — builds a value and
// does no work; it ends in a terminal, and the terminal is a Checkable. Running it is the only thing
// that touches the filesystem.
//
// Every consumer programs against Checkable and nothing else: the testing layer, a report, a user's
// own helper that loops over a list of rules. That is what keeps adding a rule family a change to
// one module — nothing downstream knows or asks whether a Checkable came from files, layers, slices
// or metrics.
//
// Two things every terminal goes through live here rather than in the families, so that no family can be
// quietly lenient about either: the empty-test guard, which is why a rule that selected nothing is a
// violation, and LoggedCheck, which is why every check writes the same three records to the log a user
// asked for. Both hang off CheckOptions, because both are decisions the user's own bag makes.
//
// Each domain module has its own fluentapi package, holding the builders the user types. This one
// holds only what all of them have in common.
package fluentapi

import "github.com/LukasNiessen/ArchUnitGo/common/assertion"

// Checkable is a rule that can be run: the seam the whole library hangs from.
//
// The interface has one method and no unexported members, so it is open — a rule family in any
// module, or a user with a rule of their own, can be a Checkable, and everything that consumes rules
// works on it unchanged.
type Checkable interface {
	// Check runs the rule and returns one Violation for every place the code disagrees with it. An
	// empty result is the pass: there is no boolean to keep in step with the list.
	//
	// A nil *CheckOptions means the defaults, so rule.Check(nil) is the ordinary call.
	//
	// The error reports a technical failure — a project that will not load, a file that will not
	// parse, an option used wrongly — and never a failing rule. A rule that fails returns violations
	// and a nil error; when the error is non-nil the violations say nothing and the caller should
	// ignore them. The error is an archerror.TechnicalError or an archerror.UserError, so that a
	// caller can read off whether the library or the rule as written is at fault.
	Check(options *CheckOptions) ([]assertion.Violation, error)
}
