package archerror_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

func TestUserErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "the call at fault, the argument that was wrong, and why it was rejected",
			err:  archerror.NewUserError("in folder", "src/[", errors.New("character class is not closed")),
			want: `archunit: in folder "src/[": character class is not closed`,
		},
		{
			name: "a misuse with no single argument to point at has no subject",
			err:  archerror.NewUserError("check options", "", errors.New("a nil writer cannot be logged to")),
			want: "archunit: check options: a nil writer cannot be logged to",
		},
		{
			name: "a rule that named a layer nobody defined needs no cause to wrap",
			err:  archerror.NewUserError("may only depend on layers", "persistence", nil),
			want: `archunit: may only depend on layers "persistence"`,
		},
		{
			name: "and the zero value still says who is at fault",
			err:  &archerror.UserError{},
			want: "archunit: the API was used incorrectly",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.want {
				t.Errorf("Error() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUserErrorWrapsTheReasonSoACallerCanRecognizeIt(t *testing.T) {
	// The reason a UserError gives is another error, not a sentence of its own: here the sentinel the
	// matching package already declares for a pattern it cannot understand.
	_, compileErr := matching.NewGlobPattern("internal/api/[", nil)
	if compileErr == nil {
		t.Fatal("NewGlobPattern(`internal/api/[`) succeeded, want a pattern that will not compile")
	}

	err := archerror.NewUserError("in folder", "internal/api/[", compileErr)

	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("errors.Is(err, matching.ErrInvalidPattern) = false, want the reason to be recognizable: %v", err)
	}
	if !strings.Contains(err.Error(), `"internal/api/["`) {
		t.Errorf("Error() = %q, want the pattern the user typed in it", err.Error())
	}
}

func TestUserErrorIsWhereAFluentStageRejectsWhatTheUserTyped(t *testing.T) {
	// The level above the unit tests: a scope verb, in the shape every one of them will have. A pattern
	// that compiles yields a selector; one that does not yields a UserError naming the verb, because
	// only the fluent stage knows which call the user made.
	graph := extraction.NewGraph(
		extraction.NewEdge("internal/api/handler.go", "internal/db/store.go", false, extraction.ImportKindPlain),
		extraction.SelfEdge("internal/db/store.go"),
	)

	selector, err := inFolder("internal/api/**")
	if err != nil {
		t.Fatalf("inFolder(`internal/api/**`) failed: %v", err)
	}
	if !selector.Matches("internal/api/handler.go") {
		t.Errorf("the selector matches none of %v, want the api file", graph.Nodes())
	}

	// Separators are the library's business, but the subject is quoted as the user wrote it: that is
	// the string they will search their own test for.
	_, err = inFolder(`internal\api\[`)

	var user *archerror.UserError
	if !errors.As(err, &user) {
		t.Fatalf("inFolder(`internal\\api\\[`) error = %v, want a UserError", err)
	}
	if user.Operation != "in folder" {
		t.Errorf("Operation = %q, want the scope verb the user typed", user.Operation)
	}
	if user.Subject != `internal\api\[` {
		t.Errorf("Subject = %q, want the pattern exactly as written", user.Subject)
	}
	if !errors.Is(err, matching.ErrInvalidPattern) {
		t.Errorf("errors.Is(err, matching.ErrInvalidPattern) = false, want the reason underneath: %v", err)
	}
}

// inFolder stands in for the scope verb of the same name until the files module lands. It is the wrap
// point: matching knows a pattern is invalid, and this is the only level that knows which call the user
// made, which is what UserError.Operation names.
func inFolder(pattern string) (matching.Filter, error) {
	selector, err := matching.NewRegexFactory(nil).FolderMatcher(pattern)
	if err != nil {
		return matching.Filter{}, archerror.NewUserError("in folder", pattern, err)
	}
	return selector, nil
}
