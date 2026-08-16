package fluentapi

import (
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/layers/projection"
)

// Except takes what these patterns name back out of the layer that was declared most recently: `layer "api"
// defined by folder "internal/api/**", except "**/generated"`.
//
//	rule := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").Except("**/generated").
//		Layer("db").DefinedByFolder("internal/db/**").
//		WhereLayer("api").MayNotDependOnLayers("db")
//
// It is the companion every selector in this library has, said here about a layer's membership: a layer is a
// folder with a hole in it more often than anybody would like — the generated client, the vendored copy, the
// one package that predates the policy — and this is how that hole is written down where a reader can see it.
// The alternative is declaring the layer as a list of the sibling folders that are in it, which goes stale
// the day somebody adds one, and that staleness is silent: the new folder is in no layer, so every clause
// about the layer ignores it.
//
// The patterns are read against the same part of an identifier as the declaration they qualify — a folder
// after `defined by folder`, a whole identifier after `defined by` — because a bare exclusion is a second
// pattern of the same clause. ExceptInFolder and ExceptInPath are the same verb with the target said out
// loud, and there are only those two, because those are the two parts of an identifier this module's
// declarations name.
//
// It qualifies the layer the chain declared most recently, and it is repeatable: several patterns in one
// call, or several calls, all veto. A layer declared twice is one layer whose declarations are combined with
// OR, and an exclusion qualifies the declaration it follows rather than the layer as a whole — so
// `layer("domain").DefinedByFolder("internal/domain/**").Layer("domain").DefinedByFolder("internal/model/**").
// Except("**/generated")` keeps the whole domain folder and takes the generated part out of the model folder
// alone. Two mistakes are reported by the terminal as a UserError, the way a pattern that will not compile
// is: `except` before any layer has been declared, which is an exclusion with nothing to qualify, and
// `except` with no pattern at all.
//
// A file taken out of a layer this way is in no layer, unless a later declaration claims it — so every
// dependency it is an end of is ignored, which is what the exclusion means. It is not moved into some other
// layer and it is not a violation of anything.
func (b LayersBuilder) Except(patterns ...string) LayersBuilder {
	return b.excepting("except", patterns, nil)
}

// ExceptInFolder takes the files in a folder matching one of these patterns out of the layer declared most
// recently, whatever that declaration is about: `layer "api" defined by "internal/api/*.go", except in folder
// "**/generated"`.
func (b LayersBuilder) ExceptInFolder(patterns ...string) LayersBuilder {
	return b.excepting("except in folder", patterns, matching.FolderMatcher)
}

// ExceptInPath takes the files whose whole identifier matches one of these patterns out of the layer declared
// most recently: `layer "api" defined by folder "internal/api/**", except in path "internal/api/legacy/*.go"`.
func (b LayersBuilder) ExceptInPath(patterns ...string) LayersBuilder {
	return b.excepting("except in path", patterns, matching.PathMatcher)
}

// excepting is every `except` verb of this module: hand the declarations of the layer named most recently to
// matching.Excepting, which attaches the patterns to the last of them, and hand back a policy in which that
// layer is redeclared from the result. build is the target an exclusion names for itself, and nil is the
// plain form that inherits the qualified declaration's own — so which part of an identifier a verb looks at
// is stated once per verb here, exactly as LayerBuilder.defining states it for the declarations themselves.
//
// The layer is looked up by name because declaring a name twice widens the layer already declared under it,
// which leaves it where it was: the layer just declared is not the last of b.layers. A policy that has
// declared nothing has no selectors to hand over, and matching.Excepting rejects that as an exclusion with
// nothing to qualify, which is the same answer for the same reason.
//
// A rejection is deferred to the terminal the way a declaration's is: the first thing the user has to fix is
// the one reported, the policy renders with the rejection visible, and no layer is quietly widened or
// dropped in the meantime.
func (b LayersBuilder) excepting(verb string, patterns []string, build func(matching.Pattern) matching.Filter) LayersBuilder {
	declared := slices.IndexFunc(b.layers, func(layer projection.Layer) bool {
		return layer.Name() == b.declared
	})
	var selectors []matching.Filter
	if declared >= 0 {
		selectors = b.layers[declared].Selectors()
	}
	excepted, err := matching.Excepting(selectors, b.factory, patterns, build)
	if err != nil {
		return b.rejecting(verb, strings.Join(patterns, ", "), err)
	}
	narrowed := b
	narrowed.layers = slices.Clone(b.layers)
	narrowed.layers[declared] = projection.NewLayer(b.declared, excepted...)
	return narrowed
}
