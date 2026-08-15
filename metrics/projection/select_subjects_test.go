package projection_test

import (
	"slices"
	"testing"

	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

func TestSelectSubjectsWithNoClassSelectorMeasuresEverythingThatWasRead(t *testing.T) {
	// A rule whose scope only named folders and filenames: every file it read is a subject, and so is every
	// class those files declare, because a metric about a class may still be asked for afterwards.
	files := fixtureFiles()

	subjects := projection.SelectSubjects(files)

	if len(subjects.Files) != len(files) {
		t.Errorf("selected %d files, want the %d that were read", len(subjects.Files), len(files))
	}
	want := []string{"internal/api.Handler", "internal/api.Router", "internal/db.Connection"}
	assertClasses(t, subjects, want)
}

func TestSelectSubjectsMatchesAClassSelectorAgainstTheBareName(t *testing.T) {
	// `for classes matching "*Service"` is about the declared name, so the pattern is written without the
	// package while the subject it selects still carries it.
	subjects := projection.SelectSubjects(fixtureFiles(), classnameMatcher(t, "*er"))

	assertClasses(t, subjects, []string{"internal/api.Handler", "internal/api.Router"})
}

func TestSelectSubjectsCombinesItsClassSelectorsWithAnd(t *testing.T) {
	both := projection.SelectSubjects(fixtureFiles(), classnameMatcher(t, "*er"), classnameMatcher(t, "H*"))
	reversed := projection.SelectSubjects(fixtureFiles(), classnameMatcher(t, "H*"), classnameMatcher(t, "*er"))

	assertClasses(t, both, []string{"internal/api.Handler"})
	assertClasses(t, reversed, []string{"internal/api.Handler"})
}

func TestSelectSubjectsNarrowsTheFilesToTheOnesDeclaringAKeptClass(t *testing.T) {
	// A selector the user typed has to change what the rule is about, whichever metric is chosen after it:
	// `for classes matching "Connection"` and then `lines of code` measures the file that declares it, not
	// every file the folder verbs kept.
	subjects := projection.SelectSubjects(fixtureFiles(), classnameMatcher(t, "Connection"))

	if len(subjects.Files) != 1 || subjects.Files[0].Path != "internal/db/conn.go" {
		t.Errorf("selected %+v, want only the file declaring Connection", paths(subjects))
	}
}

func TestSelectSubjectsKeepsTheOrderTheFilesWereReadIn(t *testing.T) {
	// The order of a report is the order of the selection, and inside one file the order the classes were
	// declared in, so the same rule prints the same list twice.
	files := []metricsextraction.FileInfo{
		fixtureFiles()[2],
		fixtureFiles()[0],
	}

	subjects := projection.SelectSubjects(files)

	if len(subjects.Files) != 2 || subjects.Files[0].Path != "internal/db/conn.go" {
		t.Errorf("selected %v, want the order the files arrived in", paths(subjects))
	}
	assertClasses(t, subjects, []string{"internal/db.Connection", "internal/api.Handler", "internal/api.Router"})

	// The same property in the branch that narrows the files to the ones declaring a kept class, which is the
	// only one that goes through a map: `*o*` keeps Connection and Router, declared in two different files, so
	// a selection reading the map back instead of the files it was given would report them in either order.
	narrowed := projection.SelectSubjects(files, classnameMatcher(t, "*o*"))

	want := []string{"internal/db/conn.go", "internal/api/handler.go"}
	if got := paths(narrowed); !slices.Equal(got, want) {
		t.Errorf("selected %v, want the order the files arrived in, %v", got, want)
	}
	assertClasses(t, narrowed, []string{"internal/db.Connection", "internal/api.Router"})
}

func TestSelectSubjectsThatMatchNoClassMeasureNothing(t *testing.T) {
	// Zero matches is an ordinary answer here — whether an empty selection is a failure is the empty-test
	// guard's question — but a rule that named classes and found none must not fall back to every file.
	subjects := projection.SelectSubjects(fixtureFiles(), classnameMatcher(t, "Missing"))

	if len(subjects.Classes) != 0 {
		t.Errorf("selected %d classes, want none", len(subjects.Classes))
	}
	if len(subjects.Files) != 0 {
		t.Errorf("selected %v, want no file measured either", paths(subjects))
	}
}

func TestSelectSubjectsOfNoFilesMeasuresNothing(t *testing.T) {
	subjects := projection.SelectSubjects(nil)

	if len(subjects.Files) != 0 || len(subjects.Classes) != 0 {
		t.Errorf("selected %+v, want nothing", subjects)
	}
}

func TestSelectSubjectsDoesNotShareItsFilesWithTheCallersSlice(t *testing.T) {
	// A projection handed to a report must not change when the caller reuses the slice it passed in.
	files := fixtureFiles()

	subjects := projection.SelectSubjects(files)
	files[0].LinesOfCode = 999

	if subjects.Files[0].LinesOfCode == 999 {
		t.Error("the selection changed with the caller's slice, want a copy of it")
	}
}

// fixtureFiles are three read files in the shape metrics/extraction describes them, hand-built so that the
// meaning of a scope can be tested without parsing anything.
func fixtureFiles() []metricsextraction.FileInfo {
	return []metricsextraction.FileInfo{
		{
			Path: "internal/api/handler.go", Directory: "internal/api",
			LinesOfCode: 40, StatementCount: 12, ImportCount: 3, FunctionCount: 1,
			Classes: []metricsextraction.ClassInfo{
				{Name: "Handler", Identifier: "internal/api.Handler", Path: "internal/api/handler.go", FieldCount: 2, MethodCount: 4},
				{Name: "Router", Identifier: "internal/api.Router", Path: "internal/api/handler.go", Interface: true, MethodCount: 1},
			},
		},
		{
			Path: "internal/api/empty.go", Directory: "internal/api",
			LinesOfCode: 3, StatementCount: 0, ImportCount: 0, FunctionCount: 0,
		},
		{
			Path: "internal/db/conn.go", Directory: "internal/db",
			LinesOfCode: 20, StatementCount: 6, ImportCount: 1, FunctionCount: 2,
			Classes: []metricsextraction.ClassInfo{
				{Name: "Connection", Identifier: "internal/db.Connection", Path: "internal/db/conn.go", FieldCount: 1, MethodCount: 3},
			},
		},
	}
}

// assertClasses checks which classes a selection is measured over, by identifier and in order.
func assertClasses(t *testing.T, subjects projection.Subjects, want []string) {
	t.Helper()

	if len(subjects.Classes) != len(want) {
		t.Fatalf("selected %d classes (%+v), want %v", len(subjects.Classes), subjects.Classes, want)
	}
	for index, identifier := range want {
		if subjects.Classes[index].Identifier != identifier {
			t.Errorf("class %d is %q, want %q", index, subjects.Classes[index].Identifier, identifier)
		}
	}
}

// paths names the files a selection is measured over, for a failure message.
func paths(subjects projection.Subjects) []string {
	selected := make([]string, 0, len(subjects.Files))
	for _, file := range subjects.Files {
		selected = append(selected, file.Path)
	}
	return selected
}
