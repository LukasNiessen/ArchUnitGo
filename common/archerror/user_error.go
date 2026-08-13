package archerror

// UserError says the API was used incorrectly: a glob that will not compile, a layer named in a rule
// but never defined, two options that contradict each other. The library is working and the code under
// analysis has not been judged yet — the rule as written cannot be run at all, so the person to tell
// is whoever wrote the test.
//
// It is not what a broken rule reports. A rule that runs and finds the code disagreeing with it returns
// violations; a UserError means there was no runnable rule to disagree with.
type UserError struct {
	// Operation is the call at fault, spelled the way it reads in the chain the user typed — `in
	// folder`, `depend on files`, `check options` — so that the message names the step to go and look
	// at. It is empty when the misuse belongs to no single step.
	Operation string
	// Subject is the argument that was wrong: the pattern, the layer name, the value of an option. It
	// is quoted in the message exactly as the user wrote it, never normalized, because a user searches
	// their own test for what they typed.
	Subject string
	// Cause is why the argument is wrong, wrapped rather than described, and reached with
	// errors.Unwrap. It is normally a sentinel declared beside the API that rejected the input, so a
	// caller can recognize a reason with errors.Is; it may be nil.
	Cause error
}

// NewUserError records that operation was used with subject, which is wrong because of cause. Any of
// the three may be empty or nil; the message says as much as it was given.
func NewUserError(operation, subject string, cause error) *UserError {
	return &UserError{Operation: operation, Subject: subject, Cause: cause}
}

// Error renders the misuse as `archunit: in folder "src/[": invalid pattern: character class in
// "src/[" is not closed`: the call, the argument, and why it was rejected.
func (e *UserError) Error() string {
	headline := "the API was used incorrectly"
	if e.Operation != "" {
		headline = e.Operation
	}
	return describe(headline, e.Subject, e.Cause)
}

// Unwrap returns the wrapped cause, so that errors.Is finds the sentinel that says which rule of the
// API was broken.
func (e *UserError) Unwrap() error {
	return e.Cause
}
