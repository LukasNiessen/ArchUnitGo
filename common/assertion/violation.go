// Package assertion holds the result vocabulary of the ASSERT stage: what a rule reports when the
// code disagrees with it.
//
// There is one base type, Violation, and one violation the kernel itself defines,
// EmptyTestViolation. Every rule family adds its own types in its own <module>/assertion package
// and gathers them with a `gather <thing> violations` function; this package is what they all have
// in common.
//
// Two conventions matter more than the code:
//
//   - A violation is data, never prose. Message construction lives in the testing layer, so that
//     one place controls phrasing, numbering and color.
//   - A rule's result is a []Violation, and an empty one means the rule passed. There is no
//     separate boolean result to keep in step with the list.
//
// The package is pure: no filesystem, no clock, no globals. That is what lets a rule's judgement be
// tested against a hand-built graph.
package assertion

// ViolationKind identifies a family of violations — "empty-test", "file-dependency", "cycle". It is
// a stable, machine-readable name, spelled the same way in every ArchUnit port: lower case, words
// separated by hyphens, no language-specific vocabulary.
//
// Each rule family declares its own as a constant beside the violation type that returns it. The
// testing layer keys its phrasing off these names, so renaming one is a breaking change.
type ViolationKind string

// KindEmptyTest is the kind of EmptyTestViolation, the one violation the kernel defines itself.
const KindEmptyTest ViolationKind = "empty-test"

// Violation is one disagreement between the code and a rule: the atom of every rule's result.
//
// A violation carries the thing that disagreed — the offending edge, the node, the cycle, the
// measured value and the threshold it broke — and not a word about it. Turning that data into a
// sentence is the testing layer's job.
//
// A []Violation is a whole result: empty means the rule passed, and every entry is one place the
// code and the rule diverge. Nothing in the library returns a boolean pass alongside it.
//
// The interface deliberately has no unexported method, so it is open: a rule family in any module
// can implement it, and Kind is the entire contract. Everything else about a violation is known
// only to the code that made it and to the testing layer that phrases it.
type Violation interface {
	// Kind names the family this violation belongs to, so that a report can group and count
	// violations, and the testing layer can choose a phrasing, without asserting on a concrete
	// type first.
	Kind() ViolationKind
}
