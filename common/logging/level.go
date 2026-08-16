package logging

import "strconv"

// Level is how much of what a check has to say reaches the log: the level a record is written at, and
// the threshold an Options bag holds one of. A record is written when its own level is at or above the
// threshold, so the threshold names the quietest thing worth seeing.
//
// There are exactly four, they are the four every logging library in every language has, and the
// vocabulary each of them carries is fixed:
//
//	debug   log progress    — the steps a check took, and how much of the project each came to
//	info    start check     — the rule, as the sentence the user typed
//	info    end check       — the same rule, and how many violations it reported
//	info    log metric      — one number a metrics rule measured
//	warn    log violation   — one place the code disagreed with the rule
//	error   end check       — a check that could not reach the point of judging anything
//
// The zero value is LevelInfo, because an options bag's every default has to be its zero value: a log
// that was asked for with a destination and nothing else holds the rule, its outcome, its numbers and
// its violations, and leaves out only the step-by-step. LevelDebug is therefore the one level below
// zero rather than the one above it.
//
// A threshold above LevelError silences everything and is a legitimate way to say so, which is why
// comparison is numeric rather than a switch over the four.
type Level int8

const (
	// LevelDebug is the step-by-step: one `log progress` record per stage of a check, saying what it
	// resolved and how much of the project that came to. It is the level to ask for when a rule reports
	// something surprising, because the counts are where a stale pattern becomes visible.
	LevelDebug Level = iota - 1
	// LevelInfo is what a check says about itself when it is working: `start check`, `end check` and the
	// numbers a metrics rule measured. It is the zero value and so the default threshold.
	LevelInfo
	// LevelWarn is a violation: one record per place the code disagreed with the rule. It is above info
	// because a violation is the answer a check was run for, and a threshold of warn is the log of a
	// suite that should be reporting nothing at all.
	LevelWarn
	// LevelError is a check that could not reach the point of judging anything — a project that will not
	// load, a pattern that will not compile — reported as the `end check` record of that check. The
	// failure itself still travels as the error Check returns; a log line is never how this library
	// reports something.
	LevelError
)

// String is the level as a log line spells it: `debug`, `info`, `warn`, `error`.
//
// The four words are lower case and unabbreviated, because a log file is grepped: `grep '^warn' ` is
// every violation of a run, and a level spelled `WARN` in one library and `warning` in the next is the
// reason that command has to be looked up every time.
//
// A level that is none of the four — a threshold a caller invented to silence the log — renders as
// `level(9)`. It is not clamped to the nearest of the four: the four are the only levels this library
// writes a record at, so a number showing up here at all is worth seeing as the number it was.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "level(" + strconv.Itoa(int(l)) + ")"
	}
}
