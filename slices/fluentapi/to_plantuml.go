package fluentapi

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/slices/rendering"
)

// The two ways the output terminals of this module can be called without there being a document to hand back.
// Both are reported as an archerror.UserError naming the terminal at fault.
var (
	// ErrNothingToDraw says the slicing found no slice at all, so the diagram would be an empty frame. It is
	// this terminal's shape of the failure assertion.EmptyTestViolation exists for — a pattern that names
	// nothing draws a blank picture, and a blank picture looks exactly like a project with nothing in it.
	//
	// It is an error rather than a violation because a report has no violation list to put one in: the terminal
	// hands back a document. Set AllowEmptyTests on the check options to permit it, the same knob that opts a
	// rule out of the same guard, and the one graph.ErrEmptySnapshot honors for the same reason.
	ErrNothingToDraw = errors.New("no slice to draw")
	// ErrMissingExportPath says `export as plantuml` was given the empty string for a path. A diagram has to be
	// written somewhere, and the working directory is a folder rather than a file, so there is nothing this
	// could have meant.
	ErrMissingExportPath = errors.New("no path to export to")
)

const (
	// toPlantUMLTerminal and exportAsPlantUMLTerminal name the two terminals in the failures they report, so
	// that a message says which call to go and look at.
	toPlantUMLTerminal       = "to plantuml"
	exportAsPlantUMLTerminal = "export as plantuml"
	// exportedFilePermissions are the mode an exported diagram is created with: readable by anyone, writable by
	// its owner. A diagram is an artifact meant to be opened, committed beside the code or attached to a
	// build's output, so the default of a file nobody but its owner can read would be the surprise.
	exportedFilePermissions = 0o644
	// exportedFolderPermissions are the mode a folder created for a diagram is given, which is the file's mode
	// plus the traversal bit a folder is useless without.
	exportedFolderPermissions = 0o755
)

// ToPlantUML draws the slices of the project as a PlantUML component diagram: `to plantuml`.
//
//	diagram, err := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").ToPlantUML(nil)
//
// It is the reverse of the rule `adhere to diagram`: that judges a project by a drawing somebody made, this
// draws the project as it is. Between the two, a diagram is a document this library both reads and writes — so
// the way to start using the rule on a codebase nobody has drawn yet is to export this, read it, delete the
// arrows that should not be there, and check the rest in as the architecture.
//
// The document is a function of the slicing alone, which is why this terminal is on the entry point rather than
// after a mood: a drawing states what a project is, and no rule about it has been written yet. The dialect is
// exactly the one extraction.ParseDiagram reads, and rendering.RenderPlantUML documents what is in the
// document.
//
// A nil *CheckOptions means the defaults, and the bag is what says how the project is read — whether test
// files are part of it, which import kinds are ignored, where the project is. It is an argument here rather
// than a stage of the chain, as it is at every terminal of this module.
//
// The error is the slicing's — a pattern that will not compile, a pattern with no capture in it, no slicing at
// all, a project that will not load — or ErrNothingToDraw when the slicing found no slice, unless
// AllowEmptyTests says an empty picture is what was wanted. When it is non-nil the document is empty: half a
// diagram is worse than none, because a reader would believe it.
func (b SlicesBuilder) ToPlantUML(options *kernel.CheckOptions) (string, error) {
	return b.rendered(toPlantUMLTerminal, options)
}

// ExportAsPlantUML writes the diagram of the project's slices to this path: `export as plantuml`.
//
//	err := archunit.ProjectSlices(nil).
//		DefinedBy("internal/(**)/**").
//		ExportAsPlantUML("docs/architecture.puml", nil)
//
// It is ToPlantUML's file form — the same document, written where a reader will find it — and what the document
// says is documented there. The file it writes is the file AdhereToDiagramInFile reads, so exporting it once and
// committing it is how a project starts being checked against its own architecture.
//
// The path is interpreted like any other path a test writes to, relative to the working directory the test runs
// in, and its folders are created if they are not there yet: a diagram exported into `docs/` should not fail
// because nobody had made `docs/` yet. An existing file is overwritten, because a diagram is the current answer
// about the project and a stale one beside it would be read as a second answer — which is the whole failure
// `adhere to diagram` exists to catch, so it must not be this terminal that causes it.
//
// The error is ToPlantUML's, or ErrMissingExportPath for an empty path, or a technical error naming the file
// when the disk refused it. Nothing is written unless the whole document was rendered, so a failure leaves no
// half-drawn diagram behind for the next reader to trust.
func (b SlicesBuilder) ExportAsPlantUML(path string, options *kernel.CheckOptions) error {
	if path == "" {
		return archerror.NewUserError(exportAsPlantUMLTerminal, path, ErrMissingExportPath)
	}

	document, err := b.rendered(exportAsPlantUMLTerminal, options)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), exportedFolderPermissions); err != nil {
		return archerror.NewTechnicalError("create the folder of the diagram", path, err)
	}
	if err := os.WriteFile(path, []byte(document), exportedFilePermissions); err != nil {
		return archerror.NewTechnicalError("write the diagram", path, err)
	}
	return nil
}

// rendered is what both output terminals are: resolve the slicing against the project, refuse to draw nothing,
// project the dependencies between the slices, render them.
//
// It is one function for the two terminals so that the string form and the file form cannot come to differ in
// what they draw or in what they refuse — the shape GraphBuilder.rendered gives the twelve terminals of the
// graph module. The terminal names itself for the failure it may have to report, because a message saying
// `export as plantuml` is what tells a reader which of the two calls to go and look at.
//
// The check options are threaded through unchanged rather than defaulted here: resolve is what reads the
// project with them, and the one thing this function reads out of the bag is the guard's own knob.
func (b SlicesBuilder) rendered(terminal string, options *kernel.CheckOptions) (string, error) {
	graph, membership, err := b.resolve(options)
	if err != nil {
		return "", err
	}

	if len(membership) == 0 && !options.WithDefaults().AllowEmptyTests {
		// A drawing of nothing is refused instead of being handed over, for the reason every terminal in this
		// library wires in the empty-test guard: a slicing whose pattern has gone stale draws an empty frame,
		// and that is the one failure this library refuses to pass off as an answer.
		return "", archerror.NewUserError(terminal, b.String(), ErrNothingToDraw)
	}

	dependencies := kernelprojection.ProjectEdges(graph, b.mapper())
	return rendering.RenderPlantUML(slices.Sorted(maps.Keys(membership)), dependencies), nil
}
