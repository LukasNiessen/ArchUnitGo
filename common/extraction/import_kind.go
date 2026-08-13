package extraction

import (
	"math/bits"
	"strings"
)

// ImportKind is the flavour of a single Go import declaration. It is the one part of the shared
// data model that is deliberately language-specific: every ArchUnit port names the import kinds
// its own language has.
type ImportKind uint8

// The four kinds of import Go has.
const (
	// ImportKindPlain is `import "fmt"`.
	ImportKindPlain ImportKind = iota
	// ImportKindAliased is `import f "fmt"`.
	ImportKindAliased
	// ImportKindBlank is `import _ "fmt"`, imported only for its side effects.
	ImportKindBlank
	// ImportKindDot is `import . "fmt"`, dumping the package into the file scope.
	ImportKindDot
)

var importKindNames = [...]string{
	ImportKindPlain:   "plain",
	ImportKindAliased: "aliased",
	ImportKindBlank:   "blank",
	ImportKindDot:     "dot",
}

// Valid reports whether k is one of the four declared import kinds.
func (k ImportKind) Valid() bool {
	return int(k) < len(importKindNames)
}

// String names the kind as it appears in reports.
func (k ImportKind) String() string {
	if !k.Valid() {
		return "unknown"
	}
	return importKindNames[k]
}

// ImportKindSet is the set of import kinds carried by one Edge. An Edge merged from parallel
// imports carries the union of their kinds, so a set — not a list — is the honest shape. It is a
// bit set so that an Edge stays comparable and usable as a map key.
type ImportKindSet uint8

// NewImportKindSet collects kinds into a set. Values that are not declared import kinds are
// ignored.
func NewImportKindSet(kinds ...ImportKind) ImportKindSet {
	return ImportKindSet(0).With(kinds...)
}

// With returns the set extended by kinds. The receiver is not modified.
func (s ImportKindSet) With(kinds ...ImportKind) ImportKindSet {
	result := s
	for _, kind := range kinds {
		if !kind.Valid() {
			continue
		}
		result |= 1 << kind
	}
	return result
}

// Union returns the set of every kind in either set.
func (s ImportKindSet) Union(other ImportKindSet) ImportKindSet {
	return s | other
}

// Contains reports whether kind is in the set.
func (s ImportKindSet) Contains(kind ImportKind) bool {
	if !kind.Valid() {
		return false
	}
	return s&(1<<kind) != 0
}

// Empty reports whether the set holds no kinds, as it does for a self-edge.
func (s ImportKindSet) Empty() bool {
	return s == 0
}

// Len counts the kinds in the set.
func (s ImportKindSet) Len() int {
	return bits.OnesCount8(uint8(s))
}

// Kinds lists the kinds in the set, in declaration order. The order is stable, because reports
// built from it must be.
func (s ImportKindSet) Kinds() []ImportKind {
	kinds := make([]ImportKind, 0, s.Len())
	for kind := range ImportKind(len(importKindNames)) {
		if s.Contains(kind) {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// String renders the set as `[plain, blank]`.
func (s ImportKindSet) String() string {
	names := make([]string, 0, s.Len())
	for _, kind := range s.Kinds() {
		names = append(names, kind.String())
	}
	return "[" + strings.Join(names, ", ") + "]"
}
