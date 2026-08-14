package archtest

// MessageOptions is the one options bag this layer takes: everything about how a report is written, as
// opposed to what it reports.
//
// It is a struct with a nil-means-defaults contract, like fluentapi.CheckOptions, and every default is a
// zero value — a plain-text report that lists every violation — so a nil bag, the zero bag and an
// explicitly empty one all describe the same report. Read a nil bag through WithDefaults rather than
// reaching for a field.
type MessageOptions struct {
	// Palette is which color each part of a message is painted in. The zero Palette paints nothing, so
	// the default is plain text: an escape sequence in a CI log is noise, and the caller who wants color
	// is the one who knows whether a terminal is there. DefaultPalette is the palette to ask for.
	Palette Palette
	// MaxViolations is how many violations a report lists before it says how many it left out. Zero, and
	// anything below it, means every violation.
	//
	// A rule that a repository has never been held to can report hundreds of files, and a test failure
	// that scrolls a terminal is one nobody reads the top of. The cut is never silent — the report says
	// how many were left out and which knob did it — because a truncated list that looks complete is
	// worse than a long one.
	MaxViolations int
}

// WithDefaults returns the options a report should actually be written with: a copy of the receiver, or
// the defaults when the receiver is nil.
//
// Both factories start here, so the nil-means-defaults contract is honored in one place instead of being
// re-derived as a nil check per field, and a default that is not a zero value can be added later without
// touching either of them. There is nothing to clone: a Palette is six colors and a limit is an int, so
// the copy shares nothing with the caller's own bag.
func (o *MessageOptions) WithDefaults() MessageOptions {
	if o == nil {
		return MessageOptions{}
	}
	return *o
}
