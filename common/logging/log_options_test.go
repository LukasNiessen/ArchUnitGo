package logging_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/LukasNiessen/ArchUnitGo/common/logging"
)

func TestNilLogOptionsMeansTheDefaults(t *testing.T) {
	var options *logging.Options

	// Every knob's default is its zero value, so a nil bag and the zero bag describe the same silence.
	if got := options.WithDefaults(); got != (logging.Options{}) {
		t.Errorf("(*Options)(nil).WithDefaults() = %+v, want the zero options", got)
	}
	// The defaults spelled out, because they are the promise the issue is about: nowhere to write, no file,
	// no stamp, appending, and the quiet level.
	defaults := options.WithDefaults()
	if defaults.Writer != nil {
		t.Errorf("Writer defaults to %v, want nil: a library logs nowhere until a writer is injected", defaults.Writer)
	}
	if defaults.File != "" {
		t.Errorf("File defaults to %q, want no log file", defaults.File)
	}
	if defaults.Level != logging.LevelInfo {
		t.Errorf("Level defaults to %v, want LevelInfo", defaults.Level)
	}
	if !defaults.Timestamp.IsZero() {
		t.Errorf("Timestamp defaults to %v, want the zero time: the library reads no clock", defaults.Timestamp)
	}
	if defaults.Overwrite {
		t.Error("Overwrite defaults to true; a log file is opened once per check, so appending is the default")
	}
	if got := options.Path(); got != "" {
		t.Errorf("(*Options)(nil).Path() = %q, want no path at all", got)
	}
}

func TestLogOptionsWithDefaultsIsACopyThatKeepsTheCallersWriter(t *testing.T) {
	destination := &bytes.Buffer{}
	options := &logging.Options{
		Writer:    destination,
		Level:     logging.LevelDebug,
		File:      "build/archunit.log",
		Timestamp: time.Date(2026, time.August, 15, 9, 30, 0, 0, time.UTC),
		Overwrite: true,
	}

	resolved := options.WithDefaults()

	// Every field has to survive the resolution, because NewLogger reads the resolved copy and nothing else.
	if resolved != *options {
		t.Errorf("WithDefaults() = %+v, want the caller's own options %+v", resolved, *options)
	}
	// The writer is copied as the interface value it is, which is the point of the whole design: the log is
	// written to the caller's own destination and not to a copy of it.
	if resolved.Writer != options.Writer {
		t.Error("the resolved writer is not the caller's own")
	}
}

func TestLogOptionsPathStampsTheFilenameWithTheInstantItWasGiven(t *testing.T) {
	stamp := time.Date(2026, time.August, 15, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		file      string
		timestamp time.Time
		want      string
	}{
		{
			name:      "the stamp goes before the extension, so the file is still a .log",
			file:      "build/archunit.log",
			timestamp: stamp,
			want:      "build/archunit-2026-08-15T09-30-00.log",
		},
		{
			name:      "a path with no extension gets the stamp at the end",
			file:      "build/archunit",
			timestamp: stamp,
			want:      "build/archunit-2026-08-15T09-30-00",
		},
		{
			name:      "no stamp leaves the filename as it was written",
			file:      "build/archunit.log",
			timestamp: time.Time{},
			want:      "build/archunit.log",
		},
		{
			name:      "no file is no path, however the bag was stamped",
			file:      "",
			timestamp: stamp,
			want:      "",
		},
		{
			name:      "only the last extension is the extension",
			file:      "build/archunit.test.log",
			timestamp: stamp,
			want:      "build/archunit.test-2026-08-15T09-30-00.log",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := &logging.Options{File: test.file, Timestamp: test.timestamp}

			if got := options.Path(); got != test.want {
				t.Errorf("Path() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLogOptionsPathIsLegalOnEveryPlatformTheLibraryBuildsFor(t *testing.T) {
	// The reason the stamp is not time.RFC3339: a colon cannot be in a filename on Windows, so a log file
	// named after the standard would be a log file half the supported platforms cannot create.
	options := &logging.Options{
		File:      "build/archunit.log",
		Timestamp: time.Date(2026, time.August, 15, 9, 30, 0, 0, time.FixedZone("CEST", 2*60*60)),
	}

	path := options.Path()

	for _, illegal := range []string{":", "*", "?", `"`, "<", ">", "|"} {
		if bytes.Contains([]byte(path), []byte(illegal)) {
			t.Errorf("Path() = %q, which holds %q and cannot be a filename on Windows", path, illegal)
		}
	}
	// The offset is left off rather than spelled with a hyphen, so the stamp is the local wall clock of the
	// time the caller passed. A run that wants an unambiguous one passes a UTC time.
	if path != "build/archunit-2026-08-15T09-30-00.log" {
		t.Errorf("Path() = %q, want the wall clock of the time it was given", path)
	}
}
