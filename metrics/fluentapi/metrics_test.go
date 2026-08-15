package fluentapi_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/metrics/calculation"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

func TestMetricsWithNoScopeVerbMeasuresEveryFileOfTheProject(t *testing.T) {
	// `metrics` with nothing chained onto it: every file of the project is a subject, in the order the
	// selection produced them.
	rule := fluentapi.Metrics(measuredProject(t))

	if selectors := rule.Selectors(); len(selectors) != 0 {
		t.Errorf("Selectors() = %v, want none before a scope verb is chained", selectors)
	}
	measurements := measure(t, rule.Count().LinesOfCode(), nil)

	wantSubjects := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go", "main.go"}
	if got := subjectsOf(measurements); !slices.Equal(got, wantSubjects) {
		t.Errorf("`metrics` measures %v, want every file %v", got, wantSubjects)
	}
	wantValues := []int{9, 4, 3, 3}
	if got := valuesOf(measurements); !slices.Equal(got, wantValues) {
		t.Errorf("lines of code = %v, want the counts the fixture was written with, %v", got, wantValues)
	}
}

func TestEachScopeVerbLooksAtThePartOfAnIdentifierItNames(t *testing.T) {
	locator := measuredProject(t)

	tests := []struct {
		name string
		rule fluentapi.MetricsBuilder
		want []string
	}{
		{
			name: "with name",
			rule: fluentapi.Metrics(locator).WithName("*.go"),
			want: []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go", "main.go"},
		},
		{
			name: "with name, wherever the file lives",
			rule: fluentapi.Metrics(locator).WithName("conn.go"),
			want: []string{"internal/db/conn.go"},
		},
		{
			name: "in folder",
			rule: fluentapi.Metrics(locator).InFolder("internal/api"),
			want: []string{"internal/api/handler.go", "internal/api/router.go"},
		},
		{
			name: "in folder, and everything below it",
			rule: fluentapi.Metrics(locator).InFolder("internal/**"),
			want: []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"},
		},
		{
			name: "in folder, the project root",
			rule: fluentapi.Metrics(locator).InFolder("."),
			want: []string{"main.go"},
		},
		{
			name: "in path",
			rule: fluentapi.Metrics(locator).InPath("internal/*/handler.go"),
			want: []string{"internal/api/handler.go"},
		},
		{
			name: "for classes matching, the file declaring the one it names",
			rule: fluentapi.Metrics(locator).ForClassesMatching("Connection"),
			want: []string{"internal/db/conn.go"},
		},
		{
			name: "for classes matching, every file declaring one, in the order they were read",
			// `*o*` keeps Router and Connection and not Handler, so a metric about a file is measured over the
			// two files declaring them, in the order the selection produced them.
			rule: fluentapi.Metrics(locator).ForClassesMatching("*o*"),
			want: []string{"internal/api/router.go", "internal/db/conn.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			measurements := measure(t, test.rule.Count().LinesOfCode(), nil)

			if got := subjectsOf(measurements); !slices.Equal(got, test.want) {
				t.Errorf("%s measures %v, want %v", test.rule, got, test.want)
			}
		})
	}
}

func TestForClassesMatchingSelectsTheClassesAMetricAboutAClassIsMeasuredOver(t *testing.T) {
	// The one scope verb about a class: matched against the bare name, so the pattern says nothing about the
	// package, while the subject a measurement is reported about still does.
	locator := measuredProject(t)

	all := measure(t, fluentapi.Metrics(locator).Count().MethodCount(), nil)
	narrowed := measure(t, fluentapi.Metrics(locator).ForClassesMatching("*er").Count().MethodCount(), nil)

	wantAll := []string{"internal/api.Handler", "internal/api.Router", "internal/db.Connection"}
	if got := subjectsOf(all); !slices.Equal(got, wantAll) {
		t.Errorf("`metrics, count, method count` measures %v, want every class %v", got, wantAll)
	}
	wantNarrowed := []string{"internal/api.Handler", "internal/api.Router"}
	if got := subjectsOf(narrowed); !slices.Equal(got, wantNarrowed) {
		t.Errorf("`for classes matching \"*er\"` measures %v, want %v", got, wantNarrowed)
	}
	if got := valuesOf(narrowed); !slices.Equal(got, []int{2, 1}) {
		t.Errorf("method count = %v, want the 2 methods of Handler and the 1 member of Router", got)
	}
}

func TestTheScopeVerbsAreChainedWithAnd(t *testing.T) {
	locator := measuredProject(t)

	narrowed := fluentapi.Metrics(locator).InFolder("internal/**").WithName("*r.go")
	reversed := fluentapi.Metrics(locator).WithName("*r.go").InFolder("internal/**")

	if selectors := narrowed.Selectors(); len(selectors) != 2 {
		t.Fatalf("Selectors() = %v, want one per chained verb", selectors)
	}
	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if got := subjectsOf(measure(t, narrowed.Count().LinesOfCode(), nil)); !slices.Equal(got, want) {
		t.Errorf("%s measures %v, want %v", narrowed, got, want)
	}
	// Each verb narrows, so their order cannot change the rule.
	if got := subjectsOf(measure(t, reversed.Count().LinesOfCode(), nil)); !slices.Equal(got, want) {
		t.Errorf("%s measures %v, want %v", reversed, got, want)
	}
}

func TestAFileVerbAndAClassVerbNarrowTogether(t *testing.T) {
	// The two kinds of verb are one AND, and which kind a verb is, is the part of an identifier its filter
	// looks at — so a rule can name a folder and a class at once and mean the intersection.
	locator := measuredProject(t)

	rule := fluentapi.Metrics(locator).InFolder("internal/api").ForClassesMatching("*er")

	classes := subjectsOf(measure(t, rule.Count().FieldCount(), nil))
	if want := []string{"internal/api.Handler", "internal/api.Router"}; !slices.Equal(classes, want) {
		t.Errorf("%s measures %v, want the classes of that folder, %v", rule, classes, want)
	}
	elsewhere := subjectsOf(measure(t, fluentapi.Metrics(locator).InFolder("internal/db").ForClassesMatching("*er").Count().FieldCount(), nil))
	if len(elsewhere) != 0 {
		t.Errorf("a folder holding no matching class measures %v, want nothing", elsewhere)
	}
}

func TestAMetricsBuilderIsImmutableAndCanBeBranchedFrom(t *testing.T) {
	// The property the builder design exists for: a half-built rule stored in a variable, branched from
	// twice, and unchanged by either branch.
	locator := measuredProject(t)
	base := fluentapi.Metrics(locator).InFolder("internal/**")

	sources := base.WithName("*r.go")
	one := base.InPath("internal/db/conn.go")

	if selectors := base.Selectors(); len(selectors) != 1 {
		t.Errorf("the stored rule's Selectors() = %v, want the one verb it was built with", selectors)
	}
	if selectors := sources.Selectors(); len(selectors) != 2 {
		t.Errorf("the branch's Selectors() = %v, want the base's verb and its own", selectors)
	}
	wantBase := []string{"internal/api/handler.go", "internal/api/router.go", "internal/db/conn.go"}
	if got := subjectsOf(measure(t, base.Count().Statements(), nil)); !slices.Equal(got, wantBase) {
		t.Errorf("the stored rule measures %v, want %v", got, wantBase)
	}
	wantOne := []string{"internal/db/conn.go"}
	if got := subjectsOf(measure(t, one.Count().Statements(), nil)); !slices.Equal(got, wantOne) {
		t.Errorf("the second branch measures %v, want %v", got, wantOne)
	}
}

func TestABranchDoesNotWriteIntoTheRuleItGrewFrom(t *testing.T) {
	// The trap a value receiver alone does not close: a struct copy shares the selectors' backing array, so
	// two branches appending to it would write over each other. The base carries three verbs, because that is
	// where append leaves spare capacity for the second branch to write into.
	locator := measuredProject(t)
	base := fluentapi.Metrics(locator).InPath("internal/**").WithName("*.go").InFolder("internal/**")

	api := base.InFolder("internal/api")
	db := base.InFolder("internal/db")

	wantAPI := []string{"internal/api/handler.go", "internal/api/router.go"}
	if got := subjectsOf(measure(t, api.Count().Imports(), nil)); !slices.Equal(got, wantAPI) {
		t.Errorf("the first branch measures %v, want %v", got, wantAPI)
	}
	wantDB := []string{"internal/db/conn.go"}
	if got := subjectsOf(measure(t, db.Count().Imports(), nil)); !slices.Equal(got, wantDB) {
		t.Errorf("the second branch measures %v, want %v", got, wantDB)
	}
}

func TestAMetricsBuildersSelectorsAreTheCallersOwnCopy(t *testing.T) {
	rule := fluentapi.Metrics(nil).InFolder("internal/**")

	rule.Selectors()[0] = matching.Filter{}

	if selectors := rule.Selectors(); selectors[0].Target() != matching.TargetPathWithoutFilename {
		t.Errorf("Selectors() = %v after a caller overwrote the result, want the rule unchanged", selectors)
	}
}

func TestTheLocatorMetricsWasGivenCannotBeChangedAfterwards(t *testing.T) {
	// The half of immutability a value receiver cannot give: the builder holds a *ProjectLocator, so a caller
	// reusing one struct to build a rule per directory would leave every stored rule pointing at the last.
	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	locator := &extraction.ProjectLocator{Directory: writeMeasuredProject(t)}

	rule := fluentapi.Metrics(locator).InFolder("internal/db").Count().LinesOfCode()
	locator.Directory = t.TempDir()

	measurements, err := rule.Measure(nil)
	if err != nil {
		t.Fatalf("Measure failed after the locator it was given was written to: %v", err)
	}
	if got := subjectsOf(measurements); !slices.Equal(got, []string{"internal/db/conn.go"}) {
		t.Errorf("the rule measures %v, want the project it was built with", got)
	}
}

func TestARejectedPatternIsAUserErrorNamingTheScopeVerb(t *testing.T) {
	tests := []struct {
		verb string
		rule fluentapi.MetricsBuilder
	}{
		{verb: "with name", rule: fluentapi.Metrics(nil).WithName("[unclosed")},
		{verb: "in folder", rule: fluentapi.Metrics(nil).InFolder("[unclosed")},
		{verb: "in path", rule: fluentapi.Metrics(nil).InPath("[unclosed")},
		{verb: "for classes matching", rule: fluentapi.Metrics(nil).ForClassesMatching("[unclosed")},
	}

	for _, test := range tests {
		t.Run(test.verb, func(t *testing.T) {
			_, err := test.rule.Count().LinesOfCode().Measure(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Measure error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the scope verb %q", user.Operation, test.verb)
			}
			if !errors.Is(err, matching.ErrInvalidPattern) {
				t.Errorf("Measure error = %v, want it to wrap matching.ErrInvalidPattern", err)
			}
			if !strings.Contains(test.rule.String(), "rejected") {
				t.Errorf("%s renders without the rejection, want it visible in a test failure", test.rule)
			}
		})
	}
}

func TestARejectedPatternIsReportedBeforeTheProjectIsRead(t *testing.T) {
	// What the user typed is wrong whatever the project turns out to be, and reading the project first would
	// answer a typo with a complaint about the locator.
	rule := fluentapi.Metrics(&extraction.ProjectLocator{Directory: t.TempDir()}).InFolder("[unclosed")

	_, err := rule.Count().LinesOfCode().Measure(nil)

	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("Measure error = %v, want the rejected pattern rather than the missing project", err)
	}
	if errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("Measure error = %v, want the project left unread", err)
	}
}

func TestARejectedPatternNarrowsNothingAndOnlyTheFirstIsReported(t *testing.T) {
	rule := fluentapi.Metrics(nil).
		InFolder("internal/**").
		WithName("[unclosed").
		InPath("[also unclosed")

	if selectors := rule.Selectors(); len(selectors) != 1 {
		// A zero Filter matches nothing, so a rejected pattern joining the selectors would report every file
		// as unselected instead of reporting the typo.
		t.Errorf("Selectors() = %v, want only the verb that compiled", selectors)
	}

	_, err := rule.Count().LinesOfCode().Measure(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Measure error = %v, want a *archerror.UserError", err)
	}
	if user.Operation != "with name" {
		t.Errorf("UserError.Operation = %q, want the first rejected verb, `with name`", user.Operation)
	}
	if user.Subject != "[unclosed" {
		t.Errorf("UserError.Subject = %q, want the pattern as the user typed it", user.Subject)
	}
}

func TestMeasureRejectsALocatorThatIsNotAProject(t *testing.T) {
	rule := fluentapi.Metrics(&extraction.ProjectLocator{Directory: t.TempDir()}).Count().LinesOfCode()

	_, err := rule.Measure(nil)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("Measure error = %v, want a *archerror.UserError", err)
	}
	if !errors.Is(err, extraction.ErrModuleFileNotFound) {
		t.Errorf("Measure error = %v, want it to wrap ErrModuleFileNotFound", err)
	}
}

func TestTheCheckOptionsReachTheExtraction(t *testing.T) {
	// The same options every other rule in the library takes, so a rule about numbers can be held to the
	// test files as well.
	rule := fluentapi.Metrics(measuredProject(t)).InFolder("internal/api").Count().LinesOfCode()

	byDefault := subjectsOf(measure(t, rule, nil))
	withTests := subjectsOf(measure(t, rule, &kernel.CheckOptions{IncludeTestFiles: true}))

	if slices.Contains(byDefault, "internal/api/handler_test.go") {
		t.Errorf("%s measures %v by default, want the test file left out", rule, byDefault)
	}
	if !slices.Contains(withTests, "internal/api/handler_test.go") {
		t.Errorf("%s measures %v with IncludeTestFiles, want the test file among them", rule, withTests)
	}
}

func TestAMetricsBuilderRendersTheScopeItDescribes(t *testing.T) {
	rule := fluentapi.Metrics(nil).InFolder("internal/**").ForClassesMatching("*Service")

	rendered := rule.String()

	want := `metrics, path without filename matches "internal/**", classname matches "*Service"`
	if rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
	if entry := fluentapi.Metrics(nil).String(); entry != "metrics" {
		t.Errorf("String() = %q, want the entry point on its own", entry)
	}
}

// measure resolves a rule against the project its locator names, which is what every stage after the metric
// will do with the numbers it hands back.
func measure(t *testing.T, rule fluentapi.MetricBuilder, options *kernel.CheckOptions) []calculation.Measurement {
	t.Helper()

	measurements, err := rule.Measure(options)
	if err != nil {
		t.Fatalf("%s failed to measure: %v", rule, err)
	}
	return measurements
}

// subjectsOf and valuesOf are the two halves of a measurement list a test asserts about: what was measured,
// and what was found.
func subjectsOf(measurements []calculation.Measurement) []string {
	subjects := make([]string, 0, len(measurements))
	for _, measurement := range measurements {
		subjects = append(subjects, measurement.Subject)
	}
	return subjects
}

func valuesOf(measurements []calculation.Measurement) []int {
	values := make([]int, 0, len(measurements))
	for _, measurement := range measurements {
		values = append(values, measurement.Value)
	}
	return values
}

// measuredProject writes the fixture project into a directory of this test's own and hands back a locator
// naming it. The graph cache is keyed by project, and cleared so that a rule resolved here never reads a
// graph another test extracted.
func measuredProject(t *testing.T) *extraction.ProjectLocator {
	t.Helper()

	t.Cleanup(extraction.ClearGraphCache)
	extraction.ClearGraphCache()
	return &extraction.ProjectLocator{Directory: writeMeasuredProject(t)}
}

// writeMeasuredProject writes the project the tests here measure: a main package at the root, two packages
// below it, three classes between them, and a test file that is a subject only when the options ask for it.
// Every count differs from every other, so a rule reading the wrong number cannot pass.
func writeMeasuredProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/fixture\n\ngo 1.26\n",
		// Three lines of code, one import, one function, one statement and no class.
		"main.go": "package main\n\nimport \"example.com/fixture/internal/api\"\n\nfunc main() { api.Handle() }\n",
		// Nine lines of code, one import, one function, three statements, and Handler with two fields and
		// two methods.
		"internal/api/handler.go": "package api\n\nimport \"example.com/fixture/internal/db\"\n\n" +
			"// Handler serves.\ntype Handler struct {\n\tname string\n\tsize int\n}\n\n" +
			"func Handle() { db.Connect() }\n\nfunc (h Handler) Name() string { return h.name }\n\n" +
			"func (h Handler) Size() int { return h.size }\n",
		// Four lines of code and the one interface of the project, with one member.
		"internal/api/router.go":       "package api\n\ntype Router interface {\n\tRoute() error\n}\n",
		"internal/api/handler_test.go": "package api\n\nimport \"testing\"\n\nfunc TestHandle(*testing.T) { Handle() }\n",
		// Three lines of code, no import, one function and a class with neither field nor method.
		"internal/db/conn.go": "package db\n\ntype Connection struct{}\n\nfunc Connect() {}\n",
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
