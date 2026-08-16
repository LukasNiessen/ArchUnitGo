package projection_test

import (
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/layers/projection"
)

func TestALayerMatchesTheFilesAnyOfItsSelectorsAccepts(t *testing.T) {
	// A layer declared twice is the union of its declarations: `layer "domain" defined by folder
	// "internal/domain" or path without filename matches "internal/model"` is one layer, and a file of
	// either folder is in it. This is the one chain in the module that widens rather than narrows.
	layer := projection.NewLayer("domain", folderMatcher(t, "internal/domain"), folderMatcher(t, "internal/model"))

	for _, identifier := range []string{"internal/domain/order.go", "internal/model/order.go"} {
		if !layer.Matches(identifier) {
			t.Errorf("%s is not in %s, want it in: a layer is the union of its declarations", identifier, layer)
		}
	}
	if layer.Matches("internal/api/handler.go") {
		t.Errorf("internal/api/handler.go is in %s, want it out: no declaration describes it", layer)
	}
}

func TestALayerWithNoSelectorMatchesNothing(t *testing.T) {
	// The zero Layer, and a layer declared with no pattern at all, are a layer no file is in — not a layer
	// every file is in. An unset pattern is a mistake, and matching everything would hide it behind a policy
	// that judges the whole project.
	for name, layer := range map[string]projection.Layer{
		"the zero layer":        {},
		"declared with no glob": projection.NewLayer("api"),
	} {
		if layer.Matches("main.go") {
			t.Errorf("main.go is in %s (%s), want it out: a layer with no selector matches nothing", layer, name)
		}
	}
}

func TestALayerRendersItsDeclarationAsTheSentenceItWasWritten(t *testing.T) {
	// A reader needs to see which part of an identifier each pattern was matched against, which is the
	// filter's own words, and `or` between two of them because that is what a layer declared twice means.
	declarations := map[string]projection.Layer{
		`layer "api" defined by nothing`: projection.NewLayer("api"),
		`layer "api" defined by path without filename matches "internal/api/**"`: projection.NewLayer(
			"api", folderMatcher(t, "internal/api/**")),
		`layer "domain" defined by path matches "internal/domain/*.go" or path without filename matches "internal/model"`: projection.NewLayer(
			"domain", pathMatcher(t, "internal/domain/*.go"), folderMatcher(t, "internal/model")),
	}

	for want, layer := range declarations {
		if rendered := layer.String(); rendered != want {
			t.Errorf("the declaration reads %q, want %q", rendered, want)
		}
	}
}

func TestALayerDoesNotChangeWhenTheSelectorsItWasDeclaredWithAre(t *testing.T) {
	// A policy is a value a user may have stored, so a Layer copies what it is given and hands back copies
	// of it: a caller reusing the slice it declared a layer with must not silently redefine the layer.
	selectors := []matching.Filter{folderMatcher(t, "internal/api/**")}
	layer := projection.NewLayer("api", selectors...)

	selectors[0] = folderMatcher(t, "internal/db/**")
	layer.Selectors()[0] = folderMatcher(t, "internal/db/**")

	if !layer.Matches("internal/api/handler.go") {
		t.Errorf("%s stopped matching its own folder: the declaration was shared rather than copied", layer)
	}
	if layer.Matches("internal/db/conn.go") {
		t.Errorf("%s matches the db folder: the declaration was shared rather than copied", layer)
	}
}

func TestLayerOfIsTheFirstDeclaredLayerThatMatches(t *testing.T) {
	// A projection labels each end of an edge with one name, so a file has to be in exactly one layer however
	// much the patterns overlap. The order the user declared them in is what resolves it: `api` before the
	// broad `internal` layer means the api files are api files.
	api := projection.NewLayer("api", folderMatcher(t, "internal/api/**"))
	everything := projection.NewLayer("internals", folderMatcher(t, "internal/**"))

	if name, layered := projection.LayerOf("internal/api/handler.go", api, everything); !layered || name != "api" {
		t.Errorf("internal/api/handler.go is in layer %q (%t), want %q: the first declaration wins", name, layered, "api")
	}
	if name, layered := projection.LayerOf("internal/api/handler.go", everything, api); !layered || name != "internals" {
		t.Errorf("declared the other way round the file is in layer %q (%t), want %q", name, layered, "internals")
	}
}

func TestLayerOfReportsAFileThatIsInNoDeclaredLayer(t *testing.T) {
	// A project is rarely layered end to end, and a file nobody assigned a layer to is not an error: the
	// policy simply says nothing about it. PerLayerEdge is where that becomes an ignored edge.
	api := projection.NewLayer("api", folderMatcher(t, "internal/api/**"))

	if name, layered := projection.LayerOf("main.go", api); layered || name != "" {
		t.Errorf("main.go is in layer %q (%t), want no layer at all", name, layered)
	}
	if name, layered := projection.LayerOf("main.go"); layered || name != "" {
		t.Errorf("with no layer declared main.go is in layer %q (%t), want no layer at all", name, layered)
	}
}

// fixtureGraph is the project every test in this package is about: a main file, an api folder of two files, a
// db folder of two files, and the dependencies between them, together with two imports that leave the project.
//
// It is the same shape the files module's fixture has, deliberately: a layer policy is a skin over the same
// vocabulary, so a reader comparing the two sees the projection and not the fixture.
func fixtureGraph() extraction.Graph {
	return extraction.NewGraph(
		extraction.SelfEdge("main.go"),
		extraction.SelfEdge("internal/api/handler.go"),
		extraction.SelfEdge("internal/api/router.go"),
		extraction.SelfEdge("internal/db/conn.go"),
		extraction.SelfEdge("internal/db/query.go"),
		extraction.NewEdge("main.go", "internal/api/handler.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/api/router.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "internal/db/query.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/query.go", "internal/api/router.go", false, extraction.ImportKindPlain),
		extraction.NewEdge("internal/api/handler.go", "fmt", true, extraction.ImportKindPlain),
		extraction.NewEdge("internal/db/conn.go", "gorm.io/gorm", true, extraction.ImportKindPlain),
	)
}

func folderMatcher(t *testing.T, glob string) matching.Filter {
	t.Helper()

	return mustMatcher(t, "folder", glob, matching.NewRegexFactory(nil).FolderMatcher)
}

func pathMatcher(t *testing.T, glob string) matching.Filter {
	t.Helper()

	return mustMatcher(t, "path", glob, matching.NewRegexFactory(nil).PathMatcher)
}

func mustMatcher(t *testing.T, kind, pattern string, build func(string) (matching.Filter, error)) matching.Filter {
	t.Helper()

	filter, err := build(pattern)
	if err != nil {
		t.Fatalf("%s matcher %q failed to compile: %v", kind, pattern, err)
	}
	return filter
}

// layerNames are the names of these layers, in the order they were declared, for a message about which
// layers a projection was built from.
func layerNames(layers []projection.Layer) []string {
	names := make([]string, 0, len(layers))
	for _, layer := range layers {
		names = append(names, layer.Name())
	}
	return names
}

// sortedStrings is a copy of these strings in order, for comparing a set a test does not care about the order
// of against the one it expects.
func sortedStrings(values []string) []string {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted
}
