package archtest_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
)

func TestANilMessageOptionsIsTheDefaults(t *testing.T) {
	// The contract every options bag in the library keeps, held to here as well: a nil bag, the zero bag
	// and an explicitly empty one describe the same report — plain text, every violation listed.
	var absent *archtest.MessageOptions

	if defaults := absent.WithDefaults(); defaults != (archtest.MessageOptions{}) {
		t.Errorf("a nil options bag resolves to %+v, want the zero one", defaults)
	}
	if defaults := (&archtest.MessageOptions{}).WithDefaults(); defaults != (archtest.MessageOptions{}) {
		t.Errorf("the zero options bag resolves to %+v, want itself", defaults)
	}
	if defaults := absent.WithDefaults(); defaults.Palette != (archtest.Palette{}) {
		t.Errorf("the defaults paint with %+v, want the plain-text palette", defaults.Palette)
	}
	if defaults := absent.WithDefaults(); defaults.MaxViolations != 0 {
		t.Errorf("the defaults list %d violations, want 0, which means every one", defaults.MaxViolations)
	}
}

func TestWithDefaultsAnswersWithTheCallersOwnKnobs(t *testing.T) {
	options := &archtest.MessageOptions{Palette: archtest.DefaultPalette(), MaxViolations: 3}

	resolved := options.WithDefaults()

	if resolved.MaxViolations != 3 {
		t.Errorf("the resolved options list %d violations, want the 3 that were asked for", resolved.MaxViolations)
	}
	if resolved.Palette != archtest.DefaultPalette() {
		t.Errorf("the resolved options paint with %+v, want the palette that was asked for", resolved.Palette)
	}
}

func TestTheResolvedOptionsAreACopyOfTheCallersOwn(t *testing.T) {
	// A factory holds the options it was built with, so a caller reusing their bag for a second report must
	// not find the first report's factory changed underneath them.
	options := &archtest.MessageOptions{MaxViolations: 3}

	resolved := options.WithDefaults()
	resolved.MaxViolations = 99

	if resolved.MaxViolations != 99 {
		t.Errorf("the resolved options list %d violations, want the 99 they were changed to", resolved.MaxViolations)
	}
	if options.MaxViolations != 3 {
		t.Errorf("the caller's own options now list %d violations, want the 3 they set", options.MaxViolations)
	}
}
