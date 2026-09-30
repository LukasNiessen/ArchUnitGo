package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
)

func TestInspectionIsDebugOnlyAndQuotesIdentifiers(t *testing.T) {
	graph := extraction.NewGraph(
		extraction.SelfEdge("api/handler.go"),
		extraction.NewEdge("api/handler.go", "database/sql", true, extraction.ImportKindPlain),
	)
	for _, level := range []logging.Level{logging.LevelDebug, logging.LevelInfo, logging.LevelWarn, logging.LevelError} {
		var output bytes.Buffer
		log, err := logging.NewLogger(&logging.Options{Writer: &output, Level: level})
		if err != nil {
			t.Fatal(err)
		}
		log.LogGraph(graph)
		log.LogSelection("selected file", []string{"name\nwith-newline.go"})
		log.LogMembership("layer", map[string][]string{"z": {"z.go"}, "a": {"a.go"}})
		if err := log.Close(); err != nil {
			t.Fatal(err)
		}
		if level != logging.LevelDebug {
			if output.Len() != 0 {
				t.Fatalf("inspection escaped threshold %v: %s", level, &output)
			}
			continue
		}
		for _, want := range []string{`discovered file: "api/handler.go"`, `dependency: "api/handler.go" -> "database/sql"`, `external=true`, `name\nwith-newline.go`} {
			if !strings.Contains(output.String(), want) {
				t.Errorf("missing %q in %s", want, &output)
			}
		}
		if strings.Index(output.String(), `layer "a"`) > strings.Index(output.String(), `layer "z"`) {
			t.Fatal("membership labels must be ordered")
		}
	}
	var disabled *logging.Logger
	disabled.LogGraph(graph)
	disabled.LogSelection("selected file", nil)
	disabled.LogMembership("layer", nil)
}
