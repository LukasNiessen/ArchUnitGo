package assertion_test

import (
	"strings"
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// NamingViolation is one of the violations every consumer of a rule programs against, so the interface is
// asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.NamingViolation{}

func TestNamingViolationIsOfTheFileNamingKind(t *testing.T) {
	// The kind names the vocabulary as well as the failure, because every vocabulary the library grows
	// judges the names of its own nodes and the testing layer picks a phrasing by this key.
	violation := assertion.NewNamingViolation("internal/db/conn.go", filenameMatcher(t, "*_service.go"), kernel.Should)

	if violation.Kind() != assertion.KindFileNaming {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindFileNaming)
	}
	if assertion.KindFileNaming != "file-naming" {
		t.Errorf("KindFileNaming = %q, want the name every ArchUnit port spells it with", assertion.KindFileNaming)
	}
}

func TestNamingViolationCarriesTheFileTheRequirementAndTheMood(t *testing.T) {
	// The three predicates differ by nothing but the part of an identifier they look at, and that part
	// travels on the filter — so a report reads it there rather than from a kind per predicate.
	required := folderMatcher(t, "internal/api/**")

	violation := assertion.NewNamingViolation("internal/db/conn.go", required, kernel.Should)

	if violation.File != "internal/db/conn.go" {
		t.Errorf("File = %q, want the identifier of the offending file", violation.File)
	}
	if source := violation.Required.Pattern().Source(); source != "internal/api/**" {
		t.Errorf("Required quotes %q, want the pattern as the user typed it", source)
	}
	if target := violation.Required.Target(); target != matching.TargetPathWithoutFilename {
		t.Errorf("Required looks at %s, want the folder `be in folder` was checked against", target)
	}
	if violation.Mood != kernel.Should {
		t.Errorf("Mood = %s, want the mood the rule was written in", violation.Mood)
	}
}

func TestNamingViolationPrintsTheFileAndTheRequirementItBroke(t *testing.T) {
	// The offender first, because it is the thing to go and look at, then the sentence the file failed — in
	// the words the rule was written in, so nothing here inverts anything.
	tests := []struct {
		name      string
		violation assertion.NamingViolation
		want      string
	}{
		{
			name:      "should",
			violation: assertion.NewNamingViolation("internal/db/conn.go", filenameMatcher(t, "*_service.go"), kernel.Should),
			want:      `internal/db/conn.go: should, filename matches "*_service.go"`,
		},
		{
			name:      "should not",
			violation: assertion.NewNamingViolation("internal/api/legacy.go", pathMatcher(t, "**/legacy*.go"), kernel.ShouldNot),
			want:      `internal/api/legacy.go: should not, path matches "**/legacy*.go"`,
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

func TestNamingViolationOfNoRequirementSaysWhatItCan(t *testing.T) {
	// The zero Filter is what a rejected pattern would leave behind, and no check reaches it — the rejection
	// is an error before the project is read. It is readable anyway rather than panicking, because a
	// violation is data a report walks over.
	violation := assertion.NewNamingViolation("main.go", matching.Filter{}, kernel.Should)

	if violation.File != "main.go" {
		t.Errorf("File = %q, want the file it was made for", violation.File)
	}
	if rendered := violation.String(); !strings.HasPrefix(rendered, "main.go: should, ") {
		t.Errorf("String() = %q, want the file and the mood followed by whatever the filter says", rendered)
	}
}

// filenameMatcher, folderMatcher and pathMatcher compile the three requirements the naming predicates
// build, through the one factory that knows what a glob is — which is how a fluent stage builds them too.
func filenameMatcher(t *testing.T, pattern string) matching.Filter {
	t.Helper()

	return compiledMatcher(t, pattern, matching.RegexFactory.FilenameMatcher)
}

func folderMatcher(t *testing.T, pattern string) matching.Filter {
	t.Helper()

	return compiledMatcher(t, pattern, matching.RegexFactory.FolderMatcher)
}

func pathMatcher(t *testing.T, pattern string) matching.Filter {
	t.Helper()

	return compiledMatcher(t, pattern, matching.RegexFactory.PathMatcher)
}

func compiledMatcher(t *testing.T, pattern string, matcher func(matching.RegexFactory, string) (matching.Filter, error)) matching.Filter {
	t.Helper()

	filter, err := matcher(matching.NewRegexFactory(nil), pattern)
	if err != nil {
		t.Fatalf("compiling %q failed: %v", pattern, err)
	}
	return filter
}
