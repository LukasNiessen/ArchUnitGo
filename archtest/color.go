package archtest

// Color is a terminal color the report paints one part of a message in: the offender, the rule it broke,
// the count at the top.
//
// It is a closed set of names rather than an escape sequence a caller hands in. Report policy is this
// layer's, so a report that is assembled in one place can be changed in one place — and a caller able to
// pass an arbitrary escape sequence could put a cursor move, or a clear-screen, into somebody's test
// output.
//
// ColorNone is the zero value and paints nothing, so the zero Palette is a plain-text report and color is
// opted into rather than out of. That is worth more than it looks: test output is read at least as often
// from a CI log, where an escape sequence is noise, as from a terminal.
type Color uint8

// The colors a report can be painted in: the eight of the ANSI basic set that are legible on a light and
// on a dark background, which rules out black and white.
const (
	// ColorNone paints nothing at all: Paint hands the text back exactly as it came in. It is the zero
	// value, and what every field of the zero Palette is.
	ColorNone Color = iota
	// ColorRed is the failure color: the count at the top of a failing report, and what an offender was
	// found to actually do.
	ColorRed
	// ColorGreen is the pass color: the one line a report of a rule that holds consists of.
	ColorGreen
	// ColorYellow is the requirement color: the rule an offender broke, in the words it was written in.
	ColorYellow
	// ColorBlue is for a palette of a caller's own; the default palette does not use it.
	ColorBlue
	// ColorMagenta is for a palette of a caller's own; the default palette does not use it.
	ColorMagenta
	// ColorCyan is the subject color: the file, or the cycle, a reader has to go and look at.
	ColorCyan
	// ColorGray is the hint color: the sentence that explains a failure rather than reporting it.
	ColorGray
)

// colorEscape starts an ANSI escape sequence, and colorReset ends the painted text by returning the
// terminal to whatever it was set to before. Every painted string is wrapped in exactly this pair, so a
// report can never leave a terminal colored after it.
const (
	colorEscape = "\x1b["
	colorReset  = "\x1b[0m"
)

//nolint:gochecknoglobals // immutable lookup table indexed by Color; Go has no const array.
var colorNames = [...]string{
	ColorNone:    "none",
	ColorRed:     "red",
	ColorGreen:   "green",
	ColorYellow:  "yellow",
	ColorBlue:    "blue",
	ColorMagenta: "magenta",
	ColorCyan:    "cyan",
	ColorGray:    "gray",
}

//nolint:gochecknoglobals // immutable lookup table indexed by Color; Go has no const array.
var colorCodes = [...]string{
	ColorNone:    "",
	ColorRed:     "31",
	ColorGreen:   "32",
	ColorYellow:  "33",
	ColorBlue:    "34",
	ColorMagenta: "35",
	ColorCyan:    "36",
	ColorGray:    "90",
}

// Valid reports whether c is one of the declared colors. A value that is not — an integer cast into the
// type — paints nothing, the way common/matching.MatchTarget answers "unknown" rather than indexing past
// its own table.
func (c Color) Valid() bool {
	return int(c) < len(colorNames)
}

// String names the color as a report and a test failure spell it: "red", "gray", "none".
//
// It is deliberately the name and not the escape sequence. A Color reaching a `%v` by accident — in a
// test's own failure message, in a log line — would otherwise print as an invisible control sequence, and
// the reader would be told nothing at all.
func (c Color) String() string {
	if !c.Valid() {
		return "unknown"
	}
	return colorNames[c]
}

// Paint returns text wrapped in this color's escape sequence, ready to be written to a terminal, and
// returns it unchanged when there is nothing to paint: ColorNone, an undeclared color, or empty text.
//
// It is the only place in the library that emits an escape sequence. Everything else names a Color and
// asks for it here, which is what keeps a plain report and a colored one the same message.
func (c Color) Paint(text string) string {
	if text == "" || !c.Valid() || colorCodes[c] == "" {
		return text
	}
	return colorEscape + colorCodes[c] + "m" + text + colorReset
}
