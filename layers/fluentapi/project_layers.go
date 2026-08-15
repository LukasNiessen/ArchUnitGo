// Package fluentapi is the chain a user types to describe a named-layer policy. It is the only part of the
// layers module that is public API, and the entry point of every rule in it is ProjectLayers:
//
//	rule := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("domain").DefinedByFolder("internal/domain/**").
//		Layer("db").DefinedByFolder("internal/db/**").
//		WhereLayer("api").MayOnlyDependOnLayers("domain", "db").
//		WhereLayer("db").MayNotDependOnLayers("api")
//	violations, err := rule.Check(nil)
//
// A rule is a value, not an action. Every stage here returns a new builder and does no work — no
// filesystem, no toolchain, nothing but a user pattern compiled to a regex — so a half-built policy can be
// stored in a variable and branched from as often as it is useful, which is what makes declaring the layers
// once and writing two policies over them possible.
//
// The chain has two halves, and both are chainable. `layer(name)`, closed by `defined by` or `defined by
// folder`, declares who exists; `where layer(name)`, closed by `may only depend on layers` or `may not
// depend on layers`, says what they may do. The second half is where the policy becomes a Checkable, which
// is why a chain that declares layers and no clause cannot be checked: it is not yet a rule about anything.
//
// A declaration takes an exclusion, which is `except` and its two targeted forms in except.go: it qualifies
// the declaration it follows, so `layer "api" defined by folder "internal/api/**", except "**/generated"` is
// one declaration and not a list of the sibling folders that are in the layer. A file taken out of a layer
// that way is in no layer, so every dependency it is an end of is ignored — the same rule as for a file no
// pattern ever claimed.
//
// This module is a convenience skin over the files module's vocabulary and it says nothing a pile of
// file-level rules could not — it exists because expressing an N-layer policy as N² pairwise rules is
// miserable, and because a report about `api` and `db` reads better than one about two folder globs. The
// semantics are three rules, no more: dependencies inside a layer are always allowed, dependencies with an
// end in no declared layer are ignored, and a blocklist clause is asked before an allowlist one.
//
// There is no mood stage here, deliberately. The two clauses carry their own polarity — `may only depend
// on` is the allowlist and `may not depend on` the blocklist — so `should` would have nothing to add and
// `should not, may not depend on layers` is not a sentence anybody would type. The mood is still what the
// two are one piece of logic through: it travels on assertion.Clause, where Should is the allowlist and
// ShouldNot the blocklist.
package fluentapi

import (
	"errors"
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
	"github.com/LukasNiessen/ArchUnitGo/layers/projection"
)

// The three ways a layer policy can be typed wrongly, as sentinels a caller can recognize with errors.Is.
// Each of them is reported as an archerror.UserError naming the step of the chain at fault: the library is
// working and the code has not been judged, there is simply no runnable rule to judge it with.
var (
	// ErrUnnamedLayer says a layer was declared, or named by a clause, with the empty string for a name. A
	// layer is a name for a set of files, and a policy cannot talk about one that has none.
	ErrUnnamedLayer = errors.New("layer without a name")
	// ErrUndeclaredLayer says a clause names a layer the policy never declared — the typo the sibling
	// libraries hit most, and one that would otherwise make the clause quietly judge nothing at all,
	// because a layer with no files is in no projected dependency.
	ErrUndeclaredLayer = errors.New("undeclared layer")
	// ErrNoLayersNamed says `may not depend on layers` was called with no layer at all. A blocklist that
	// forbids nothing holds forever, which is the same failure the empty-test guard exists for one stage
	// later. It is not the sealed layer: that is `may only depend on layers` with no argument, which
	// forbids everything outside the layer and is a policy people really write.
	ErrNoLayersNamed = errors.New("no layer named")
)

// LayersBuilder is the declaration stage of a layer policy: `project layers`, plus every layer declared on
// it and every clause written about them. It is what ProjectLayers returns, and what `defined by` hands
// back so that the next layer can be declared.
//
// A LayersBuilder is immutable. Every method takes a value receiver and hands back a new builder, so
// storing one and branching from it is safe and is the point — declaring a project's layers is the
// expensive half of writing a policy, and two policies over one set of layers should not mean typing them
// twice:
//
//	layers := archunit.ProjectLayers(nil).
//		Layer("api").DefinedByFolder("internal/api/**").
//		Layer("db").DefinedByFolder("internal/db/**")
//	strict := layers.WhereLayer("db").MayOnlyDependOnLayers()
//	loose := layers.WhereLayer("db").MayNotDependOnLayers("api")
//
// It is not a Checkable, and that is the grammar rather than an omission: a chain that has declared layers
// but no clause is not yet a rule about anything, so there is nothing for a terminal to report. The
// Checkable is LayersPolicyCondition, which the two predicates return.
//
// The zero value is `project layers` over the whole project, auto-detected, with no layer declared — the
// same builder ProjectLayers(nil) returns.
type LayersBuilder struct {
	// locator is where the project is, and nil means auto-detect, as it does at every entry point.
	locator *extraction.ProjectLocator
	// factory compiles the strings `defined by` and `defined by folder` take. It is the one place this
	// module decides that a user pattern is a glob, which is also why neither verb mentions glob syntax.
	factory matching.RegexFactory
	// layers are the declared layers, in the order they were declared, with unique names: a second
	// declaration of a name already here widens that layer instead of adding another. Order is what
	// resolves an overlap, per projection.LayerOf, so it is the user's and never sorted.
	layers []projection.Layer
	// declared is the name of the layer the chain declared most recently, which is the one an `except`
	// qualifies. It is a name rather than an index into the layers above because a name declared twice
	// widens the layer it already named, so the layer just declared is not always the last of them.
	declared string
	// clauses are the policy's clauses, in the order they were written. All of them are in force; the
	// order they are evaluated in is assertion.GatherDependencyViolations's, not this one.
	clauses []layersassertion.Clause
	// err is the first thing the user typed that this library cannot understand — a pattern that will not
	// compile, a layer without a name, a blocklist naming nothing — kept until a terminal can return it. A
	// fluent method has nowhere to put an error, and failing at the end of the chain is what lets the
	// chain read as a sentence.
	err error
}

// ProjectLayers is the entry point of every named-layer policy: `project layers`.
//
// The locator says where the project is and is always optional — nil means auto-detect, which walks up from
// the working directory to the nearest go.mod and is what a test run inside the project it is about always
// wants:
//
//	thisProject := archunit.ProjectLayers(nil)
//	thatProject := archunit.ProjectLayers(&archunit.ProjectLocator{Directory: dir})
//
// Nothing is read here. The returned builder describes layers that do not exist yet, and only a terminal
// resolves them against a project.
//
// The locator is copied rather than kept, so a caller may reuse one struct to build a policy per directory
// and each policy still means the directory it was built with.
func ProjectLayers(locator *extraction.ProjectLocator) LayersBuilder {
	if locator != nil {
		copied := *locator
		locator = &copied
	}
	return LayersBuilder{locator: locator, factory: matching.NewRegexFactory(nil)}
}

// Layers is ProjectLayers under the shorter name the family also gives it, for a chain that reads better
// without the verb. The two are one entry point: `layers, layer "api" defined by "internal/api/**"` and
// `project layers, layer "api" defined by "internal/api/**"` build the same policy.
func Layers(locator *extraction.ProjectLocator) LayersBuilder {
	return ProjectLayers(locator)
}

// declaredLayers are the layers this policy has declared, in the order they were declared: their names and
// the compiled patterns that say which files are in them, as the projection and the empty-test guard need
// them.
//
// The result is a copy, because a builder that has been stored must not change when a projection is built
// from it. A layer whose pattern this library rejected is not among them; the rejection is reported as an
// error by the terminal instead. What a user sees of the declarations is SelectLayerFiles — the files each
// layer came to — rather than the compiled patterns themselves.
func (b LayersBuilder) declaredLayers() []projection.Layer {
	return slices.Clone(b.layers)
}

// SelectLayerFiles resolves the declared layers against the project: the identifiers of the files in each
// of them, sorted, keyed by the layer's name. A nil *CheckOptions means the defaults.
//
// It is the half of every layer policy that runs before anything is judged — locate the project, extract
// it, assign each file to the first layer whose pattern describes it — so it is how a user sees what a
// half-built policy is talking about, and it is what a report of a layer nobody is in is built from.
//
// Every declared layer is a key of the result, including the ones no file is in. An empty layer is neither
// an error nor a violation here: whether it is a failure is a question only a rule that judges something
// can ask, so the empty-test guard belongs to the terminal.
//
// The error is something the chain could not make sense of — a pattern that will not compile, a layer
// without a name, a clause naming a layer that was never declared — or a project that cannot be located or
// extracted. It is never a rule failure.
func (b LayersBuilder) SelectLayerFiles(options *kernel.CheckOptions) (map[string][]string, error) {
	_, membership, err := b.resolve(options)
	return membership, err
}

// String renders the policy as far as it has been built, as `project layers, layer "api" defined by path
// without filename matches "internal/api/**", where layer "api", may not depend on layers "db"`.
//
// Each declaration and each clause describes itself, which is what a reader needs in order to see both what
// the layers are and what was said about them; user-facing violation messages are built in the testing
// layer, not here.
func (b LayersBuilder) String() string {
	return strings.Join(b.stages(), ", ") + b.rejected()
}

// resolve is the SOURCE-and-EXTRACT-plus-declaration half of a layer policy, in one call: the graph the
// policy is to be judged against, and the files of each declared layer.
//
// It is what SelectLayerFiles hands out the second half of and what the terminal runs first. A terminal
// needs both — the layers, to count for the empty-test guard, and the graph, because the dependencies
// between the layers' files are edges of it — and asking for them separately would extract the project
// twice or, worse, resolve the layers against a second graph.
//
// Anything the user typed that this library could not understand is returned before the project is read, and
// it is the first such thing rather than the last: a pattern that will not compile, a layer without a name, a
// clause about a layer nobody declared. The error is otherwise a project that cannot be located or extracted,
// and it is never a rule failure.
func (b LayersBuilder) resolve(options *kernel.CheckOptions) (extraction.Graph, map[string][]string, error) {
	if b.err != nil {
		return nil, nil, b.err
	}
	graph, err := options.ExtractGraph(b.locator)
	if err != nil {
		return nil, nil, err
	}
	return graph, projection.SelectLayerFiles(graph, b.layers...), nil
}

// declares reports whether this policy has declared a layer under this name, which is what every clause is
// asked before it is written: a clause is about the layers of its own policy, and a name that is not one of
// them is a typo.
func (b LayersBuilder) declares(name string) bool {
	for _, layer := range b.layers {
		if layer.Name() == name {
			return true
		}
	}
	return false
}

// populations are the policy's layers as the empty-test guard is asked about them: one population per
// declared layer, in the order they were declared, each carrying what it was selecting, how many files it
// came to and the patterns that described them.
//
// A policy has as many populations as it has layers, because any one of its patterns can be the stale one
// and a layer nobody is in is the whole failure: every clause about it is then vacuous, and a policy of
// vacuous clauses is green forever. The guard reports every empty population rather than the first, so a
// reader who renamed two folders is told about both.
//
// The subject names the layer as well as the vocabulary — `files in layer "api"` — because `layers` alone
// would not say which of a policy's patterns a reader has to go and fix.
func (b LayersBuilder) populations(membership map[string][]string) []kernel.EmptyTestPopulation {
	populations := make([]kernel.EmptyTestPopulation, 0, len(b.layers))
	for _, layer := range b.layers {
		populations = append(populations, kernel.EmptyTestPopulation{
			Subject:   `files in layer "` + layer.Name() + `"`,
			Matched:   len(membership[layer.Name()]),
			Selectors: layer.Selectors(),
		})
	}
	return populations
}

// stages are the parts of the sentence this policy has been built from, in the order the user typed them:
// the entry point, then one per declared layer, then one per clause, ready to be joined with ", ".
//
// It is a fresh slice, and the rejection below is rendered separately rather than as the last stage, because
// a rejected pattern ends the sentence instead of sitting inside it.
func (b LayersBuilder) stages() []string {
	stages := make([]string, 0, len(b.layers)+len(b.clauses)+1)
	stages = append(stages, "project layers")
	for _, layer := range b.layers {
		stages = append(stages, layer.String())
	}
	for _, clause := range b.clauses {
		stages = append(stages, clause.String())
	}
	return stages
}

// rejected renders the thing the user typed that this library could not understand as a parenthesis closing
// the sentence, and the empty string when the whole chain compiled. A rejected pattern declared no layer, so
// without it a policy would render as the rule the user thought they wrote.
func (b LayersBuilder) rejected() string {
	if b.err == nil {
		return ""
	}
	return " (rejected: " + b.err.Error() + ")"
}

// declaring adds a layer to the policy, or widens the one already declared under that name, and hands back a
// new builder — the shape every stage of this module returns a copy rather than mutating a receiver a user
// may have stored.
//
// A name declared twice is one layer whose selectors are the union of both declarations, for the reason
// projection.Layer gives: `layer "domain" defined by folder "internal/domain"` followed by `layer "domain"
// defined by folder "internal/model"` can only mean that the domain is both. Merging here is also what
// keeps the names in b.layers unique, so that everything downstream — the projection, the guard's
// populations, a clause's lookup — can take one layer per name for granted.
func (b LayersBuilder) declaring(name string, selector matching.Filter) LayersBuilder {
	declared := b
	declared.declared = name
	declared.layers = slices.Clone(b.layers)
	for index, layer := range declared.layers {
		if layer.Name() == name {
			declared.layers[index] = projection.NewLayer(name, append(layer.Selectors(), selector)...)
			return declared
		}
	}
	declared.layers = append(declared.layers, projection.NewLayer(name, selector))
	return declared
}

// clausing adds a clause to the policy and hands back the terminal it makes the chain into. Both predicates
// are this function with one mood, so what a clause is, where it is recorded and what is checked about it is
// stated once.
//
// A clause naming a layer the policy never declared is rejected here, in the words of the predicate it was
// written with — and it is a UserError rather than a violation for a reason worth spelling out: an undeclared
// layer has no files, so it is at neither end of any projected dependency, so the clause would judge nothing
// and pass forever. That is the same failure the empty-test guard catches one stage later, caught here where
// the name a user mistyped can be quoted back at them.
func (b LayersBuilder) clausing(layer string, named []string, mood assertion.Mood) LayersPolicyCondition {
	clause := layersassertion.NewClause(layer, named, mood)
	written := b
	for _, name := range clause.Named() {
		if !b.declares(name) {
			written = written.rejecting(clause.Predicate(), name, ErrUndeclaredLayer)
		}
	}
	written.clauses = append(slices.Clone(b.clauses), clause)
	return LayersPolicyCondition{policy: written}
}

// rejecting records that the user typed something this library cannot understand: a UserError naming the
// step of the chain at fault and quoting the argument as it was written, wrapping the reason.
//
// The first rejection wins and the ones after it are dropped, because the first is the one a user has to fix
// and a chain reporting the last would point at the wrong line. A rejected declaration does not join the
// layers: a zero Filter matches nothing, so the policy would report an empty layer instead of the typo.
func (b LayersBuilder) rejecting(verb, subject string, cause error) LayersBuilder {
	if b.err != nil {
		return b
	}
	rejected := b
	rejected.err = archerror.NewUserError(verb, subject, cause)
	return rejected
}
