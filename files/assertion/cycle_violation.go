// Package assertion is the files module's half of the ASSERT stage: it judges a projected file
// structure and reports one violation per place the code disagrees with a rule about files.
//
// It holds one violation type per rule family in the module — CycleViolation is the first — and one
// `gather <thing> violations` function per predicate, which is the only way a violation of that family
// is ever made. Both halves are data: a violation says which files disagreed with the rule and not a
// word about it, because message construction belongs to the testing layer, where one place controls
// phrasing, numbering and color.
//
// The package is pure, like every assertion package in the library: no filesystem, no clock, no
// globals, and nothing in it knows Go. It takes the projected structure common/projection produced and
// hands back a []assertion.Violation, so a rule's judgement can be tested against a hand-built
// projection before any project is extracted at all.
package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
)

// KindFileCycle is the kind of CycleViolation: a circular dependency between files.
//
// It names the vocabulary as well as the failure, the way `file-dependency` does, because a rule about
// cycles exists in every vocabulary the library grows — slices, layers, packages — and each of them
// reports its own type. The kind is what the testing layer picks a phrasing by, so two families sharing
// one name would be two shapes of data under one key.
const KindFileCycle kernel.ViolationKind = "file-cycle"

// CycleViolation says that some of the files a rule selected depend on each other in a circle, so that
// none of them can be read, tested or moved without the others.
//
// It is what `project files, ..., should, have no cycles` reports, one per cycle, and it carries the
// cycle itself rather than a sentence about it: the chain of dependencies in the order they close, each
// still holding the raw edges it was projected from. That is what lets a report print the cycle as the
// readable path `internal/api/handler.go -> internal/db/conn.go -> internal/api/handler.go` and still
// name the import that made each step.
type CycleViolation struct {
	// Cycle is the circular chain of dependencies between the rule's files: at least two of them, each
	// visited once, the last depending on the first. It is a cycles.Circuit, so Labels are the files in
	// cycle order and Edges are the dependencies along it.
	Cycle cycles.Circuit
}

// NewCycleViolation records that the files of this cycle depend on each other in a circle.
//
// The circuit is immutable and reads through its own accessors, which copy, so the violation shares
// nothing with the projection it was found in.
func NewCycleViolation(cycle cycles.Circuit) CycleViolation {
	return CycleViolation{Cycle: cycle}
}

// Kind is KindFileCycle.
func (CycleViolation) Kind() kernel.ViolationKind {
	return KindFileCycle
}

// Files are the identifiers of the files the cycle runs through, in cycle order and each exactly once.
// The file it closes onto is the first one and is not repeated at the end.
//
// They are the data a report names the offenders with, and the caller's own copy.
func (v CycleViolation) Files() []string {
	return v.Cycle.Labels()
}

// String renders the violation as the readable path the cycle is — `internal/api/handler.go ->
// internal/db/conn.go -> internal/api/handler.go`, the closing file included — for a log line or a test
// failure.
//
// The user-facing message is still the testing layer's to build, from Files and from the edges of the
// cycle; this is the same latitude every other type in the library takes with String.
func (v CycleViolation) String() string {
	return v.Cycle.String()
}
