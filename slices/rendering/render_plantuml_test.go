package rendering_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernelprojection "github.com/LukasNiessen/ArchUnitGo/common/projection"
	slicesextraction "github.com/LukasNiessen/ArchUnitGo/slices/extraction"
	"github.com/LukasNiessen/ArchUnitGo/slices/rendering"
)

// fixtureComponents are the slices of the fixture project, in the order the keys of a map arrive in — which
// is to say in no order at all.
func fixtureComponents() []string {
	return []string{"domain", "api", "db"}
}

// fixtureDependencies are the projected dependencies of the fixture project, in the order
// projection.ProjectEdges hands them over: by source, then by target.
func fixtureDependencies() []kernelprojection.ProjectedEdge {
	return []kernelprojection.ProjectedEdge{
		kernelprojection.NewProjectedEdge("api", "db",
			extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
			extraction.NewEdge("internal/api/router.go", "internal/db/query.go", false, extraction.ImportKindPlain)),
		kernelprojection.NewProjectedEdge("api", "domain",
			extraction.NewEdge("internal/api/handler.go", "internal/domain/order.go", false, extraction.ImportKindPlain)),
	}
}

func TestRenderPlantUMLDrawsTheSlicesAndTheDependenciesBetweenThem(t *testing.T) {
	// The whole document, asserted as itself: a summary comment over the components, the components in
	// alphabetical order, then the arrows in the projection's order. No count on an arrow and no label, because
	// this is meant to be read back by the parser that judges a project against a drawing.
	want := `@startuml
' 3 components, 2 dependencies
component [api]
component [db]
component [domain]
[api] --> [db]
[api] --> [domain]
@enduml
`

	if got := rendering.RenderPlantUML(fixtureComponents(), fixtureDependencies()); got != want {
		t.Errorf("the project is drawn as\n%s\nwant\n%s", got, want)
	}
}

func TestRenderPlantUMLDrawsTheSameBytesForTheSameProject(t *testing.T) {
	// A document that changed between two runs could not be committed beside the code, and the components arrive
	// as the keys of a map: they are sorted here, and a name that arrives twice is drawn once.
	first := rendering.RenderPlantUML([]string{"db", "api", "domain"}, fixtureDependencies())
	second := rendering.RenderPlantUML([]string{"domain", "db", "api", "api"}, fixtureDependencies())

	if first != second {
		t.Errorf("the same project drew\n%s\nand\n%s", first, second)
	}
	if strings.Count(first, "component [api]") != 1 {
		t.Errorf("the document draws api more than once:\n%s", first)
	}
}

func TestRenderPlantUMLDrawsAProjectWithNothingInIt(t *testing.T) {
	// The document of an empty projection is still a document: the terminal that hands it out is what decides
	// whether a project with no slice in it is worth reporting, exactly as the empty-test guard does for a rule.
	want := `@startuml
' 0 components, 0 dependencies
@enduml
`

	if got := rendering.RenderPlantUML(nil, nil); got != want {
		t.Errorf("an empty project is drawn as\n%s\nwant\n%s", got, want)
	}
}

func TestRenderPlantUMLCountsOneOfEachInTheSingular(t *testing.T) {
	// A summary that says "1 components" is the first thing a reader distrusts.
	document := rendering.RenderPlantUML([]string{"api", "db"},
		[]kernelprojection.ProjectedEdge{kernelprojection.NewProjectedEdge("api", "db")})

	if want := "' 2 components, 1 dependency\n"; !strings.Contains(document, want) {
		t.Errorf("the document summarizes itself as\n%s\nwant a line %q", document, want)
	}
}

func TestRenderPlantUMLDrawsANameTheDialectCouldNotReadBack(t *testing.T) {
	// This format's own escaping, in one place: a bracket says a name is a component's and an angle bracket
	// starts a stereotype or an arrow, so neither may be inside a name — and a newline in one would break the
	// line a component is drawn on. A name that went out unescaped would draw a document this library's own
	// reader refuses.
	document := rendering.RenderPlantUML(
		[]string{"a[b]c", "d<e>f", "g\nh"},
		[]kernelprojection.ProjectedEdge{kernelprojection.NewProjectedEdge("a[b]c", "g\nh")})

	for _, want := range []string{"component [a(b)c]", "component [d(e)f]", "component [g h]", "[a(b)c] --> [g h]"} {
		if !strings.Contains(document, want) {
			t.Errorf("the document is\n%s\nwant a line %q in it", document, want)
		}
	}
}

func TestADrawnProjectIsADiagramThisLibraryReadsBack(t *testing.T) {
	// The round trip, and the reason the two halves of this feature are worth having in one library: a project's
	// own diagram, drawn by this function, is the drawing the next run of `adhere to diagram` judges it against.
	// Nothing in the dialect this draws may drift out of the dialect the parser reads.
	document := rendering.RenderPlantUML(fixtureComponents(), fixtureDependencies())

	diagram, err := slicesextraction.ParseDiagram(document)
	if err != nil {
		t.Fatalf("reading back the drawn document failed with %v, want it read:\n%s", err, document)
	}

	wantComponents := []string{"api", "db", "domain"}
	if got := diagram.Components(); !slices.Equal(got, wantComponents) {
		t.Errorf("the drawn document declares %v, want %v", got, wantComponents)
	}
	wantDependencies := []slicesextraction.Dependency{{From: "api", To: "db"}, {From: "api", To: "domain"}}
	if got := diagram.Dependencies(); !slices.Equal(got, wantDependencies) {
		t.Errorf("the drawn document draws %v, want %v", got, wantDependencies)
	}
}

func TestADrawnNameIsADiagramThisLibraryReadsBack(t *testing.T) {
	// The escaping is what the round trip rests on, so it is asserted through the parser too: whatever the
	// projection produced, the document is readable and every component in it is one line.
	document := rendering.RenderPlantUML([]string{"a[b]c", "d<e>f", "g\nh"}, nil)

	diagram, err := slicesextraction.ParseDiagram(document)
	if err != nil {
		t.Fatalf("reading back a drawn document with escaped names failed with %v:\n%s", err, document)
	}

	want := []string{"a(b)c", "d(e)f", "g h"}
	if got := diagram.Components(); !slices.Equal(got, want) {
		t.Errorf("the drawn document declares %v, want %v", got, want)
	}
}
