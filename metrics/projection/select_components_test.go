package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
	"github.com/LukasNiessen/ArchUnitGo/metrics/projection"
)

func TestSelectComponentsCountsTheTypesOfEveryFileOfAFolder(t *testing.T) {
	// A component is a folder, so its numbers are the sum over the files the rule selected in it — including
	// the ones that declare nothing, which is what makes a package of mostly plumbing look as concrete as it
	// is.
	components := projection.SelectComponents(fixtureGraph(), fixtureFiles())

	want := []projection.Component{
		{Label: "internal/api", Classes: 2, Interfaces: 1, DependsOn: []string{"internal/db"}},
		{Label: "internal/db", Classes: 1, DependedOnBy: []string{"internal/api"}},
	}
	assertComponents(t, components, want)
}

func TestSelectComponentsReadsTheCouplingFromBothSidesAtOnce(t *testing.T) {
	// Ce of the one end is Ca of the other, and reading both off one projection is what stops them from
	// disagreeing: no test could tell them apart afterwards, and every distance metric is a ratio between the
	// two.
	components := projection.SelectComponents(fixtureGraph(), fixtureFiles())

	if len(components) != 2 {
		t.Fatalf("selected %+v, want the two folders of the fixture", components)
	}
	depending, dependedOn := components[0], components[1]
	if !slices.Equal(depending.DependsOn, []string{dependedOn.Label}) {
		t.Errorf("%s depends on %v, want [%s]", depending.Label, depending.DependsOn, dependedOn.Label)
	}
	if !slices.Equal(dependedOn.DependedOnBy, []string{depending.Label}) {
		t.Errorf("%s is depended on by %v, want [%s]", dependedOn.Label, dependedOn.DependedOnBy, depending.Label)
	}
}

func TestSelectComponentsCountsEachNeighbourOnce(t *testing.T) {
	// Coupling is between packages and not between files, so two files of one folder reaching for the same
	// other folder is one dependency. Ce is the length of this list, and a neighbor counted twice would make
	// a package look twice as unstable as it is.
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/router.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
	)
	files := []metricsextraction.FileInfo{
		{Path: "internal/api/handler.go", Directory: "internal/api"},
		{Path: "internal/api/router.go", Directory: "internal/api"},
		{Path: "internal/db/conn.go", Directory: "internal/db"},
	}

	components := projection.SelectComponents(graph, files)

	want := []projection.Component{
		{Label: "internal/api", DependsOn: []string{"internal/db"}},
		{Label: "internal/db", DependedOnBy: []string{"internal/api"}},
	}
	assertComponents(t, components, want)
}

func TestSelectComponentsGathersEveryNeighbourAndEveryInterfaceOfAComponent(t *testing.T) {
	// The three numbers a component accumulates are Ce, Ca and the interfaces, and each of them is a sum that
	// only a second item can tell from an overwrite: `internal/api` depends on two components, is depended on
	// by two, and declares three types of which two are interfaces. A component that kept the last neighbor
	// it saw instead of all of them would cap Ce and Ca at 1 — the whole of instability and of the coupling
	// factor — and a file whose second interface went uncounted would look half as abstract as it is, which is
	// what hides a package in the zone of uselessness.
	graph := extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("cmd/serve/main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/cache/store.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("cmd/serve/main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/cache/store.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
	)
	files := []metricsextraction.FileInfo{
		{Path: "main.go", Directory: "."},
		{Path: "cmd/serve/main.go", Directory: "cmd/serve"},
		{
			Path: "internal/api/handler.go", Directory: "internal/api",
			Classes: []metricsextraction.ClassInfo{
				{Name: "Handler", Identifier: "internal/api.Handler", Path: "internal/api/handler.go"},
				{Name: "Router", Identifier: "internal/api.Router", Path: "internal/api/handler.go", Interface: true},
				{Name: "Store", Identifier: "internal/api.Store", Path: "internal/api/handler.go", Interface: true},
			},
		},
		{Path: "internal/cache/store.go", Directory: "internal/cache"},
		{Path: "internal/db/conn.go", Directory: "internal/db"},
	}

	components := projection.SelectComponents(graph, files)

	want := []projection.Component{
		{Label: ".", DependsOn: []string{"internal/api"}},
		{Label: "cmd/serve", DependsOn: []string{"internal/api"}},
		{
			Label: "internal/api", Classes: 3, Interfaces: 2,
			DependsOn:    []string{"internal/cache", "internal/db"},
			DependedOnBy: []string{".", "cmd/serve"},
		},
		{Label: "internal/cache", DependedOnBy: []string{"internal/api"}},
		{Label: "internal/db", DependedOnBy: []string{"internal/api"}},
	}
	assertComponents(t, components, want)

	// The numbers those lists are read as, so that the reading and its consequence are pinned together: two
	// interfaces of three types is an abstractness of 2/3, two efferent couplings against two afferent an
	// instability of 0.5, and four couplings out of the eight a component of five could carry a coupling factor
	// of 0.5 — the three every zone check is a region of.
	api := components[2]
	if got := calculation.AbstractnessOf(api); got != 2.0/3.0 {
		t.Errorf("the abstractness of %s is %g, want %g from two interfaces among three types", api.Label, got, 2.0/3.0)
	}
	if got := calculation.InstabilityOf(api); got != 0.5 {
		t.Errorf("the instability of %s is %g, want 0.5 from two couplings each way", api.Label, got)
	}
	factors := calculation.CouplingFactor().Measure(projection.Subjects{Components: components})
	if got := factors[2].Value; got != 0.5 {
		t.Errorf("the coupling factor of %s is %g, want 0.5 from four of the eight couplings it could carry",
			factors[2].Subject, got)
	}
}

func TestSelectComponentsKeepsAFolderWithNoCouplingAtAll(t *testing.T) {
	// A package the scope selected is a package the rule is about, so it is measured rather than dropped: a
	// folder nothing imports and that imports nothing is the most stable, most concrete component there is,
	// which is exactly what the zone of pain is about.
	files := []metricsextraction.FileInfo{{Path: "internal/util/text.go", Directory: "internal/util"}}

	components := projection.SelectComponents(nil, files)

	want := []projection.Component{{Label: "internal/util"}}
	assertComponents(t, components, want)
}

func TestSelectComponentsNamesTheProjectRootAsAFolder(t *testing.T) {
	// The root is `.`, as it is everywhere in this library, so `in folder "."` selects the component a rule
	// about the top-level package means.
	files := []metricsextraction.FileInfo{
		{Path: "main.go", Directory: "."},
		{Path: "internal/api/handler.go", Directory: "internal/api"},
	}

	components := projection.SelectComponents(fixtureGraph(), files)

	want := []projection.Component{
		{Label: ".", DependsOn: []string{"internal/api"}},
		{Label: "internal/api", DependedOnBy: []string{"."}},
	}
	assertComponents(t, components, want)
}

func TestSelectComponentsSortsItsResultByLabel(t *testing.T) {
	// The folders are summed through a map, so the order has to be re-established: a report of a project's
	// numbers must not come out shuffled between two runs of the same rule.
	files := []metricsextraction.FileInfo{
		{Path: "internal/db/conn.go", Directory: "internal/db"},
		{Path: "main.go", Directory: "."},
		{Path: "internal/api/handler.go", Directory: "internal/api"},
	}

	labels := make([]string, 0, len(files))
	for _, component := range projection.SelectComponents(nil, files) {
		labels = append(labels, component.Label)
	}

	if want := []string{".", "internal/api", "internal/db"}; !slices.Equal(labels, want) {
		t.Errorf("selected %v, want %v", labels, want)
	}
}

func TestSelectComponentsOfNoFilesSelectsNothing(t *testing.T) {
	// Not an error: whether a rule that selected nothing is a failure is the empty-test guard's question, and
	// a terminal asks it before anything is judged.
	if components := projection.SelectComponents(fixtureGraph(), nil); len(components) != 0 {
		t.Errorf("selected %+v, want nothing", components)
	}
}

// assertComponents checks a whole selection field by field, in order, so a test says which numbers a component
// came to rather than only that it exists.
func assertComponents(t *testing.T, got, want []projection.Component) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("selected %d components (%+v), want %d (%+v)", len(got), got, len(want), want)
	}
	for index, wanted := range want {
		component := got[index]
		if component.Label != wanted.Label {
			t.Errorf("component %d is %q, want %q", index, component.Label, wanted.Label)
			continue
		}
		if component.Classes != wanted.Classes || component.Interfaces != wanted.Interfaces {
			t.Errorf("%s declares %d types, %d of them interfaces, want %d and %d",
				component.Label, component.Classes, component.Interfaces, wanted.Classes, wanted.Interfaces)
		}
		if !slices.Equal(component.DependsOn, wanted.DependsOn) {
			t.Errorf("%s depends on %v, want %v", component.Label, component.DependsOn, wanted.DependsOn)
		}
		if !slices.Equal(component.DependedOnBy, wanted.DependedOnBy) {
			t.Errorf("%s is depended on by %v, want %v", component.Label, component.DependedOnBy, wanted.DependedOnBy)
		}
	}
}
