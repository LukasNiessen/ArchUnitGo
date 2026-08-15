package calculation

import "fmt"

// Measurement is one number a metric read off one subject: what was measured, what it was measured about,
// and the answer.
//
// It is what a threshold predicate is judged over and what a report of the numbers is printed from, and it
// says what it measured rather than leaving that to the caller's memory: a measurement that has been
// collected into a list, sorted or handed to a report is still self-describing.
//
// A Measurement is plain data, and the zero value is the absence of one.
type Measurement struct {
	// Metric is what was measured, spelled as the family spells it — `lines of code`, `method count`. It
	// is CountMetric.Name of the metric that produced this measurement.
	Metric string
	// Subject is what the number is about: a file identifier — `internal/api/handler.go` — for a metric
	// about a file, and a class identifier — `internal/api.Handler` — for a metric about a class. It is
	// the string a report names the offender by, and it is never a host path.
	Subject string
	// Value is the number itself. A count is never negative, and 0 is a perfectly ordinary answer: a file
	// with no imports has none.
	Value int
}

// String renders the measurement for logs and test failures, as `internal/api/handler.go: lines of code =
// 120`. User-facing violation messages are built in the testing layer, not here.
func (m Measurement) String() string {
	return fmt.Sprintf("%s: %s = %d", m.Subject, m.Metric, m.Value)
}
