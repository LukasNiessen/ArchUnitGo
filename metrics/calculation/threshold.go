package calculation

import "strconv"

// Threshold is what a number has to be for a rule about it to hold: one figure the user typed, and which side
// of that figure satisfies the comparison.
//
// The five threshold predicates that compare a measurement against a figure — `should be below`, `should be
// above`, `should be`, `should be below or equal`, `should be above or equal` — are five factories of this one
// type and nothing else, so the arithmetic of a comparison is written once. Which of the five a rule was
// written with is the three flags in here, and the words the sentence renders are Comparison's.
//
// A Threshold is data plus one question — Holds — and that question is arithmetic rather than a judgement:
// whether failing it is a violation is the mood's business, in metrics/assertion, and the words a report says
// about it are the testing layer's. That is the same split Zone takes, for the same reason.
//
// A Threshold is immutable: get one from a factory below and read it through its methods. The zero Threshold
// admits no side of its figure at all, so it holds for no number whatsoever — a comparison nobody wrote is not
// a comparison a number can pass — which is the answer the zero Zone gives to the same question.
type Threshold struct {
	// comparison is the words between `be` and the figure, spelled as the grammar spells them — `below`,
	// `below or equal` — and the empty string for `should be`, where the figure follows the verb on its own.
	comparison string
	// limit is the figure the number is compared against, as the user typed it.
	limit float64
	// below, equal and above are which of the three answers to that comparison satisfy it: `below or equal`
	// is the first two, `should be` the middle one alone. Three flags rather than one operator string,
	// because Holds is then one expression that no new factory has to be added to.
	below bool
	equal bool
	above bool
}

// Below is the comparison `should be below` asks: strictly under the figure, which is the reading a limit
// somebody wrote as a maximum-plus-one has — `below 400` holds for 399 and not for 400.
func Below(limit float64) Threshold {
	return Threshold{comparison: "below", limit: limit, below: true}
}

// Above is the comparison `should be above` asks: strictly over the figure. It is the floor half of the
// family, and the one a rule about a number that must not collapse is written with — `above 0`.
func Above(limit float64) Threshold {
	return Threshold{comparison: "above", limit: limit, above: true}
}

// Exactly is the comparison `should be` asks: the number is the figure and no other number.
//
// It is named Exactly rather than EqualTo because `should equal` is a synonym AGENTS.md forbids the grammar,
// and a factory carrying that word is how the verb would get written next. The predicate it serves spells no
// comparison at all — `should be 0` — which is why Comparison is empty for it.
func Exactly(limit float64) Threshold {
	return Threshold{comparison: "", limit: limit, equal: true}
}

// BelowOrEqual is the comparison `should be below or equal` asks: under the figure or at it, which is the
// reading a limit somebody wrote as a maximum has — `below or equal 400` holds for 400 and not for 401.
func BelowOrEqual(limit float64) Threshold {
	return Threshold{comparison: "below or equal", limit: limit, below: true, equal: true}
}

// AboveOrEqual is the comparison `should be above or equal` asks: over the figure or at it, which is the
// reading a minimum has — `above or equal 1` holds for 1 and not for 0.
func AboveOrEqual(limit float64) Threshold {
	return Threshold{comparison: "above or equal", limit: limit, above: true, equal: true}
}

// Holds reports whether this number satisfies the comparison: whether the side of the figure it falls on is
// one of the sides the threshold admits.
//
// It is written as one expression over the three flags rather than as a switch over the side the number is
// on, because a number that is on no side of the figure is a real case: a NaN measurement compares false
// against everything, so it satisfies no threshold at all and is reported under `should` instead of quietly
// passing a comparison it was never in.
//
// The zero Threshold admits nothing and therefore holds for no number, whatever it is asked about. An
// infinite limit is an ordinary figure here — `below +Inf` holds for every finite number — and no factory
// rejects one, because a rule saying a count must be finite is a rule somebody could mean.
func (t Threshold) Holds(value float64) bool {
	return (t.below && value < t.limit) || (t.equal && value == t.limit) || (t.above && value > t.limit)
}

// Comparison is the words between `be` and the figure, as the rule was written — `below`, `above`, `below or
// equal`, `above or equal` — and the empty string for `should be`, whose comparison is the equality itself and
// has no word of its own.
//
// It is what the sentence a rule renders as is assembled from and what a violation of this family carries, so
// that a report quotes the comparison the user typed rather than a symbol this package chose.
func (t Threshold) Comparison() string {
	return t.comparison
}

// Limit is the figure the number is compared against, as the user typed it. It is carried into a violation
// beside the number that was found, because a reader told only that a threshold was broken still has to know
// what it was.
func (t Threshold) Limit() float64 {
	return t.limit
}

// String renders the threshold as the words that follow `be` in the sentence the rule was written as —
// `below 400`, `above or equal 0.5`, and `400` for the comparison that is the equality itself.
//
// The figure is printed with as many digits as it takes to say exactly which float64 it is, for the reason
// Measurement's own rendering does: a rule whose limit reads as a rounded number in a test failure is a rule
// nobody can compare against the figure a report printed.
func (t Threshold) String() string {
	figure := strconv.FormatFloat(t.limit, 'g', -1, 64)
	if t.comparison == "" {
		return figure
	}
	return t.comparison + " " + figure
}
