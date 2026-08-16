package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/layers/fluentapi"
)

func TestExceptTakesAFolderBackOutOfALayer(t *testing.T) {
	// The sentence the issue asks for, about a layer: everything under the api folder is the api layer, except
	// the generated client that happens to live in it. Written as a declaration per sibling folder instead, the
	// layer goes stale the day somebody adds one — and silently, because a file in no layer is in no clause.
	root := writeExcludedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**")

	excepted := policy.Except("**/generated")

	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if got := mustSelect(t, excepted)["api"]; !slices.Equal(got, want) {
		t.Errorf("%s came to %v, want %v", excepted, got, want)
	}
	if got := mustSelect(t, policy)["api"]; len(got) != 3 {
		t.Errorf("%s came to %v, want the three files the exclusion is subtracted from", policy, got)
	}
}

func TestTheTargetedExclusionsOfALayerNameTheirOwnTarget(t *testing.T) {
	// An exclusion may look at a part of an identifier its own declaration does not, which is the half of the
	// verb that is not sugar: `defined by "internal/api/**", except in folder "**/generated"` cannot be
	// written as a pattern of either verb alone.
	root := writeExcludedFixtureProject(t)

	tests := []struct {
		name   string
		policy fluentapi.LayersBuilder
		want   []string
	}{
		{
			name: "a path declaration excepting a folder",
			policy: fluentapi.ProjectLayers(fixtureLocator(t, root)).
				Layer("api").DefinedBy("internal/api/**").ExceptInFolder("**/generated"),
			want: []string{"internal/api/handler.go", "internal/api/router.go"},
		},
		{
			name: "a folder declaration excepting a whole path",
			policy: fluentapi.ProjectLayers(fixtureLocator(t, root)).
				Layer("api").DefinedByFolder("internal/api/**").ExceptInPath("internal/api/generated/*.go"),
			want: []string{"internal/api/handler.go", "internal/api/router.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mustSelect(t, test.policy)["api"]; !slices.Equal(got, test.want) {
				t.Errorf("%s came to %v, want %v", test.policy, got, test.want)
			}
		})
	}
}

func TestAPlainExclusionOfALayerIsReadAgainstTheDeclarationItFollows(t *testing.T) {
	// A bare pattern is a second pattern of the same clause, so after `defined by folder` it is about folders
	// and after `defined by` about whole identifiers. The pair below is the same pattern read both ways: as a
	// folder it takes the generated package out, and as a path it names no file at all.
	root := writeExcludedFixtureProject(t)

	folder := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").Except("internal/api/generated")
	path := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedBy("internal/api/**").Except("internal/api/generated")

	if got := mustSelect(t, folder)["api"]; slices.Contains(got, "internal/api/generated/client.go") {
		t.Errorf("%s came to %v, want the exclusion read as the folder its declaration is about", folder, got)
	}
	if got := mustSelect(t, path)["api"]; !slices.Contains(got, "internal/api/generated/client.go") {
		t.Errorf("%s came to %v, want the exclusion read as the whole identifier its declaration is about", path, got)
	}
}

func TestAnExclusionQualifiesTheLayerDeclaredMostRecently(t *testing.T) {
	// A policy is a list of declarations and an exclusion belongs to the one it was written after, so the
	// layers declared before it are untouched — the same rule the files module's scope verbs follow.
	root := writeExcludedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").
		Layer("model").DefinedByFolder("internal/model/**").Except("**/generated")

	membership := mustSelect(t, policy)

	if got := membership["api"]; !slices.Contains(got, "internal/api/generated/client.go") {
		t.Errorf("the api layer came to %v, want it untouched by the exclusion of another layer", got)
	}
	if got := membership["model"]; !slices.Equal(got, []string{"internal/model/thing.go"}) {
		t.Errorf("the model layer came to %v, want its generated package taken out of it", got)
	}
}

func TestAnExclusionQualifiesTheDeclarationItFollowsAndNotTheWholeLayer(t *testing.T) {
	// A layer declared twice is one layer whose declarations are combined with OR, so the exclusion is read
	// against the declaration in front of it rather than against the layer's whole membership. This is the one
	// place in the library where a chain widens, and an exclusion has to mean the clause it was typed in.
	root := writeExcludedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("domain").DefinedByFolder("internal/domain/**").
		Layer("domain").DefinedByFolder("internal/model/**").Except("**/generated")

	want := []string{
		"internal/domain/generated/mapper.go",
		"internal/domain/order.go",
		"internal/model/thing.go",
	}
	if got := mustSelect(t, policy)["domain"]; !slices.Equal(got, want) {
		t.Errorf("%s came to %v, want %v", policy, got, want)
	}
}

func TestAnExclusionQualifiesTheLayerDeclaredLastAndNotTheOneDeclaredLastInThePolicy(t *testing.T) {
	// The hazard the by-name lookup exists for: redeclaring a name widens the layer already declared under it,
	// which leaves that layer where it was in the policy — so the layer declared most recently is not the last
	// one in the list, and an exclusion bound to the last entry would land on a layer the user typed earlier.
	// Both policies below declare three times with a repeat in the middle, and each asserts the whole
	// membership of the layer the exclusion belongs to *and* of the one it must have left alone.
	root := writeExcludedFixtureProject(t)

	tests := []struct {
		name   string
		policy fluentapi.LayersBuilder
		want   map[string][]string
	}{
		{
			name: "the repeated layer excepts its second declaration",
			policy: fluentapi.ProjectLayers(fixtureLocator(t, root)).
				Layer("domain").DefinedByFolder("internal/domain/**").
				Layer("model").DefinedByFolder("internal/model/**").
				Layer("domain").DefinedByFolder("internal/api/**").Except("**/generated"),
			want: map[string][]string{
				"domain": {
					"internal/api/handler.go",
					"internal/api/router.go",
					"internal/domain/generated/mapper.go",
					"internal/domain/order.go",
				},
				// The layer declared between the two `domain` declarations is the last entry of the policy, so
				// it is what an exclusion bound to the wrong layer would have taken the generated package out of.
				"model": {"internal/model/generated/dto.go", "internal/model/thing.go"},
			},
		},
		{
			name: "the layer in between keeps its own generated package",
			policy: fluentapi.ProjectLayers(fixtureLocator(t, root)).
				Layer("domain").DefinedByFolder("internal/domain/**").
				Layer("api").DefinedByFolder("internal/api/**").
				Layer("domain").DefinedByFolder("internal/model/**").Except("**/generated"),
			want: map[string][]string{
				"domain": {
					"internal/domain/generated/mapper.go",
					"internal/domain/order.go",
					"internal/model/thing.go",
				},
				"api": {
					"internal/api/generated/client.go",
					"internal/api/handler.go",
					"internal/api/router.go",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			membership := mustSelect(t, test.policy)

			for layer, want := range test.want {
				if got := membership[layer]; !slices.Equal(got, want) {
					t.Errorf("the %s layer of %s came to %v, want %v", layer, test.policy, got, want)
				}
			}
		})
	}
}

func TestExclusionsOfALayerAccumulateAndCanBeBranchedFrom(t *testing.T) {
	// Several patterns in one call and several calls are the same thing, and a stored policy is unchanged by
	// either: a builder is a value, so two policies derived from one cannot see each other's exclusions.
	root := writeExcludedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("everything").DefinedByFolder("**")

	together := policy.Except("**/generated", "internal/db")
	apart := policy.Except("**/generated").Except("internal/db")

	want := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/domain/order.go",
		"internal/model/thing.go",
	}
	if got := mustSelect(t, together)["everything"]; !slices.Equal(got, want) {
		t.Errorf("%s came to %v, want %v", together, got, want)
	}
	if got := mustSelect(t, apart)["everything"]; !slices.Equal(got, want) {
		t.Errorf("%s came to %v, want %v", apart, got, want)
	}
	if got := mustSelect(t, policy)["everything"]; len(got) != 8 {
		t.Errorf("%s came to %v, want the eight files it came to before either exclusion was derived", policy, got)
	}
}

func TestAnExcludedFileIsInNoLayerSoItsDependenciesAreIgnored(t *testing.T) {
	// What the exclusion is worth, through the whole pipeline: the generated client is the only thing in the
	// api folder that reaches the database, so the same clause fails over the layer and holds over the layer
	// with the generated package taken out of it. The file is not moved to another layer and is not judged at
	// all — a dependency with an end in no declared layer is ignored, which is what excluding it means.
	root := writeExcludedFixtureProject(t)
	layers := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("db").DefinedByFolder("internal/db")

	broken := layers.Layer("api").DefinedByFolder("internal/api/**").
		WhereLayer("api").MayNotDependOnLayers("db")
	held := layers.Layer("api").DefinedByFolder("internal/api/**").Except("**/generated").
		WhereLayer("api").MayNotDependOnLayers("db")

	violations, err := broken.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", broken, err)
	}
	if got := offendingPairs(t, violations); !slices.Equal(got, []string{"api -> db"}) {
		t.Fatalf("%s reported %v, want the generated client's dependency on the database", broken, got)
	}
	if got := brokenBy(layerViolation(t, violations[0])); !slices.Equal(got,
		[]string{"internal/api/generated/client.go -> internal/db/conn.go"}) {
		t.Errorf("the violation was broken by %v, want the generated client's dependency alone", got)
	}

	remaining, err := held.Check(nil)
	if err != nil {
		t.Fatalf("checking %s failed: %v", held, err)
	}
	if len(remaining) != 0 {
		t.Errorf("%s reported %v, want the clause to hold once the generated package is in no layer",
			held, messages(t, remaining))
	}
}

func TestARejectedExclusionOfALayerIsAUserErrorNamingTheExceptVerb(t *testing.T) {
	// The three ways an exclusion is typed wrongly, each naming the verb the user has to go and fix, and each
	// deferred to the terminal because a fluent method has nowhere to put an error.
	tests := []struct {
		name    string
		policy  fluentapi.LayersBuilder
		verb    string
		subject string
		cause   error
	}{
		{
			name: "a pattern that will not compile",
			policy: fluentapi.ProjectLayers(nil).
				Layer("api").DefinedByFolder("internal/api/**").Except("[unclosed"),
			verb:    "except",
			subject: "[unclosed",
			cause:   matching.ErrInvalidPattern,
		},
		{
			name:    "an exclusion with nothing to qualify",
			policy:  fluentapi.ProjectLayers(nil).Except("**/generated"),
			verb:    "except",
			subject: "**/generated",
			cause:   matching.ErrExclusionWithoutSelector,
		},
		{
			name: "an exclusion with no pattern",
			policy: fluentapi.ProjectLayers(nil).
				Layer("api").DefinedByFolder("internal/api/**").ExceptInFolder(),
			verb:    "except in folder",
			subject: "",
			cause:   matching.ErrExclusionWithoutPattern,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			membership, err := test.policy.SelectLayerFiles(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("SelectLayerFiles error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the verb %q", user.Operation, test.verb)
			}
			if user.Subject != test.subject {
				t.Errorf("UserError.Subject = %q, want the patterns as the user typed them, %q", user.Subject, test.subject)
			}
			if !errors.Is(err, test.cause) {
				t.Errorf("SelectLayerFiles error = %v, want it to wrap %v", err, test.cause)
			}
			if membership != nil {
				t.Errorf("SelectLayerFiles reported %v beside the error, want no layer at all", membership)
			}
			if rendered := test.policy.String(); !strings.Contains(rendered, "rejected") {
				t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
			}
		})
	}
}

func TestARejectedExclusionDoesNotWidenALayer(t *testing.T) {
	// The hazard of deferring the error: a policy whose exclusion was refused must not run as the policy
	// without it, because that is a rule about more files than the user asked about — and it would pass or
	// fail on files they had taken out.
	root := writeExcludedFixtureProject(t)
	policy := fluentapi.ProjectLayers(fixtureLocator(t, root)).
		Layer("api").DefinedByFolder("internal/api/**").Except("[unclosed").
		Layer("db").DefinedByFolder("internal/db").
		WhereLayer("api").MayNotDependOnLayers("db")

	violations, err := policy.Check(nil)

	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Fatalf("Check error = %v, want the rejected exclusion", err)
	}
	if len(violations) != 0 {
		t.Errorf("Check reported %v beside the error, want no judgement of a rule that could not be built",
			messages(t, violations))
	}
}

func TestAnExclusionRendersInThePolicySentence(t *testing.T) {
	// A policy prints as the sentence the user typed, exclusions included, because that string is what a
	// failing clause is reported beside and a reader has to be able to see the carve-out in it.
	policy := fluentapi.ProjectLayers(nil).
		Layer("api").DefinedByFolder("internal/api/**").
		Except("**/generated").
		ExceptInPath("internal/api/legacy/*.go").
		WhereLayer("api").MayOnlyDependOnLayers()

	want := `project layers, ` +
		`layer "api" defined by path without filename matches "internal/api/**", ` +
		`excluding "**/generated", excluding path matches "internal/api/legacy/*.go", ` +
		`where layer "api", may only depend on no layers`
	if rendered := policy.String(); rendered != want {
		t.Errorf("the policy reads\n%q, want\n%q", rendered, want)
	}
}

// writeExcludedFixtureProject writes a project whose folders each hold a generated package: the api folder,
// whose generated client is the only thing in it that reaches the database, a domain folder and a model folder
// that make one layer declared twice, and the database itself.
//
// The generated client's dependency is what every exclusion in this file is worth: it is a real edge of the
// project, it breaks a clause about the api layer, and it is nobody's decision — which is the whole argument
// for taking a folder back out of a layer rather than declaring the layer as a list of the folders that are in
// it.
func writeExcludedFixtureProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                  "module example.com/excluded\n\ngo 1.26\n",
		"internal/api/handler.go": "package api\n\nfunc Handle() {}\n",
		"internal/api/router.go":  "package api\n\nfunc Route() {}\n",
		"internal/api/generated/client.go": "package generated\n\nimport \"example.com/excluded/internal/db\"\n\n" +
			"func Call() { db.Save(nil) }\n",
		"internal/domain/order.go":            "package domain\n\ntype Order struct{}\n",
		"internal/domain/generated/mapper.go": "package generated\n\nfunc Map() {}\n",
		"internal/model/thing.go":             "package model\n\ntype Thing struct{}\n",
		"internal/model/generated/dto.go":     "package generated\n\ntype DTO struct{}\n",
		"internal/db/conn.go":                 "package db\n\nfunc Save(any) {}\n",
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
