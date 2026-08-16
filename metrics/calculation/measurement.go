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
	// Metric is what was measured, spelled as the family spells it — `lines of code`, `abstractness`. It
	// is Metric.Name of the metric that produced this measurement.
	Metric string
	// Subject is what the number is about: a file identifier — `internal/api/handler.go` — for a metric
	// about a file, a class identifier — `internal/api.Handler` — for a metric about a class, and a folder
	// identifier — `internal/api` — for a metric about a package. It is the string a report names the
	// offender by, and it is never a host path.
	Subject string
	// Value is the number itself. A count is never negative and always whole, and 0 is a perfectly
	// ordinary answer: a file with no imports has none.
	//
	// It is a float64 rather than an int because half the family is a ratio — abstractness, instability,
	// the distance from the main sequence — and a count is a ratio's whole-numbered special case, while a
	// ratio rounded into an int is a number nobody could act on. One measurement type is what lets a
	// report, a threshold and the fluent stage in between be written once for every metric this library
	// has.
	Value float64
}

// String renders the measurement for logs and test failures, as `internal/api/handler.go: lines of code =
// 120` and `internal/api: abstractness = 0.5`. User-facing violation messages are built in the testing
// layer, not here.
//
// The number is printed with as many digits as it takes to say exactly which float64 it is, and no more, so
// a count reads as `120` rather than `120.000000` and a ratio is never quietly rounded into a different
// number in a test failure a reader is trying to explain.
func (m Measurement) String() string {
	return fmt.Sprintf("%s: %s = %g", m.Subject, m.Metric, m.Value)
}
