package rendering

import "time"

// defaultHeadline is what a report with no title of its own is called, in the two places a reader sees a page's
// name: its heading and the tab it is opened in. It is stated once, for the reason graph/rendering's headline
// fallback is: two spellings of the same fallback is a page whose heading and tab disagree.
const defaultHeadline = "metrics report"

// ReportOptions is what an exported metrics report says about itself: what it is called, when it was taken, and
// the stylesheet it is painted with beyond the library's own.
//
// It is an options bag, so a *ReportOptions is always allowed to be nil and every default is a zero value: the
// library's own headline, no timestamp on the page, and no styling but the one built into it. Read one through
// WithDefaults, Headline and Stamp rather than reaching for a field, so that the nil case is handled once —
// every method takes a pointer receiver for the reason fluentapi.CheckOptions' do, because a nil-safe read has
// to be one and a type with both kinds of receiver is a finding.
//
// The three knobs are what a report needs beyond its numbers, and none of them changes what it says about the
// project: a page rendered with a title and a page rendered without one hold the same measurements.
type ReportOptions struct {
	// Title is what the report is called, and the empty string means the library's own headline.
	//
	// A report written off a rule is titled with the rule's own sentence — `metrics, path without filename
	// matches "internal/**", count` — so that a page found in a build's output says which rule produced it.
	// A caller with something better to call it says so here.
	Title string
	// Timestamp is when the report was taken, as the page states it, and the zero time — the default — means
	// the page carries none.
	//
	// The time is a field rather than a clock this library reads, which is the one thing worth knowing about
	// it: a page that stamped itself would render differently on every run, so a report committed beside the
	// code would show up in every diff and a test could not assert on a page at all. A caller who wants the
	// stamp passes time.Now() and owns that decision.
	Timestamp time.Time
	// Style is a stylesheet of the caller's own, added to the page after the library's own.
	//
	// After, and not instead of, because CSS resolves a tie in favor of the later rule: a caller who names an
	// element the library also styles wins, and everything they do not name is still styled rather than
	// falling back to a browser's defaults. It is written into the page as it is — a stylesheet that has been
	// escaped is not one any more — so it is the one thing on a page that is not, and the reason that is safe
	// is that it is the caller's own text rather than anything read out of the project.
	Style string
}

// WithDefaults returns the options a report should actually be rendered with: a copy of the receiver, or the
// defaults when the receiver is nil. RenderHTML starts with this, so the "nil means defaults" contract lives in
// one place instead of being re-derived as a nil check per field.
//
// Nothing here is a slice or a map, so the copy is the whole of it: a rendered page cannot reach into the bag it
// was rendered from.
func (o *ReportOptions) WithDefaults() ReportOptions {
	if o == nil {
		return ReportOptions{}
	}
	return *o
}

// Headline is what the report is called: the title the caller gave it, or the library's own for a report that
// was not given one.
func (o *ReportOptions) Headline() string {
	if resolved := o.WithDefaults(); resolved.Title != "" {
		return resolved.Title
	}
	return defaultHeadline
}

// Stamp is when the report says it was taken, as RFC 3339 — `2026-08-15T09:30:00Z` — and the empty string for a
// report with no timestamp, which is the default.
//
// RFC 3339 rather than a friendlier phrasing because a stamp on a report is read by a person and sorted by a
// script, and it is the one format that both can do. The instant is printed in the location the caller's own
// time.Time carries, so a stamp taken in local time reads as local time and says which offset that was.
func (o *ReportOptions) Stamp() string {
	resolved := o.WithDefaults()
	if resolved.Timestamp.IsZero() {
		return ""
	}
	return resolved.Timestamp.Format(time.RFC3339)
}
