package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// SnapshotOptions is the query a snapshot answers: which nodes the report is about, what they are drawn as,
// and what it is called.
//
// It is an options bag, so a *SnapshotOptions is always allowed to be nil and every default is a zero value:
// the project's own files, one node per file, every dependency between them, nothing collapsed and no title.
// Read one through WithDefaults rather than reaching for a field, so that the nil case is handled once. Every
// method of it takes a pointer receiver, for the reason fluentapi.CheckOptions' do: a nil-safe read has to be
// one, and a type with both kinds of receiver is a finding.
//
// The three `which nodes` fields — Focus, ReachableFrom and DependentsOf — each describe a set of the graph's
// nodes, and a query holding more than one keeps only the nodes in all of them. They are lists because
// asking about two parts of a system at once is normal, and combining them by intersection rather than by
// sequence is what makes them order-independent, as a fluent modifier has to be.
//
// The two collapse fields answer the same question differently and are asked in a fixed order:
// CollapseGroups first, because it names its groups and a query that named one meant it, then
// CollapseToFolderDepth for whatever no group claimed.
//
// What every field means precisely — and it is worth being precise, because these compose — is in
// ProjectSnapshot, which is the only thing that reads them.
type SnapshotOptions struct {
	// Title is what the report is called, and the empty string leaves the headline to the renderer.
	Title string
	// IncludeExternalDependencies draws the standard library and the modules this project depends on as
	// nodes too, instead of keeping the report to the project's own code.
	//
	// Off by default: a diagram of the project's own structure is what a reader almost always wants, and
	// one where `net/http` and `fmt` are nodes is mostly somebody else's code. Turn it on for the report
	// that asks what a project depends on rather than how it is arranged.
	IncludeExternalDependencies bool
	// IncludeSelfDependencies draws a node's dependency on itself, which after a collapse says that the
	// files inside a folder depend on each other.
	//
	// Off by default, and only ever a question after a collapse: a file does not depend on itself, so
	// without a collapse there is no such dependency to draw. Turn it on for the report about cohesion —
	// a folder with a loud self-dependency is one whose files belong together.
	IncludeSelfDependencies bool
	// Focus keeps the nodes a pattern names together with their neighborhood, a given number of hops out
	// in both directions. It is the `zoom in on this part of the system` option, and one Focus is one
	// `focus on` modifier.
	Focus []Focus
	// ReachableFrom keeps the nodes a pattern names and everything they depend on, transitively: what this
	// code pulls in, however far away.
	ReachableFrom []matching.Filter
	// DependentsOf keeps the nodes a pattern names and everything that depends on them, transitively: who
	// would notice if this code changed. It is ReachableFrom with the arrows followed backwards, which is
	// the impact-analysis report.
	DependentsOf []matching.Filter
	// CollapseToFolderDepth draws each of the project's files as the folder it lives in, truncated to this
	// many path segments: 1 turns `internal/api/handler.go` into `internal`, 2 into `internal/api`. Zero —
	// the default — collapses nothing, and a file whose folder has fewer segments than asked for is drawn
	// as its whole folder.
	//
	// It is the option that turns an unreadable diagram of four hundred files into a readable one of nine
	// modules, and the dependencies it merges are counted rather than lost. External nodes are never
	// folded this way: an import path is not a folder of this project.
	CollapseToFolderDepth int
	// CollapseGroups draws every node a group's pattern names as one node under that group's name, and it
	// is asked before CollapseToFolderDepth. One CollapseGroup is one `collapse by pattern` modifier; the
	// first group whose pattern matches a node wins, so the order they were written in is the user's and
	// is never sorted.
	CollapseGroups []CollapseGroup
}

// WithDefaults returns the query a snapshot should actually be built from: a copy of the receiver, or the
// defaults when the receiver is nil. ProjectSnapshot starts with this, so the "nil means defaults" contract
// lives in one place instead of being re-derived as a nil check per field.
//
// All four slices are cloned, for the reason fluentapi.CheckOptions.WithDefaults clones its own: a struct
// copy shares the backing array, so a builder appending a second `focus on` to its resolved query would
// reach into the query a stored half-built report shares — and an immutable builder that is immutable in
// every field but its slices is the more surprising of the two contracts.
func (o *SnapshotOptions) WithDefaults() SnapshotOptions {
	if o == nil {
		return SnapshotOptions{}
	}
	resolved := *o
	resolved.Focus = slices.Clone(o.Focus)
	resolved.ReachableFrom = slices.Clone(o.ReachableFrom)
	resolved.DependentsOf = slices.Clone(o.DependentsOf)
	resolved.CollapseGroups = slices.Clone(o.CollapseGroups)
	return resolved
}

// Focus is one `focus on` modifier: the nodes a pattern names, and how much of their neighborhood is shown
// with them.
//
// Depth is a number of hops, followed in both directions, because the question `what is around this` is not
// the same question as `what does this depend on` — a reader zooming in on one folder wants its collaborators
// on both sides. Following the arrows one way only is what ReachableFrom and DependentsOf are for, and they
// follow them all the way rather than a fixed number of hops.
//
// Depth 0 is the selected nodes alone, which is the `show me only these` report, and a negative depth means
// the same rather than an unbounded traversal: a mistyped depth should narrow a report, never blow it up.
type Focus struct {
	// Selector says which nodes the report is centered on. A selector matching nothing centers it on
	// nothing, and the empty snapshot that follows is what the terminal's guard is there to catch.
	Selector matching.Filter
	// Depth is how many hops out from those nodes to keep, in both directions.
	Depth int
}

// String renders the modifier's argument as `path matches "internal/api/**" within 2 hops`, which is how a
// builder prints itself.
func (f Focus) String() string {
	return f.Selector.String() + " within " + pluralize(f.hops(), "hop", "hops")
}

// hops is Depth as the traversal is given it: never negative, so that a mistyped depth keeps the report
// small instead of turning it into the whole graph. It is unexported because clamping is this type's
// business and a caller who wants to know what it asked for has the field.
func (f Focus) hops() int {
	return max(f.Depth, 0)
}

// CollapseGroup is one `collapse by pattern` modifier: the name a set of nodes is drawn under, and the
// pattern that says which nodes are in it.
//
// A group carries its label rather than deriving one from the pattern, because a collapsed node has to be
// called something and `internal/{api,web}/**` is not a name anybody wants to read on a diagram. Naming it
// is also what makes the option composable with the layers vocabulary: the groups of a report and the layers
// of a policy can be given the same names, and then the two describe the same architecture.
type CollapseGroup struct {
	// Label is the name the group's nodes are drawn under, and it is what makes the group a node.
	Label string
	// Selector says which nodes are in the group. Nodes outside the project can be in one too, which is
	// how a report draws every dependency module as a single `third party` node.
	Selector matching.Filter
}

// String renders the modifier's argument as `"api" by path matches "internal/api/**"`, which is how a
// builder prints itself.
func (g CollapseGroup) String() string {
	return `"` + g.Label + `" by ` + g.Selector.String()
}
