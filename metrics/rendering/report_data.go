package rendering

import (
	"maps"
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
)

// ReportData is what a metrics report is written from: the numbers that were measured, grouped under the
// heading each group of them is listed under.
//
// The heading is the metric's own name — `lines of code`, `abstractness` — for every report this library
// writes off a rule, because a group of a report is a metric there and the eight counts of a scope are eight
// groups. It is a string rather than a calculation.Metric so that it does not have to be: a caller assembling
// a report of their own groups their measurements however they mean them to be compared — one group per
// folder, one per release, one per metric of their own — and a measurement already says which metric it came
// from, so a heading that is not one loses nothing.
//
// A report with no measurement in it is not an error here. What a report is written from is the caller's own
// data, and whether a rule that measured nothing is a failure is the empty-test guard's question, asked by the
// `export as html` terminals, which are the stages that resolved a scope and can therefore tell an empty
// project from a stale pattern.
type ReportData map[string][]calculation.Measurement

// Headings are the groups this report holds, in the sorted order they are rendered in.
//
// Sorted, and not the order a caller filled the map in, because a map has no order to keep: iterating one
// twice hands its keys back in two different orders, and a report that renders differently on every run is a
// report nobody can diff between two commits.
func (d ReportData) Headings() []string {
	return slices.Sorted(maps.Keys(d))
}

// Measured is how many measurements the whole report holds, across every group. It is what the page states
// above its groups, and what the question of an empty report is answered from.
func (d ReportData) Measured() int {
	total := 0
	for _, measurements := range d {
		total += len(measurements)
	}
	return total
}

// Empty reports whether this report holds no measurement at all — an empty map, and equally a map of groups
// that each turned out to have nothing in them, which is what a scope that selected no subject leaves behind.
//
// It is the question the `export as html` terminals guard with, for the reason
// graph/projection.Snapshot.Empty exists: a page of headings with no number under any of them looks exactly
// like a project with nothing to report.
func (d ReportData) Empty() bool {
	return d.Measured() == 0
}
