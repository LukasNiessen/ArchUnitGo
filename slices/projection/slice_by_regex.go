package projection

import (
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// SliceByRegex is SliceByCapture over a regular expression whose one group names the slice:
// `internal/([a-z]+)/.*` is `internal/(**)/**` written in the substrate.
//
// It is the projection behind `project slices, defined by regex "internal/([a-z]+)/.*"`, and it exists
// for the slicings a glob cannot spell — an alternation, a repetition, a character class with a
// quantifier. The expression is taken as written and anchored at both ends, exactly as
// matching.NewRegexCapturePattern takes it, so greediness is the caller's own choice here: `(.*)` names
// the longest run and `(.*?)` the shortest, where the glob spelling picks the short one for you.
//
// An expression with no capturing group, or with more than one, is matching.ErrOneCapture. The groups
// that only hold an alternation together are spelled `(?:...)` and do not count.
func SliceByRegex(expression string) (kernel.MapFunction, error) {
	pattern, err := matching.NewRegexCapturePattern(expression, nil)
	if err != nil {
		return nil, err
	}
	return SliceByCapture(pattern), nil
}
