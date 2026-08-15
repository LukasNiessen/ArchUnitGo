package fluentapi

import (
	"errors"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

// ErrEmptySnapshot says the query described a report with no node in it, which is this module's shape of the
// failure assertion.EmptyTestViolation exists for: a pattern that names nothing renders a blank diagram, and
// a blank diagram looks exactly like a project that is clean.
//
// It is an error rather than a violation because a report has no violation list to put one in — the terminal
// hands back an artifact — and it is reported at all because the alternative is a stale glob quietly
// producing an empty picture for a year. Set AllowEmptyTests on the check options to permit it, the same knob
// that opts a rule out of the same guard.
var ErrEmptySnapshot = errors.New("no node in the snapshot")

// Snapshot builds the report: the nodes the query describes, the dependencies between them, the counts that
// summarize both, and the title it is rendered under.
//
//	snapshot, err := archunit.ProjectGraph(nil).CollapseToFolderDepth(2).Snapshot()
//	fmt.Println(snapshot.Summary())
//
// It is the terminal of this module and the only stage of the chain that reads anything: the project is
// located, extracted and projected here, and by nothing else. `with check options` is how it is told to do
// that differently, which is why it takes no argument of its own — a rendered diagram and a file written to
// disk will be terminals over the same query, and the bag they share belongs on the chain rather than
// repeated at each of them.
//
// It is also the seam every output format is written against. Rendering is a function of the returned
// projection.Snapshot alone, so a format added later needs nothing from this package, and a modifier added to
// this package needs nothing from a format.
//
// The error is technical or the user's — a pattern a modifier could not compile, a group without a name, a
// folder depth below one, a locator naming no Go project, a project that will not load — or ErrEmptySnapshot
// when the query described nothing. When it is non-nil the snapshot is the zero value and says nothing. There
// is no third possibility here: a report judges no code, so it has no failure of its own to report.
func (b GraphBuilder) Snapshot() (projection.Snapshot, error) {
	if b.err != nil {
		return projection.Snapshot{}, b.err
	}

	check := b.check.WithDefaults()
	graph, err := check.ExtractGraph(b.locator)
	if err != nil {
		return projection.Snapshot{}, err
	}

	query := b.query.WithDefaults()
	snapshot := projection.ProjectSnapshot(graph, &query)
	if snapshot.Empty() && !check.AllowEmptyTests {
		// A report of nothing is reported instead of being rendered, for the reason every terminal in this
		// library wires in the empty-test guard: a query whose patterns have gone stale draws a clean
		// project, and that is the one failure this library refuses to pass off as a pass.
		return projection.Snapshot{}, archerror.NewUserError("snapshot", b.String(), ErrEmptySnapshot)
	}
	return snapshot, nil
}
