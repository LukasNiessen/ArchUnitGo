package assertion_test

import (
	"testing"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/files/assertion"
)

// AdherenceViolation is one of the violations every consumer of a rule programs against, so the interface is
// asserted at compile time rather than in a test that could be deleted.
var _ kernel.Violation = assertion.AdherenceViolation{}

func TestAdherenceViolationIsOfTheFileAdherenceKind(t *testing.T) {
	// The kind names the vocabulary as well as the failure, because every vocabulary the library grows can be
	// judged by a user's own function and the testing layer picks a phrasing by this key.
	violation := assertion.NewAdherenceViolation("internal/db/conn.go", "be at most 400 lines long", kernel.Should)

	if violation.Kind() != assertion.KindFileAdherence {
		t.Errorf("Kind() = %q, want %q", violation.Kind(), assertion.KindFileAdherence)
	}
	if assertion.KindFileAdherence != "file-adherence" {
		t.Errorf("KindFileAdherence = %q, want the name every ArchUnit port spells it with", assertion.KindFileAdherence)
	}
}

func TestAdherenceViolationCarriesTheFileTheRequirementAndTheMood(t *testing.T) {
	// The rule was a Go function, so the words the user gave alongside it are the only description a report
	// can have — which is why `adhere to` takes them and why they travel on the violation.
	violation := assertion.NewAdherenceViolation("internal/db/conn.go", "take a context.Context", kernel.Should)

	if violation.File != "internal/db/conn.go" {
		t.Errorf("File = %q, want the identifier of the offending file", violation.File)
	}
	if violation.Requirement != "take a context.Context" {
		t.Errorf("Requirement = %q, want the message the user wrote", violation.Requirement)
	}
	if violation.Mood != kernel.Should {
		t.Errorf("Mood = %s, want the mood the rule was written in", violation.Mood)
	}
}

func TestAdherenceViolationPrintsTheFileAndTheRequirementItBroke(t *testing.T) {
	// The offender first, because it is the thing to go and look at, then the sentence the file failed — in the
	// words the rule was written in, so nothing here inverts anything.
	tests := []struct {
		name      string
		violation assertion.AdherenceViolation
		want      string
	}{
		{
			name:      "should",
			violation: assertion.NewAdherenceViolation("internal/db/conn.go", "be at most 400 lines long", kernel.Should),
			want:      `internal/db/conn.go: should, adhere to "be at most 400 lines long"`,
		},
		{
			name:      "should not",
			violation: assertion.NewAdherenceViolation("internal/api/legacy.go", "mention the legacy client", kernel.ShouldNot),
			want:      `internal/api/legacy.go: should not, adhere to "mention the legacy client"`,
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
