package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

func TestPerExternalDependencyEdgeKeepsTheDependenciesFromTheFilesToTheModules(t *testing.T) {
	// The projection `project files, in folder "internal/db/**", should not, depend on external modules,
	// matching "gorm.io/**"` is judged over: the dependencies those files have on those modules, and nothing
	// else — neither the modules the rest of the project depends on nor the other modules those same files do.
	graph := externalFixtureGraph()
	db := projection.SelectFiles(graph, folderMatcher(t, "internal/db/**"))
	gorm := projection.SelectExternalModules(graph, pathMatcher(t, "gorm.io/**"))

	projected := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(db, gorm))

	want := []string{
		"internal/db/conn.go -> gorm.io/gorm",
		"internal/db/conn.go -> gorm.io/gorm/clause",
	}
	if dependencies := edgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the dependencies of %v on %v are %v, want %v", db, gorm, dependencies, want)
	}
}

func TestPerExternalDependencyEdgeTestsMembershipAtBothEnds(t *testing.T) {
	// Two populations, two membership tests: a file outside the scope importing a named module is not this
	// rule's business, and a scoped file importing an unnamed module is not either. Dropping either test would
	// make `internal/api/**` should not depend on `gorm.io/**` report the db folder's imports, or the pq ones.
	graph := externalFixtureGraph()
	api := projection.SelectFiles(graph, folderMatcher(t, "internal/api/**"))
	gorm := projection.SelectExternalModules(graph, pathMatcher(t, "gorm.io/**"))

	// The api folder depends on external modules, and something depends on gorm — but not this pair.
	projected := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(api, gorm))

	if len(projected) != 0 {
		t.Errorf("the dependencies of %v on %v are %v, want none: neither end alone is enough",
			api, gorm, edgeStrings(projected))
	}
}

func TestPerExternalDependencyEdgeProjectsNothingWhenEitherPopulationIsEmpty(t *testing.T) {
	// The two empty populations mean different things and project the same nothing. No subject is the loud
	// direction the empty-test guard reports on; no module is the quiet one — a project that depends on none of
	// the modules a rule names is what a rule forbidding them asks for, and reports nothing.
	graph := externalFixtureGraph()
	every := projection.SelectFiles(graph)
	modules := projection.SelectExternalModules(graph)

	populations := []struct {
		name  string
		from  []string
		toMod []string
	}{
		{name: "no subject", from: nil, toMod: modules},
		{name: "no module", from: every, toMod: []string{}},
		{name: "neither", from: []string{}, toMod: nil},
		{
			name:  "a module the project does not depend on",
			from:  every,
			toMod: projection.SelectExternalModules(graph, pathMatcher(t, "github.com/deprecated/**")),
		},
	}

	for _, population := range populations {
		t.Run(population.name, func(t *testing.T) {
			projected := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(population.from, population.toMod))

			if len(projected) != 0 {
				t.Errorf("the projection of %v -> %v is %v, want nothing projected",
					population.from, population.toMod, edgeStrings(projected))
			}
		})
	}
}

func TestPerExternalDependencyEdgeKeepsOnlyTheDependenciesThatLeaveTheProject(t *testing.T) {
	// Which code this project owns was decided once, in extraction, so a caller who passes one of the
	// project's own files among the modules gets nothing for it. Otherwise a `depend on external modules` rule
	// and a `depend on files` rule would be two answers to that one question.
	graph := externalFixtureGraph()
	every := projection.SelectFiles(graph)

	projected := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(every, every))

	if len(projected) != 0 {
		t.Errorf("the projection of the project onto itself is %v, want nothing: none of those edges is external",
			edgeStrings(projected))
	}
}

func TestPerExternalDependencyEdgeReadsOneWayOnly(t *testing.T) {
	// Unlike PerDependencyEdge the direction is not a choice: nothing in the graph points from somebody else's
	// module back into this project, so the converse rule cannot be written and swapping the arguments is the
	// mistake of passing the modules as the subject, which projects nothing.
	graph := externalFixtureGraph()
	db := projection.SelectFiles(graph, folderMatcher(t, "internal/db/**"))
	modules := projection.SelectExternalModules(graph)

	forwards := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(db, modules))
	backwards := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(modules, db))

	if len(forwards) == 0 {
		t.Errorf("the dependencies of the db folder on %v are none, want the ones the fixture has", modules)
	}
	if len(backwards) != 0 {
		t.Errorf("the dependencies of %v on the db folder are %v, want none: nothing points back into the project",
			modules, edgeStrings(backwards))
	}
}

func TestPerExternalDependencyEdgeDropsTheSelfEdgeThatOnlyNamesAFile(t *testing.T) {
	// The rule of the whole `per <thing> edge` family. It cannot arise from a canonical graph here — an
	// external edge from a node to itself is not a thing extraction.NewGraph produces — but the mapper is
	// handed one edge at a time and answers about the edge in front of it.
	mapper := projection.PerExternalDependencyEdge([]string{"main.go"}, []string{"main.go"})

	if _, kept := mapper(extraction.SelfEdge("main.go")); kept {
		t.Error("PerExternalDependencyEdge keeps a file's self-edge, want it dropped")
	}
}

func TestPerExternalDependencyEdgeLabelsBothEndsByTheirIdentifierAndKeepsTheRawEdges(t *testing.T) {
	// The labels are the identifiers both populations already carry — the file as the extractor minted it, the
	// module as the file imported it — and the raw edges survive under them, which is what lets a report name
	// the blank import that made the dependency the rule was broken by.
	graph := externalFixtureGraph()
	db := projection.SelectFiles(graph, folderMatcher(t, "internal/db/**"))
	pq := projection.SelectExternalModules(graph, pathMatcher(t, "github.com/lib/pq"))

	projected := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(db, pq))

	if len(projected) != 1 {
		t.Fatalf("the projection is %v, want the one dependency on %v", edgeStrings(projected), pq)
	}
	edge := projected[0]
	if edge.SourceLabel() != "internal/db/conn.go" || edge.TargetLabel() != "github.com/lib/pq" {
		t.Errorf("the dependency is %s, want it labeled by the file and the import path", edge)
	}
	raw := edge.CumulatedEdges()
	if len(raw) != 1 {
		t.Fatalf("%s cumulates %v, want the one raw edge it was projected from", edge, raw)
	}
	if !raw[0].ImportKinds.Contains(extraction.ImportKindBlank) {
		t.Errorf("%s cumulates %v, want the import kind the extractor read", edge, raw)
	}
}

func TestPerExternalDependencyEdgeIsPerExternalEdgeNarrowedToTwoPopulations(t *testing.T) {
	// The kernel's `per external edge` is this mapper with every file and every module named, so the two agree
	// there — which is why what external means is tested once, where it is decided, and not again here.
	graph := externalFixtureGraph()
	every := projection.SelectFiles(graph)
	modules := projection.SelectExternalModules(graph)

	narrowed := kernel.ProjectEdges(graph, projection.PerExternalDependencyEdge(every, modules))
	kernelwide := kernel.ProjectEdges(graph, kernel.PerExternalEdge())

	if !slices.Equal(edgeStrings(narrowed), edgeStrings(kernelwide)) {
		t.Errorf("PerExternalDependencyEdge over the whole project projects %v and PerExternalEdge projects %v, want the same dependencies",
			edgeStrings(narrowed), edgeStrings(kernelwide))
	}
	if len(narrowed) == 0 {
		t.Error("both projections are empty, want a fixture with external dependencies for the two mappers to agree on")
	}
}
