package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// KindMetricsSatisfaction is the kind of SatisfactionViolation: a number a rule measured that does not
// satisfy the predicate the user wrote about it.
//
// It names the vocabulary as well as the failure, the way KindMetricsZone does, because a number judged by
// a function of the user's own is a rule any vocabulary the library grows metrics for can be written in,
// and each of them reports its own type. The kind is what the testing layer picks a phrasing by, so two
// families sharing one name would be two shapes of data under one key.
const KindMetricsSatisfaction kernel.ViolationKind = "metrics-satisfaction"

// SatisfactionViolation says that one of the numbers a rule measured does not satisfy the predicate the
// user gave it — or does satisfy it, where the rule forbade it.
//
// It is what `metrics, ..., should satisfy` reports, one per offending measurement, and it carries the
// number that was found beside the words the rule was asked in: the subject the number is about, which
// metric it was, the number itself, the requirement as the user phrased it and the mood.
//
// The requirement is prose, as it is in AdherenceViolation and for the same unavoidable reason: the rule
// was a Go function, so there is nothing else to carry. The number is carried with it because it is the
// whole diagnosis — a reader who is told a class fails a threshold they wrote still has to know what it
// actually came to, and the alternative is running the rule again by hand.
type SatisfactionViolation struct {
	// Subject is what the number was about, as the metric reported it: a file identifier —
	// `internal/api/handler.go` — for a metric about a file, a class identifier — `internal/api.Handler` —
	// for a metric about a class, and a folder identifier — `internal/api` — for a metric about a package.
	// It is never a host path.
	Subject string
	// Metric is what was measured, in the words the rule named it in — `method count`, `abstractness`, or
	// the user's own name for a custom metric. It is what tells a reader which number the value is.
	Metric string
	// Value is the number that was found. It is a float64 for the reason calculation.Measurement's own
	// value is one: half this family is a ratio and a count is a ratio's whole-numbered special case.
	Value float64
	// Requirement is what the rule asked of each measurement, in the user's own words: the message
	// argument of `should satisfy`, phrased as a bare infinitive so that the mood reads onto it — `be at
	// most 10 methods deep`.
	Requirement string
	// Mood is which way round the requirement was written. Satisfying the predicate is what `should`
	// demands and what `should not` forbids, so without the mood a report could not tell one failure from
	// the other — and the same number and requirement describe both.
	Mood kernel.Mood
}

// NewSatisfactionViolation records that this number, measured off this subject by this metric, does not
// satisfy the predicate of a rule written in this mood, and what that predicate was asking for.
//
// It is the only way a violation of this family is made, and every field of it is immutable: three
// strings, a number and a flag. The user's function is deliberately not among them — a violation is a
// value a report reads, and a closure is neither printable nor safe to call a second time.
func NewSatisfactionViolation(subject, metric string, value float64, requirement string, mood kernel.Mood) SatisfactionViolation {
	return SatisfactionViolation{
		Subject:     subject,
		Metric:      metric,
		Value:       value,
		Requirement: requirement,
		Mood:        mood,
	}
}

// Kind is KindMetricsSatisfaction.
func (SatisfactionViolation) Kind() kernel.ViolationKind {
	return KindMetricsSatisfaction
}

// String renders the violation as the offending subject, the requirement it broke in the words the rule
// was written in, and the number that broke it — `internal/api.Handler: should, satisfy "be at most 10
// methods deep" (method count = 40)` — for a log line or a test failure.
//
// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. The number is printed with as
// many digits as it takes to say exactly which float64 it is, for the reason ZoneViolation's coordinates
// are. The user-facing message is still the testing layer's to build, from these same fields.
func (v SatisfactionViolation) String() string {
	return v.Subject + ": " + v.Mood.String() + `, satisfy "` + v.Requirement + `" (` +
		v.Metric + " = " + format(v.Value) + ")"
}
