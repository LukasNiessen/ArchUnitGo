// Package projection is the layers module's half of the PROJECT stage: it says which layer each of the
// project's files belongs to, and reshapes an extracted graph into the dependencies between those
// layers.
//
// Four exported names, one type and three functions:
//
//   - Layer is one declared layer — the name a policy talks about it by, and the selectors that say
//     which files are in it. It is what `layer "api" defined by "internal/api/**"` becomes.
//   - LayerOf answers the one question this package exists for: which layer does this file belong to?
//     The layers are asked in the order they were declared and the first one that matches wins, so a
//     file is in at most one layer however the patterns overlap.
//   - SelectLayerFiles resolves every declared layer against a graph: the files of each of them, which
//     is what a report needs in order to say that a layer's pattern matched nothing.
//   - PerLayerEdge is this module's MapFunction. Projected through common/projection.ProjectEdges it
//     turns the dependencies between files into the dependencies between layers, dropping the edges
//     that leave the project and the ones either of whose ends is in no declared layer.
//
// Two of the layer policy's three semantic rules are settled here rather than in the assertion, because
// they are facts about the projection and not about a rule: an edge whose two ends are in the same layer
// projects to a self-edge, which ProjectEdges drops — that is "intra-layer dependencies are always
// allowed" — and an edge with an end in no layer is dropped by PerLayerEdge, which is "edges where
// either end belongs to no declared layer are ignored". The third rule, blocklist before allowlist, is
// the assertion's.
//
// Everything here is pure — a graph and compiled filters in, labels out — so what a policy is about can
// be tested against a hand-built graph before any project is extracted at all.
package projection

import (
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// Layer is one declared layer of a policy: the name every clause of that policy talks about it by, and
// the compiled selectors that say which of the project's files are in it.
//
// It is what the fluent API's `layer "api", defined by "internal/api/**"` compiles to, and the whole of
// what the projection needs in order to relabel a graph — a layer is a name for a set of files, and
// nothing else about it is a fact of the source.
//
// The selectors are combined with OR: a layer declared twice is one layer whose membership is the union
// of its declarations, because `layer "domain" defined by folder "internal/domain"` and `layer "domain"
// defined by folder "internal/model"` can only mean that the domain is both of them. That is the one
// place in this library where chaining widens rather than narrows, and it is why membership cannot be a
// single Filter.
//
// A Layer is immutable, and the zero Layer is a nameless layer no file is in.
type Layer struct {
	name      string
	selectors []matching.Filter
}

// NewLayer declares the layer called name, whose files are the ones any of these selectors accepts.
//
// The selectors are copied, because a Layer is immutable and is held by a builder a user may have
// stored: spreading a caller's slice into a variadic parameter shares its backing array. A layer
// declared with no selector at all is a layer no file is in, which the empty-test guard reports rather
// than this constructor rejecting — whether an empty layer is a failure is a question only a rule that
// judges something gets to ask.
func NewLayer(name string, selectors ...matching.Filter) Layer {
	return Layer{name: name, selectors: slices.Clone(selectors)}
}

// Name is what a policy's clauses call this layer.
func (l Layer) Name() string {
	return l.name
}

// Selectors are the compiled patterns that describe this layer's files, in the order they were
// declared and combined with OR. They are the data a report needs in order to say which pattern
// selected nothing, and they are what fluentapi hands the empty-test guard.
//
// The result is the caller's own copy, because a Layer that has been stored must not change afterwards.
func (l Layer) Selectors() []matching.Filter {
	return slices.Clone(l.selectors)
}

// Matches reports whether this file is one of the layer's own: any selector accepting it is enough,
// because a layer declared more than once is the union of its declarations.
//
// A layer with no selector matches nothing, which is the zero Layer as well — for the reason
// matching.Filter gives about its own zero value: an unset pattern is a mistake, and matching
// everything would hide it.
func (l Layer) Matches(identifier string) bool {
	for _, selector := range l.selectors {
		if selector.Matches(identifier) {
			return true
		}
	}
	return false
}

// String renders the declaration as the sentence the user typed, as `layer "api" defined by path
// matches "internal/api/**"`. Each selector describes itself, which is what a reader needs in order to
// see which part of an identifier a pattern was matched against, and several of them are joined with
// `or` because that is what a layer declared twice means.
//
// User-facing violation messages are built in the testing layer, not here.
func (l Layer) String() string {
	declaration := `layer "` + l.name + `"`
	if len(l.selectors) == 0 {
		return declaration + " defined by nothing"
	}
	definitions := make([]string, 0, len(l.selectors))
	for _, selector := range l.selectors {
		definitions = append(definitions, selector.String())
	}
	return declaration + " defined by " + strings.Join(definitions, " or ")
}

// LayerOf is the layer this file belongs to, and whether it belongs to one at all.
//
// The layers are asked in the order they were declared and the first one that matches wins, so a file is
// in exactly one layer however much the patterns overlap. That matters because a projection labels each
// end of an edge with one name: `layer "api" defined by "internal/api/**"` declared before `layer "all"
// defined by "internal/**"` means the api files are api files and the rest of internal is the broad
// layer, which is the reading a user who ordered their declarations that way meant.
//
// A file no declared layer matches is in no layer, and that is not an error: a project is rarely
// layered end to end, and the policy simply says nothing about such a file. PerLayerEdge is where that
// becomes "edges where either end belongs to no declared layer are ignored".
func LayerOf(identifier string, layers ...Layer) (string, bool) {
	for _, layer := range layers {
		if layer.Matches(identifier) {
			return layer.Name(), true
		}
	}
	return "", false
}
