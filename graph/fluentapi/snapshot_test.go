package fluentapi_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
)

func TestSnapshotReportsAQueryThatDescribesNoNodeAtAll(t *testing.T) {
	// This module's shape of the failure the empty-test guard exists for. A folder that has been renamed leaves
	// a pattern that names nothing, and the report it draws is a blank diagram — which looks exactly like a
	// project that is clean. It is an error rather than a violation because a report has no violation list to
	// put one in.
	root := writeFixtureProject(t)
	report := fluentapi.ProjectGraph(fixtureLocator(t, root)).FocusOn("internal/transport/**", 2)

	snapshot, err := report.Snapshot()

	if !errors.Is(err, fluentapi.ErrEmptySnapshot) {
		t.Fatalf("Snapshot error = %v, want it to wrap ErrEmptySnapshot", err)
	}
	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Snapshot error = %v, want a *archerror.UserError", err)
	}
	if !strings.Contains(user.Subject, "internal/transport/**") {
		t.Errorf("UserError.Subject = %q, want the query that named nothing, so a reader knows what to fix", user.Subject)
	}
	if !snapshot.Empty() {
		t.Errorf("Snapshot reports %v beside the error, want nothing said about the project", snapshot)
	}
}

func TestSnapshotAllowsAReportOfNothingWhenTheCheckOptionsSaySo(t *testing.T) {
	// The same opt-out a rule has, through the same knob: a caller who really means to ask a question whose
	// answer may be nothing says so once, on the chain.
	root := writeFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		FocusOn("internal/transport/**", 2).
		WithCheckOptions(&kernel.CheckOptions{AllowEmptyTests: true}).
		Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !snapshot.Empty() {
		t.Errorf("the report drew %v, want nothing: no file is in that folder", snapshot)
	}
	if summary := snapshot.Summary(); summary.Nodes != 0 || summary.Edges != 0 {
		t.Errorf("the summary is %v, want zeros", summary)
	}
}

func TestSnapshotIsNotEmptyForAProjectWhoseFilesDependOnNothing(t *testing.T) {
	// A set of files that depend on nothing is a real answer and not the stale-glob failure, so the guard must
	// not fire on it: the report is the nodes, and there is nothing wrong with a diagram of boxes.
	root := writeIsolatedFixtureProject(t)

	snapshot, err := fluentapi.ProjectGraph(fixtureLocator(t, root)).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if summary := snapshot.Summary(); summary.Nodes != 2 || summary.Edges != 0 {
		t.Errorf("the summary is %v, want the project's 2 files and no dependency between them", summary)
	}
}

func TestSnapshotRejectsALocatorThatIsNotAProject(t *testing.T) {
	// Nothing is read while a report is described, so a locator naming no Go project is the terminal's error and
	// not the entry point's.
	report := fluentapi.ProjectGraph(&extraction.ProjectLocator{Directory: t.TempDir()})

	snapshot, err := report.Snapshot()

	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Fatalf("Snapshot error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
	if !snapshot.Empty() {
		t.Errorf("Snapshot reports %v beside the error, want nothing said about the project", snapshot)
	}
}

func TestSnapshotBuiltTwiceFromOneStoredReportIsTheSameReport(t *testing.T) {
	// A report is a value and the terminal is a function of it, so a stored query can be rendered as often as
	// there are formats to render it in — and must not drift between two of them.
	root := writeFixtureProject(t)
	report := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		IncludingExternalDependencies().
		IncludingSelfDependencies().
		CollapseToFolderDepth(1).
		Titled("twice")

	first, err := report.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	second, err := report.Snapshot()
	if err != nil {
		t.Fatalf("the second Snapshot failed: %v", err)
	}

	if first.String() != second.String() {
		t.Errorf("one report rendered\n%s\nand then\n%s", first, second)
	}
}

// writeIsolatedFixtureProject writes a project of two files that import nothing, which is what a report of
// nodes and no arrows is built from. Each file is a node because of its self-edge, which is the extractor's
// promise.
func writeIsolatedFixtureProject(t *testing.T) string {
	t.Helper()

	return writeProject(t, map[string]string{
		"go.mod":               "module example.com/isolated\n\ngo 1.26\n",
		"main.go":              "package main\n\nfunc main() {}\n",
		"internal/util/log.go": "package util\n\nfunc Log() {}\n",
	})
}
