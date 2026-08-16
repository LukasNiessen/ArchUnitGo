package fluentapi_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

func TestEveryTerminalOfTheMetricsModuleLogsWhatEachOfItsStepsCameToAndEveryNumberItRead(t *testing.T) {
	// The counts and the numbers, over the fixture project this package measures, whose every count is known:
	// four files measured, and one `metric` record for each of them. One record per measurement is the whole
	// contract of that half of the log — a rule whose numbers a reader is looking for is a rule that measured
	// more than one thing — so the records are held against the four values the fixture was written with
	// rather than against the first of them.
	locator := measuredProject(t)

	for _, family := range []struct {
		name     string
		rule     kernel.Checkable
		progress []string
		metrics  []string
	}{
		{
			name: "should be below",
			rule: fluentapi.Metrics(locator).Count().LinesOfCode().ShouldBeBelow(400),
			progress: []string{
				"debug progress: measurements: 4",
			},
			metrics: []string{
				"info  metric: internal/api/handler.go: lines of code = 9",
				"info  metric: internal/api/router.go: lines of code = 4",
				"info  metric: internal/db/conn.go: lines of code = 3",
				"info  metric: main.go: lines of code = 3",
			},
		},
		{
			name: "should satisfy",
			rule: fluentapi.Metrics(locator).Count().Statements().
				ShouldSatisfy(func(measurement calculation.Measurement, _ metricsextraction.ClassInfo) bool {
					return measurement.Value < 400
				}, "hold fewer than 400 statements"),
			progress: []string{
				"debug progress: measurements: 4",
			},
			metrics: []string{
				"info  metric: internal/api/handler.go: statements = 3",
				"info  metric: internal/api/router.go: statements = 0",
				"info  metric: internal/db/conn.go: statements = 0",
				"info  metric: main.go: statements = 1",
			},
		},
		{
			// The three folders of the fixture, and no `metric` record: a zone check is about where a package
			// sits rather than about a number the rule named, so it has none to write.
			name:     "should not be in zone of uselessness",
			rule:     fluentapi.Metrics(locator).Distance().ShouldNotBeInZoneOfUselessness(),
			progress: []string{"debug progress: selected components: 3"},
			metrics:  []string{},
		},
	} {
		t.Run(family.name, func(t *testing.T) {
			if records := loggedRecords(t, family.rule, "debug progress: "); !slices.Equal(records, family.progress) {
				t.Errorf("%v logged\n%s\nwant\n%s", family.rule,
					strings.Join(records, "\n"), strings.Join(family.progress, "\n"))
			}
			if records := loggedRecords(t, family.rule, "info  metric: "); !slices.Equal(records, family.metrics) {
				t.Errorf("%v logged\n%s\nwant\n%s", family.rule,
					strings.Join(records, "\n"), strings.Join(family.metrics, "\n"))
			}
		})
	}
}

// loggedRecords are the records of one shape a rule wrote while it ran, in the order it wrote them and as
// whole lines: the prefix is the level and the vocabulary word a record of that shape opens with, `debug
// progress: ` for the steps a check took and `info  metric: ` for the numbers it read.
//
// The log is turned all the way up, because progress is the quietest of the five records, and it is read back
// unedited — a step's count is half of what its record says, and a measurement's value is the whole of what
// its own record is for.
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
