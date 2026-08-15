package logging_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/logging"
)

func TestLevelRendersAsTheWordALogLineIsGreppedBy(t *testing.T) {
	tests := []struct {
		name  string
		level logging.Level
		want  string
	}{
		{name: "the step-by-step", level: logging.LevelDebug, want: "debug"},
		{name: "what a working check says about itself", level: logging.LevelInfo, want: "info"},
		{name: "a violation", level: logging.LevelWarn, want: "warn"},
		{name: "a check that could not be run", level: logging.LevelError, want: "error"},
		{name: "the zero value is info", level: logging.Level(0), want: "info"},
		{name: "a threshold above error, which is how a caller silences the log", level: logging.Level(9), want: "level(9)"},
		{name: "and one below debug, which is not clamped either", level: logging.Level(-9), want: "level(-9)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.level.String(); got != test.want {
				t.Errorf("Level(%d).String() = %q, want %q", test.level, got, test.want)
			}
		})
	}
}

func TestTheFourLevelsAreOrderedFromDebugToError(t *testing.T) {
	// The ordering is the whole of how a threshold works — a record is written when its own level is at or
	// above the threshold — so it is asserted rather than left to the iota it is spelled with.
	ordered := []logging.Level{logging.LevelDebug, logging.LevelInfo, logging.LevelWarn, logging.LevelError}
	for index := 1; index < len(ordered); index++ {
		if ordered[index-1] >= ordered[index] {
			t.Errorf("%v is not below %v; the levels have to increase in severity", ordered[index-1], ordered[index])
		}
	}

	// The zero value is info and not debug, because every default in this library is a zero value and a log
	// that was asked for with a destination and nothing else must not be the step-by-step.
	var zero logging.Level
	if zero != logging.LevelInfo {
		t.Errorf("the zero Level is %v, want LevelInfo, which is what makes the options bag's default the quiet one", zero)
	}
	if logging.LevelDebug >= 0 {
		t.Errorf("LevelDebug is %d, want the one level below zero", logging.LevelDebug)
	}
}
