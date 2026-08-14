package assertion_test

import (
	"slices"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// A DependencyViolation is a Violation, which is the one thing a consumer of a rule's result has to know
// about it. Asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.DependencyViolation{}

func TestADependencyViolationCarriesTheFileTheObjectAndTheDependenciesFound(t *testing.T) {
	// The data a report is built from, and no prose: the offending file, what the object described, what it
	// was actually found to depend on, and which way round the rule was written.
	required := []matching.Filter{folderMatcher(t, "internal/db/**")}

	violation := assertion.NewDependencyViolation(
		"internal/api/handler.go",
		required,
		[]string{"internal/db/conn.go"},
		kernel.ShouldNot,
	)

	if violation.Kind() != assertion.KindFileDependency {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindFileDependency)
	}
	if violation.File != "internal/api/handler.go" {
		t.Errorf("File = %q, want the file the rule does not hold for", violation.File)
	}
	if len(violation.Required) != 1 || violation.Required[0].Target() != matching.TargetPathWithoutFilename {
		t.Errorf("Required = %v, want the object selector as the rule compiled it", violation.Required)
	}
	if !slices.Equal(violation.Dependencies, []string{"internal/db/conn.go"}) {
		t.Errorf("Dependencies = %v, want the object's files this one depends on", violation.Dependencies)
	}
	if violation.Mood != kernel.ShouldNot {
		t.Errorf("Mood = %s, want the mood the rule was written in", violation.Mood)
	}
}

func TestADependencyViolationSortsTheDependenciesAndCopiesBothSlices(t *testing.T) {
	// A violation that has been reported must not change when the projection it was found in is walked on, and
	// a violation built from a hand-written list has to read like one built from a projection — so the
	// dependencies are sorted here rather than trusted to arrive that way.
	required := []matching.Filter{folderMatcher(t, "internal/db/**"), filenameMatcher(t, "*.go")}
	dependencies := []string{"internal/db/query.go", "internal/db/conn.go"}

	violation := assertion.NewDependencyViolation("internal/api/handler.go", required, dependencies, kernel.ShouldNot)

	if want := []string{"internal/db/conn.go", "internal/db/query.go"}; !slices.Equal(violation.Dependencies, want) {
		t.Errorf("Dependencies = %v, want them sorted: %v", violation.Dependencies, want)
	}
	dependencies[0] = "internal/db/overwritten.go"
	required[0] = matching.Filter{}
	if slices.Contains(violation.Dependencies, "internal/db/overwritten.go") {
		t.Errorf("Dependencies = %v after the caller's slice was written to, want the violation unchanged", violation.Dependencies)
	}
	if violation.Required[0].Target() != matching.TargetPathWithoutFilename {
		t.Errorf("Required = %v after the caller's slice was written to, want the violation unchanged", violation.Required)
	}
}

func TestADependencyViolationRendersTheRuleItBrokeAndWhatItDependsOn(t *testing.T) {
	tests := []struct {
		name      string
		violation assertion.DependencyViolation
		want      string
	}{
		{
			name: "should not, with the dependencies it was broken by",
			violation: assertion.NewDependencyViolation(
				"internal/api/handler.go",
				[]matching.Filter{folderMatcher(t, "internal/db/**")},
				[]string{"internal/db/conn.go", "internal/db/query.go"},
				kernel.ShouldNot,
			),
			want: `internal/api/handler.go: should not, depend on files, path without filename matches "internal/db/**" -> internal/db/conn.go, internal/db/query.go`,
		},
		{
			name: "should, whose offense is that there are none",
			violation: assertion.NewDependencyViolation(
				"internal/api/router.go",
				[]matching.Filter{folderMatcher(t, "internal/domain/**")},
				nil,
				kernel.Should,
			),
			want: `internal/api/router.go: should, depend on files, path without filename matches "internal/domain/**" -> nothing`,
		},
		{
			name: "an object of several verbs, combined with AND",
			violation: assertion.NewDependencyViolation(
				"internal/api/handler.go",
				[]matching.Filter{folderMatcher(t, "internal/**"), filenameMatcher(t, "conn.go")},
				[]string{"internal/db/conn.go"},
				kernel.ShouldNot,
			),
			want: `internal/api/handler.go: should not, depend on files, path without filename matches "internal/**", filename matches "conn.go" -> internal/db/conn.go`,
		},
		{
			name: "an object nothing was chained onto, which is every file of the project",
			violation: assertion.NewDependencyViolation(
				"internal/api/router.go",
				nil,
				nil,
				kernel.Should,
			),
			want: `internal/api/router.go: should, depend on files -> nothing`,
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
