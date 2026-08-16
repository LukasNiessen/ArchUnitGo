package fluentapi_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestCollapseToFolderDepthDrawsTheProjectAsItsFolders(t *testing.T) {
	// The modifier that turns an unreadable diagram of four hundred files into a readable one of nine modules,
	// and the dependencies it merges are counted rather than lost.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).CollapseToFolderDepth(2).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{".", "internal/api", "internal/db"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the collapsed report drew %v, want %v", labels, wantNodes)
	}
	wantEdges := []string{
		". -> internal/api [2 dependencies] [plain]",
		"internal/api -> internal/db [2 dependencies] [plain]",
	}
	if edges := edgeDescriptions(snapshot.Edges()); !slices.Equal(edges, wantEdges) {
		t.Errorf("the collapsed report's edges are %v, want %v", edges, wantEdges)
	}
	want := projection.Summary{Nodes: 3, Edges: 2, Dependencies: 4}
	if summary := snapshot.Summary(); summary != want {
		t.Errorf("the summary is %+v, want %+v: no dependency is lost when the arrows merge", summary, want)
	}
}

func TestCollapseToFolderDepthDrawsAShallowerFolderAsItself(t *testing.T) {
	// A file whose folder has fewer segments than asked for is drawn as its whole folder, and a file at the
	// project root lives in `.` — which is the root's own identifier.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).CollapseToFolderDepth(9).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{".", "internal/api", "internal/db"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("collapsing to nine segments drew %v, want the folders that exist %v", labels, wantNodes)
	}
}

func TestCollapseToFolderDepthNeverFoldsAnImportPath(t *testing.T) {
	// An import path is not a folder of this project, so a collapsed report must not grow a node called `net`.
	// Grouping those is what `collapse by pattern` is for.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		IncludingExternalDependencies().
		CollapseToFolderDepth(1).
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	labels := nodeLabels(snapshot.Nodes())
	if !slices.Contains(labels, "net/http") || !slices.Contains(labels, "database/sql") {
		t.Errorf("the report's nodes are %v, want the import paths drawn whole", labels)
	}
	if slices.Contains(labels, "net") || slices.Contains(labels, "database") {
		t.Errorf("the report's nodes are %v, want no import path truncated to its first segment", labels)
	}
}

func TestCollapseToFolderDepthTwiceKeepsTheLastDepth(t *testing.T) {
	// A report draws its nodes at one granularity, not two.
	report := fluentapi.ProjectGraph(nil).CollapseToFolderDepth(3).CollapseToFolderDepth(1)

	if got, want := report.String(), "project graph, collapse to folder depth 1"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCollapseToFolderDepthRejectsADepthBelowOne(t *testing.T) {
	// Zero path segments is not a folder any file lives in, and asking not to collapse at all is not calling the
	// modifier. Reporting it is what keeps a mistyped depth from quietly drawing every file.
	for _, depth := range []int{0, -2} {
		report := fluentapi.ProjectGraph(nil).CollapseToFolderDepth(depth)

		snapshot, err := report.Snapshot()

		var user *archerror.UserError
		if !errors.As(err, &user) {
			t.Fatalf("Snapshot error = %v, want a *archerror.UserError", err)
		}
		if user.Operation != "collapse to folder depth" {
			t.Errorf("UserError.Operation = %q, want the modifier at fault", user.Operation)
		}
		if !errors.Is(err, fluentapi.ErrInvalidFolderDepth) {
			t.Errorf("Snapshot error = %v, want it to wrap ErrInvalidFolderDepth", err)
		}
		if !snapshot.Empty() {
			t.Errorf("Snapshot reports %v beside the error, want nothing said about the project", snapshot)
		}
		if rendered := report.String(); !strings.Contains(rendered, "rejected") {
			t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
		}
	}
}

func TestCollapseByPatternDrawsEveryFileAGroupNamesAsThatGroup(t *testing.T) {
	// The diagram whose boxes are the architecture rather than the directory tree — and giving the groups the
	// names a layer policy declares is what makes a report and a rule describe the same thing.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		CollapseByPattern("api", "internal/api/**").
		CollapseByPattern("db", "internal/db/**").
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{"api", "db", "main.go"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the grouped report drew %v, want %v: what no group claims keeps its identifier", labels, wantNodes)
	}
	wantEdges := []string{
		"api -> db [2 dependencies] [plain]",
		"main.go -> api [2 dependencies] [plain]",
	}
	if edges := edgeDescriptions(snapshot.Edges()); !slices.Equal(edges, wantEdges) {
		t.Errorf("the grouped report's edges are %v, want %v", edges, wantEdges)
	}
}

func TestCollapseByPatternDrawsTheProjectsDependenciesAsOneThirdPartyNode(t *testing.T) {
	// The report a group is most worth having for: a family of modules as a single box, and a box holding
	// nothing of this project is drawn as somebody else's code. The catch-all goes last, because the first
	// group whose pattern names a node draws it.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		IncludingExternalDependencies().
		CollapseByPattern("this project", "internal/**").
		CollapseByPattern("this project", "main.go").
		CollapseByPattern("third party", "**").
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{"third party", "this project"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the grouped report drew %v, want %v", labels, wantNodes)
	}
	for _, node := range snapshot.Nodes() {
		if node.Label() == "third party" && !node.IsExternal() {
			t.Error("the third-party group is drawn as the project's own code, want it external")
		}
		if node.Label() == "this project" && node.IsExternal() {
			t.Error("the group holding the project's files is drawn as external, want it internal")
		}
	}
	want := "this project -> third party [2 dependencies] (external) [plain, blank]"
	if edges := edgeDescriptions(snapshot.Edges()); !slices.Contains(edges, want) {
		t.Errorf("the grouped report's edges are %v, want %q among them", edges, want)
	}
}

func TestCollapseByPatternIsAskedBeforeTheFolderDepth(t *testing.T) {
	// The two compose in one order: a named group where the report has a name for something, folders for
	// everything else.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		CollapseToFolderDepth(1).
		CollapseByPattern("the database", "internal/db/**").
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	wantNodes := []string{".", "internal", "the database"}
	if labels := nodeLabels(snapshot.Nodes()); !slices.Equal(labels, wantNodes) {
		t.Errorf("the report drew %v, want %v", labels, wantNodes)
	}
}

func TestCollapseByPatternRejectsAGroupWithoutAName(t *testing.T) {
	// A group is a node of the diagram and a node has to be called something, so a nameless one would draw
	// every file it matched under a blank label.
	report := fluentapi.ProjectGraph(nil).CollapseByPattern("", "internal/api/**")

	snapshot, err := report.Snapshot()

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Snapshot error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "collapse by pattern" || user.Subject != "internal/api/**" {
		t.Errorf("UserError = %v, want the modifier and the pattern as the user typed it", user)
	}
	if !errors.Is(err, fluentapi.ErrUnnamedGroup) {
		t.Errorf("Snapshot error = %v, want it to wrap ErrUnnamedGroup", err)
	}
	if !snapshot.Empty() {
		t.Errorf("Snapshot reports %v beside the error, want nothing said about the project", snapshot)
	}
}

func TestCollapseByPatternKeepsTheGroupsInTheOrderTheyWereWritten(t *testing.T) {
	// The order decides an overlap — the first group whose pattern names a node draws it — so it is the user's
	// and is never sorted, and the sentence the report renders as shows it.
	report := fluentapi.ProjectGraph(nil).
		CollapseByPattern("api", "internal/api/**").
		CollapseByPattern("everything else", "**")

	want := `project graph, ` +
		`collapse by pattern "api" by path matches "internal/api/**", ` +
		`collapse by pattern "everything else" by path matches "**"`
	if got := report.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
