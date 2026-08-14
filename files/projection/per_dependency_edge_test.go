package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

func TestPerDependencyEdgeKeepsTheDependenciesFromTheOnePopulationToTheOther(t *testing.T) {
	// The projection `project files, in folder "internal/api/**", should not, depend on files, in folder
	// "internal/db/**"` is judged over: the dependencies the subject has on the object, and nothing else.
	graph := fixtureGraph()
	api := projection.SelectFiles(graph, folderMatcher(t, "internal/api/**"))
	db := projection.SelectFiles(graph, folderMatcher(t, "internal/db/**"))

	projected := kernel.ProjectEdges(graph, projection.PerDependencyEdge(api, db))

	want := []string{"internal/api/handler.go -> internal/db/conn.go"}
	if dependencies := edgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the dependencies of %v on %v are %v, want %v", api, db, dependencies, want)
	}
}

func TestPerDependencyEdgeIsDirected(t *testing.T) {
	// The two arguments are the two halves of the sentence, and swapping them is the converse rule: the api
	// folder depends on the db folder, so the db folder does not depend on the api folder. A mapper that read
	// the two populations symmetrically would report the same violation for both rules.
	graph := fixtureGraph()
	api := projection.SelectFiles(graph, folderMatcher(t, "internal/api/**"))
	db := projection.SelectFiles(graph, folderMatcher(t, "internal/db/**"))

	forwards := kernel.ProjectEdges(graph, projection.PerDependencyEdge(api, db))
	backwards := kernel.ProjectEdges(graph, projection.PerDependencyEdge(db, api))

	if len(forwards) != 1 {
		t.Errorf("the dependencies of the api folder on the db folder are %v, want the one there is", edgeStrings(forwards))
	}
	if len(backwards) != 0 {
		t.Errorf("the dependencies of the db folder on the api folder are %v, want none: the sentence reads one way", edgeStrings(backwards))
	}
}

func TestPerDependencyEdgeProjectsNothingWhenEitherPopulationIsEmpty(t *testing.T) {
	// A sentence one half of which named no file has no dependency to judge, whichever half it was. Projecting
	// nothing is the loud direction: it is what the empty-test guard reports on, where projecting everything
	// would hold a rule over files nobody named.
	graph := fixtureGraph()
	every := projection.SelectFiles(graph)

	populations := []struct {
		name string
		from []string
		to   []string
	}{
		{name: "no subject", from: nil, to: every},
		{name: "no object", from: every, to: []string{}},
		{name: "neither", from: []string{}, to: nil},
		{name: "an object no file is in", from: every, to: projection.SelectFiles(graph, folderMatcher(t, "internal/renamed/**"))},
	}

	for _, population := range populations {
		t.Run(population.name, func(t *testing.T) {
			projected := kernel.ProjectEdges(graph, projection.PerDependencyEdge(population.from, population.to))

			if len(projected) != 0 {
				t.Errorf("the projection of %v -> %v is %v, want nothing projected",
					population.from, population.to, edgeStrings(projected))
			}
		})
	}
}

func TestPerDependencyEdgeNeverKeepsADependencyThatLeavesTheProject(t *testing.T) {
	// Only the project's own files are selectable, so an import path cannot be an end of a kept edge — and it
	// stays that way when a caller passes one among the identifiers, which is the mistake a mapper reading the
	// populations alone would honor. `depend on files` is about this project's files; a rule about the
	// standard library or a third-party module is `depend on external modules`.
	graph := fixtureGraph()
	external := []string{"fmt", "golang.org/x/tools/go/packages"}
	from := projection.SelectFiles(graph)

	projected := kernel.ProjectEdges(graph, projection.PerDependencyEdge(from, append(slices.Clone(from), external...)))

	for _, edge := range projected {
		if slices.Contains(external, edge.TargetLabel()) {
			t.Errorf("the projection holds %s, want the dependencies that leave the project dropped", edge)
		}
	}
}

func TestPerDependencyEdgeDropsTheSelfEdgeThatOnlyNamesAFile(t *testing.T) {
	// The rule of the whole `per <thing> edge` family, and here it is also the rule's own meaning: a file that
	// is in both halves of the sentence does not break it by existing. Only a dependency on another file counts.
	mapper := projection.PerDependencyEdge([]string{"main.go"}, []string{"main.go"})

	if _, kept := mapper(extraction.SelfEdge("main.go")); kept {
		t.Error("PerDependencyEdge keeps a file's self-edge, want it dropped")
	}
}

func TestPerDependencyEdgeLabelsAFileByItsIdentifierAndKeepsTheRawEdges(t *testing.T) {
	// The labels are the identifiers the extractor minted, because a rule about files is written against
	// exactly those strings — and the raw edges survive under them, which is what lets a report name the import
	// that made the dependency the rule was broken by.
	graph := fixtureGraph()
	from := projection.SelectFiles(graph, folderMatcher(t, "internal/api/**"))
	to := projection.SelectFiles(graph, folderMatcher(t, "internal/db/**"))

	projected := kernel.ProjectEdges(graph, projection.PerDependencyEdge(from, to))

	if len(projected) != 1 {
		t.Fatalf("the projection is %v, want the one dependency between the two folders", edgeStrings(projected))
	}
	edge := projected[0]
	if edge.SourceLabel() != "internal/api/handler.go" || edge.TargetLabel() != "internal/db/conn.go" {
		t.Errorf("the dependency is %s, want it labeled by the two file identifiers", edge)
	}
	raw := edge.CumulatedEdges()
	if len(raw) != 1 {
		t.Fatalf("%s cumulates %v, want the one raw edge it was projected from", edge, raw)
	}
	if !raw[0].ImportKinds.Contains(extraction.ImportKindPlain) {
		t.Errorf("%s cumulates %v, want the import kind the extractor read", edge, raw)
	}
}

func TestPerSelectedFileEdgeIsPerDependencyEdgeWithOnePopulationAtBothEnds(t *testing.T) {
	// "The dependencies between these files" is the relational projection with one population at both ends, so
	// the two mappers agree wherever the subject and the object are the same selection — which is why there is
	// one membership test in this package and not two.
	graph := cyclicFixtureGraph()
	selected := projection.SelectFiles(graph, folderMatcher(t, "internal/**"))

	between := kernel.ProjectEdges(graph, projection.PerSelectedFileEdge(selected))
	relational := kernel.ProjectEdges(graph, projection.PerDependencyEdge(selected, selected))

	if !slices.Equal(edgeStrings(between), edgeStrings(relational)) {
		t.Errorf("PerSelectedFileEdge projects %v and PerDependencyEdge projects %v, want the same dependencies",
			edgeStrings(between), edgeStrings(relational))
	}
	if len(between) == 0 {
		t.Error("both projections are empty, want a fixture with dependencies for the two mappers to agree on")
	}
}
