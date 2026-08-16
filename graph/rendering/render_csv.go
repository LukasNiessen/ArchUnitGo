package rendering

import (
	"strconv"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// RenderCSV renders the snapshot as one comma-separated table, which is the format to reach for when the report
// is to be counted rather than looked at — sorted in a spreadsheet, grouped by a script, diffed between two
// commits to see which coupling grew:
//
//	kind,source,target,dependencies,external,import kinds
//	node,internal/api,,,false,
//	node,net/http,,,true,
//	edge,internal/api,internal/db,6,false,"plain, blank"
//
// The nodes are rows of the same table as the dependencies, told apart by the first column, and that is the one
// decision in this format worth explaining. A table of arrows alone would silently drop every node that has no
// arrow on it — and a file that depends on nothing and that nothing depends on is a real answer, often the
// interesting one, which is exactly why projection.Snapshot carries its nodes separately from its edges. Two
// tables in one file would not be a CSV, so one table with a `kind` column is what carries both.
//
// A node row leaves the columns that are about an arrow empty rather than writing a zero in them: a node stands
// for no dependencies, and `0` in the dependencies column would read as an arrow that stands for none.
//
// The title has nowhere to go here, and it is dropped rather than smuggled in. A headline above the header row,
// or a `#` comment line, stops the file being a table that a spreadsheet or a script can read — which is the
// whole reason to ask for this format.
//
// The result is quoted the way RFC 4180 says, ends in a newline, and holds no timestamp, so exporting the same
// report twice writes the same file.
func RenderCSV(snapshot projection.Snapshot) string {
	nodes, edges := snapshot.Nodes(), snapshot.Edges()
	rows := make([][]string, 0, len(nodes)+len(edges)+1)
	rows = append(rows, []string{"kind", "source", "target", "dependencies", "external", "import kinds"})
	for _, node := range nodes {
		rows = append(rows, []string{"node", node.Label(), "", "", strconv.FormatBool(node.IsExternal()), ""})
	}
	for _, edge := range edges {
		rows = append(rows, []string{
			"edge",
			edge.SourceLabel(),
			edge.TargetLabel(),
			strconv.Itoa(edge.Count()),
			strconv.FormatBool(edge.IsExternal()),
			strings.Join(importKindNames(edge), ", "),
		})
	}

	lines := make([]string, 0, len(rows)+1)
	for _, row := range rows {
		fields := make([]string, 0, len(row))
		for _, field := range row {
			fields = append(fields, csvField(field))
		}
		lines = append(lines, strings.Join(fields, ","))
	}
	return strings.Join(append(lines, ""), "\n")
}

// csvField is one cell as RFC 4180 wants it written: a cell holding a comma, a double quote, a carriage return
// or a newline is wrapped in double quotes with its own doubled, and anything else is written as it is.
//
// It is four lines of this package rather than a call to encoding/csv, and the reason is the error. A csv.Writer
// over a strings.Builder cannot fail, so writing through one would leave a branch in this file that no test can
// reach — inside a renderer whose signature says it cannot fail. Quoting a field is small enough to own, and
// this way the format has no error to swallow.
func csvField(value string) string {
	if !strings.ContainsAny(value, ",\"\r\n") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
