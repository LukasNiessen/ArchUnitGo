package rendering_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
)

func TestReportDataHeadingsAreSortedSoAPageRendersTheSameTwice(t *testing.T) {
	// A map hands its keys back in a different order on every iteration, so the sorted order is what makes a
	// report a document two commits apart can be diffed.
	data := rendering.ReportData{
		"statements":    {measurement("statements", "main.go", 3)},
		"lines of code": {measurement("lines of code", "main.go", 3)},
		"imports":       {measurement("imports", "main.go", 1)},
	}

	if headings := data.Headings(); !slices.Equal(headings, []string{"imports", "lines of code", "statements"}) {
		t.Errorf("the headings are %v, want them sorted", headings)
	}
	if headings := (rendering.ReportData{}).Headings(); len(headings) != 0 {
		t.Errorf("an empty report holds the headings %v, want none", headings)
	}
}

func TestReportDataCountsTheMeasurementsOfEveryGroup(t *testing.T) {
	data := rendering.ReportData{
		"lines of code": {measurement("lines of code", "main.go", 3), measurement("lines of code", "db.go", 9)},
		"imports":       {measurement("imports", "main.go", 1)},
	}

	if measured := data.Measured(); measured != 3 {
		t.Errorf("the report holds %d measurements, want 3", measured)
	}
}

func TestReportDataIsEmptyWhenNoGroupHoldsANumber(t *testing.T) {
	// The two shapes of nothing: no group at all, and groups whose population a scope did not select. A page of
	// headings with no number under any of them is as empty as a page with no heading, and the terminals guard
	// both with this one question.
	for name, data := range map[string]rendering.ReportData{
		"no group":     {},
		"nil map":      nil,
		"empty groups": {"lines of code": nil, "imports": {}},
	} {
		if !data.Empty() {
			t.Errorf("the report with %s is not empty, want it empty", name)
		}
	}
	measured := rendering.ReportData{"imports": {measurement("imports", "main.go", 0)}}
	if measured.Empty() {
		t.Error("a report holding a measurement of 0 is empty, want 0 to count as a measurement")
	}
}

// measurement is one number a metric read, as the tests of this package hand it to a report.
func measurement(metric, subject string, value float64) calculation.Measurement {
	return calculation.Measurement{Metric: metric, Subject: subject, Value: value}
}
