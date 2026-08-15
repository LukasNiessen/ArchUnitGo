package rendering_test

import (
	"testing"
	"time"

	"github.com/LukasNiessen/ArchUnitGo/metrics/rendering"
)

func TestReportOptionsWithDefaultsHandlesTheNilBag(t *testing.T) {
	// The whole contract of an options bag in this library: a nil one is the defaults, and every default is a zero
	// value. A caller who wants a plain page passes nothing.
	var missing *rendering.ReportOptions

	if resolved := missing.WithDefaults(); resolved != (rendering.ReportOptions{}) {
		t.Errorf("the nil bag resolves to %+v, want the zero value", resolved)
	}
	given := rendering.ReportOptions{Title: "the numbers of this project", Style: "h1 { color: red; }"}
	if resolved := (&given).WithDefaults(); resolved != given {
		t.Errorf("the bag resolves to %+v, want %+v", resolved, given)
	}
}

func TestReportOptionsHeadlineFallsBackToTheLibrarysOwn(t *testing.T) {
	var missing *rendering.ReportOptions
	untitled := &rendering.ReportOptions{}
	titled := &rendering.ReportOptions{Title: "the numbers of this project"}

	for name, options := range map[string]*rendering.ReportOptions{"nil bag": missing, "empty title": untitled} {
		if headline := options.Headline(); headline != "metrics report" {
			t.Errorf("the report with a %s is headlined %q, want the library's own", name, headline)
		}
	}
	if headline := titled.Headline(); headline != "the numbers of this project" {
		t.Errorf("the report is headlined %q, want the caller's own title", headline)
	}
}

func TestReportOptionsStampsAPageOnlyWhenTheCallerBroughtATime(t *testing.T) {
	// The zero time means no stamp, which is how this library renders a page without reading a clock: a report
	// that stamped itself would render different bytes on every run.
	var missing *rendering.ReportOptions
	taken := &rendering.ReportOptions{Timestamp: time.Date(2026, time.August, 15, 9, 30, 0, 0, time.UTC)}

	if stamp := missing.Stamp(); stamp != "" {
		t.Errorf("the nil bag stamps the page %q, want no stamp", stamp)
	}
	if stamp := (&rendering.ReportOptions{}).Stamp(); stamp != "" {
		t.Errorf("the bag with the zero time stamps the page %q, want no stamp", stamp)
	}
	if stamp := taken.Stamp(); stamp != "2026-08-15T09:30:00Z" {
		t.Errorf("the report is stamped %q, want RFC 3339", stamp)
	}
}

func TestReportOptionsStampKeepsTheOffsetTheCallerMeasuredIn(t *testing.T) {
	// A stamp is printed in the location the caller's own time.Time carries, so a report taken in a build's
	// timezone says which offset that was rather than being silently moved to UTC.
	berlin := time.FixedZone("CEST", 2*60*60)
	taken := &rendering.ReportOptions{Timestamp: time.Date(2026, time.August, 15, 11, 30, 0, 0, berlin)}

	if stamp := taken.Stamp(); stamp != "2026-08-15T11:30:00+02:00" {
		t.Errorf("the report is stamped %q, want the caller's own offset", stamp)
	}
}
