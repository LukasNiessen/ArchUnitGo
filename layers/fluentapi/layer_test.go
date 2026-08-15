package fluentapi_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
)

func TestDefinedByFolderIsTheFolderAloneOrTheFolderAndEverythingBelowIt(t *testing.T) {
	// The declaration nearly every layer is written with, and the one distinction a policy has to be clear
	// about: `internal/api` is that folder, `internal/api/**` is it together with what is below it.
	root := writeNestedFixtureProject(t)

	shallow := fluentapi.ProjectLayers(fixtureLocator(t, root)).Layer("api").DefinedByFolder("internal/api")
	deep := fluentapi.ProjectLayers(fixtureLocator(t, root)).Layer("api").DefinedByFolder("internal/api/**")

	if files := mustSelect(t, shallow)["api"]; !slices.Equal(files, []string{"internal/api/handler.go"}) {
		t.Errorf(`layer "api" defined by folder "internal/api" is %v, want the file of that folder alone`, files)
	}
	want := []string{"internal/api/handler.go", "internal/api/rest/get.go"}
	if files := mustSelect(t, deep)["api"]; !slices.Equal(files, want) {
		t.Errorf(`layer "api" defined by folder "internal/api/**" is %v, want %v`, files, want)
	}
}

func TestDefinedByMatchesTheWholeIdentifier(t *testing.T) {
	// The general form: folder and name at once, for a layer defined by how its files are named rather than by
	// where they live.
	root := writeNestedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).Layer("handlers").DefinedBy("internal/**/handler.go")

	if files := mustSelect(t, policy)["handlers"]; !slices.Equal(files, []string{"internal/api/handler.go"}) {
		t.Errorf(`layer "handlers" is %v, want the one file named that way`, files)
	}
}

func TestALayerDeclarationRendersWithTheLayerLeftOpen(t *testing.T) {
	// A chain caught between the name and the pattern still says what it was about, which is what a log line of
	// a half-built policy needs.
	declaring := fluentapi.ProjectLayers(nil).
		Layer("api").DefinedByFolder("internal/api/**").
		Layer("db")

	want := `project layers, layer "api" defined by path without filename matches "internal/api/**", layer "db"`
	if rendered := declaring.String(); rendered != want {
		t.Errorf("the open declaration reads %q, want %q", rendered, want)
	}
}

func TestALayerDeclarationRendersTheRejectionThatEndedTheChain(t *testing.T) {
	// A rejected pattern ends the sentence rather than sitting inside it: without it the policy would render as
	// the rule the user thought they wrote.
	rejected := fluentapi.ProjectLayers(nil).Layer("api").DefinedByFolder("internal/[").Layer("db")

	rendered := rejected.String()
	if !strings.HasPrefix(rendered, "project layers (rejected: ") {
		t.Errorf("the rejected policy reads %q, want the rejection closing the sentence", rendered)
	}
	for _, part := range []string{"defined by folder", `internal/[`} {
		if !strings.Contains(rendered, part) {
			t.Errorf("the rejected policy reads %q, want %q named in it", rendered, part)
		}
	}
}

func TestARejectedDeclarationDoesNotDeclareTheLayer(t *testing.T) {
	// A zero Filter matches nothing, so a layer declared from a pattern that would not compile would be
	// reported as an empty layer — which sends the reader to the guard's message instead of to the typo.
	root := writeNestedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/[").
		Layer("db").DefinedByFolder("internal/db/**")

	membership, err := policy.SelectLayerFiles(nil)

	if err == nil {
		t.Fatalf("the policy resolved to %v, want the rejection of the pattern that would not compile", membership)
	}
	if membership != nil {
		t.Errorf("the policy resolved to %v beside the error, want nothing", membership)
	}
}

// writeNestedFixtureProject writes a project with a folder inside a folder, which is the fixture the two
// readings of `defined by folder` are told apart on: internal/api holds one file and internal/api/rest another,
// so a layer declared as the folder alone and one declared as the folder and everything below it differ.
func writeNestedFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                  "module example.com/nested\n\ngo 1.26\n",
		"internal/api/handler.go": "package api\n\nfunc Handle() {}\n",
		"internal/api/rest/get.go": "package rest\n\nimport \"example.com/nested/internal/api\"\n\n" +
			"func Get() { api.Handle() }\n",
		"internal/db/conn.go": "package db\n\nfunc Connect() {}\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating the folder of %q failed: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %q failed: %v", name, err)
		}
	}
	return root
}

// mustSelect resolves a policy's declarations against its project, failing the test when the chain could not
// be understood or the project could not be read.
func mustSelect(t *testing.T, policy fluentapi.LayersBuilder) map[string][]string {
	t.Helper()

	membership, err := policy.SelectLayerFiles(nil)
	if err != nil {
		t.Fatalf("resolving %s failed: %v", policy, err)
	}
	return membership
}
