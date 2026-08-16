package fluentapi_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/logging"
	filesextraction "github.com/LukasNiessen/ArchUnitGo/files/extraction"
	"github.com/LukasNiessen/ArchUnitGo/files/fluentapi"
)

func TestEveryTerminalOfTheFilesModuleLogsWhatEachOfItsStepsCameTo(t *testing.T) {
	// The counts, and not only the steps: half of what a `progress` record says is the number, because the
	// question a log is opened for is *how much* of the project a step came to. So each row holds the whole of
	// what its terminal wrote, byte for byte, over a fixture project this package writes itself — a scope of
	// this repository could not be asserted that way, since every count of it moves with the next commit.
	fixture := fixtureLocator(t, writeFixtureProject(t))
	cyclic := fixtureLocator(t, writeCyclicFixtureProject(t))
	external := fixtureLocator(t, writeExternalFixtureProject(t))

	for _, family := range []struct {
		name string
		rule kernel.Checkable
		want []string
	}{
		{
			// The four files of the fixture project, the three dependencies between them — main.go on both files
			// of the api package, the handler on the database — and no circle among them.
			name: "have no cycles",
			rule: fluentapi.ProjectFiles(fixture).Should().HaveNoCycles(),
			want: []string{
				"debug progress: selected files: 4",
				"debug progress: dependencies between the selected files: 3",
				"debug progress: cycles: 0",
			},
		},
		{
			// The same three steps over the project that has a cycle, which is what tells the last two counts
			// apart: the three files under internal/ hold three dependencies — the handler on both files of the
			// database package, the connection back on the handler — and exactly one of those is a circle.
			name: "have no cycles, over a project that has one",
			rule: fluentapi.ProjectFiles(cyclic).InFolder("internal/**").Should().HaveNoCycles(),
			want: []string{
				"debug progress: selected files: 3",
				"debug progress: dependencies between the selected files: 3",
				"debug progress: cycles: 1",
			},
		},
		{
			// Two selected files, three files to depend on, and the one dependency between the two populations:
			// the handler on the database's connection. The three counts differ, so the two adjacent populations
			// cannot be reported the wrong way round.
			name: "depend on files",
			rule: fluentapi.ProjectFiles(fixture).InFolder("internal/api/**").
				ShouldNot().DependOnFiles().InFolder("internal/**"),
			want: []string{
				"debug progress: selected files: 2",
				"debug progress: files to depend on: 3",
				"debug progress: dependencies from the selected files to the object's: 1",
			},
		},
		{
			name: "have name",
			rule: fluentapi.ProjectFiles(fixture).InFolder("internal/**").Should().HaveName("*.go"),
			want: []string{"debug progress: selected files: 3"},
		},
		{
			name: "adhere to",
			rule: fluentapi.ProjectFiles(fixture).InFolder("internal/api/**").
				Should().AdhereTo(func(file filesextraction.FileInfo) bool {
				return strings.HasPrefix(file.Source, "package ")
			}, "begin with its package clause"),
			want: []string{
				"debug progress: selected files: 2",
				"debug progress: source files read: 2",
			},
		},
		{
			// The two files of the api package, the three modules the whole project depends on — fmt, net/http
			// and database/sql, discovered from the graph rather than from the selection — and the one edge from
			// the selection to one of them.
			name: "depend on external modules",
			rule: fluentapi.ProjectFiles(external).InFolder("internal/api/**").
				ShouldNot().DependOnExternalModules(),
			want: []string{
				"debug progress: selected files: 2",
				"debug progress: external modules matched: 3",
				"debug progress: dependencies from the selected files to those modules: 1",
			},
		},
	} {
		t.Run(family.name, func(t *testing.T) {
			if records := loggedRecords(t, family.rule, "debug progress: "); !slices.Equal(records, family.want) {
				t.Errorf("%v logged\n%s\nwant\n%s", family.rule,
					strings.Join(records, "\n"), strings.Join(family.want, "\n"))
			}
		})
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
