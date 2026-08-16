package assertion

import (
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// KindFileNaming is the kind of NamingViolation: a file that is not named, or not placed, the way a rule
// requires.
//
// It names the vocabulary as well as the failure, the way KindFileCycle does, because every vocabulary the
// library grows judges the names of its own nodes — a slice, a layer, a package — and each of them reports
// its own type. The kind is what the testing layer picks a phrasing by, so two families sharing one name
// would be two shapes of data under one key.
//
// One kind covers all three of `have name`, `be in folder` and `be in path`. They differ only in the part
// of an identifier they look at, and that part travels on the filter the violation carries, so a report
// reads it there rather than from three kinds it would have to keep in step.
const KindFileNaming kernel.ViolationKind = "file-naming"

// NamingViolation says that one of the files a rule selected disagrees with what the rule required of its
// name or of its place in the project.
//
// It is what the three self-contained predicates report — `project files, ..., should, have name "*.go"`,
// `..., should, be in folder "internal/**"`, `..., should not, be in path "**/legacy/**"` — one per
// offending file, and it carries the file, the requirement and the mood rather than a sentence about them.
// That is enough for a report to say which file, which part of its identifier was looked at, which pattern
// the user wrote, and whether matching that pattern was the requirement or the offense.
type NamingViolation struct {
	// File is the identifier of the file the rule does not hold for, as the graph spells it —
	// `internal/db/conn.go`.
	File string
	// Required is what the rule asked of that file: the compiled pattern together with the part of an
	// identifier it is matched against, which is what a reader needs in order to see that `*.go` was
	// checked against a filename and `internal/**` against a folder. It quotes the pattern as the user
	// typed it, so a report never has to reconstruct the sentence.
	Required matching.Filter
	// Mood is which way round the requirement was written. Matching Required is what `should` demands and
	// what `should not` forbids, so without the mood a report could not tell one failure from the other —
	// and the same file, pattern and target describe both.
	Mood kernel.Mood
}

// NewNamingViolation records that this file is not named, or not placed, as the rule written in this mood
// requires.
//
// It is the only way a violation of this family is made, and every field of it is already immutable: an
// identifier is a string, and a matching.Filter shares nothing with the rule it was compiled for.
func NewNamingViolation(file string, required matching.Filter, mood kernel.Mood) NamingViolation {
	return NamingViolation{File: file, Required: required, Mood: mood}
}

// Kind is KindFileNaming.
func (NamingViolation) Kind() kernel.ViolationKind {
	return KindFileNaming
}

// String renders the violation as the offending file and then the requirement it broke, in the words the
// rule was written in — `internal/db/conn.go: should, filename matches "*_service.go"` — for a log line or
// a test failure.
//
// The requirement is rendered as the rule stated it rather than as its negation, which is what keeps
// assertion.Mood.Holds the one place in the library that inverts anything: a reader sees the sentence the
// file failed, and the file is named first because it is the thing to go and look at. The user-facing
// message is still the testing layer's to build, from File, Required and Mood.
func (v NamingViolation) String() string {
	return v.File + ": " + v.Mood.String() + ", " + v.Required.String()
}
