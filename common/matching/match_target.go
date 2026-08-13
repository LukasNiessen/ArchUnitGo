package matching

import (
	"path"
	"strings"
)

// MatchTarget is the part of an identifier a Filter looks at. It is bound to the filter rather than
// passed at the call site, which is what keeps matching a single function instead of one function
// per target.
//
// Identifiers reaching a target are in the canonical form described in
// common/extraction/identifier.go: forward slashes, lexically clean, no trailing separator.
type MatchTarget uint8

// The four things a pattern can be matched against.
const (
	// TargetPath is the whole identifier — `internal/api/handler.go`. It is the zero value,
	// because looking at all of the identifier is the least surprising default.
	TargetPath MatchTarget = iota
	// TargetFilename is the last segment — `handler.go` of `internal/api/handler.go`.
	TargetFilename
	// TargetPathWithoutFilename is everything but the last segment — `internal/api` of
	// `internal/api/handler.go`. For a file at the project root it is `.`, which is the root's own
	// identifier.
	TargetPathWithoutFilename
	// TargetClassname is the declared name on its own, with any package or path qualification
	// stripped — `Handler` of `internal/api.Handler` and of `Handler`. Go has no classes; the
	// vocabulary is the family's, and here it means a declared type.
	TargetClassname
)

//nolint:gochecknoglobals // an immutable lookup table indexed by MatchTarget; Go has no const array.
var matchTargetNames = [...]string{
	TargetPath:                "path",
	TargetFilename:            "filename",
	TargetPathWithoutFilename: "path without filename",
	TargetClassname:           "classname",
}

// Valid reports whether t is one of the four declared match targets.
func (t MatchTarget) Valid() bool {
	return int(t) < len(matchTargetNames)
}

// String names the target as it appears in reports.
func (t MatchTarget) String() string {
	if !t.Valid() {
		return "unknown"
	}
	return matchTargetNames[t]
}

// extract pulls the part of identifier that this target names. It reports false when there is no
// such part — an empty identifier, or a target that is not one of the four — which is a non-match
// rather than an error: a rule matching nothing is caught by the empty-test guard, not here.
func (t MatchTarget) extract(identifier string) (string, bool) {
	if identifier == "" {
		return "", false
	}
	switch t {
	case TargetPath:
		return identifier, true
	case TargetFilename:
		return path.Base(identifier), true
	case TargetPathWithoutFilename:
		return path.Dir(identifier), true
	case TargetClassname:
		return classname(identifier), true
	default:
		return "", false
	}
}

// classname strips path and package qualification from an identifier, leaving the declared name.
func classname(identifier string) string {
	name := path.Base(identifier)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}
