package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// Except takes what these patterns name back out of the scope verb it follows: `metrics, in folder
// "app/**", except "**/generated"`.
//
//	measurements, err := archunit.Metrics(nil).
//		InFolder("app/**").
//		Except("**/generated").
//		Count().
//		LinesOfCode().
//		Measure(nil)
//
// It is the companion every selector in this library has, and in this family it is the difference between a
// number a team can act on and one they cannot: generated code is long, imported everywhere and nobody's
// decision, so a threshold measured over it is a threshold about the generator. Without an exclusion that
// scope is written as an inverted rule about the generated folder, which says the opposite of what the team
// means and hides a stale glob.
//
// The patterns are read against the same part of an identifier as the verb they qualify — a folder after `in
// folder`, a name after `with name`, a whole identifier after `in path`, a classname after `for classes
// matching` — because a bare exclusion is a second pattern of the same clause. ExceptWithName,
// ExceptInFolder, ExceptInPath and ExceptClassesMatching are the same verb with the target said out loud.
//
// It qualifies the verb the chain wrote most recently, and it is repeatable: several patterns in one call, or
// several calls, all veto. Two mistakes are reported by the resolving stage as a UserError, the way a pattern
// that will not compile is: `except` before any scope verb, which is an exclusion with nothing to qualify,
// and `except` with no pattern at all.
//
// One more is reported here and nowhere else in the library, because this is the one family whose scope
// describes two populations: an exclusion about classes cannot qualify a verb about files, or the other way
// round. An exclusion is asked about the identifier its verb is asked about, and a class identifier —
// `internal/api.UserService` — has no folder, so the question would have a wrong answer rather than no answer.
func (b MetricsBuilder) Except(patterns ...string) MetricsBuilder {
	return b.excepting("except", patterns, nil)
}

// ExceptWithName takes the files whose name matches one of these patterns out of the verb it follows,
// whatever that verb is about: `in folder "app/**", except with name "*_gen.go"`.
func (b MetricsBuilder) ExceptWithName(patterns ...string) MetricsBuilder {
	return b.excepting("except with name", patterns, matching.FilenameMatcher)
}

// ExceptInFolder takes the files in a folder matching one of these patterns out of the verb it follows:
// `with name "*.go", except in folder "**/generated"`.
func (b MetricsBuilder) ExceptInFolder(patterns ...string) MetricsBuilder {
	return b.excepting("except in folder", patterns, matching.FolderMatcher)
}

// ExceptInPath takes the files whose whole identifier matches one of these patterns out of the verb it
// follows: `in folder "app/**", except in path "app/legacy/*.go"`.
func (b MetricsBuilder) ExceptInPath(patterns ...string) MetricsBuilder {
	return b.excepting("except in path", patterns, matching.PathMatcher)
}

// ExceptClassesMatching takes the classes whose bare name matches one of these patterns out of the verb it
// follows: `for classes matching "*Service", except classes matching "*TestService"`. It is the one targeted
// exclusion of the class population, and it qualifies `for classes matching` alone, for the reason Except
// gives.
func (b MetricsBuilder) ExceptClassesMatching(patterns ...string) MetricsBuilder {
	return b.excepting("except classes matching", patterns, matching.ClassnameMatcher)
}

// excepting is every `except` verb of this family: hand the patterns to matching.Excepting, which attaches
// them to the last scope verb this builder was narrowed by, and hand back a new builder narrowed by the
// result. build is the target an exclusion names for itself, and nil is the plain form that inherits the
// qualified verb's own — so which part of an identifier a verb looks at is stated once per verb here, exactly
// as MetricsBuilder.selecting states it for the scope verbs themselves.
//
// A rejection is deferred to the resolving stage the way a scope verb's is: the first thing the user has to
// fix is the one reported, the rule renders with the rejection visible, and no selector is quietly dropped or
// widened in the meantime.
func (b MetricsBuilder) excepting(verb string, patterns []string, build func(matching.Pattern) matching.Filter) MetricsBuilder {
	excepted, err := matching.Excepting(b.selectors, b.factory, patterns, build)
	if err != nil {
		return b.rejecting(verb, strings.Join(patterns, ", "), err)
	}
	narrowed := b
	narrowed.selectors = excepted
	return narrowed
}
