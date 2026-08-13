// Package archerror holds the library's two error types, and the distinction between them is about
// who is at fault. TechnicalError — the library or the environment failed. UserError — the API was
// used incorrectly.
//
// A failing architecture rule is neither. It is an assertion.Violation in the list a check returns,
// and nothing in the library ever turns a rule failure into an error: that is what lets a test suite
// run every rule and report all of them, instead of stopping at the first disagreement. The two are
// mutually exclusive in fluentapi.Checkable's contract — when the error is non-nil the violations say
// nothing, and when a rule merely fails the error is nil.
//
// Which of the two a failure is decides what a reader should do about it, so a caller reads the blame
// off the type:
//
//	var user *archerror.UserError
//	if errors.As(err, &user) {
//		// the rule as written is wrong — fix the test
//	}
//
// Both types wrap a cause, and neither carries a message string of its own. The reason a failure gives
// is always another error, usually a sentinel declared beside the API that produced it —
// matching.ErrInvalidPattern is the first of them — so that a caller can test for a reason with
// errors.Is instead of matching on prose, and so that a wrapped cause from the toolchain reaches the
// user intact.
//
// Rendering those parts into a sentence is the one place in the library where an Error method builds
// prose, because Go's error interface requires it. Violations still carry data only, and the testing
// layer still owns their phrasing.
package archerror

import "strconv"

// messagePrefix names the library in every message both types render, so that a failure surfacing in
// a test log far from here says where it came from.
const messagePrefix = "archunit: "

// TechnicalError says the library or its environment failed: a project that will not load, a file
// that will not parse, a toolchain that is not there. Neither the code under analysis nor the rule
// asking about it is at fault, and there is usually nothing a user can change in a rule to make it go
// away.
//
// It is the error a check returns when it could not reach the point of judging anything. A rule that
// did reach that point and disagreed with the code returns violations instead.
type TechnicalError struct {
	// Operation is what the library was doing, as a bare infinitive verb phrase in the library's own
	// vocabulary — `load the project`, `extract the dependency graph`, `parse a source file` — so
	// that it reads after "could not". It is empty when the caller has nothing more precise to say.
	Operation string
	// Subject is the thing the operation was working on: a normalized identifier, a project root, a
	// package path. It is empty when the operation was not about one thing in particular.
	Subject string
	// Cause is the underlying failure, wrapped rather than described, and reached with errors.Unwrap.
	// It is what carries a toolchain's own diagnostics through to the user, and it may be nil.
	Cause error
}

// NewTechnicalError records that the library failed at operation, on subject, because of cause. Any
// of the three may be empty or nil; the message says as much as it was given.
func NewTechnicalError(operation, subject string, cause error) *TechnicalError {
	return &TechnicalError{Operation: operation, Subject: subject, Cause: cause}
}

// Error renders the failure as `archunit: could not load the project "/src/app": no go.mod found`.
func (e *TechnicalError) Error() string {
	headline := "the library or its environment failed"
	if e.Operation != "" {
		headline = "could not " + e.Operation
	}
	return describe(headline, e.Subject, e.Cause)
}

// Unwrap returns the wrapped cause, so that errors.Is and errors.As reach through a TechnicalError to
// whatever the library or the toolchain actually reported.
func (e *TechnicalError) Unwrap() error {
	return e.Cause
}

// describe renders the message of both error types: the library's name, then a headline saying what
// failed, then the thing it was about, then the wrapped reason — each part omitted when there is
// nothing to say. It lives here, beside the prefix, so the two types differ only in the words that
// name the fault.
func describe(headline, subject string, cause error) string {
	message := messagePrefix + headline
	if subject != "" {
		message += " " + strconv.Quote(subject)
	}
	if cause != nil {
		message += ": " + cause.Error()
	}
	return message
}
