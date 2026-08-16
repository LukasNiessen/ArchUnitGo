package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestProjectSnapshotDrawsOneNodePerFileOfTheProjectByDefault(t *testing.T) {
	// The default report: the project's own code, one node per file, every dependency between them. A nil
	// query is the same as an empty one, which is the options-bag contract.
	graph := fixtureGraph()

	snapshot := projection.ProjectSnapshot(graph, nil)

	wantNodes := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"internal/db/query.go",
		"internal/util/orphan.go",
		"main.go",
	}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the snapshot's nodes are %v, want %v", labels, wantNodes)
	}
	if got := len(snapshot.Edges()); got != countInternalDependencies(graph) {
		t.Errorf("the snapshot has %d edges, want the %d dependencies between the project's files", got, countInternalDependencies(graph))
	}
}

func TestProjectSnapshotKeepsAFileNothingDependsOnAndThatDependsOnNothing(t *testing.T) {
	// An isolated file is a node of the report, which is why the projection reads the graph's nodes and not
	// only its edges. For a report about how a project is arranged it is often the interesting one.
	snapshot := projection.ProjectSnapshot(fixtureGraph(), nil)

	if !slices.Contains(nodeLabels(snapshot.Nodes()), "internal/util/orphan.go") {
		t.Errorf("the isolated file is missing from %v, want it drawn as a node of its own", nodeLabels(snapshot.Nodes()))
	}
}

func TestProjectSnapshotLeavesTheStandardLibraryOutUnlessTheQueryAsksForIt(t *testing.T) {
	// A diagram of the project's own structure is what a reader almost always wants, so external nodes are
	// off by default — and then no edge of the report leaves the project either.
	snapshot := projection.ProjectSnapshot(fixtureGraph(), nil)

	if labels := nodeLabels(snapshot.Nodes()); slices.Contains(labels, "net/http") || slices.Contains(labels, "database/sql") {
		t.Errorf("the snapshot's nodes are %v, want no import path among them", labels)
	}
	if summary := snapshot.Summary(); summary.ExternalNodes != 0 || summary.ExternalEdges != 0 {
		t.Errorf("the summary is %v, want nothing external counted", summary)
	}
}

func TestProjectSnapshotDrawsExternalNodesWhenTheQueryIncludesThem(t *testing.T) {
	// The other report: what the project depends on rather than how it is put together. The external flag is
	// read off the extractor's edges, so `net/http` is somebody else's code because the extractor said so.
	query := projection.SnapshotOptions{IncludeExternalDependencies: true}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	labels := nodeLabels(snapshot.Nodes())
	if !slices.Contains(labels, "net/http") || !slices.Contains(labels, "database/sql") {
		t.Errorf("the snapshot's nodes are %v, want the two import paths among them", labels)
	}
	if summary := snapshot.Summary(); summary.ExternalNodes != 2 || summary.ExternalEdges != 2 {
		t.Errorf("the summary is %v, want 2 external nodes and 2 external edges", summary)
	}
	for _, node := range snapshot.Nodes() {
		if node.Label() == "net/http" && !node.IsExternal() {
			t.Error("net/http is drawn as the project's own code, want it marked external")
		}
	}
}

func TestProjectSnapshotCollapsesTheProjectsFilesOntoTheirFolders(t *testing.T) {
	// The modifier that turns an unreadable diagram of four hundred files into a readable one of nine
	// modules. Depth 2 keeps two path segments, and a file at the project root lives in `.`.
	query := projection.SnapshotOptions{CollapseToFolderDepth: 2}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{".", "internal/api", "internal/db", "internal/util"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the collapsed nodes are %v, want %v", labels, wantNodes)
	}
	wantEdges := []string{
		". -> internal/api [1 dependency] [plain]",
		". -> internal/db [1 dependency] [plain]",
		"internal/api -> internal/db [2 dependencies] [plain]",
	}
	if edges := edgeDescriptions(snapshot.Edges()); !slices.Equal(edges, wantEdges) {
		t.Errorf("the collapsed edges are %v, want %v", edges, wantEdges)
	}
}

func TestProjectSnapshotCountsTheDependenciesACollapseMergesIntoOneEdge(t *testing.T) {
	// The count is what keeps a collapsed diagram honest: two files' worth of dependencies between two
	// folders is one arrow, and an arrow that does not say so invites the reader to think they are barely
	// coupled. The raw total is in the summary, and it does not shrink when the arrows do.
	query := projection.SnapshotOptions{CollapseToFolderDepth: 2}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	edge, found := findEdge(snapshot, "internal/api", "internal/db")
	if !found {
		t.Fatalf("there is no edge from internal/api to internal/db in %v", edgeDescriptions(snapshot.Edges()))
	}
	if edge.Count() != 2 {
		t.Errorf("the edge stands for %d dependencies, want the 2 that were merged into it", edge.Count())
	}
	if summary := snapshot.Summary(); summary.Edges != 3 || summary.Dependencies != 4 {
		t.Errorf("the summary is %v, want 3 edges standing for 4 dependencies", summary)
	}
}

func TestProjectSnapshotUnionsTheImportKindsBehindOneCollapsedEdge(t *testing.T) {
	// Parallel edges are merged with their kinds unioned in the raw graph, and an aggregated edge is the same
	// decision one level up: an arrow that is one blank import and one real one is a different fact about a
	// codebase than an arrow that is two real ones.
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/router.go", "internal/db/conn.go", false, extraction.ImportKindBlank),
	)
	query := projection.SnapshotOptions{CollapseToFolderDepth: 2}

	snapshot := projection.ProjectSnapshot(graph, &query)

	edge, found := findEdge(snapshot, "internal/api", "internal/db")
	if !found {
		t.Fatalf("there is no edge from internal/api to internal/db in %v", edgeDescriptions(snapshot.Edges()))
	}
	if got := edge.ImportKinds().String(); got != "[plain, blank]" {
		t.Errorf("the edge's import kinds are %s, want the union [plain, blank]", got)
	}
}

func TestProjectSnapshotDrawsAFileAtTheProjectRootAsTheRootFolder(t *testing.T) {
	// `.` is the root's own identifier, the same answer matching.TargetPathWithoutFilename gives, and a file
	// there has fewer segments than any depth asks for.
	graph := extraction.NewGraph(extraction.SelfEdge("main.go"))
	query := projection.SnapshotOptions{CollapseToFolderDepth: 3}

	snapshot := projection.ProjectSnapshot(graph, &query)

	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, []string{"."}) {
		t.Errorf("main.go is drawn as %v at folder depth 3, want the root folder .", labels)
	}
}

func TestProjectSnapshotNeverFoldsAnImportPathOntoAFolder(t *testing.T) {
	// An import path is not a folder of this project, so collapsing must not draw a node called `github.com`.
	// Grouping those is `collapse by pattern`'s job, which is asked first.
	query := projection.SnapshotOptions{IncludeExternalDependencies: true, CollapseToFolderDepth: 1}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	labels := nodeLabels(snapshot.Nodes())
	if !slices.Contains(labels, "net/http") {
		t.Errorf("the nodes are %v, want net/http drawn under its own import path", labels)
	}
	if slices.Contains(labels, "net") || slices.Contains(labels, "database") {
		t.Errorf("the nodes are %v, want no import path truncated to its first segment", labels)
	}
}

func TestProjectSnapshotDrawsEveryNodeAGroupsPatternNamesAsThatGroup(t *testing.T) {
	// The diagram whose boxes are the architecture rather than the directory tree.
	query := projection.SnapshotOptions{
		CollapseGroups: []projection.CollapseGroup{
			{Label: "api", Selector: pathMatcher(t, "internal/api/**")},
			{Label: "db", Selector: pathMatcher(t, "internal/db/**")},
		},
	}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{"api", "db", "internal/util/orphan.go", "main.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the grouped nodes are %v, want %v: what no group claims keeps its identifier", labels, wantNodes)
	}
	edge, found := findEdge(snapshot, "api", "db")
	if !found {
		t.Fatalf("there is no edge from api to db in %v", edgeDescriptions(snapshot.Edges()))
	}
	if edge.Count() != 2 {
		t.Errorf("the edge from api to db stands for %d dependencies, want 2", edge.Count())
	}
}

func TestProjectSnapshotDrawsANodeTwoGroupsNameAsTheOneWrittenFirst(t *testing.T) {
	// Two patterns can name one node and the order they were written in is the only answer that is the
	// user's — the same rule a layer policy resolves overlapping layers by. It is what makes the specific
	// group plus a catch-all work.
	query := projection.SnapshotOptions{
		CollapseGroups: []projection.CollapseGroup{
			{Label: "api", Selector: pathMatcher(t, "internal/api/**")},
			{Label: "everything else", Selector: pathMatcher(t, "**")},
		},
	}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{"api", "everything else"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the grouped nodes are %v, want %v", labels, wantNodes)
	}
}

func TestProjectSnapshotAsksTheGroupsBeforeTheFolderDepth(t *testing.T) {
	// The two collapse modifiers compose in one order: a named group where the report has a name for
	// something, folders for everything else.
	query := projection.SnapshotOptions{
		CollapseToFolderDepth: 1,
		CollapseGroups: []projection.CollapseGroup{
			{Label: "the database", Selector: pathMatcher(t, "internal/db/**")},
		},
	}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{".", "internal", "the database"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the collapsed nodes are %v, want %v", labels, wantNodes)
	}
}

func TestProjectSnapshotDrawsAGroupOfNothingButImportPathsAsAnExternalNode(t *testing.T) {
	// The `third party` node: a group is external when nothing of this project is in it, and internal as soon
	// as one file of the project is, because a node a reader can go and edit is not somebody else's code.
	query := projection.SnapshotOptions{
		IncludeExternalDependencies: true,
		CollapseGroups: []projection.CollapseGroup{
			{Label: "third party", Selector: pathMatcher(t, "net/**")},
			{Label: "everything", Selector: pathMatcher(t, "**")},
		},
	}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	for _, node := range snapshot.Nodes() {
		switch node.Label() {
		case "third party":
			if !node.IsExternal() {
				t.Error("the group of import paths is drawn as the project's own code, want it external")
			}
		case "everything":
			if node.IsExternal() {
				t.Error("the group holding the project's files is drawn as external, want it internal")
			}
		}
	}
}

func TestProjectSnapshotDropsASelfDependencyUnlessTheQueryAsksForIt(t *testing.T) {
	// A file does not depend on itself, so a self-dependency only exists after a collapse — where it says the
	// files inside a folder depend on each other, which is a real fact and a query option rather than an
	// invariant. This is also why the aggregation is not common/projection.ProjectEdges, which drops it.
	graph := fixtureGraph()
	collapsed := projection.SnapshotOptions{CollapseToFolderDepth: 2}
	withSelf := projection.SnapshotOptions{CollapseToFolderDepth: 2, IncludeSelfDependencies: true}

	without := projection.ProjectSnapshot(graph, &collapsed)
	with := projection.ProjectSnapshot(graph, &withSelf)

	if _, found := findEdge(without, "internal/api", "internal/api"); found {
		t.Errorf("a self-dependency is drawn by default: %v", edgeDescriptions(without.Edges()))
	}
	edge, found := findEdge(with, "internal/api", "internal/api")
	if !found {
		t.Fatalf("the self-dependency is missing from %v", edgeDescriptions(with.Edges()))
	}
	if !edge.IsSelfDependency() || edge.Count() != 1 {
		t.Errorf("the self-dependency is %v, want one dependency from internal/api to itself", edge)
	}
	if with.Summary().Dependencies != without.Summary().Dependencies+2 {
		t.Errorf("including self dependencies counted %d dependencies, want the %d others plus the 2 inside the folders",
			with.Summary().Dependencies, without.Summary().Dependencies)
	}
}

func TestProjectSnapshotIncludingSelfDependenciesDrawsNoneWithoutACollapse(t *testing.T) {
	// The modifier is only ever a question after a collapse: no raw dependency runs from a file to itself, so
	// there is nothing for it to keep.
	query := projection.SnapshotOptions{IncludeSelfDependencies: true}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	for _, edge := range snapshot.Edges() {
		if edge.IsSelfDependency() {
			t.Errorf("the uncollapsed report draws %v, want no dependency from a file to itself", edge)
		}
	}
}

func TestProjectSnapshotDropsADependencyWhoseTargetTheQueryFilteredOut(t *testing.T) {
	// An arrow is dropped for either of its ends, and the target end is the half a report about a narrowed set
	// of nodes gets wrong most easily: the api folder alone keeps two files that between them depend on both
	// files of the db folder, and neither db file is on the diagram. An arrow to a node that is not drawn is an
	// arrow to nowhere, so only the dependency inside the folder survives.
	query := projection.SnapshotOptions{Focus: []projection.Focus{{Selector: pathMatcher(t, "internal/api/**")}}}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	wantNodes := []string{"internal/api/handler.go", "internal/api/router.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Fatalf("focusing on the api folder kept %v, want %v", labels, wantNodes)
	}
	wantEdges := []string{"internal/api/handler.go -> internal/api/router.go [1 dependency] [plain]"}
	if edges := edgeDescriptions(snapshot.Edges()); !slices.Equal(edges, wantEdges) {
		t.Errorf("the edges are %v, want %v: handler.go -> internal/db/conn.go and router.go -> internal/db/query.go both point at a node that was filtered out",
			edges, wantEdges)
	}
}

func TestProjectSnapshotIsReproducible(t *testing.T) {
	// The same graph and the same query render the same diagram, byte for byte, which is what makes a
	// checked-in artifact reviewable in a pull request — and what the sorting in NewSnapshot is for, since
	// the aggregation runs over maps.
	graph := fixtureGraph()
	query := projection.SnapshotOptions{IncludeExternalDependencies: true, CollapseToFolderDepth: 2, Title: "twice"}

	first := projection.ProjectSnapshot(graph, &query)
	for range 8 {
		if again := projection.ProjectSnapshot(graph, &query); again.String() != first.String() {
			t.Fatalf("the same query rendered\n%s\nand then\n%s", first, again)
		}
	}
}

func TestProjectSnapshotTitlesTheReportWithWhatTheQueryAsked(t *testing.T) {
	query := projection.SnapshotOptions{Title: "the modules of this project"}

	snapshot := projection.ProjectSnapshot(fixtureGraph(), &query)

	if snapshot.Title() != "the modules of this project" {
		t.Errorf("the report is titled %q, want the title the query carried", snapshot.Title())
	}
}

// fixtureGraph is a small project with two layers, a file at its root, a file nothing touches and two
// dependencies leaving the project — enough that every query option has something to say about it, and
// hand-built so that the projection is tested without an extractor.
//
//	main.go                    -> internal/api/handler.go, internal/db/conn.go
//	internal/api/handler.go    -> internal/api/router.go, internal/db/conn.go, net/http
//	internal/api/router.go     -> internal/db/query.go
//	internal/db/conn.go        -> internal/db/query.go, database/sql (blank)
//	internal/util/orphan.go    -> nothing, and nothing depends on it
func fixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.SelfEdge("internal/db/query.go"),
		extraction.SelfEdge("internal/util/orphan.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("main.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/api/router.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/router.go", "internal/db/query.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "internal/db/query.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "net/http", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "database/sql", true, extraction.ImportKindBlank),
	)
}

// pathMatcher is the compiled glob a query option is given, matched against the whole identifier — the same
// matcher the fluent modifiers compile theirs with.
func pathMatcher(t *testing.T, glob string) matching.Filter {
	t.Helper()

	filter, err := matching.NewRegexFactory(nil).PathMatcher(glob)
	if err != nil {
		t.Fatalf("path matcher %q failed to compile: %v", glob, err)
	}
	return filter
}

// nodeLabels are the labels a snapshot's nodes are drawn under, in order, for a message about what a report
// came out as.
func nodeLabels(nodes []projection.Node) []string {
	labels := make([]string, 0, len(nodes))
	for _, node := range nodes {
		labels = append(labels, node.Label())
	}
	return labels
}

// edgeDescriptions are a snapshot's dependencies as they render, in order.
func edgeDescriptions(edges []projection.Edge) []string {
	descriptions := make([]string, 0, len(edges))
	for _, edge := range edges {
		descriptions = append(descriptions, edge.String())
	}
	return descriptions
}

// findEdge is the snapshot's dependency between these two labels, if it drew one.
func findEdge(snapshot projection.Snapshot, source, target string) (projection.Edge, bool) {
	for _, edge := range snapshot.Edges() {
		if edge.SourceLabel() == source && edge.TargetLabel() == target {
			return edge, true
		}
	}
	return projection.Edge{}, false
}

// countInternalDependencies is how many dependencies of this graph stay inside the project, which is how many
// edges the default report draws.
func countInternalDependencies(graph extraction.Graph) int {
	count := 0
	for _, edge := range graph.Dependencies() {
		if !edge.External {
			count++
		}
	}
	return count
}
