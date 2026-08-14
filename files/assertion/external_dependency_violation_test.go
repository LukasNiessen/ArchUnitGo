package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// An ExternalDependencyViolation is a Violation, which is the one thing a consumer of a rule's result has to
// know about it. Asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.ExternalDependencyViolation{}

func TestAnExternalDependencyViolationCarriesTheFileTheObjectAndTheModulesFound(t *testing.T) {
	// The data a report is built from, and no prose: the offending file, which modules the object described,
	// which of them it was actually found to import, and which way round the rule was written.
	required := []matching.Filter{pathMatcher(t, "github.com/deprecated/**")}

	violation := assertion.NewExternalDependencyViolation(
		"internal/domain/order.go",
		required,
		[]string{"github.com/deprecated/orm"},
		kernel.ShouldNot,
	)

	if violation.Kind() != assertion.KindFileExternalDependency {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindFileExternalDependency)
	}
	if assertion.KindFileExternalDependency != "file-external-dependency" {
		t.Errorf("KindFileExternalDependency = %q, want the name every ArchUnit port spells it with — and one"+
			" no other kind answers to", assertion.KindFileExternalDependency)
	}
	if violation.File != "internal/domain/order.go" {
		t.Errorf("File = %q, want the file the rule does not hold for", violation.File)
	}
	if len(violation.Required) != 1 || violation.Required[0].Target() != matching.TargetPath {
		t.Errorf("Required = %v, want the object selector as the rule compiled it", violation.Required)
	}
	if !slices.Equal(violation.Modules, []string{"github.com/deprecated/orm"}) {
		t.Errorf("Modules = %v, want the object's modules this file depends on", violation.Modules)
	}
	if violation.Mood != kernel.ShouldNot {
		t.Errorf("Mood = %s, want the mood the rule was written in", violation.Mood)
	}
}

func TestAnExternalDependencyViolationKeepsTheImportPathAsTheFileWroteIt(t *testing.T) {
	// A dependency is on a package, not on the module it was published as, and the violation says which:
	// shortening `golang.org/x/tools/go/packages` to its module would name an import nothing in the project
	// wrote and no editor could jump to.
	violation := assertion.NewExternalDependencyViolation(
		"common/extraction/extract_graph.go",
		[]matching.Filter{pathMatcher(t, "golang.org/**")},
		[]string{"golang.org/x/tools/go/packages"},
		kernel.ShouldNot,
	)

	if !slices.Equal(violation.Modules, []string{"golang.org/x/tools/go/packages"}) {
		t.Errorf("Modules = %v, want the import path exactly as the file wrote it", violation.Modules)
	}
}

func TestAnExternalDependencyViolationSortsTheModulesAndCopiesBothSlices(t *testing.T) {
	// A violation that has been reported must not change when the projection it was found in is walked on, and
	// a violation built from a hand-written list has to read like one built from a projection — so the modules
	// are sorted here rather than trusted to arrive that way.
	required := []matching.Filter{pathMatcher(t, "gorm.io/**"), pathMatcher(t, "github.com/lib/**")}
	modules := []string{"gorm.io/gorm", "github.com/lib/pq"}

	violation := assertion.NewExternalDependencyViolation("internal/db/conn.go", required, modules, kernel.ShouldNot)

	if want := []string{"github.com/lib/pq", "gorm.io/gorm"}; !slices.Equal(violation.Modules, want) {
		t.Errorf("Modules = %v, want them sorted: %v", violation.Modules, want)
	}
	modules[0] = "gorm.io/overwritten"
	required[0] = matching.Filter{}
	if slices.Contains(violation.Modules, "gorm.io/overwritten") {
		t.Errorf("Modules = %v after the caller's slice was written to, want the violation unchanged", violation.Modules)
	}
	if violation.Required[0].Pattern().Source() != "gorm.io/**" {
		t.Errorf("Required = %v after the caller's slice was written to, want the violation unchanged", violation.Required)
	}
}

func TestAnExternalDependencyViolationRendersTheRuleItBrokeAndWhatItDependsOn(t *testing.T) {
	tests := []struct {
		name      string
		violation assertion.ExternalDependencyViolation
		want      string
	}{
		{
			name: "should not, with the modules it was broken by",
			violation: assertion.NewExternalDependencyViolation(
				"internal/domain/order.go",
				[]matching.Filter{pathMatcher(t, "*.*/**")},
				[]string{"github.com/gin-gonic/gin", "gorm.io/gorm"},
				kernel.ShouldNot,
			),
			want: `internal/domain/order.go: should not, depend on external modules, path matches "*.*/**" -> github.com/gin-gonic/gin, gorm.io/gorm`,
		},
		{
			name: "should, whose offense is that there are none",
			violation: assertion.NewExternalDependencyViolation(
				"internal/api/router.go",
				[]matching.Filter{pathMatcher(t, "github.com/approved/**")},
				nil,
				kernel.Should,
			),
			want: `internal/api/router.go: should, depend on external modules, path matches "github.com/approved/**" -> nothing`,
		},
		{
			// The one place a chained object widens instead of narrowing, and the rendering has to say so: a
			// module cannot be two modules, so `and` here would read as a requirement nothing could ever meet.
			name: "an object of several verbs, combined with OR",
			violation: assertion.NewExternalDependencyViolation(
				"internal/db/conn.go",
				[]matching.Filter{pathMatcher(t, "gorm.io/**"), pathMatcher(t, "github.com/lib/pq")},
				[]string{"github.com/lib/pq"},
				kernel.ShouldNot,
			),
			want: `internal/db/conn.go: should not, depend on external modules, path matches "gorm.io/**" or path matches "github.com/lib/pq" -> github.com/lib/pq`,
		},
		{
			name: "an object nothing was chained onto, which is every module the project depends on",
			violation: assertion.NewExternalDependencyViolation(
				"internal/api/router.go",
				nil,
				nil,
				kernel.Should,
			),
			want: `internal/api/router.go: should, depend on external modules -> nothing`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if rendered := test.violation.String(); rendered != test.want {
				t.Errorf("String() = %q, want %q", rendered, test.want)
			}
		})
	}
}
