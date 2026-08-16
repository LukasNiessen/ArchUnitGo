package extraction

import (
	"os"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
)

// ExtractDiagram reads the component diagram written in this file.
//
//	diagram, err := extraction.ExtractDiagram("docs/architecture.puml")
//
// It is ParseDiagram over the contents of a file, and that is the whole of it: the dialect, the grammar
// and the refusals are documented there, and a diagram in a file means exactly what the same diagram
// means as a string. A diagram belongs in a file once more than one test asks about it, or once somebody
// other than a test reader is meant to look at it — `.puml` is what such a file is conventionally
// called, and this library neither requires nor checks the extension.
//
// The path is interpreted like any other path a test reads, relative to the working directory the test
// runs in.
//
// A file that cannot be read is an archerror.TechnicalError naming it — the environment failed, and
// there is nothing in the rule to fix. A file that can be read but is not a diagram comes back as
// ParseDiagram's own error, unchanged: that one is the user's, so the stage of the chain that was given
// the path is what names itself around it.
func ExtractDiagram(path string) (Diagram, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return Diagram{}, archerror.NewTechnicalError("read the diagram file", path, err)
	}
	return ParseDiagram(string(text))
}
