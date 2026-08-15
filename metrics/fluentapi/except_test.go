package fluentapi_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/metrics/fluentapi"
)

func TestExceptTakesAFolderBackOutOfTheMeasuredScope(t *testing.T) {
	// A number is only worth a threshold if it is about code somebody decided: generated code is long,
	// nobody's choice and the reason a measured average says nothing, so taking it out of the scope is the
	// difference between a metric a team can act on and one they cannot.
	scope := fluentapi.Metrics(measuredProject(t)).InFolder("internal/**")

	excepted := scope.Except("**/db")

	measurements := measure(t, excepted.Count().LinesOfCode(), nil)
	want := []string{"internal/api/handler.go", "internal/api/router.go"}
	if got := subjectsOf(measurements); !slices.Equal(got, want) {
		t.Errorf("%s measures %v, want %v", excepted, got, want)
	}
	if got := subjectsOf(measure(t, scope.Count().LinesOfCode(), nil)); len(got) != 3 {
		t.Errorf("%s measures %v, want the three files the exclusion is subtracted from", scope, got)
	}
}

func TestTheTargetedExclusionsOfAMetricsScopeNameTheirOwnTarget(t *testing.T) {
	// An exclusion may look at a part of an identifier its own verb does not, which is the half of the verb
	// that is not sugar: `in folder "internal/**", except with name "*_gen.go"` cannot be written as a
	// pattern of either verb alone.
	locator := measuredProject(t)

	tests := []struct {
		name  string
		scope fluentapi.MetricsBuilder
		want  []string
	}{
		{
			name:  "a folder verb excepting a filename",
			scope: fluentapi.Metrics(locator).InFolder("internal/**").ExceptWithName("router.go"),
			want:  []string{"internal/api/handler.go", "internal/db/conn.go"},
		},
		{
			name:  "a filename verb excepting a folder",
			scope: fluentapi.Metrics(locator).WithName("*.go").ExceptInFolder("internal/**"),
			want:  []string{"main.go"},
		},
		{
			name:  "a folder verb excepting a whole path",
			scope: fluentapi.Metrics(locator).InFolder("internal/**").ExceptInPath("internal/api/*.go"),
			want:  []string{"internal/db/conn.go"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := subjectsOf(measure(t, test.scope.Count().LinesOfCode(), nil)); !slices.Equal(got, test.want) {
				t.Errorf("%s measures %v, want %v", test.scope, got, test.want)
			}
		})
	}
}

func TestExceptClassesMatchingTakesAClassBackOutOfTheMeasuredScope(t *testing.T) {
	// The class population's own exclusion, over a metric whose subject is a class: every class of the
	// project except the one the rule is not about.
	scope := fluentapi.Metrics(measuredProject(t)).ForClassesMatching("*").ExceptClassesMatching("Router")

	measurements := measure(t, scope.Count().MethodCount(), nil)

	want := []string{"internal/api.Handler", "internal/db.Connection"}
	if got := subjectsOf(measurements); !slices.Equal(got, want) {
		t.Errorf("%s measures %v, want %v", scope, got, want)
	}
	if got := valuesOf(measurements); !slices.Equal(got, []float64{2, 0}) {
		t.Errorf("method counts = %v, want the counts the fixture was written with", got)
	}
}

func TestAPlainExclusionOfAMetricsScopeIsReadAgainstTheVerbItFollows(t *testing.T) {
	// A bare pattern is a second pattern of the same clause, so after `for classes matching` it is about
	// classes and after `in folder` about folders — one verb, two populations, and no ambiguity.
	locator := measuredProject(t)

	folder := fluentapi.Metrics(locator).InFolder("internal/**").Except("**/db")
	classes := fluentapi.Metrics(locator).ForClassesMatching("*").Except("Router")

	if got := subjectsOf(measure(t, folder.Count().LinesOfCode(), nil)); slices.Contains(got, "internal/db/conn.go") {
		t.Errorf("%s measures %v, want the exclusion read as the folder its verb is about", folder, got)
	}
	if got := subjectsOf(measure(t, classes.Count().MethodCount(), nil)); slices.Contains(got, "internal/api.Router") {
		t.Errorf("%s measures %v, want the exclusion read as the classname its verb is about", classes, got)
	}
}

func TestExclusionsOfAMetricsScopeAccumulateAndCanBeBranchedFrom(t *testing.T) {
	// Several patterns in one call and several calls are the same thing, and a stored scope is unchanged by
	// either: a builder is a value, so two rules derived from one cannot see each other's exclusions.
	scope := fluentapi.Metrics(measuredProject(t)).WithName("*.go")

	together := scope.Except("conn.go", "router.go")
	apart := scope.Except("router.go").Except("conn.go")

	want := []string{"internal/api/handler.go", "main.go"}
	if got := subjectsOf(measure(t, together.Count().LinesOfCode(), nil)); !slices.Equal(got, want) {
		t.Errorf("%s measures %v, want %v", together, got, want)
	}
	if got := subjectsOf(measure(t, apart.Count().LinesOfCode(), nil)); !slices.Equal(got, want) {
		t.Errorf("%s measures %v, want %v", apart, got, want)
	}
	if got := subjectsOf(measure(t, scope.Count().LinesOfCode(), nil)); len(got) != 4 {
		t.Errorf("%s measures %v, want the four files it measured before either exclusion was derived", scope, got)
	}
}

func TestAnExclusionQualifiesTheVerbItFollowsAndNotTheWholeMetricsScope(t *testing.T) {
	// A scope narrowed twice and then excepted once still means what it reads as: the exclusion belongs to
	// the clause it was written in.
	scope := fluentapi.Metrics(nil).InFolder("internal/**").WithName("*.go").Except("*_test.go")

	selectors := scope.Selectors()

	if len(selectors) != 2 {
		t.Fatalf("Selectors() = %v, want the two verbs that were typed", selectors)
	}
	if rendered := selectors[0].String(); rendered != `path without filename matches "internal/**"` {
		t.Errorf("the first verb reads %q, want it untouched by the exclusion", rendered)
	}
	if rendered := selectors[1].String(); rendered != `filename matches "*.go", excluding "*_test.go"` {
		t.Errorf("the second verb reads %q, want the exclusion on the verb it followed", rendered)
	}
}

func TestAnExclusionAboutTheOtherPopulationIsRejected(t *testing.T) {
	// This is the one family whose scope describes two populations, so it is the one place the guard can
	// fire: a class identifier has no folder and a file has no bare classname, and either question would be
	// answered wrongly rather than not at all.
	tests := []struct {
		name  string
		scope fluentapi.MetricsBuilder
		verb  string
	}{
		{
			name:  "a folder excluded from a class verb",
			scope: fluentapi.Metrics(nil).ForClassesMatching("*Service").ExceptInFolder("internal/legacy/**"),
			verb:  "except in folder",
		},
		{
			name:  "a filename excluded from a class verb",
			scope: fluentapi.Metrics(nil).ForClassesMatching("*Service").ExceptWithName("*_gen.go"),
			verb:  "except with name",
		},
		{
			name:  "a class excluded from a file verb",
			scope: fluentapi.Metrics(nil).InFolder("internal/**").ExceptClassesMatching("*Service"),
			verb:  "except classes matching",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.scope.Count().LinesOfCode().Measure(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Measure error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the verb %q", user.Operation, test.verb)
			}
			if !errors.Is(err, matching.ErrExclusionOfAnotherPopulation) {
				t.Errorf("Measure error = %v, want it to wrap ErrExclusionOfAnotherPopulation", err)
			}
		})
	}
}

func TestARejectedExclusionOfAMetricsScopeIsAUserErrorNamingTheExceptVerb(t *testing.T) {
	// The three ways an exclusion is typed wrongly, each naming the verb the user has to go and fix, and
	// each deferred to the resolving stage because a fluent method has nowhere to put an error.
	tests := []struct {
		name    string
		scope   fluentapi.MetricsBuilder
		verb    string
		subject string
		cause   error
	}{
		{
			name:    "a pattern that will not compile",
			scope:   fluentapi.Metrics(nil).InFolder("internal/**").Except("[unclosed"),
			verb:    "except",
			subject: "[unclosed",
			cause:   matching.ErrInvalidPattern,
		},
		{
			name:    "an exclusion with nothing to qualify",
			scope:   fluentapi.Metrics(nil).Except("**/generated"),
			verb:    "except",
			subject: "**/generated",
			cause:   matching.ErrExclusionWithoutSelector,
		},
		{
			name:    "an exclusion with no pattern",
			scope:   fluentapi.Metrics(nil).InFolder("internal/**").ExceptWithName(),
			verb:    "except with name",
			subject: "",
			cause:   matching.ErrExclusionWithoutPattern,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			measurements, err := test.scope.Count().LinesOfCode().Measure(nil)

			var user *archerror.UserError
			if !errors.As(err, &user) {
				t.Fatalf("Measure error = %v, want a *archerror.UserError", err)
			}
			if user.Operation != test.verb {
				t.Errorf("UserError.Operation = %q, want the verb %q", user.Operation, test.verb)
			}
			if user.Subject != test.subject {
				t.Errorf("UserError.Subject = %q, want the patterns as the user typed them, %q", user.Subject, test.subject)
			}
			if !errors.Is(err, test.cause) {
				t.Errorf("Measure error = %v, want it to wrap %v", err, test.cause)
			}
			if measurements != nil {
				t.Errorf("Measure reports %v beside the error, want no number at all", measurements)
			}
			if rendered := test.scope.String(); !strings.Contains(rendered, "rejected") {
				t.Errorf("String() = %q, want the rejection visible in a test failure", rendered)
			}
		})
	}
}

func TestAnExclusionRendersInTheMeasuredSentence(t *testing.T) {
	// A rule prints as the sentence the user typed, exclusions included, because that string is what a
	// failing threshold shows and a reader has to be able to see the carve-out in it.
	scope := fluentapi.Metrics(nil).
		InFolder("app/**").
		Except("**/generated").
		ExceptWithName("*_gen.go")

	want := `metrics, path without filename matches "app/**", excluding "**/generated", ` +
		`excluding filename matches "*_gen.go"`
	if rendered := scope.String(); rendered != want {
		t.Errorf("String() = %q, want %q", rendered, want)
	}
}
