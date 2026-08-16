package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
)

// KindMetricsThreshold is the kind of ThresholdViolation: a number a rule measured that is on the wrong side
// of the figure the rule compared it against.
//
// It names the vocabulary as well as the failure, the way KindMetricsZone does, because a number held to a
// figure is a rule any vocabulary the library grows metrics for can be written in, and each of them reports its
// own type. The kind is what the testing layer picks a phrasing by, so two families sharing one name would be
// two shapes of data under one key.
//
// The five comparisons are one kind and not five. `should be below` and `should be above or equal` differ by
// the words and the side, both of which are fields below, and a reader who has learned to read one of them has
// learned all five.
const KindMetricsThreshold kernel.ViolationKind = "metrics-threshold"

// ThresholdViolation says that one of the numbers a rule measured is not what the rule required it to be — over
// a limit, under a floor, or not the figure it was told to equal.
//
// It is what the five threshold predicates that compare a measurement against a figure report — `should be
// below`, `should be above`, `should be`, `should be below or equal`, `should be above or equal` — one per
// offending measurement, and it carries both halves of the comparison that failed: the number that was found
// and the figure it was held to. That is the whole diagnosis, because a reader told only that a file is too
// long has to go and measure it again to know by how much.
type ThresholdViolation struct {
	// Subject is what the number was about, as the metric reported it: a file identifier —
	// `internal/api/handler.go` — for a metric about a file, a class identifier — `internal/api.Handler` —
	// for a metric about a class, and a folder identifier — `internal/api` — for a metric about a package.
	// It is never a host path.
	Subject string
	// Metric is what was measured, in the words the rule named it in — `lines of code`, `abstractness`, or
	// the user's own name for a custom metric. It is what tells a reader which number the value is.
	Metric string
	// Value is the number that was found. It is a float64 for the reason calculation.Measurement's own value
	// is one: half this family is a ratio and a count is a ratio's whole-numbered special case.
	Value float64
	// Comparison is the words between `be` and the figure, as the rule was written — `below`, `above`, `below
	// or equal`, `above or equal` — and the empty string for `should be`, whose comparison is the equality
	// itself and has no word of its own.
	//
	// It is the words and not the calculation.Threshold, for the reason ZoneViolation carries the zone's name
	// rather than the region: the testing layer phrases this violation and may not import the module's
	// arithmetic, so what crosses into a report is the comparison the user typed. Which side of the figure
	// that means is calculation.Threshold's business, one layer down, and it has already been answered by the
	// time a violation exists.
	Comparison string
	// Limit is the figure the number was compared against, as the user typed it. It is carried beside the
	// value because the two together are the finding, and either one alone is half of it.
	Limit float64
	// Mood is which way round the requirement was written. It is `should` for all five predicates this family
	// offers — each of them spells its own mood — and it is carried rather than assumed so that a report reads
	// the requirement off the violation exactly as it does for every other family.
	Mood kernel.Mood
}

// NewThresholdViolation records that this number, measured off this subject by this metric, is not on the side
// of this figure that a rule written in this mood required.
//
// It is the only way a violation of this family is made, and every field of it is immutable: three strings, two
// numbers and a flag. The calculation.Threshold it was judged against is deliberately not among them — a
// violation is a value a report reads, and the comparison has already done its work.
func NewThresholdViolation(
	subject, metric string,
	value float64,
	comparison string,
	limit float64,
	mood kernel.Mood,
) ThresholdViolation {
	return ThresholdViolation{
		Subject:    subject,
		Metric:     metric,
		Value:      value,
		Comparison: comparison,
		Limit:      limit,
		Mood:       mood,
	}
}

// Kind is KindMetricsThreshold.
func (ThresholdViolation) Kind() kernel.ViolationKind {
	return KindMetricsThreshold
}

// String renders the violation as the offending subject, the requirement it broke in the words the rule was
// written in, and the number that broke it — `internal/api/handler.go: should, be below 400 (lines of code =
// 900)` — for a log line or a test failure. The comparison that is the equality itself has no word, so it reads
// as `should, be 0`.
//
// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. Both numbers are printed with as
// many digits as it takes to say exactly which float64 they are, for the reason ZoneViolation's coordinates
// are. The user-facing message is still the testing layer's to build, from these same fields.
func (v ThresholdViolation) String() string {
	return v.Subject + ": " + v.Mood.String() + ", be " + v.required() + " (" +
		v.Metric + " = " + format(v.Value) + ")"
}

// required is the comparison this violation broke, as the words that follow `be` in the rule's own sentence —
// `below 400`, and `400` for the comparison with no word of its own.
//
// It is assembled here rather than carried as one string, because the two halves are what a caller reading the
// violation as data wants: a report grouping every file over 400 lines together compares the figure, not a
// sentence about it.
func (v ThresholdViolation) required() string {
	if v.Comparison == "" {
		return format(v.Limit)
	}
	return v.Comparison + " " + format(v.Limit)
}
