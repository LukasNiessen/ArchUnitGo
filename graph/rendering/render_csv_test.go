package rendering_test

import (
	"encoding/csv"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
	"github.com/LukasNiessen/ArchUnitGo/graph/rendering"
)

func TestRenderCSVRendersTheWholeReportAsOneTable(t *testing.T) {
	want := `kind,source,target,dependencies,external,import kinds
node,internal/api,,,false,
node,internal/db,,,false,
node,net/http,,,true,
edge,internal/api,internal/db,6,false,"plain, blank"
edge,internal/api,net/http,1,true,plain
`

	if got := rendering.RenderCSV(fixtureSnapshot()); got != want {
		t.Errorf("RenderCSV() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderCSVIsATableTheStandardLibraryCanRead(t *testing.T) {
	// The format's whole reason to exist is a reader that is a spreadsheet or a script, so it is asserted through
	// a parser rather than only as a string: every row has the same number of fields and the header names them.
	rows, err := csv.NewReader(strings.NewReader(rendering.RenderCSV(fixtureSnapshot()))).ReadAll()
	if err != nil {
		t.Fatalf("the table does not parse as a CSV: %v", err)
	}

	header := []string{"kind", "source", "target", "dependencies", "external", "import kinds"}
	if len(rows) == 0 || !slices.Equal(rows[0], header) {
		t.Fatalf("the header row is %v, want %v", rows, header)
	}
	if len(rows) != 6 {
		t.Errorf("the table has %d rows, want the header, the three nodes and the two dependencies", len(rows))
	}
	if got := rows[4]; !slices.Equal(got, []string{"edge", "internal/api", "internal/db", "6", "false", "plain, blank"}) {
		t.Errorf("the aggregated dependency is the row %v, want its count and both of its import kinds", got)
	}
}

func TestRenderCSVCarriesTheNodesAndTheDependenciesInTheSameTable(t *testing.T) {
	// A table of arrows alone would silently drop a node with no arrow on it, and a file that depends on nothing
	// is a real answer. Two tables in one file would not be a CSV, so the first column tells the two kinds apart.
	snapshot := projection.NewSnapshot("", []projection.Node{projection.NewNode("internal/util/log.go")}, nil)

	if got, want := rendering.RenderCSV(snapshot), "node,internal/util/log.go,,,false,\n"; !strings.HasSuffix(got, want) {
		t.Errorf("RenderCSV() =\n%s\nwant the isolated node as the row %q", got, want)
	}
}

func TestRenderCSVQuotesACellThatWouldOtherwiseSplitTheRow(t *testing.T) {
	// A label is whatever a folder may be called, and a comma in one would move every column after it — while a
	// line break in one would split a single row in two, which is the same table saying something else entirely.
	snapshot := projection.NewSnapshot("", []projection.Node{
		projection.NewNode("odd,name"),
		projection.NewNode(`quoted"name`),
		projection.NewNode("line\nbreak"),
		projection.NewNode("carriage\rreturn"),
	}, nil)

	document := rendering.RenderCSV(snapshot)

	if !strings.Contains(document, `node,"odd,name",,,false,`) {
		t.Errorf("RenderCSV() =\n%s\nwant the label holding a comma quoted", document)
	}
	if !strings.Contains(document, `node,"quoted""name",,,false,`) {
		t.Errorf("RenderCSV() =\n%s\nwant the quote in the label doubled", document)
	}
	if !strings.Contains(document, "node,\"line\nbreak\",,,false,") {
		t.Errorf("RenderCSV() =\n%q\nwant the label holding a line break quoted", document)
	}
	if !strings.Contains(document, "node,\"carriage\rreturn\",,,false,") {
		t.Errorf("RenderCSV() =\n%q\nwant the label holding a carriage return quoted", document)
	}
	rows, err := csv.NewReader(strings.NewReader(document)).ReadAll()
	if err != nil {
		t.Fatalf("the table does not parse as a CSV: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("the table reads back as %d rows, want the header and the four nodes: %q", len(rows), rows)
	}
}

func TestRenderCSVRendersASnapshotWithNothingInItAsTheHeaderAlone(t *testing.T) {
	// A table with no row is still a table, and a reader of it learns that the report was empty rather than that
	// the export failed.
	want := "kind,source,target,dependencies,external,import kinds\n"

	if got := rendering.RenderCSV(projection.Snapshot{}); got != want {
		t.Errorf("RenderCSV() = %q, want %q", got, want)
	}
}
