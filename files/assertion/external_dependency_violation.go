package assertion

import (
	"slices"
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// KindFileExternalDependency is the kind of ExternalDependencyViolation: a file that depends on the external
// modules a rule named, or depends on none of them.
//
// It is a kind of its own rather than KindFileDependency with a flag, because the two carry different data
// under the same field names: the object of `depend on files` is a population of this project's files,
// resolved from the graph's self-edges and combined with AND, and the object of `depend on external modules`
// is a set of import paths that leave the project, combined with OR. The kind is what the testing layer picks
// a phrasing by, and those two need different sentences — "must not depend on" against a folder of ours reads
// nothing like it does against somebody else's module.
const KindFileExternalDependency kernel.ViolationKind = "file-external-dependency"

// ExternalDependencyViolation says that one of the files a rule selected disagrees with the rule about which
// of somebody else's modules it may depend on: it depends on the modules the object named where that was
// forbidden, or on none of them where that was required.
//
// It is what `project files, in folder "internal/domain/**", should not, depend on external modules, matching
// "*.*/**"` reports — one violation per offending file rather than one per offending import — and it carries
// the file, what the object described, the import paths actually found and the mood, not a sentence about
// them.
//
// It is DependencyViolation across the project's boundary, and the two are deliberately separate types. One
// violation per file is the same choice for the same reasons DependencyViolation gives; what differs is what
// the object was, and a report that could not tell a folder of this project from a third-party module would
// phrase both failures the same way.
type ExternalDependencyViolation struct {
	// File is the identifier of the file the rule does not hold for, as the graph spells it —
	// `internal/domain/order.go`. It is always one of the files the rule's scope selected.
	File string
	// Required is what the rule said about the modules this file may depend on: the object selectors, in the
	// order the user chained them and combined with OR, each already knowing the part of an identifier it
	// looks at. It is empty when the object was `depend on external modules` with nothing chained onto it,
	// which is every module the project depends on — the standard library among them.
	Required []matching.Filter
	// Modules are the import paths this file depends on that the object describes, sorted, spelled exactly as
	// the file imported them: `golang.org/x/tools/go/packages`, which names a package rather than the module
	// it came from.
	//
	// Under `should not` they are the offense itself, which is what a reader has to go and unpick. Under
	// `should` they are empty, because a file that depended on one of them would satisfy the rule — so the
	// absence is the offense, and an empty list is data rather than a gap.
	Modules []string
	// Mood is which way round the rule was written. Depending on the object's modules is what `should`
	// demands and what `should not` forbids, so without the mood a report could not tell one failure from the
	// other — and for this family the negated mood is the one nearly every rule is written in.
	Mood kernel.Mood
}

// NewExternalDependencyViolation records that this file, judged in this mood, disagrees with a rule about the
// external modules it may depend on: required is what the object described, and modules are the import paths
// it actually depends on — none of them, under the positive mood.
//
// It is the only way a violation of this family is made. Both slices are copied and the modules sorted, for
// the reasons NewDependencyViolation gives: a violation that has been reported must not change when the
// projection it was found in is walked on, and one built from a hand-written list has to read exactly like
// one built from a projection.
func NewExternalDependencyViolation(
	file string,
	required []matching.Filter,
	modules []string,
	mood kernel.Mood,
) ExternalDependencyViolation {
	found := slices.Clone(modules)
	slices.Sort(found)
	return ExternalDependencyViolation{
		File:     file,
		Required: slices.Clone(required),
		Modules:  found,
		Mood:     mood,
	}
}

// Kind is KindFileExternalDependency.
func (ExternalDependencyViolation) Kind() kernel.ViolationKind {
	return KindFileExternalDependency
}

// String renders the violation as the offending file, the rule it broke in the words the rule was written in,
// and what it was found to depend on — `internal/domain/order.go: should not, depend on external modules,
// path matches "*.*/**" -> github.com/gin-gonic/gin` — for a log line or a test failure.
//
// The object's selectors are joined with ` or ` where DependencyViolation joins its own with `, `, because
// that is the difference between the two families and a rendering that hid it would read as a requirement no
// module could ever meet. A file that depends on none of them renders as `-> nothing`, which is the whole of
// what a violation of the positive mood found; nothing here branches on the mood to decide that, since the
// modules are empty exactly when their absence is the offense.
//
// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. The user-facing message is still
// the testing layer's to build, from File, Required, Modules and Mood.
func (v ExternalDependencyViolation) String() string {
	stages := []string{v.File + ": " + v.Mood.String(), "depend on external modules"}
	if alternatives := v.alternatives(); alternatives != "" {
		stages = append(stages, alternatives)
	}
	return strings.Join(stages, ", ") + " -> " + v.found()
}

// alternatives renders the object's selectors as the one stage of the sentence they are, joined with ` or `,
// and the empty string when the rule named no module at all. It is the one place that join is spelled on this
// side of the library, as projection.SelectExternalModules is the one place the OR itself lives.
func (v ExternalDependencyViolation) alternatives() string {
	sources := make([]string, 0, len(v.Required))
	for _, required := range v.Required {
		sources = append(sources, required.String())
	}
	return strings.Join(sources, " or ")
}

// found renders the modules the rule was broken by, and `nothing` when the offense is that there are none. It
// is the one place the empty list is spelled, because a report of either mood reads it.
func (v ExternalDependencyViolation) found() string {
	if len(v.Modules) == 0 {
		return "nothing"
	}
	return strings.Join(v.Modules, ", ")
}
