package archtest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
)

func TestEveryColorPaintsItsOwnCodeAndResetsAfterwards(t *testing.T) {
	// The escape sequences spelled out rather than derived, because this is the one place in the library
	// that emits them and a table that computed them would agree with any bug in the code it tests.
	for _, wanted := range []struct {
		color archtest.Color
		name  string
		text  string
	}{
		{archtest.ColorNone, "none", "boom"},
		{archtest.ColorRed, "red", "\x1b[31mboom\x1b[0m"},
		{archtest.ColorGreen, "green", "\x1b[32mboom\x1b[0m"},
		{archtest.ColorYellow, "yellow", "\x1b[33mboom\x1b[0m"},
		{archtest.ColorBlue, "blue", "\x1b[34mboom\x1b[0m"},
		{archtest.ColorMagenta, "magenta", "\x1b[35mboom\x1b[0m"},
		{archtest.ColorCyan, "cyan", "\x1b[36mboom\x1b[0m"},
		{archtest.ColorGray, "gray", "\x1b[90mboom\x1b[0m"},
	} {
		t.Run(wanted.name, func(t *testing.T) {
			if !wanted.color.Valid() {
				t.Errorf("%d is not a valid color, want every declared one to be", wanted.color)
			}
			if name := wanted.color.String(); name != wanted.name {
				t.Errorf("the color is named %q, want %q", name, wanted.name)
			}
			if painted := wanted.color.Paint("boom"); painted != wanted.text {
				t.Errorf("painting %q gives %q, want %q", "boom", painted, wanted.text)
			}
		})
	}
}

func TestColorNonePaintsNothingAtAll(t *testing.T) {
	// The zero value, and what every field of the zero Palette is: a plain-text report has to be the text
	// itself and not the text wrapped in an escape sequence that happens to select nothing.
	if painted := archtest.ColorNone.Paint("boom"); painted != "boom" {
		t.Errorf("ColorNone paints %q, want the text unchanged", painted)
	}
	if strings.Contains(archtest.ColorNone.Paint("boom"), "\x1b") {
		t.Error("ColorNone emits an escape sequence, want none")
	}
}

func TestPaintingNothingIsNothing(t *testing.T) {
	// An empty piece of a message — a violation with no selectors, a cycle with no files — must not turn
	// into a bare escape sequence, which would color the rest of somebody's test output.
	for _, color := range []archtest.Color{archtest.ColorNone, archtest.ColorRed, archtest.ColorGray} {
		if painted := color.Paint(""); painted != "" {
			t.Errorf("%s paints the empty string as %q, want it left empty", color, painted)
		}
	}
}

func TestAnUndeclaredColorPaintsNothingAndSaysSo(t *testing.T) {
	// An integer cast into the type, the way common/matching.MatchTarget answers "unknown" rather than
	// indexing past its own table. A report is not the place to find out about a bad constant.
	undeclared := archtest.Color(200)

	if undeclared.Valid() {
		t.Error("Color(200) reports itself valid, want not")
	}
	if name := undeclared.String(); name != "unknown" {
		t.Errorf("Color(200) is named %q, want %q", name, "unknown")
	}
	if painted := undeclared.Paint("boom"); painted != "boom" {
		t.Errorf("Color(200) paints %q, want the text unchanged", painted)
	}
}

func TestAColorPrintsItsNameRatherThanItsEscapeSequence(t *testing.T) {
	// A Color reaching a `%v` — in a test's own failure message, in a log line — has to say something a
	// reader can see. An escape sequence there would tell them nothing at all.
	printed := fmt.Sprintf("%v", archtest.ColorCyan)

	if printed != "cyan" {
		t.Errorf("a color prints as %q, want %q", printed, "cyan")
	}
	if strings.Contains(printed, "\x1b") {
		t.Errorf("a color prints as %q, want no escape sequence in it", printed)
	}
}
