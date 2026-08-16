package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/projection"
)

// externalFixtureGraph is a small project that depends on somebody else's code in every way a project can:
// two packages of one third-party module, a package of another, and two of the standard library. The
// internal dependencies and self-edges of fixtureGraph are here too, because the thing under test is which
// of the two halves of a graph this selection reads.
func externalFixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("main.go", "fmt", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "net/http", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "github.com/gin-gonic/gin", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "github.com/lib/pq", true, extraction.ImportKindBlank),
		extraction.NewEdge("internal/db/conn.go", "gorm.io/gorm", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "gorm.io/gorm/clause", true, extraction.ImportKindPlain),
	)
}

func TestSelectExternalModulesWithNoSelectorIsEveryModuleTheProjectDependsOn(t *testing.T) {
	// `depend on external modules` with nothing chained onto it: every import path that leaves the project,
	// the standard library included, because what external means is the one thing the extractor already
	// decided and this function does not re-decide.
	selected := projection.SelectExternalModules(externalFixtureGraph())

	want := []string{
		"fmt",
		"github.com/gin-gonic/gin",
		"github.com/lib/pq",
		"gorm.io/gorm",
		"gorm.io/gorm/clause",
		"net/http",
	}
	if !slices.Equal(selected, want) {
		t.Errorf("SelectExternalModules() = %v, want %v", selected, want)
	}
}

func TestSelectExternalModulesNeverSelectsTheProjectsOwnFiles(t *testing.T) {
	// The population is somebody else's code, so a pattern that matches everything still selects none of the
	// project's own files — which is the mistake that would make `should not depend on external modules
	// matching "**"` report every internal dependency in the project as a third-party one.
	graph := externalFixtureGraph()

	for _, pattern := range []string{"**", "*", "internal/**", "main.go"} {
		selected := projection.SelectExternalModules(graph, pathMatcher(t, pattern))

		for _, file := range projection.SelectFiles(graph) {
			if slices.Contains(selected, file) {
				t.Errorf("SelectExternalModules(matching %q) = %v, want no file of the project among them", pattern, selected)
			}
		}
	}
}

func TestSelectExternalModulesCombinesItsSelectorsWithOr(t *testing.T) {
	// The one place in this library where chaining widens instead of narrowing: a third-party policy is a list
	// of alternatives, and a module cannot be two modules at once — so ANDing two of these would name the
	// empty set and a second `matching` verb would be meaningless.
	graph := externalFixtureGraph()
	gin := pathMatcher(t, "github.com/gin-gonic/**")
	gorm := pathMatcher(t, "gorm.io/**")

	either := projection.SelectExternalModules(graph, gin, gorm)
	reversed := projection.SelectExternalModules(graph, gorm, gin)

	want := []string{"github.com/gin-gonic/gin", "gorm.io/gorm", "gorm.io/gorm/clause"}
	if !slices.Equal(either, want) {
		t.Errorf("SelectExternalModules(gin, gorm) = %v, want %v", either, want)
	}
	// Combined with OR, so the order of the verbs cannot matter either.
	if !slices.Equal(reversed, either) {
		t.Errorf("SelectExternalModules(gorm, gin) = %v, want the same selection %v", reversed, either)
	}
}

func TestSelectExternalModulesLooksAtTheWholeImportPath(t *testing.T) {
	// The target of an external edge is the import path exactly as the file wrote it, so a pattern is written
	// against that whole string: a module-wide rule is a trailing `/**`, which matches the module path itself
	// as well as every package under it.
	graph := externalFixtureGraph()

	tests := []struct {
		name     string
		selector matching.Filter
		want     []string
	}{
		{
			name:     "one module and every package of it",
			selector: pathMatcher(t, "gorm.io/gorm/**"),
			want:     []string{"gorm.io/gorm", "gorm.io/gorm/clause"},
		},
		{
			name:     "one package of one module",
			selector: pathMatcher(t, "gorm.io/gorm"),
			want:     []string{"gorm.io/gorm"},
		},
		{
			name:     "every module of one host",
			selector: pathMatcher(t, "github.com/**"),
			want:     []string{"github.com/gin-gonic/gin", "github.com/lib/pq"},
		},
		{
			// The idiom for "third-party, but not the standard library": a first segment with a dot in it is
			// a domain, and no package of the standard library has one.
			name:     "everything but the standard library",
			selector: pathMatcher(t, "*.*/**"),
			want:     []string{"github.com/gin-gonic/gin", "github.com/lib/pq", "gorm.io/gorm", "gorm.io/gorm/clause"},
		},
		{
			name:     "one package of the standard library",
			selector: pathMatcher(t, "net/http"),
			want:     []string{"net/http"},
		},
		{
			name:     "a module this project does not depend on",
			selector: pathMatcher(t, "github.com/deprecated/**"),
			want:     []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected := projection.SelectExternalModules(graph, test.selector)

			if !slices.Equal(selected, test.want) {
				t.Errorf("SelectExternalModules(%s) = %v, want %v", test.selector, selected, test.want)
			}
		})
	}
}

func TestSelectExternalModulesNamesEachModuleOnceHoweverManyFilesImportIt(t *testing.T) {
	// It is a population and not a list of edges: three files importing one module are one module, so a
	// report cannot name it three times and a rule cannot be judged against a multiset.
	graph := extraction.NewGraph(
		extraction.SelfEdge("a.go"),
		extraction.SelfEdge("b.go"),
		extraction.SelfEdge("c.go"),
		extraction.NewEdge("a.go", "github.com/lib/pq", true, extraction.ImportKindPlain),
		extraction.NewEdge("b.go", "github.com/lib/pq", true, extraction.ImportKindBlank),
		extraction.NewEdge("c.go", "github.com/lib/pq", true, extraction.ImportKindPlain),
	)

	selected := projection.SelectExternalModules(graph)

	if !slices.Equal(selected, []string{"github.com/lib/pq"}) {
		t.Errorf("SelectExternalModules() = %v, want the one module named once", selected)
	}
}

func TestSelectExternalModulesIsSortedEvenForAnUnsortedGraph(t *testing.T) {
	// A hand-written graph literal need not be ordered, and a report built from a selection has to be
	// reproducible either way.
	unsorted := extraction.Graph{
		extraction.NewEdge("main.go", "gorm.io/gorm", true, extraction.ImportKindPlain),
		extraction.NewEdge("main.go", "fmt", true, extraction.ImportKindPlain),
		extraction.NewEdge("main.go", "github.com/lib/pq", true, extraction.ImportKindPlain),
	}

	selected := projection.SelectExternalModules(unsorted)

	want := []string{"fmt", "github.com/lib/pq", "gorm.io/gorm"}
	if !slices.Equal(selected, want) {
		t.Errorf("SelectExternalModules() = %v, want %v", selected, want)
	}
}

func TestSelectExternalModulesWithAZeroFilterSelectsNothing(t *testing.T) {
	// A filter that was never built is a mistake, and matching.Filter answers nothing rather than everything
	// so that the mistake cannot pass for a rule about every module the project depends on.
	selected := projection.SelectExternalModules(externalFixtureGraph(), matching.Filter{})

	if len(selected) != 0 {
		t.Errorf("SelectExternalModules(zero filter) = %v, want nothing selected", selected)
	}
}

func TestSelectExternalModulesOfAProjectThatDependsOnNothingSelectsNothing(t *testing.T) {
	// The ordinary answer for this population, and the reason the terminal does not put it through the
	// empty-test guard: a project that depends on nobody else's code is what a rule forbidding a module asks
	// for, not a stale pattern.
	tests := []struct {
		name  string
		graph extraction.Graph
	}{
		{name: "a nil graph", graph: nil},
		{name: "an empty graph", graph: extraction.NewGraph()},
		{name: "a project whose files depend only on each other", graph: fixtureInternalGraph()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if selected := projection.SelectExternalModules(test.graph); len(selected) != 0 {
				t.Errorf("SelectExternalModules() = %v, want nothing selected", selected)
			}
		})
	}
}

// fixtureInternalGraph is a project whose files depend only on each other, which is the graph an external
// selection has to answer nothing about.
func fixtureInternalGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
	)
}
