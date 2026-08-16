package calculation_test

import (
	"math"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

func TestEachThresholdAdmitsTheSideOfItsFigureItsVerbNames(t *testing.T) {
	// The five comparisons over the same figure, at the figure and one step either side of it, which is where a
	// strict comparison and an inclusive one are told apart. Every off-by-one this family can have is in here.
	tests := []struct {
		name      string
		threshold calculation.Threshold
		want      map[float64]bool
	}{
		{
			name:      "should be below",
			threshold: calculation.Below(400),
			want:      map[float64]bool{399: true, 400: false, 401: false},
		},
		{
			name:      "should be above",
			threshold: calculation.Above(400),
			want:      map[float64]bool{399: false, 400: false, 401: true},
		},
		{
			name:      "should be",
			threshold: calculation.Exactly(400),
			want:      map[float64]bool{399: false, 400: true, 401: false},
		},
		{
			name:      "should be below or equal",
			threshold: calculation.BelowOrEqual(400),
			want:      map[float64]bool{399: true, 400: true, 401: false},
		},
		{
			name:      "should be above or equal",
			threshold: calculation.AboveOrEqual(400),
			want:      map[float64]bool{399: false, 400: true, 401: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for value, want := range test.want {
				if got := test.threshold.Holds(value); got != want {
					t.Errorf("`be %s` holds for %g = %t, want %t", test.threshold, value, got, want)
				}
			}
		})
	}
}

func TestTheFiveThresholdsCoverEveryNumberBetweenThem(t *testing.T) {
	// The comparisons are complementary by construction, which is what says none of the five is a synonym of
	// another: exactly one of `below`, `should be` and `above` holds for any number, and each inclusive
	// comparison is its strict twin plus the equality.
	limit := 0.5

	for step := range 21 {
		value := float64(step) / 20
		strictlyBelow := calculation.Below(limit).Holds(value)
		exactly := calculation.Exactly(limit).Holds(value)
		strictlyAbove := calculation.Above(limit).Holds(value)

		sides := 0
		for _, holds := range []bool{strictlyBelow, exactly, strictlyAbove} {
			if holds {
				sides++
			}
		}
		if sides != 1 {
			t.Errorf("%g is on %d of the three sides of %g, want exactly one", value, sides, limit)
		}
		if got, want := calculation.BelowOrEqual(limit).Holds(value), strictlyBelow || exactly; got != want {
			t.Errorf("`be below or equal %g` holds for %g = %t, want `below` or the equality, %t", limit, value, got, want)
		}
		if got, want := calculation.AboveOrEqual(limit).Holds(value), strictlyAbove || exactly; got != want {
			t.Errorf("`be above or equal %g` holds for %g = %t, want `above` or the equality, %t", limit, value, got, want)
		}
	}
}

func TestAThresholdCarriesTheFigureAndTheWordsItWasWrittenWith(t *testing.T) {
	// The two halves a violation of this family carries and the sentence a rule renders as are read off here, so
	// what a report quotes is the comparison the user typed rather than a symbol this package chose.
	tests := []struct {
		name           string
		threshold      calculation.Threshold
		wantComparison string
		wantRendered   string
	}{
		{name: "below", threshold: calculation.Below(400), wantComparison: "below", wantRendered: "below 400"},
		{name: "above", threshold: calculation.Above(0), wantComparison: "above", wantRendered: "above 0"},
		{
			// The one comparison with no word of its own: `should be 400` is the whole sentence, so a word here
			// would be printed twice by a rule that already said `be`.
			name: "the equality itself", threshold: calculation.Exactly(400),
			wantComparison: "", wantRendered: "400",
		},
		{
			name: "below or equal", threshold: calculation.BelowOrEqual(0.25),
			wantComparison: "below or equal", wantRendered: "below or equal 0.25",
		},
		{
			name: "above or equal", threshold: calculation.AboveOrEqual(1),
			wantComparison: "above or equal", wantRendered: "above or equal 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.threshold.Comparison(); got != test.wantComparison {
				t.Errorf("Comparison() = %q, want %q", got, test.wantComparison)
			}
			if got := test.threshold.String(); got != test.wantRendered {
				t.Errorf("String() = %q, want %q", got, test.wantRendered)
			}
		})
	}
}

func TestAThresholdKeepsTheFigureItWasGiven(t *testing.T) {
	// The figure travels into a violation beside the number that was found, so a reader is told what the limit
	// was rather than only that it was broken.
	for _, limit := range []float64{0, 1, 400, -1, 0.1} {
		for _, threshold := range thresholdsOf(limit) {
			if got := threshold.Limit(); got != limit {
				t.Errorf("`be %s` reports its limit as %g, want the figure it was built with, %g", threshold, got, limit)
			}
		}
	}
}

func TestAThresholdRendersItsFigureExactly(t *testing.T) {
	// The same rendering every other number in this library gets: as many digits as it takes to say which
	// float64 it is, so a limit is never quietly rounded into a different one in a sentence somebody is
	// comparing against a report.
	if got := calculation.BelowOrEqual(0.1).String(); got != "below or equal 0.1" {
		t.Errorf("String() = %q, want the figure unrounded", got)
	}
	if got := calculation.Exactly(1.0 / 3.0).String(); got != "0.3333333333333333" {
		t.Errorf("String() = %q, want every digit of the float64 it is", got)
	}
	if got := calculation.Below(math.Inf(1)).String(); got != "below +Inf" {
		t.Errorf("String() = %q, want the infinite limit as it is", got)
	}
}

func TestANotANumberMeasurementSatisfiesNoThreshold(t *testing.T) {
	// A number that is on no side of the figure is a real case — a ratio of nothing over nothing is one — and it
	// is reported rather than passed: a comparison it was never in is not a comparison it met.
	for _, threshold := range thresholdsOf(400) {
		if threshold.Holds(math.NaN()) {
			t.Errorf("`be %s` holds for NaN, want a number on no side of the figure to satisfy nothing", threshold)
		}
	}
}

func TestAnInfiniteLimitIsAnOrdinaryFigure(t *testing.T) {
	// No factory rejects one, because `be below +Inf` is the rule that a count is finite at all, and somebody
	// could mean it. It is the fluent stage that rejects the one float64 that is not a figure.
	if !calculation.Below(math.Inf(1)).Holds(400) {
		t.Error("`be below +Inf` does not hold for 400, want every finite number to be under it")
	}
	if calculation.Below(math.Inf(1)).Holds(math.Inf(1)) {
		t.Error("`be below +Inf` holds for +Inf, want a strict comparison to exclude its own figure")
	}
	if !calculation.AboveOrEqual(math.Inf(-1)).Holds(-400) {
		t.Error("`be above or equal -Inf` does not hold for -400, want every number to be over it")
	}
}

func TestTheZeroThresholdHoldsForNoNumber(t *testing.T) {
	// A comparison nobody wrote is not one a number can pass, so a rule written with one reports every
	// measurement under `should` and none under `should not` — the same shape the zero Zone gives, and it cannot
	// arrive from the fluent API, where each verb names its comparison.
	zero := calculation.Threshold{}

	if zero.Comparison() != "" || zero.Limit() != 0 {
		t.Errorf("the zero Threshold is %q of %g, want no comparison at all", zero.Comparison(), zero.Limit())
	}
	for _, value := range []float64{-1, 0, 0.5, 1, 400} {
		if zero.Holds(value) {
			t.Errorf("the zero Threshold holds for %g, want a comparison that admits nothing", value)
		}
	}
}

// thresholdsOf is the five comparisons over one figure, for the tests that are about what every one of them
// does with a number rather than about the side of the figure each admits.
func thresholdsOf(limit float64) []calculation.Threshold {
	return []calculation.Threshold{
		calculation.Below(limit),
		calculation.Above(limit),
		calculation.Exactly(limit),
		calculation.BelowOrEqual(limit),
		calculation.AboveOrEqual(limit),
	}
}
