package fluentapi_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestExceptTakesAFolderBackOutOfAFocus(t *testing.T) {
	// The sentence the issue asks for, about a report: everything under the api folder, but not the generated
	// package that happens to live in it. Written the other way round the pattern has to enumerate the folders
	// a reader does want, which goes stale the day somebody adds one.
	root := writeGeneratingFixtureProject(t)
	report := fluentapi.ProjectGraph(fixtureLocator(t, root)).FocusOn("internal/api/**", 0)

	excepted := report.Except("**/generated/**")

	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if got := nodeLabels(mustSnapshot(t, excepted).Nodes()); !slices.Equal(got, want) {
		t.Errorf("%s drew %v, want %v", excepted, got, want)
	}
	if got := nodeLabels(mustSnapshot(t, report).Nodes()); len(got) != 3 {
		t.Errorf("%s drew %v, want the three files the exclusion is subtracted from", report, got)
	}
}

func TestAnExclusionQualifiesTheModifierTheChainWroteMostRecently(t *testing.T) {
	// One verb over all four pattern modifiers, each excluding something only that modifier selects — so a
	// report says which of them the exclusion belongs to, rather than the exclusion being a filter over the
	// whole query.
	root := writeGeneratingFixtureProject(t)

	tests := []struct {
		name   string
		report fluentapi.GraphBuilder
		want   []string
	}{
		{
			name: "a focus, except the generated package in it",
			report: fluentapi.ProjectGraph(fixtureLocator(t, root)).
				FocusOn("internal/api/**", 0).Except("**/generated/**"),
			want: []string{"internal/api/handler.go", "internal/api/router.go"},
		},
		{
			name: "what the api reaches, except what only its generated package reaches",
			report: fluentapi.ProjectGraph(fixtureLocator(t, root)).
				ReachableFrom("internal/api/**").Except("**/generated/**"),
			want: []string{
				"internal/api/handler.go",
				"internal/api/router.go",
				"internal/db/conn.go",
			},
		},
		{
			name: "who depends on the database, except on the part of it nobody does",
			report: fluentapi.ProjectGraph(fixtureLocator(t, root)).
				DependentsOf("internal/db/**").Except("internal/db/generated/*.go"),
			want: []string{
				"internal/api/generated/client.go",
				"internal/api/handler.go",
				"internal/db/conn.go",
				"main.go",
			},
		},
		{
			name: "a group of everything internal, except the generated packages, which stay themselves",
			report: fluentapi.ProjectGraph(fixtureLocator(t, root)).
				CollapseByPattern("internal", "internal/**").Except("**/generated/**"),
			want: []string{
				"internal",
				"internal/api/generated/client.go",
				"internal/db/generated/rows.go",
				"main.go",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nodeLabels(mustSnapshot(t, test.report).Nodes()); !slices.Equal(got, test.want) {
				t.Errorf("%s drew %v, want %v", test.report, got, test.want)
			}
		})
	}
}

func TestAnExclusionQualifiesTheLastModifierAndNotTheOnesBeforeIt(t *testing.T) {
	// Two modifiers of the same kind and an exclusion after them: the second one carries it and the first is
	// left as it was written, which is what makes a chain of modifiers a chain of clauses. All four kinds are
	// here, because each of them is a list of its own that the exclusion has to be attached to the end of.
	tests := []struct {
		name   string
		report fluentapi.GraphBuilder
		want   string
	}{
		{
			name: "two focuses",
			report: fluentapi.ProjectGraph(nil).
				FocusOn("app/**", 1).
				FocusOn("internal/**", 0).
				Except("**/generated/**"),
			want: `project graph, ` +
				`focus on path matches "app/**" within 1 hop, ` +
				`focus on path matches "internal/**", excluding "**/generated/**" within 0 hops`,
		},
		{
			name: "two reachable-froms",
			report: fluentapi.ProjectGraph(nil).
				ReachableFrom("cmd/**").
				ReachableFrom("internal/**").
				Except("**/generated/**"),
			want: `project graph, ` +
				`reachable from path matches "cmd/**", ` +
				`reachable from path matches "internal/**", excluding "**/generated/**"`,
		},
		{
			name: "two dependents-ofs",
			report: fluentapi.ProjectGraph(nil).
				DependentsOf("internal/db/**").
				DependentsOf("internal/api/**").
				Except("**/generated/**"),
			want: `project graph, ` +
				`dependents of path matches "internal/db/**", ` +
				`dependents of path matches "internal/api/**", excluding "**/generated/**"`,
		},
		{
			name: "two collapse groups",
			report: fluentapi.ProjectGraph(nil).
				CollapseByPattern("api", "internal/api/**").
				CollapseByPattern("third party", "**").
				Except("golang.org/x/tools/**"),
			want: `project graph, ` +
				`collapse by pattern "api" by path matches "internal/api/**", ` +
				`collapse by pattern "third party" by path matches "**", excluding "golang.org/x/tools/**"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if rendered := test.report.String(); rendered != test.want {
				t.Errorf("the report reads\n%q, want\n%q", rendered, test.want)
			}
		})
	}
}

func TestAnExclusionQualifiesTheKindOfModifierWrittenLastAndNotAnotherKind(t *testing.T) {
	// Two modifiers of *different* kinds and an exclusion after them: which of the four lists the exclusion
	// is attached to is what the chain wrote last, not the order this module prints its modifiers in. Written
	// the other way round — the first list that has anything in it — every case below carries it on the wrong
	// clause, and a report would then narrow a modifier the user never excluded anything from.
	tests := []struct {
		name   string
		report fluentapi.GraphBuilder
		want   string
	}{
		{
			name: "a focus, then what a folder reaches",
			report: fluentapi.ProjectGraph(nil).
				FocusOn("internal/**", 0).
				ReachableFrom("internal/api/**").
				Except("**/generated/**"),
			want: `project graph, ` +
				`focus on path matches "internal/**" within 0 hops, ` +
				`reachable from path matches "internal/api/**", excluding "**/generated/**"`,
		},
		{
			name: "what a folder reaches, then a focus",
			report: fluentapi.ProjectGraph(nil).
				ReachableFrom("internal/api/**").
				FocusOn("internal/db/**", 0).
				Except("**/generated/**"),
			want: `project graph, ` +
				`focus on path matches "internal/db/**", excluding "**/generated/**" within 0 hops, ` +
				`reachable from path matches "internal/api/**"`,
		},
		{
			name: "a group, then a focus",
			report: fluentapi.ProjectGraph(nil).
				CollapseByPattern("internal", "internal/**").
				FocusOn("**", 0).
				Except("**/generated/**"),
			want: `project graph, ` +
				`focus on path matches "**", excluding "**/generated/**" within 0 hops, ` +
				`collapse by pattern "internal" by path matches "internal/**"`,
		},
		{
			name: "a focus, then who depends on a folder",
			report: fluentapi.ProjectGraph(nil).
				FocusOn("internal/**", 1).
				DependentsOf("internal/db/**").
				Except("**/generated/**"),
			want: `project graph, ` +
				`focus on path matches "internal/**" within 1 hop, ` +
				`dependents of path matches "internal/db/**", excluding "**/generated/**"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if rendered := test.report.String(); rendered != test.want {
				t.Errorf("the report reads\n%q, want\n%q", rendered, test.want)
			}
		})
	}
}

func TestAnExclusionSurvivesTheModifiersThatTakeNoPattern(t *testing.T) {
	// `titled` and `including external dependencies` are not selectors, so they do not come between an
	// exclusion and the pattern it reads as following — a chain that names its report before excluding
	// something still excludes it from the focus.
	report := fluentapi.ProjectGraph(nil).
		FocusOn("app/**", 0).
		IncludingExternalDependencies().
		Titled("the app").
		Except("**/generated/**")

	want := `project graph, including external dependencies, ` +
		`focus on path matches "app/**", excluding "**/generated/**" within 0 hops, ` +
		`titled "the app"`
	if rendered := report.String(); rendered != want {
		t.Errorf("the report reads\n%q, want\n%q", rendered, want)
	}
}

func TestExclusionsOfAReportAccumulateAndCanBeBranchedFrom(t *testing.T) {
	// Several patterns in one call and several calls are the same thing, and a stored report is unchanged by
	// either: a builder is a value, so two reports derived from one cannot see each other's exclusions.
	root := writeGeneratingFixtureProject(t)
	report := fluentapi.ProjectGraph(fixtureLocator(t, root)).FocusOn("**", 0)

	together := report.Except("**/generated/**", "internal/schema/*.go")
	apart := report.Except("**/generated/**").Except("internal/schema/*.go")

	want := []string{
		"internal/api/handler.go",
		"internal/api/router.go",
		"internal/db/conn.go",
		"main.go",
	}
	if got := nodeLabels(mustSnapshot(t, together).Nodes()); !slices.Equal(got, want) {
		t.Errorf("%s drew %v, want %v", together, got, want)
	}
	if got := nodeLabels(mustSnapshot(t, apart).Nodes()); !slices.Equal(got, want) {
		t.Errorf("%s drew %v, want %v", apart, got, want)
	}
	if got := nodeLabels(mustSnapshot(t, report).Nodes()); len(got) != 7 {
		t.Errorf("%s drew %v, want the seven files it drew before either exclusion was derived", report, got)
	}
}

func TestARejectedExclusionOfAReportIsAUserErrorNamingTheExceptVerb(t *testing.T) {
	// The three ways an exclusion is typed wrongly, each naming the verb the user has to go and fix, and each
	// deferred to the terminal because a fluent method has nowhere to put an error.
	tests := []struct {
		name    string
		report  fluentapi.GraphBuilder
		subject string
		cause   error
	}{
		{
			name:    "a pattern that will not compile",
			report:  fluentapi.ProjectGraph(nil).FocusOn("internal/**", 0).Except("[unclosed"),
			subject: "[unclosed",
			cause:   matching.ErrInvalidPattern,
		},
		{
			name:    "an exclusion with nothing to qualify",
			report:  fluentapi.ProjectGraph(nil).Titled("everything").Except("**/generated/**"),
			subject: "**/generated/**",
			cause:   matching.ErrExclusionWithoutSelector,
		},
		{
			name:    "an exclusion with no pattern",
			report:  fluentapi.ProjectGraph(nil).FocusOn("internal/**", 0).Except(),
			subject: "",
			cause:   matching.ErrExclusionWithoutPattern,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := test.report.Snapshot()

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Snapshot error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != "except" {
				t.Errorf("UserError.Operation = %q, want the verb `except`", user.Operation)
			}
			if user.Subject != test.subject {
				t.Errorf("UserError.Subject = %q, want the patterns as the user typed them, %q", user.Subject, test.subject)
			}
			if !errors.Is(err, test.cause) {
				t.Errorf("Snapshot error = %v, want it to wrap %v", err, test.cause)
			}
			if !snapshot.Empty() {
				t.Errorf("Snapshot reports %v beside the error, want nothing said about the project", snapshot)
			}
			if rendered := test.report.String(); !strings.Contains(rendered, "rejected") {
				t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
			}
		})
	}
}

func TestARejectedExclusionDoesNotWidenAReport(t *testing.T) {
	// The hazard of deferring the error: a report whose exclusion was refused must not be drawn as the report
	// without it, because that is a diagram of more of the project than was asked about — and it would look
	// like an answer.
	root := writeGeneratingFixtureProject(t)
	report := fluentapi.ProjectGraph(fixtureLocator(t, root)).
		FocusOn("internal/**", 0).
		Except("[unclosed")

	snapshot, err := report.Snapshot()

	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Fatalf("Snapshot error = %v, want the rejected exclusion", err)
	}
	if !snapshot.Empty() {
		t.Errorf("Snapshot drew %v beside the error, want no report at all", nodeLabels(snapshot.Nodes()))
	}
}

// writeGeneratingFixtureProject writes a project with a generated package in two of its folders: the api's
// generated client, which is the only thing that reaches the schema, and the database's generated rows, which
// nothing reaches at all.
//
//	main.go                            -> internal/api
//	internal/api/handler.go            -> internal/db, net/http
//	internal/api/generated/client.go   -> internal/db, internal/schema
//	internal/db/conn.go                -> database/sql (blank)
//
// The two generated packages are what every exclusion in this file is about, and they are placed so that each
// modifier has something only it selects: the api's is inside a focus and reaches a file nothing else does,
// and the database's is a file no dependent of the database depends on.
func writeGeneratingFixtureProject(t *testing.T) string {
	t.Helper()

	return writeProject(t, map[string]string{
		"go.mod":  "module example.com/generating\n\ngo 1.26\n",
		"main.go": "package main\n\nimport \"example.com/generating/internal/api\"\n\nfunc main() { api.Handle() }\n",
		"internal/api/handler.go": "package api\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/generating/internal/db\"\n)" +
			"\n\nfunc Handle() *http.Client { db.Connect(); return nil }\n",
		"internal/api/router.go": "package api\n\nfunc Route() {}\n",
		"internal/api/generated/client.go": "package generated\n\nimport (\n\t\"example.com/generating/internal/db\"\n\t" +
			"\"example.com/generating/internal/schema\"\n)\n\nfunc Call() schema.Row { db.Connect(); return schema.Row{} }\n",
		"internal/schema/schema.go":     "package schema\n\ntype Row struct{}\n",
		"internal/db/conn.go":           "package db\n\nimport _ \"database/sql\"\n\nfunc Connect() {}\n",
		"internal/db/generated/rows.go": "package generated\n\ntype Rows struct{}\n",
	})
}

// mustSnapshot resolves a report against its project, failing the test when the chain could not be understood
// or the project could not be read.
func mustSnapshot(t *testing.T, report fluentapi.GraphBuilder) projection.Snapshot {
	t.Helper()

	snapshot, err := report.Snapshot()
	if err != nil {
		t.Fatalf("drawing %s failed: %v", report, err)
	}
	return snapshot
}
