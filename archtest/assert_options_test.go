package archtest_test

import (
	"reflect"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
)

func TestANilAssertOptionsIsTheDefaults(t *testing.T) {
	// The contract every options bag in the library keeps: a nil bag, the zero bag and an explicitly empty
	// one all describe the same assertion, so that AssertPasses(t, rule, nil) is the ordinary call.
	var absent *archtest.AssertOptions

	resolved := absent.WithDefaults()

	if want := (&archtest.AssertOptions{}).WithDefaults(); !reflect.DeepEqual(resolved, want) {
		t.Errorf("a nil bag resolves to %+v, want the zero bag's %+v", resolved, want)
	}
	if resolved.Check.AllowEmptyTests {
		t.Error("the default assertion allows an empty rule, want the strict check every terminal does")
	}
	if resolved.Message.MaxViolations != 0 || resolved.Message.Palette != (archtest.Palette{}) {
		t.Errorf("the default report is %+v, want plain text with every violation listed", resolved.Message)
	}
}

func TestBothHalvesOfTheBagAreAnsweredWithTheCallersOwnKnobs(t *testing.T) {
	// The bag spans the two halves of what AssertPasses does — it runs the rule and it writes the report — and
	// a knob of either half has to survive the resolution or it would be a knob that silently does nothing.
	options := &archtest.AssertOptions{
		Check:   fluentapi.CheckOptions{AllowEmptyTests: true, IncludeTestFiles: true},
		Message: archtest.MessageOptions{Palette: archtest.DefaultPalette(), MaxViolations: 3},
	}

	resolved := options.WithDefaults()

	if !resolved.Check.AllowEmptyTests || !resolved.Check.IncludeTestFiles {
		t.Errorf("the resolved check options are %+v, want the caller's own", resolved.Check)
	}
	if resolved.Message.MaxViolations != 3 || resolved.Message.Palette != archtest.DefaultPalette() {
		t.Errorf("the resolved message options are %+v, want the caller's own", resolved.Message)
	}
}

func TestTheResolvedCheckOptionsShareNoSliceWithTheCallersOwn(t *testing.T) {
	// The reason the inner bags resolve through their own WithDefaults rather than being copied field by
	// field: a struct copy shares a slice's backing array, so an assertion that resolved by hand could reach
	// into the options a suite keeps and into every other assertion made with them.
	options := &archtest.AssertOptions{Check: fluentapi.CheckOptions{BuildTags: []string{"integration"}}}

	resolved := options.WithDefaults()
	resolved.Check.BuildTags[0] = "unit"

	if options.Check.BuildTags[0] != "integration" {
		t.Errorf("the caller's own build tags read %v, want them untouched by the resolved copy", options.Check.BuildTags)
	}
}
