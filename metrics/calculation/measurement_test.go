package calculation_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

func TestMeasurementRendersWhatItMeasuredAndWhatItFound(t *testing.T) {
	// A measurement says what it is about, so one that has been collected, sorted or handed to a report is
	// still readable on its own.
	tests := []struct {
		name        string
		measurement calculation.Measurement
		want        string
	}{
		{
			name:        "a metric about a file",
			measurement: calculation.Measurement{Metric: "lines of code", Subject: "internal/api/handler.go", Value: 120},
			want:        "internal/api/handler.go: lines of code = 120",
		},
		{
			name:        "a metric about a class",
			measurement: calculation.Measurement{Metric: "method count", Subject: "internal/api.Handler", Value: 7},
			want:        "internal/api.Handler: method count = 7",
		},
		{
			name:        "zero is an answer like any other",
			measurement: calculation.Measurement{Metric: "imports", Subject: "main.go", Value: 0},
			want:        "main.go: imports = 0",
		},
		{
			// The reason the value is a float64: half the family is a ratio, and a count is the whole-numbered
			// special case above rather than the other way round.
			name:        "a metric about a package",
			measurement: calculation.Measurement{Metric: "abstractness", Subject: "internal/api", Value: 0.5},
			want:        "internal/api: abstractness = 0.5",
		},
		{
			// A ratio is printed with as many digits as it takes to say exactly which float64 it is, so a
			// reader explaining a test failure is never shown a rounded number.
			name:        "a ratio that does not divide out",
			measurement: calculation.Measurement{Metric: "instability", Subject: "internal/db", Value: 1.0 / 3.0},
			want:        "internal/db: instability = 0.3333333333333333",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.measurement.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}
