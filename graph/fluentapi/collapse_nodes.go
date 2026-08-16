package fluentapi

import (
	"strconv"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// CollapseToFolderDepth draws each of the project's files as the folder it lives in, truncated to this many
// path segments:
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		CollapseToFolderDepth(2).
//		Snapshot()
//
// Depth 1 turns `internal/api/handler.go` into `internal`, 2 into `internal/api`, and a file whose folder has
// fewer segments than asked for is drawn as its whole folder — a file at the project root as `.`, which is
// the root's own identifier. It is the modifier that turns an unreadable diagram of four hundred files into a
// readable one of nine modules, and no dependency is lost in the process: the ones that merge onto the same
// pair of folders are counted on the edge that stands for them.
//
// Files outside the project are not folded this way, because an import path is not a folder of this project
// and `github.com` is not a module of it. CollapseByPattern is how those are grouped, and it is asked first.
//
// A depth below one is rejected: zero path segments is not a folder any file lives in, and asking not to
// collapse at all is not calling the modifier. Calling it twice keeps the last depth — a report draws its
// nodes at one granularity, not two.
func (b GraphBuilder) CollapseToFolderDepth(depth int) GraphBuilder {
	if depth < 1 {
		return b.rejecting("collapse to folder depth", strconv.Itoa(depth), ErrInvalidFolderDepth)
	}
	collapsed := b.modifying()
	collapsed.query.CollapseToFolderDepth = depth
	return collapsed
}

// CollapseByPattern draws every node this pattern names as one node under this label:
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		IncludingExternalDependencies().
//		CollapseByPattern("api", "internal/api/**").
//		CollapseByPattern("third party", "**").
//		Snapshot()
//
// It is the modifier for a diagram whose boxes are the architecture rather than the directory tree: two
// folders that are one component, a family of dependency modules as a single `third party` node, a legacy
// corner as one box nobody wants to look inside. The label is what the node is called, and it is asked for
// rather than derived, because a box on a diagram has to be called something and `internal/{api,web}/**` is
// not a name anybody wants to read. Giving groups the same names as a layer policy's layers is what makes a
// report and a rule describe the same architecture.
//
// The pattern is matched against the whole identifier and it claims nodes outside the project as well as the
// project's own files — the `third party` example above is exactly that. Groups are asked before
// `collapse to folder depth`, so the two compose: named groups where a report has a name for something,
// folders for everything else.
//
// The modifier is chainable and order-independent in the sense the others are — the report does not depend on
// when in the chain it was written — but the order of the groups themselves is the user's, and it decides an
// overlap: the first group whose pattern names a node draws it, which is the rule a layer policy resolves
// overlapping layers by. So write the specific group before the catch-all, as above.
//
// A group without a name is rejected, since a nameless node is a blank box.
func (b GraphBuilder) CollapseByPattern(label, pattern string) GraphBuilder {
	if label == "" {
		return b.rejecting("collapse by pattern", pattern, ErrUnnamedGroup)
	}
	selector, err := b.factory.PathMatcher(pattern)
	if err != nil {
		return b.rejecting("collapse by pattern", pattern, err)
	}
	collapsed := b.modifying()
	collapsed.qualified = collapsedByPattern
	group := projection.CollapseGroup{Label: label, Selector: selector}
	collapsed.query.CollapseGroups = append(collapsed.query.CollapseGroups, group)
	return collapsed
}
