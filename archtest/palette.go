package archtest

// Palette is which color each part of a report is painted in: one field per role a piece of a message
// plays, rather than one per violation kind.
//
// Roles rather than kinds is the whole design. Every message this layer builds is the same sentence —
// this subject broke this requirement, and here is what was found instead — so a reader who has learned
// that the cyan word is the file to go and look at has learned it for every rule family the library ever
// grows, and a rule family that lands later needs no color of its own.
//
// The zero Palette paints nothing, because every field of it is ColorNone: a report is plain text unless
// a caller asks for DefaultPalette, or fills in the roles it cares about. A nil *MessageOptions therefore
// means plain text, which is the right default for a CI log.
type Palette struct {
	// Failure paints the count at the top of a failing report — `3 violations:`.
	Failure Color
	// Pass paints the one line a report of a rule that holds consists of.
	Pass Color
	// Subject paints the thing that disagreed with the rule: the offending file, or the files a cycle
	// runs through. It is what a reader has to go and open, so it is the piece worth finding first.
	Subject Color
	// Requirement paints the rule the subject broke, in the words it was written in: the mood, the
	// predicate and the patterns the user typed.
	Requirement Color
	// Finding paints what was found instead of the rule holding — the files an import reached, the
	// absence of one — which is the offense itself under the negated mood.
	Finding Color
	// Hint paints a sentence that explains a failure rather than reporting it: the note that an empty
	// rule is a violation, the note that a list was cut short. A reader who already knows can skip it,
	// which is why it is the quietest color in the default palette.
	Hint Color
}

// DefaultPalette is the palette a caller who wants color and does not want to choose gets: the failing
// count and what was found in red, a rule that holds in green, the offender in cyan, the requirement in
// yellow and the explanatory notes in gray.
//
// It is a function rather than a package-level variable so that a caller can neither change the library's
// idea of a default report nor be surprised by another caller having done so. Fill in a Palette of your
// own to depart from it; the zero value of any field is ColorNone, so a partial palette is a partially
// colored report rather than a broken one.
func DefaultPalette() Palette {
	return Palette{
		Failure:     ColorRed,
		Pass:        ColorGreen,
		Subject:     ColorCyan,
		Requirement: ColorYellow,
		Finding:     ColorRed,
		Hint:        ColorGray,
	}
}
