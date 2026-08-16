package fluentapi

import (
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// LayerBuilder is a layer that has been named and not yet described: the stage between `layer("api")` and
// the `defined by` that says which files are in it.
//
// It exists so that a layer cannot be declared without being described. A name on its own selects nothing,
// and a policy about a layer nobody is in is a policy that passes forever, so the type system asks for the
// pattern here rather than letting the chain move on and the empty-test guard report it later:
//
//	layers := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("db").DefinedByFolder("internal/db/**")
//
// Both verbs hand back the LayersBuilder they came from, widened by this layer, so declaring the next layer
// or writing the first clause carries on from the same chain. It is immutable like every other stage, which
// is what makes `layer("api")` a value that can be described twice.
type LayerBuilder struct {
	// policy is the builder this layer will join, carrying the layers declared before it, the clauses
	// written so far and any pattern this library has already rejected.
	policy LayersBuilder
	// name is the layer's name, as the user typed it. It is what the policy's clauses and every violation
	// of it will call this set of files.
	name string
}

// Layer names a layer of the project: `layer("api")`.
//
// The name is what the policy's clauses refer to and what a report calls the layer, so it is worth being a
// word the team already uses for that part of the project. The layer has no files yet — the `defined by`
// verb that closes this stage is what says which files are in it — and nothing is read here.
//
// Declaring the same name twice widens that layer instead of shadowing it: `layer("domain").DefinedByFolder
// ("internal/domain/**").Layer("domain").DefinedByFolder("internal/model/**")` is one layer, `domain`, whose
// files are those of both folders. That is the only reading of a repeated name that is not a silent mistake,
// and it is how a layer that lives in two places is spelled.
//
// The empty string is not a layer and is reported by the terminal as a UserError wrapping ErrUnnamedLayer: a
// layer is a name for a set of files, and a policy cannot say anything about one that has no name.
func (b LayersBuilder) Layer(name string) LayerBuilder {
	if name == "" {
		return LayerBuilder{policy: b.rejecting("layer", name, ErrUnnamedLayer), name: name}
	}
	return LayerBuilder{policy: b, name: name}
}

// DefinedBy says which files are in this layer by matching the whole identifier of each, folder and name at
// once: `defined by "internal/**/*_repository.go"`.
//
// It is the general form of the declaration — `**` crosses folder boundaries and `*` does not — and it is
// what a layer defined by how its files are named rather than where they live needs. A layer whose files sit
// in one folder is better spelled with DefinedByFolder, which says so.
//
// The pattern is matched against the file's identifier: its path from the project root, in slash form, with
// the extension, so `internal/api/handler.go`. Calling this verb closes the declaration and hands the policy
// back, ready for the next layer or the first clause.
func (l LayerBuilder) DefinedBy(pattern string) LayersBuilder {
	return l.defining("defined by", pattern, l.policy.factory.PathMatcher)
}

// DefinedByFolder says which files are in this layer by the folder they live in: `defined by folder
// "internal/api"` is that folder alone, and `defined by folder "internal/api/**"` is it together with
// everything below it.
//
// This is the declaration nearly every layer is written with, because a layer in Go is a folder — so it is
// worth being clear which of the two forms a policy means. A layer declared as `internal/api` does not
// contain `internal/api/handlers/get.go`, and a layer declared as `internal/api/**` does.
//
// The pattern is matched against the identifier without its last segment, so a file at the project root is
// in the folder `.`. Calling this verb closes the declaration and hands the policy back.
func (l LayerBuilder) DefinedByFolder(pattern string) LayersBuilder {
	return l.defining("defined by folder", pattern, l.policy.factory.FolderMatcher)
}

// String renders the policy so far, with this layer's declaration left open — `project layers, layer "api"`
// — so that a chain caught mid-declaration in a log line still says what it was about.
func (l LayerBuilder) String() string {
	if l.policy.err != nil {
		return l.policy.String()
	}
	return l.policy.String() + `, layer "` + l.name + `"`
}

// defining is both `defined by` verbs: compile the string the user typed with the policy's own factory and
// hand back the policy widened by this layer. Which part of an identifier a verb looks at is the compile
// function it passes in, so that pairing is stated once per verb — the shape FilesBuilder.selecting gives
// the scope verbs of the files module.
//
// A pattern this library cannot understand is deferred to the terminal, as everywhere else in the family:
// the rejection joins the policy, Check returns it as a UserError naming this verb before the project is
// read, and the layer is not declared — a zero Filter matches nothing, so declaring it anyway would report
// an empty layer instead of the typo the user has to fix.
func (l LayerBuilder) defining(verb, pattern string, compile func(string) (matching.Filter, error)) LayersBuilder {
	selector, err := compile(pattern)
	if err != nil {
		return l.policy.rejecting(verb, pattern, err)
	}
	if l.policy.err != nil {
		// The name was empty, or an earlier pattern did not compile. Either way the first rejection is the one
		// to report, and this layer must not join a policy that cannot run.
		return l.policy
	}
	return l.policy.declaring(l.name, selector)
}
