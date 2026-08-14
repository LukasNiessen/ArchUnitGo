package archtest

import "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"

// AssertOptions is the one options bag the assert helper takes: how the rule is run, and how the failure is
// written.
//
// AssertPasses spans both halves of the pipeline's tail — it checks and then it reports — so it needs the
// knobs of both, and it holds the two existing bags rather than re-declaring their fields. A flat bag with
// `AllowEmptyTests` and `MaxViolations` side by side would read more briefly and would be a second place
// where every knob the library ever grows has to be added, in step, forever.
//
// It is a struct with a nil-means-defaults contract, like the two bags inside it, and every default is a
// zero value — a quiet, strict check reported as plain text with every violation listed — so a nil bag, the
// zero bag and an explicitly empty one all describe the same assertion. That is why the ordinary call is
// AssertPasses(t, rule, nil). Read a nil bag through WithDefaults rather than reaching for a field.
type AssertOptions struct {
	// Check is how the rule is run, and reaches Checkable.Check unchanged: whether an empty selection is
	// allowed, where the progress log goes, which build tags the project is analyzed under. The zero value
	// is what rule.Check(nil) would have done.
	Check fluentapi.CheckOptions
	// Message is how a rule that does not hold is reported: the palette the report is painted in, and how
	// many violations it lists. The zero value is plain text and every violation, which is the right default
	// for output that is read from a CI log as often as from a terminal.
	Message MessageOptions
}

// WithDefaults returns the options an assertion should actually run with: a copy of the receiver, or the
// defaults when the receiver is nil.
//
// It resolves through each inner bag's own WithDefaults rather than copying the fields, so that a default
// added to CheckOptions or MessageOptions later is honored here without this file being touched — and so
// that the check options arrive with their slices already cloned, as every other caller of them gets them.
func (o *AssertOptions) WithDefaults() AssertOptions {
	if o == nil {
		return AssertOptions{}
	}
	return AssertOptions{Check: o.Check.WithDefaults(), Message: o.Message.WithDefaults()}
}
