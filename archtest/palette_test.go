package archtest_test

import (
	"reflect"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
)

func TestTheZeroPaletteIsPlainText(t *testing.T) {
	// Every role at once, by walking the fields rather than listing them, so that a role added later is
	// held to the same promise without anybody editing this test: a nil *MessageOptions is the zero
	// palette, and a report of it has to be printable into a CI log.
	for role, color := range paletteRoles(t, archtest.Palette{}) {
		if color != archtest.ColorNone {
			t.Errorf("the zero palette paints %s in %s, want %s", role, color, archtest.ColorNone)
		}
		if painted := color.Paint("boom"); painted != "boom" {
			t.Errorf("the zero palette paints %s as %q, want the text unchanged", role, painted)
		}
	}
}

func TestTheDefaultPaletteGivesEveryPartOfAReportItsOwnColor(t *testing.T) {
	// The other half of the same sweep: a caller who asks for color and does not want to choose must not
	// get a role that was forgotten, because a piece of a message painted in nothing while its neighbors
	// are painted reads as the report having broken off.
	for role, color := range paletteRoles(t, archtest.DefaultPalette()) {
		if !color.Valid() {
			t.Errorf("the default palette paints %s in the undeclared color %d", role, color)
		}
		if color == archtest.ColorNone {
			t.Errorf("the default palette leaves %s unpainted, want a color for every role", role)
		}
	}
}

func TestTheDefaultPaletteIsAFreshValueEveryTime(t *testing.T) {
	// It is a function and not a package-level variable, so that one caller cannot change the library's
	// idea of a default report underneath another one.
	changed := archtest.DefaultPalette()
	changed.Subject = archtest.ColorMagenta

	if changed.Subject != archtest.ColorMagenta {
		t.Fatalf("the changed palette paints the subject in %s, want the color it was set to", changed.Subject)
	}
	if again := archtest.DefaultPalette(); again.Subject == archtest.ColorMagenta {
		t.Error("changing a palette changed DefaultPalette, want a fresh value each time")
	}
}

func TestAPartialPaletteIsAPartiallyColoredReport(t *testing.T) {
	// A caller filling in the one role they care about gets that role colored and the rest as plain text,
	// rather than a palette that has to be built whole.
	partial := archtest.Palette{Subject: archtest.ColorRed}

	if painted := partial.Subject.Paint("conn.go"); painted != "\x1b[31mconn.go\x1b[0m" {
		t.Errorf("the subject is painted %q, want the color that was asked for", painted)
	}
	if painted := partial.Requirement.Paint("should"); painted != "should" {
		t.Errorf("the requirement is painted %q, want it left plain", painted)
	}
}

// paletteRoles are the palette's fields by name, which is how a test says "every role" without keeping a
// list of them that a new role would silently fall out of.
func paletteRoles(t *testing.T, palette archtest.Palette) map[string]archtest.Color {
	t.Helper()

	fields := reflect.TypeOf(palette)
	roles := make(map[string]archtest.Color, fields.NumField())
	for index := range fields.NumField() {
		color, ok := reflect.ValueOf(palette).Field(index).Interface().(archtest.Color)
		if !ok {
			t.Fatalf("the palette's %s field is not a Color, want every role to be one", fields.Field(index).Name)
		}
		roles[fields.Field(index).Name] = color
	}
	if len(roles) == 0 {
		t.Fatal("the palette has no roles, want the sweep to have found some")
	}
	return roles
}
