package projection

import (
	"path"
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// ProjectSnapshot reshapes an extracted graph into the snapshot a report is rendered from, under this query.
// A nil *SnapshotOptions means the defaults: one node per file of the project, every dependency between
// them, nothing collapsed and no title.
//
// It is four steps, and they are in this order because none of them can be undone by the next:
//
//  1. Filter. Everything outside the project goes, unless the query asked for it, and then the three `which
//     nodes` options narrow what is left.
//  2. Collapse. Every surviving node is given the label it is drawn under: a group's name, a folder, or its
//     own identifier.
//  3. Aggregate. Dependencies that landed on the same pair of labels become one edge, counting how many
//     there were and unioning their import kinds.
//  4. Count. The summary is derived from the result, in NewSnapshot.
//
// Filtering before collapsing is the step worth explaining, because the other order is a tempting mistake:
// `focus on "internal/api/**" within 1 hop` means the neighbors of those files, and the neighbors of a
// folder that forty files were already merged into is a different and much larger set. So the query's
// patterns always match identifiers — what the user typed them against — and labels only ever exist in the
// drawn result.
//
// Nothing here reads the filesystem and nothing is cached: the same graph and the same query produce the
// same snapshot, every time.
func ProjectSnapshot(graph extraction.Graph, options *SnapshotOptions) Snapshot {
	query := options.WithDefaults()
	external := externalNodes(graph)
	dependencies := query.keptDependencies(graph)
	kept := query.keptNodes(query.universe(graph, external), dependencies)
	label := query.labeling(external)
	return NewSnapshot(
		query.Title,
		snapshotNodes(kept, external, label),
		snapshotEdges(dependencies, kept, query.IncludeSelfDependencies, label),
	)
}

// externalNodes are the identifiers in this graph that are not the project's code: the target of every
// external edge.
//
// It is read off the edges rather than guessed from an identifier, because whether `net/http` is outside the
// project is the extractor's answer and not a matter of what a path looks like. A node is external when it is
// the target of an external dependency; a file of the project is the source of its own self-edge, and that
// edge is never external.
func externalNodes(graph extraction.Graph) nodeSet {
	external := make(nodeSet)
	for _, edge := range graph {
		if edge.External {
			external.add(edge.Target)
		}
	}
	return external
}

// universe are the nodes this query could possibly draw, in the graph's own sorted order: every node the
// graph mentions, minus what is outside the project when the query did not ask for it.
//
// Isolated nodes are in here, which is why it starts from the graph's nodes rather than from its
// dependencies: a file that depends on nothing and that nothing depends on is a node of the report, and for
// a report about how a project is arranged it is often the interesting one.
func (o *SnapshotOptions) universe(graph extraction.Graph, external nodeSet) []string {
	nodes := graph.Nodes()
	if o.IncludeExternalDependencies {
		return nodes
	}
	own := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if !external.contains(node) {
			own = append(own, node)
		}
	}
	return own
}

// keptDependencies are the edges this query could possibly draw: every edge of the graph that carries a
// dependency rather than a node, minus the ones leaving the project when the query did not ask for those.
//
// Self-edges are dropped here because they carry nodes and not dependencies, which is the invariant
// extraction.Graph documents. A drawn self-dependency comes from the collapse instead, where it is a real
// fact about a folder — IncludeSelfDependencies is that question, and it is asked when the labels are known.
func (o *SnapshotOptions) keptDependencies(graph extraction.Graph) extraction.Graph {
	kept := make(extraction.Graph, 0, len(graph))
	for _, dependency := range graph.Dependencies() {
		if dependency.External && !o.IncludeExternalDependencies {
			continue
		}
		kept = append(kept, dependency)
	}
	return kept
}

// keptNodes is the set of nodes the query's three `which nodes` options leave: the universe narrowed by every
// `focus on`, every `reachable from` and every `dependents of`, intersected.
//
// Each option is resolved against the whole universe rather than against what an earlier one left, and that
// is what makes them order-independent, as a modifier of this library has to be. Intersecting the results is
// the same decision the file module's scope verbs make: each modifier narrows the report, so a query holding
// two of them means the nodes both of them describe.
//
// A pattern that names nothing leaves nothing, and the empty snapshot that follows is caught by the
// terminal's guard rather than quietly rendered.
func (o *SnapshotOptions) keptNodes(universe []string, dependencies extraction.Graph) nodeSet {
	kept := newNodeSet(universe...)
	if len(o.Focus)+len(o.ReachableFrom)+len(o.DependentsOf) == 0 {
		return kept
	}
	links := newAdjacency(dependencies)
	for _, focus := range o.Focus {
		kept = kept.intersect(links.reach(selected(universe, focus.Selector), directionBoth, focus.hops()))
	}
	for _, selector := range o.ReachableFrom {
		kept = kept.intersect(links.reach(selected(universe, selector), directionForward, unlimited))
	}
	for _, selector := range o.DependentsOf {
		kept = kept.intersect(links.reach(selected(universe, selector), directionBackward, unlimited))
	}
	return kept
}

// selected are the nodes of the universe this pattern names, in the universe's order: the seeds of one
// traversal. Matching is matching.Filter's job, so a report's patterns look at the same part of an identifier,
// and reject the same syntax, as a rule's do.
func selected(universe []string, selector matching.Filter) []string {
	seeds := make([]string, 0, len(universe))
	for _, node := range universe {
		if selector.Matches(node) {
			seeds = append(seeds, node)
		}
	}
	return seeds
}

// labeling is the function that says what a node is drawn as, built once per snapshot because both the nodes
// and the edges have to agree about it.
//
// The collapse groups are asked first and the first match wins — the same rule projection.LayerOf uses to
// resolve overlapping layers, for the same reason: two patterns can name one node, and the order the user
// wrote them in is the only answer that is theirs. Folder depth is asked next, and only about the project's
// own nodes, because an import path is not a folder of this project and truncating `github.com/x/y` to one
// segment would draw a node called `github.com`. Whatever neither claims is drawn under its own identifier.
func (o *SnapshotOptions) labeling(external nodeSet) func(string) string {
	return func(identifier string) string {
		for _, group := range o.CollapseGroups {
			if group.Selector.Matches(identifier) {
				return group.Label
			}
		}
		if o.CollapseToFolderDepth > 0 && !external.contains(identifier) {
			return folderAtDepth(identifier, o.CollapseToFolderDepth)
		}
		return identifier
	}
}

// folderAtDepth is the folder an identifier collapses onto: the first depth segments of the folder it lives
// in, or that whole folder when it has fewer. A file at the project root lives in `.`, which is the root's
// own identifier, so it collapses onto that at every depth.
func folderAtDepth(identifier string, depth int) string {
	folder := path.Dir(identifier)
	segments := strings.Split(folder, "/")
	if len(segments) <= depth {
		return folder
	}
	return strings.Join(segments[:depth], "/")
}

// snapshotNodes are the report's nodes: one per label the kept identifiers are drawn under, each of them
// external only when every identifier behind it is.
//
// That last rule is what a collapse group holding both the project's code and a dependency of it means. A
// group is external when nothing of this project is in it — a `third party` node — and internal as soon as
// one file of the project is, because a node a reader can go and edit is not somebody else's code.
func snapshotNodes(kept, external nodeSet, label func(string) string) []Node {
	own := make(map[string]bool, len(kept))
	labels := make([]string, 0, len(kept))
	for identifier := range kept {
		drawn := label(identifier)
		if _, seen := own[drawn]; !seen {
			labels = append(labels, drawn)
		}
		own[drawn] = own[drawn] || !external.contains(identifier)
	}
	slices.Sort(labels)

	nodes := make([]Node, 0, len(labels))
	for _, drawn := range labels {
		if own[drawn] {
			nodes = append(nodes, NewNode(drawn))
		} else {
			nodes = append(nodes, NewExternalNode(drawn))
		}
	}
	return nodes
}

// snapshotEdges are the report's dependencies: every kept dependency between two kept nodes, relabeled onto
// the pair of nodes it is drawn between and merged with everything else that landed on the same pair.
//
// The merge is what a collapse is for, and it is also why this does not go through
// common/projection.ProjectEdges: that function drops an edge whose two labels are equal, which after a
// collapse is exactly the self-dependency `include self dependencies` asks to see. Here it is a query option,
// and the raw dependencies behind each edge are handed to NewEdge so that the count, the external flag and
// the import kinds are derived from them rather than tracked alongside them.
//
// A dependency with an end outside the kept set is dropped whichever end it is: a report about a set of
// nodes draws the dependencies between them, and an arrow to a node that is not on the diagram is an arrow
// to nowhere.
func snapshotEdges(dependencies extraction.Graph, kept nodeSet, includeSelf bool, label func(string) string) []Edge {
	cumulated := make(map[labelPair]extraction.Graph, len(dependencies))
	pairs := make([]labelPair, 0, len(dependencies))
	for _, dependency := range dependencies {
		if !kept.contains(dependency.Source) || !kept.contains(dependency.Target) {
			continue
		}
		pair := labelPair{source: label(dependency.Source), target: label(dependency.Target)}
		if pair.source == pair.target && !includeSelf {
			continue
		}
		if _, seen := cumulated[pair]; !seen {
			pairs = append(pairs, pair)
		}
		cumulated[pair] = append(cumulated[pair], dependency)
	}

	edges := make([]Edge, 0, len(pairs))
	for _, pair := range pairs {
		edges = append(edges, NewEdge(pair.source, pair.target, cumulated[pair]...))
	}
	return edges
}

// labelPair is the pair of labels a dependency is drawn between, and the key the aggregation groups by. It is
// a struct rather than a joined string because a label can hold any character a folder can, and `a/b + c` and
// `a + b/c` must not collide.
type labelPair struct {
	source string
	target string
}
