// Package projection is the slices module's half of the PROJECT stage: it says which slice each of the
// project's files belongs to, and reshapes an extracted graph into the dependencies between those
// slices.
//
// A slice is a name cut out of a file's identifier, and that one sentence is the whole difference from a
// layer. A layer is declared — a name, plus the patterns whose files are in it — so layers/projection has
// a Layer type and a rule carries a list of them. A slice is declared nowhere: `internal/(**)/**` says
// both which files are sliced and what every slice is called, so the mapper is the only thing in the
// library that knows the names, and the slices of a project can only be found by running it over that
// project.
//
// Five exported functions, four that name slices and one that resolves their membership:
//
//   - SliceByCapture is the slicing MapFunction: one mapped edge per dependency between two sliced
//     files, labeled by the names a matching.Pattern with one capturing group cut out of them.
//   - SliceByPattern is SliceByCapture over a glob — `internal/(**)/**` — which is what `project
//     slices, defined by "internal/(**)/**"` compiles to.
//   - SliceByRegex is SliceByCapture over a regular expression, for the patterns a glob cannot spell.
//   - SliceByFileSuffix slices by what a file is rather than by where it lives: the last word of its
//     filename, so `order_handler.go` and `user_handler.go` are both in the slice `handler`.
//   - SelectSliceFiles resolves the slices of a graph through any of them: the files of each slice,
//     which is what a report needs in order to say that a slicing pattern matched nothing.
//
// Every mapper here drops the dependencies that leave the project, because a slice is a set of this
// project's own files and an import of the standard library is not one of them however well it matches.
// Every mapper here keeps the self-edges, relabelled, which is Identity's exception rather than the `per
// <thing> edge` family's rule: a self-edge is how a file that depends on nothing says it exists, and
// since nobody declared the slice names, reading the membership of a slice means reading those edges
// through the very mapper that names them. It costs the dependency rule nothing — ProjectEdges drops an
// edge whose two labels are equal in any case, and a self-edge's two ends are one file.
//
// Everything here is pure — a graph and a compiled pattern in, labels out — so what a slicing rule is
// about can be tested against a hand-built graph before any project is extracted at all.
package projection

import (
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// SliceByCapture is the slicing MapFunction of a rule about slices: one mapped edge per dependency
// between two of the project's files that pattern names, labeled by the names it cut out of them.
//
// The pattern is one with a capturing group — matching.NewGlobCapturePattern,
// matching.NewRegexCapturePattern or matching.RegexFactory.CapturePattern build one — and the slice of a
// file is what that group matched. A file the pattern does not describe, and a file it describes without
// naming anything in it, is in no slice, and an edge with such a file at either end is dropped. That is
// what makes a rule about part of a project possible: the files no pattern slices are not silently
// everybody's business.
//
// Projected through common/projection.ProjectEdges it is the structure a whole rule is judged over — one
// projected edge per (depending slice, depended-on slice) pair, cumulating the concrete file
// dependencies that produced it, which is what lets a violation about slices still name the files a
// reader has to go and open. The dependencies inside one slice project to two equal labels and
// ProjectEdges drops those, which is "a slice may always depend on itself" said as a fact about the
// projection rather than as a comparison in the judgement.
//
// The zero matching.Pattern names nothing, so it projects nothing, which is the loud direction: a
// projection with no edges is what the empty-test guard reports on, rather than a rule that quietly
// passes because the folders it was written about have been renamed.
func SliceByCapture(pattern matching.Pattern) kernel.MapFunction {
	return sliceBy(pattern.Capture)
}

// SliceByPattern is SliceByCapture over a glob whose one capture names the slice: `internal/(**)/**`
// slices a project by the folders under internal, and `(*)/**` by its top-level folders.
//
// It is the projection behind `project slices, defined by "internal/(**)/**"`, and the one an exported
// slicing projection is most often reached for directly. The glob syntax is
// matching.NewGlobCapturePattern's, which is the ordinary one plus the capture: a glob with no
// parentheses, or with more than one pair, is matching.ErrOneCapture, because a pattern that has to name
// a slice must say what the name is exactly once.
func SliceByPattern(glob string) (kernel.MapFunction, error) {
	pattern, err := matching.NewGlobCapturePattern(glob, nil)
	if err != nil {
		return nil, err
	}
	return SliceByCapture(pattern), nil
}

// sliceBy is every mapper in this package: drop what leaves the project, name both ends, and drop the
// edge unless both of them have a name. The naming function is the only thing that differs between
// slicing by a pattern and slicing by a filename, so the rest is stated once — including the two
// promises of the whole family, that an external dependency is in no slice and that a self-edge is kept
// under the name of the slice its file is in.
func sliceBy(name func(identifier string) (string, bool)) kernel.MapFunction {
	return func(edge extraction.Edge) (kernel.MappedEdge, bool) {
		if edge.External {
			return kernel.MappedEdge{}, false
		}
		source, sliced := name(edge.Source)
		if !sliced {
			return kernel.MappedEdge{}, false
		}
		target, sliced := name(edge.Target)
		if !sliced {
			return kernel.MappedEdge{}, false
		}
		return kernel.MappedEdge{SourceLabel: source, TargetLabel: target}, true
	}
}
