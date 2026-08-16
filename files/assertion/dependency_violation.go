package assertion

import (
	"slices"
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// KindFileDependency is the kind of DependencyViolation: a file that depends on the files a rule named, or
// depends on none of them.
//
// It names the vocabulary as well as the failure, the way KindFileCycle does, because every vocabulary the
// library grows has a rule about what may depend on what — a slice, a layer, a package — and each of them
// reports its own type over its own labels. The kind is what the testing layer picks a phrasing by, so two
// families sharing one name would be two shapes of data under one key.
const KindFileDependency kernel.ViolationKind = "file-dependency"

// DependencyViolation says that one of the files a rule selected disagrees with the rule about what it may
// depend on: it depends on the files the object named where that was forbidden, or on none of them where
// that was required.
//
// It is what `project files, in folder "internal/api/**", should not, depend on files, in folder
// "internal/db/**"` reports, one violation per offending file rather than one per offending import, and it
// carries the file, what the object described, the dependencies actually found and the mood — not a
// sentence about them.
//
// One violation per file is what makes the two moods one shape of data. Under `should not` the offense is
// the dependencies, and they are all here, so a reader sees every file to unpick in one line instead of
// the same source file repeated per import. Under `should` the offense is their absence, which no
// per-dependency violation could carry at all: there is no edge to point at, only a file and the object it
// was supposed to reach.
type DependencyViolation struct {
	// File is the identifier of the file the rule does not hold for, as the graph spells it —
	// `internal/api/handler.go`. It is always one of the files the rule's scope selected.
	File string
	// Required is what the rule said about the files this one may depend on: the object selectors, in the
	// order the user chained them and combined with AND, each already knowing the part of an identifier it
	// looks at. It is empty when the object was `depend on files` with nothing chained onto it, which is
	// every file of the project.
	Required []matching.Filter
	// Dependencies are the identifiers of the files this one depends on that the object describes, sorted.
	//
	// Under `should not` they are the offense itself, which is what a reader has to go and unpick. Under
	// `should` they are empty, because a file that depended on one of them would satisfy the rule — so the
	// absence is the offense, and an empty list is data rather than a gap.
	Dependencies []string
	// Mood is which way round the rule was written. Depending on the object's files is what `should`
	// demands and what `should not` forbids, so without the mood a report could not tell one failure from
	// the other.
	Mood kernel.Mood
}

// NewDependencyViolation records that this file, judged in this mood, disagrees with a rule about the files
// it may depend on: required is what the object described, and dependencies are the object's files it
// actually depends on — none of them, under the positive mood.
//
// It is the only way a violation of this family is made. Both slices are copied, for the reason
// assertion.NewEmptyTestViolation copies its selectors: a violation that has been reported must not change
// when the projection it was found in is walked on. The dependencies are sorted here rather than trusted to
// arrive that way, so that a violation built from a hand-written list reads exactly like one built from a
// projection.
func NewDependencyViolation(file string, required []matching.Filter, dependencies []string, mood kernel.Mood) DependencyViolation {
	found := slices.Clone(dependencies)
	slices.Sort(found)
	return DependencyViolation{
		File:         file,
		Required:     slices.Clone(required),
		Dependencies: found,
		Mood:         mood,
	}
}

// Kind is KindFileDependency.
func (DependencyViolation) Kind() kernel.ViolationKind {
	return KindFileDependency
}

// String renders the violation as the offending file, the rule it broke in the words the rule was written
// in, and what it was found to depend on — `internal/api/handler.go: should not, depend on files, path
// without filename matches "internal/db/**" -> internal/db/conn.go` — for a log line or a test failure.
//
// A file that depends on none of the object's files renders as `-> nothing`, which is the whole of what a
// violation of the positive mood found. Nothing here branches on the mood to decide that: the dependencies
// are empty exactly when their absence is the offense.
//
// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything. The user-facing message is still
// the testing layer's to build, from File, Required, Dependencies and Mood.
func (v DependencyViolation) String() string {
	stages := make([]string, 0, len(v.Required)+2)
	stages = append(stages, v.File+": "+v.Mood.String(), "depend on files")
	for _, required := range v.Required {
		stages = append(stages, required.String())
	}
	return strings.Join(stages, ", ") + " -> " + v.found()
}

// found renders the dependencies the rule was broken by, and `nothing` when the offense is that there are
// none. It is the one place the empty list is spelled, because a report of either mood reads it.
func (v DependencyViolation) found() string {
	if len(v.Dependencies) == 0 {
		return "nothing"
	}
	return strings.Join(v.Dependencies, ", ")
}
