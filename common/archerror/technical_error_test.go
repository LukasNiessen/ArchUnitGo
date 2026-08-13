package archerror_test

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/archerror"
	"github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/fluentapi"
)

// The pointer is the error, for both types: it is the form every caller writes — `var user *UserError;
// errors.As(err, &user)` — and a value receiver would quietly not satisfy it. That is proved at compile
// time by the `err error` field of the tables here and in user_error_test.go.

func TestTechnicalErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "the operation reads after could not, the subject is quoted, the cause is the reason",
			err:  archerror.NewTechnicalError("load the project", "/src/app", errors.New("no go.mod found")),
			want: `archunit: could not load the project "/src/app": no go.mod found`,
		},
		{
			name: "an operation about no one thing in particular has no subject",
			err:  archerror.NewTechnicalError("extract the dependency graph", "", errors.New("the type checker gave up")),
			want: "archunit: could not extract the dependency graph: the type checker gave up",
		},
		{
			name: "a failure the library diagnosed itself has no cause to wrap",
			err:  archerror.NewTechnicalError("parse a source file", "internal/api/handler.go", nil),
			want: `archunit: could not parse a source file "internal/api/handler.go"`,
		},
		{
			name: "and the zero value still says who is at fault, because an empty message says nothing",
			err:  &archerror.TechnicalError{},
			want: "archunit: the library or its environment failed",
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

func TestTechnicalErrorCarriesItsCauseThroughAWrappedChain(t *testing.T) {
	// What the toolchain reported, wrapped by the library, wrapped again by whoever was running rules.
	// A caller at the far end can still recognize the original.
	failure := archerror.NewTechnicalError("parse a source file", "internal/api/handler.go",
		fmt.Errorf("open internal/api/handler.go: %w", fs.ErrPermission))
	err := fmt.Errorf("checking the architecture rules: %w", failure)

	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("errors.Is(err, fs.ErrPermission) = false, want the wrapped cause to be reachable: %v", err)
	}

	var technical *archerror.TechnicalError
	if !errors.As(err, &technical) {
		t.Fatalf("errors.As(err, **TechnicalError) = false, want the library's own error: %v", err)
	}
	if technical.Operation != "parse a source file" || technical.Subject != "internal/api/handler.go" {
		t.Errorf("the error carries %q / %q, want the operation and the file it failed on",
			technical.Operation, technical.Subject)
	}
	if !errors.Is(technical.Unwrap(), fs.ErrPermission) {
		t.Errorf("Unwrap() = %v, want the cause the library was handed", technical.Unwrap())
	}
}

func TestTheTwoErrorTypesAreNeverEachOther(t *testing.T) {
	// The whole point of having two types is that the blame is read off the type, so neither may be
	// mistaken for the other and a plain error is neither.
	tests := []struct {
		name          string
		err           error
		wantTechnical bool
		wantUser      bool
	}{
		{
			name:          "the library failed",
			err:           archerror.NewTechnicalError("load the project", "/src/app", nil),
			wantTechnical: true,
		},
		{
			name:     "the rule was written wrongly",
			err:      archerror.NewUserError("in folder", "src/[", nil),
			wantUser: true,
		},
		{
			name: "and something from elsewhere is unclassified rather than assumed",
			err:  errors.New("out of file descriptors"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				technical *archerror.TechnicalError
				user      *archerror.UserError
			)
			if got := errors.As(test.err, &technical); got != test.wantTechnical {
				t.Errorf("errors.As(err, **TechnicalError) = %t, want %t", got, test.wantTechnical)
			}
			if got := errors.As(test.err, &user); got != test.wantUser {
				t.Errorf("errors.As(err, **UserError) = %t, want %t", got, test.wantUser)
			}
		})
	}
}

func TestCheckTellsAFailureApartFromAFailingRule(t *testing.T) {
	// The level above the unit tests: the two error types where they will actually travel, out of
	// fluentapi.Checkable, and the third outcome they are not — a rule that ran and disagreed with the
	// code.
	tests := []struct {
		name string
		rule projectRule
		want string
	}{
		{
			name: "a project that will not load is the library's or the environment's problem",
			rule: projectRule{failure: archerror.NewTechnicalError("load the project", "/src/app",
				errors.New("no go.mod found"))},
			want: "the library",
		},
		{
			name: "a rule built from a pattern that will not compile is the test author's problem",
			rule: projectRule{failure: archerror.NewUserError("in folder", "src/[", nil)},
			want: "the user's rule",
		},
		{
			name: "a rule the code disagrees with is nobody's error: it is a violation",
			rule: projectRule{violations: []assertion.Violation{assertion.NewEmptyTestViolation("files")}},
			want: "the code under analysis",
		},
		{
			name: "and a rule that passes reports neither",
			rule: projectRule{},
			want: "nobody",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The consumer holds a Checkable and reads the blame off what came back.
			var rule fluentapi.Checkable = test.rule

			violations, err := rule.Check(nil)

			if got := blame(violations, err); got != test.want {
				t.Errorf("blame(%v, %v) = %q, want %q", violations, err, got, test.want)
			}
			if err != nil && len(violations) > 0 {
				t.Errorf("Check(nil) = %v alongside %v, want no violations beside an error: they say nothing",
					violations, err)
			}
		})
	}
}

// projectRule stands in for a terminal until the domain modules land: it reports whatever it was built
// with. Only the shape matters here — either an error or violations, never both.
type projectRule struct {
	failure    error
	violations []assertion.Violation
}

func (r projectRule) Check(*fluentapi.CheckOptions) ([]assertion.Violation, error) {
	if r.failure != nil {
		return nil, r.failure
	}
	return r.violations, nil
}

// blame is what a consumer does with what a check returned: the error's type says who has to fix it,
// and a violation is not an error at all. Nothing here asserts on a message.
func blame(violations []assertion.Violation, err error) string {
	var (
		technical *archerror.TechnicalError
		user      *archerror.UserError
	)
	switch {
	case errors.As(err, &user):
		return "the user's rule"
	case errors.As(err, &technical):
		return "the library"
	case err != nil:
		return "unclassified"
	case len(violations) > 0:
		return "the code under analysis"
	default:
		return "nobody"
	}
}
