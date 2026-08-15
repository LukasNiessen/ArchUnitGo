package fluentapi

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

// ErrMissingExportPath says `export as <format>` was given the empty string for a path. A report has to be
// written somewhere, and the working directory is a folder rather than a file, so there is nothing this could
// have meant.
//
// It is reported as an archerror.UserError naming the terminal at fault, the way a modifier's rejected pattern
// is: the library is working and the code has not been judged, the call simply cannot be run as written.
var ErrMissingExportPath = errors.New("no path to export to")

const (
	// exportedFilePermissions are the mode an exported report is created with: readable by anyone, writable by
	// its owner. A diagram is an artifact meant to be opened, attached to a build's output or committed beside
	// the code, so the default of a file nobody but its owner can read would be the surprise.
	exportedFilePermissions = 0o644
	// exportedFolderPermissions are the mode a folder created for a report is given, which is the file's mode
	// plus the traversal bit a folder is useless without.
	exportedFolderPermissions = 0o755
)

// ExportAsDot writes the report to this path as a Graphviz DOT digraph: `export as dot`.
//
//	err := archunit.ProjectGraph(nil).
//		CollapseToFolderDepth(2).
//		Titled("the modules of this project").
//		ExportAsDot("docs/architecture.dot")
//
// It is one of the six `export as <format>` terminals, each the file form of the `to <format>` twin that hands
// the same document back as a string — ToDot here — and what that format is good for is documented there.
//
// The path is interpreted like any other path a test writes to, relative to the working directory the test runs
// in, and its folders are created if they are not there yet: a report exported into `docs/` should not fail
// because nobody had made `docs/` yet. An existing file is overwritten, because a report is the current answer
// about the project and a stale one beside it would be read as a second answer.
//
// The error is Snapshot's — a pattern that would not compile, a project that will not load, ErrEmptySnapshot when
// the query described nothing — or ErrMissingExportPath for an empty path, or a technical error naming the file
// when the disk refused it. Nothing is written unless the whole document was rendered, so a failure leaves no
// half-written diagram behind for the next reader to trust.
func (b GraphBuilder) ExportAsDot(path string) error {
	return b.exported("export as dot", path, rendering.RenderDot)
}

// ExportAsMermaid writes the report to this path as a Mermaid flowchart: `export as mermaid`.
//
//	err := archunit.ProjectGraph(nil).CollapseToFolderDepth(1).ExportAsMermaid("docs/architecture.mmd")
//
// It is ToMermaid's file form and behaves exactly as ExportAsDot does about the path, the folders and the errors.
func (b GraphBuilder) ExportAsMermaid(path string) error {
	return b.exported("export as mermaid", path, rendering.RenderMermaid)
}

// ExportAsD2 writes the report to this path as a D2 declaration: `export as d2`.
//
//	err := archunit.ProjectGraph(nil).CollapseByPattern("api", "internal/api/**").ExportAsD2("docs/architecture.d2")
//
// It is ToD2's file form and behaves exactly as ExportAsDot does about the path, the folders and the errors.
func (b GraphBuilder) ExportAsD2(path string) error {
	return b.exported("export as d2", path, rendering.RenderD2)
}

// ExportAsCSV writes the report to this path as one comma-separated table: `export as csv`.
//
//	err := archunit.ProjectGraph(nil).CollapseToFolderDepth(2).ExportAsCSV("build/dependencies.csv")
//
// It is ToCSV's file form and behaves exactly as ExportAsDot does about the path, the folders and the errors.
func (b GraphBuilder) ExportAsCSV(path string) error {
	return b.exported("export as csv", path, rendering.RenderCSV)
}

// ExportAsJSON writes the report to this path as a JSON document: `export as json`.
//
//	err := archunit.ProjectGraph(nil).IncludingExternalDependencies().ExportAsJSON("build/dependencies.json")
//
// It is ToJSON's file form and behaves exactly as ExportAsDot does about the path, the folders and the errors.
func (b GraphBuilder) ExportAsJSON(path string) error {
	return b.exported("export as json", path, rendering.RenderJSON)
}

// ExportAsHTML writes the report to this path as one self-contained HTML page: `export as html`.
//
//	err := archunit.ProjectGraph(nil).CollapseToFolderDepth(2).ExportAsHTML("build/architecture.html")
//
// It is ToHTML's file form and behaves exactly as ExportAsDot does about the path, the folders and the errors. The
// page it writes needs nothing beside it: one file is the whole report, which is what makes it the one to attach
// to a build's output.
func (b GraphBuilder) ExportAsHTML(path string) error {
	return b.exported("export as html", path, rendering.RenderHTML)
}

// exported is what every `export as <format>` terminal is: render the report through the `to <format>` twin, make
// sure the folder it is going into exists, write the file.
//
// The order matters and it is the reason this is one function rather than six. Nothing touches the disk until the
// whole document has been rendered, so a query that describes nothing, or a pattern that will not compile, leaves
// no file behind at all — where writing as the document was produced would leave a half-written diagram that
// looks like a report of a project with three files in it.
//
// The terminal names itself for the error message, as a modifier does when it rejects a pattern, so that a
// failure says which of the twelve calls to go and look at instead of `export`.
func (b GraphBuilder) exported(terminal, path string, render func(projection.Snapshot) string) error {
	if path == "" {
		return archerror.NewUserError(terminal, path, ErrMissingExportPath)
	}
	document, err := b.rendered(render)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), exportedFolderPermissions); err != nil {
		return archerror.NewTechnicalError("create the folder of the report", path, err)
	}
	if err := os.WriteFile(path, []byte(document), exportedFilePermissions); err != nil {
		return archerror.NewTechnicalError("write the report", path, err)
	}
	return nil
}
