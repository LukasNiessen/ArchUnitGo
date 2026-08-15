// Package calculation is the metrics module's numeric half: it says which number a metric is, and reads
// that number off every subject a rule selected.
//
// It is pure. A metric is a name plus one function of a single file or a single class — nothing here opens
// a file, parses Go or decides what a rule is about — so the eight count metrics can be tested against a
// hand-built extraction.FileInfo, and the arithmetic of a rule is separate from the reading that fed it.
//
// The eight are the counts this library can take of a project as it is written: `lines of code`,
// `statements`, `imports`, `functions`, `classes` and `interfaces` about one file, and `method count` and
// `field count` about one class. A Measurement is what any of them produces, and comparing one against a
// threshold is the assertion stage's business rather than this package's.
package calculation

import (
	"github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

// CountMetric is one of the eight numbers this library counts: what it is called, and how it is read off
// one subject.
//
// A metric is about a file or about a class, and which it is, is the field that is set rather than a flag
// beside them — so a metric that is about neither, which is the zero CountMetric, measures nothing at all
// instead of measuring the wrong population. Measure is where that choice is made, once, so nothing
// upstream of a metric has to know what kind it is.
//
// A CountMetric is immutable: get one from a factory below and read it through its methods.
type CountMetric struct {
	// name is what the metric is called in a report, spelled as the family spells it — `lines of code`.
	name string
	// file is how the metric reads one file, and nil for a metric about a class.
	file func(extraction.FileInfo) int
	// class is how the metric reads one class, and nil for a metric about a file.
	class func(extraction.ClassInfo) int
}

// LinesOfCode is how many of a file's lines carry code: comments and blank lines left out, a line with
// code and a trailing comment counted.
func LinesOfCode() CountMetric {
	return CountMetric{name: "lines of code", file: func(file extraction.FileInfo) int {
		return file.LinesOfCode
	}}
}

// Statements is how many statements a file's function bodies are made of, at every depth. It is the
// coarsest measure of how much a file does, as against how long it is.
func Statements() CountMetric {
	return CountMetric{name: "statements", file: func(file extraction.FileInfo) int {
		return file.StatementCount
	}}
}

// Imports is how many import specs a file holds, blank and dot imports included. It is how much of the
// rest of the world one file reaches for, counted without asking what it does with any of it.
func Imports() CountMetric {
	return CountMetric{name: "imports", file: func(file extraction.FileInfo) int {
		return file.ImportCount
	}}
}

// Functions is how many functions a file declares at package level. A method belongs to its type and is
// MethodCount's business, so the two never count the same declaration twice.
func Functions() CountMetric {
	return CountMetric{name: "functions", file: func(file extraction.FileInfo) int {
		return file.FunctionCount
	}}
}

// Classes is how many types a file declares — structs, interfaces and names given to other types alike.
// Go has no classes; the vocabulary is the family's, and it means exactly the population `for classes
// matching` selects.
func Classes() CountMetric {
	return CountMetric{name: "classes", file: func(file extraction.FileInfo) int {
		return len(file.Classes)
	}}
}

// Interfaces is how many of the types a file declares are interfaces. It is a count of the same population
// Classes counts, narrowed to one kind, which is why a rule about interfaces needs no second scope verb.
func Interfaces() CountMetric {
	return CountMetric{name: "interfaces", file: countInterfaces}
}

// MethodCount is how many methods one class has: an interface's own members, and for every other type the
// functions declared with it as their receiver.
//
// It is a metric about a class, so its subjects are the classes a rule selected rather than its files, and
// its measurements are reported per class identifier — `internal/api.Handler`.
func MethodCount() CountMetric {
	return CountMetric{name: "method count", class: func(class extraction.ClassInfo) int {
		return class.MethodCount
	}}
}

// FieldCount is how many fields one class declares, and 0 for a class that is not a struct. It is a metric
// about a class, like MethodCount.
func FieldCount() CountMetric {
	return CountMetric{name: "field count", class: func(class extraction.ClassInfo) int {
		return class.FieldCount
	}}
}

// Name is what this metric is called in a report, spelled as the family spells it — `lines of code`, `method
// count`. It is the word a violation quotes, and the zero CountMetric has none.
func (m CountMetric) Name() string {
	return m.name
}

// Measure reads this metric off every subject it is about: one Measurement per file for a metric about a
// file, one per class for a metric about a class, in the order the subjects arrived.
//
// Both populations are handed over and the metric takes the one it is about, which is what keeps the fluent
// layer from branching on the kind of metric a user chose. The zero CountMetric measures nothing, because a
// metric that reads neither population has no number to report about either.
//
// No subjects at all is an empty result rather than an error. Whether a rule that measured nothing is a
// failure is the empty-test guard's question, asked where the rule is judged.
func (m CountMetric) Measure(subjects projection.Subjects) []Measurement {
	if m.class != nil {
		measurements := make([]Measurement, 0, len(subjects.Classes))
		for _, class := range subjects.Classes {
			measurements = append(measurements, Measurement{
				Metric:  m.name,
				Subject: class.Identifier,
				Value:   m.class(class),
			})
		}
		return measurements
	}
	if m.file == nil {
		return nil
	}

	measurements := make([]Measurement, 0, len(subjects.Files))
	for _, file := range subjects.Files {
		measurements = append(measurements, Measurement{
			Metric:  m.name,
			Subject: file.Path,
			Value:   m.file(file),
		})
	}
	return measurements
}

// String renders the metric as its name, for logs and test failures.
func (m CountMetric) String() string {
	return m.name
}

// countInterfaces counts the interfaces among the types a file declares. It is a named function rather
// than a closure in Interfaces because it is the one count of the eight that is a loop instead of a field.
func countInterfaces(file extraction.FileInfo) int {
	count := 0
	for _, class := range file.Classes {
		if class.Interface {
			count++
		}
	}
	return count
}
