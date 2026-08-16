package fluentapi_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
)

func TestThePolicyTerminalLogsWhatEachOfItsStepsCameTo(t *testing.T) {
	// The counts, and not only the steps: half of what a `progress` record says is the number, and the two a
	// policy reports first come out of two near-identical expressions — the layers it declared and the clauses
	// it was written with — so a log that swapped them would read as a policy nobody wrote. They are asserted
	// over the fixture project of this package, whose shape the tests fix.
	//
	// Three layers, two clauses, and three dependencies between the layers: the api on the domain and on the
	// database, the domain on the database. main.go is in no declared layer, so the edge it is the source of is
	// not one of them.
	policy := fixturePolicy(t, writeLayeredFixtureProject(t)).
		WhereLayer("api").MayOnlyDependOnLayers("domain").
		WhereLayer("domain").MayNotDependOnLayers("db")

	want := []string{
		"debug progress: declared layers: 3",
		"debug progress: clauses: 2",
		"debug progress: dependencies between the layers: 3",
	}
	if records := loggedRecords(t, policy, "debug progress: "); !slices.Equal(records, want) {
		t.Errorf("%s logged\n%s\nwant\n%s", policy, strings.Join(records, "\n"), strings.Join(want, "\n"))
	}
}

// loggedRecords are the records of one shape a rule wrote while it ran, in the order it wrote them and as
// whole lines: the prefix is the level and the vocabulary word a record of that shape opens with, `debug
// progress: ` for the steps a check took.
//
// The log is turned all the way up, because progress is the quietest of the five records, and it is read back
// unedited — a step's count is half of what its record says, and a test that compares the steps alone leaves
// every number this module logs unpinned.
func loggedRecords(t *testing.T, rule kernel.Checkable, prefix string) []string {
	t.Helper()

	log := &bytes.Buffer{}
	if _, err := rule.Check(&kernel.CheckOptions{
		Logging: &logging.Options{Writer: log, Level: logging.LevelDebug},
	}); err != nil {
		t.Fatalf("%v failed: %v", rule, err)
	}

	records := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(log.String(), "\n"), "\n") {
		if strings.HasPrefix(line, prefix) {
			records = append(records, line)
		}
	}
	return records
}
