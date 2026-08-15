// Package archtest is the REPORT stage of the pipeline: it turns the violations a rule reported into the
// message a human reads, and a list of them into a pass flag and one report.
//
// It is the only place in the library where a message is built. A violation carries data — the offending
// file, the pattern it broke, the cycle it sits in, the mood the rule was written in — and never a
// sentence about it, so that phrasing, numbering and color are decided once, for every rule family, here.
// An adapter to a test framework asks ResultFactory for a Result and prints it; it does not format, and
// neither does a domain module.
//
// Two doors, and the one a test should reach for is the assert helper. It checks the rule and reports what it
// found in one call, with nothing to register and nothing to configure — AssertPasses for one rule, in any
// framework, and AssertAllPass for a suite of them, one named subtest per rule on the standard library's own
// handle:
//
//	archtest.AssertPasses(t, rule, nil)
//	archtest.AssertAllPass(t, rules, nil)
//
// The other door is the two factories the helpers are written over, for a caller assembling a report of its
// own shape — a summary line, a file of its own, a framework this package has never heard of:
//
//	violation := archtest.NewViolationFactory(nil).Message(oneViolation)
//	result := archtest.NewResultFactory(nil).Result(everyViolation)
//	if !result.Passed {
//		t.Error(result.Message)
//	}
//
// Every message has one shape — the subject that disagreed with the rule, the requirement it broke in the
// words the rule was written in, and what was found instead — because a reader who has learned to read
// one rule family's failure has then learned all of them. Palette names the parts of that shape rather
// than the rule families, for the same reason.
//
// A nil *MessageOptions means the defaults everywhere: plain text, every violation listed. Color is opted
// into, because test output is read from a CI log as often as from a terminal.
//
// The package is named archtest and not testing, which is the name the layout table and the sibling ports
// use. A package called testing shadows the stdlib testing in exactly the file that needs both — a test —
// and forces every user to alias one of the two imports. It is the answer AGENTS.md's Go-specifics
// section reaches for, and the same answer common/archerror already took for `error`.
package archtest

import (
	"fmt"
	"strconv"
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
)

// emptyTestHint is what a report adds to an empty-test violation, and the one message in this layer that
// explains the library rather than the code under it: a rule that selected nothing is the highest-value
// defensive check there is, and it is also the failure a reader is most likely to think is a bug in the
// library. Naming the opt-out is what turns the report into an answer.
const emptyTestHint = "an empty rule would hold forever, so selecting nothing is a violation rather " +
	"than a pass (AllowEmptyTests opts out)"

// cyclesRequirement is the sentence a cycle violation broke. It is spelled out rather than read off the
// violation because `have no cycles` is the one predicate that exists in a single mood: its negation would
// demand that the files be cyclic and could report nothing but the absence of a cycle, so the fluent API
// offers it on `should` alone and there is no other rule this violation could have come from.
const cyclesRequirement = "should, have no cycles"

// ViolationFactory phrases one violation: the collection of constructors that turns the data a rule
// reported into the sentence a reader gets.
//
// It knows the library's own violation types by sight, because a message is built from a violation's
// fields — the file, the compiled pattern, the mood, the dependencies actually found — and each family
// has its own. What it does not recognize is phrased from ViolationKind and whatever the violation can
// say about itself, so a rule family in a module written later, or a Checkable of a user's own, still
// reports something a human can read while step 8 of AGENTS.md's "Adding a new rule" is outstanding.
//
// It is immutable and cheap: a palette and nothing else. Build one per report, or keep one for a suite.
type ViolationFactory struct {
	palette Palette
}

// NewViolationFactory returns the factory that phrases violations under these options. A nil
// *MessageOptions means the defaults, so NewViolationFactory(nil) is the ordinary call and gives plain
// text.
func NewViolationFactory(options *MessageOptions) ViolationFactory {
	return ViolationFactory{palette: options.WithDefaults().Palette}
}

// Message is the sentence one violation reads as: what disagreed with the rule, the requirement it broke
// in the words the rule was written in, and what was found instead.
//
//	common/matching/filter.go: should, filename matches "regex_factory.go"; it does not
//	files/api/handler.go: should not, depend on files, path without filename matches "files/db"; it depends on files/db/conn.go
//	files/domain/order.go: should not, depend on external modules, path matches "*.*/**"; it depends on gorm.io/gorm
//	common/a.go: should, have no cycles; it depends on itself through common/a.go -> common/b.go -> common/a.go
//	layer "db": may not depend on layers "api"; it depends on api through db/conn.go -> api/handler.go
//	no files matched: path without filename matches "common/renamed"; an empty rule would hold forever, ...
//
// The requirement is always rendered as the rule stated it, never as its negation — `should not, filename
// matches "*_test.go"` and not "filename does not match" — which is what keeps assertion.Mood.Holds the
// one place in the library that inverts anything. The mood is one word of the sentence, and what was
// found follows from the violation existing at all: under `should` the requirement does not hold, under
// `should not` it does.
//
// A violation of a kind this layer has not been taught is phrased from its kind and its own String, and a
// nil violation reads as "(no violation)". Neither is a panic: this layer's whole job is to describe
// somebody else's failing test, and taking their test process down while doing it is the one outcome
// worse than a vague message.
func (f ViolationFactory) Message(violation kernel.Violation) string {
	if violation == nil {
		// A nil in a []Violation is a bug in whatever built the list, and it is reported rather than
		// skipped: a report with a numbered gap is how a reader finds out.
		return "(no violation)"
	}
	switch reported := violation.(type) {
	case kernel.EmptyTestViolation:
		return f.emptyTest(reported)
	case filesassertion.CycleViolation:
		return f.cycle(reported)
	case filesassertion.NamingViolation:
		return f.naming(reported)
	case filesassertion.DependencyViolation:
		return f.dependency(reported)
	case filesassertion.ExternalDependencyViolation:
		return f.externalDependency(reported)
	case filesassertion.AdherenceViolation:
		return f.adherence(reported)
	case layersassertion.DependencyViolation:
		return f.layerDependency(reported)
	default:
		return f.unphrased(violation)
	}
}

// Messages phrases every violation of a list, in the order they were reported, which is the order the
// rule found them in. It is what ResultFactory numbers, and what a caller assembling a report of its own
// shape asks for instead.
//
// A nil or empty list is no messages rather than one saying so: an empty result is the pass, and how a
// pass reads is ResultFactory.Result's to say.
func (f ViolationFactory) Messages(violations []kernel.Violation) []string {
	messages := make([]string, 0, len(violations))
	for _, violation := range violations {
		messages = append(messages, f.Message(violation))
	}
	return messages
}

// emptyTest phrases a rule that selected nothing: what it was selecting, the selectors that described the
// empty set, and the note that this is a violation on purpose.
//
// The hint is painted in the palette's quietest color rather than left out, because the alternative is a
// reader who believes the library is broken. The selectors are the whole diagnosis — one of those patterns
// is the stale one — so a violation carrying none says only that nothing matched.
func (f ViolationFactory) emptyTest(violation kernel.EmptyTestViolation) string {
	matched := "nothing matched"
	if violation.Subject != "" {
		matched = "no " + violation.Subject + " matched"
	}
	message := f.palette.Subject.Paint(matched)
	if len(violation.Selectors) > 0 {
		message += ": " + f.palette.Requirement.Paint(clauses(violation.Selectors))
	}
	return message + "; " + f.palette.Hint.Paint(emptyTestHint)
}

// cycle phrases a circular dependency between files as the path that has to be broken, closing onto the
// file it started from.
//
// The subject is the file the cycle closes onto rather than the set of them, because a reader has to open
// one file to start unpicking it and the whole path follows in the finding. The path is assembled here
// rather than taken from the circuit's own String, so that it is one piece a palette can paint and so that
// how a cycle reads stays this layer's decision.
func (f ViolationFactory) cycle(violation filesassertion.CycleViolation) string {
	files := violation.Files()
	if len(files) == 0 {
		// The enumeration cannot produce a circuit of no files; a violation built by hand can, and a
		// report of it says so rather than naming a file that is not there.
		return f.sentence("a cycle through no files", cyclesRequirement, "")
	}
	path := strings.Join(files, " -> ") + " -> " + files[0]
	return f.sentence(files[0], cyclesRequirement, "it depends on itself through "+path)
}

// naming phrases a file that is not named, or not placed, the way a rule requires. Which part of the
// identifier was looked at is the filter's own, so `have name`, `be in folder` and `be in path` are one
// phrasing here exactly as they are one violation type and one gather function.
func (f ViolationFactory) naming(violation filesassertion.NamingViolation) string {
	requirement := violation.Mood.String() + ", " + violation.Required.String()
	return f.sentence(violation.File, requirement, broke(violation.Mood))
}

// adherence phrases a file that does not satisfy a predicate the user wrote themselves. The requirement is
// the words they typed beside their function — this is the one rule whose requirement the library could not
// have derived — and it is quoted so that a reader can see where their sentence begins and ends.
func (f ViolationFactory) adherence(violation filesassertion.AdherenceViolation) string {
	requirement := violation.Mood.String() + `, adhere to "` + violation.Requirement + `"`
	return f.sentence(violation.File, requirement, broke(violation.Mood))
}

// dependency phrases a file that depends on the files a rule named where that was forbidden, or on none of
// them where it was required.
//
// The finding is the dependencies the violation carries, which is what a reader has to go and unpick under
// `should not`, and their absence — "it depends on none of them" — is the whole of the offense under
// `should`. Nothing here branches on the mood to decide which: the list is empty exactly when the absence
// is the offense.
func (f ViolationFactory) dependency(violation filesassertion.DependencyViolation) string {
	requirement := violation.Mood.String() + ", depend on files"
	if len(violation.Required) > 0 {
		requirement += ", " + clauses(violation.Required)
	}
	finding := "it depends on none of them"
	if len(violation.Dependencies) > 0 {
		finding = "it depends on " + strings.Join(violation.Dependencies, ", ")
	}
	return f.sentence(violation.File, requirement, finding)
}

// externalDependency phrases a file that depends on the external modules a rule named where that was
// forbidden, or on none of them where it was required.
//
// It is dependency's sentence over the other kind of object, and it is written out beside it rather than
// folded into it — as naming and adherence are — because phrasing is this layer's whole deliverable and the
// two objects are different things to a reader: one names folders of this project, the other names somebody
// else's modules. The selectors are joined with `or` rather than with a comma, which is the difference that
// makes them alternatives, and it is the one thing a report of this family must not blur.
func (f ViolationFactory) externalDependency(violation filesassertion.ExternalDependencyViolation) string {
	requirement := violation.Mood.String() + ", depend on external modules"
	if len(violation.Required) > 0 {
		requirement += ", " + alternatives(violation.Required)
	}
	finding := "it depends on none of them"
	if len(violation.Modules) > 0 {
		finding = "it depends on " + strings.Join(violation.Modules, ", ")
	}
	return f.sentence(violation.File, requirement, finding)
}

// layerDependency phrases one layer of a policy depending on another layer it may not: the pair of layers,
// the clause that forbade it in the words it was written in, and the file dependencies that connect them.
//
// The subject is the depending layer rather than a file, because a layer policy fails per pair of layers and
// the offense is that these two are connected at all — so the finding names the other layer first and the
// concrete dependencies after it, which is the order a reader needs them in: what is wrong, then where to
// look. A violation carrying no dependencies names the pair alone.
//
// The mood is one word of the requirement here as it is everywhere else, but it is `may only`/`may not`
// rather than `should`/`should not`: a layer policy is the one family whose user spells the mood as part of
// the predicate, so `should not, may not depend on layers` would be the sentence nobody typed. Nothing is
// inverted, exactly as in every other phrasing in this file — the requirement is what the clause said.
func (f ViolationFactory) layerDependency(violation layersassertion.DependencyViolation) string {
	finding := "it depends on " + violation.DependsOn
	if len(violation.Dependencies) > 0 {
		rendered := make([]string, 0, len(violation.Dependencies))
		for _, dependency := range violation.Dependencies {
			rendered = append(rendered, dependency.Source+" -> "+dependency.Target)
		}
		finding += " through " + strings.Join(rendered, ", ")
	}
	return f.sentence(`layer "`+violation.Layer+`"`, layerClause(violation.Mood, violation.Named), finding)
}

// unphrased phrases a violation this layer has not been taught: its kind, and whatever it can say about
// itself.
//
// Every violation type in the library renders itself for a log line, so a family whose phrasing is
// outstanding reports its own String and a reader loses the wording rather than the information. A
// violation that cannot even do that says so, and names what has to be taught — which is step 8 of
// AGENTS.md's "Adding a new rule", and the only reminder a report can give.
func (f ViolationFactory) unphrased(violation kernel.Violation) string {
	kind := string(violation.Kind())
	if kind == "" {
		kind = "unknown"
	}
	if described, ok := violation.(fmt.Stringer); ok {
		return f.sentence(kind, described.String(), "")
	}
	return f.sentence(kind, "archtest.ViolationFactory has not been taught to phrase this kind", "")
}

// sentence assembles the one shape every message in this layer has: the subject that disagreed with the
// rule, the requirement it broke, and what was found instead when there is anything to add.
//
// It is the reason Palette names roles instead of rule families. A reader learns the shape once — cyan is
// the thing to open, yellow is the rule, red is what it actually does — and then reads every family of
// violation the library will ever grow.
func (f ViolationFactory) sentence(subject, requirement, finding string) string {
	message := f.palette.Subject.Paint(subject) + ": " + f.palette.Requirement.Paint(requirement)
	if finding == "" {
		return message
	}
	return message + "; " + f.palette.Finding.Paint(finding)
}

// broke is what a violation of this mood found, given that it exists: the requirement does not hold where
// `should` demanded it, and does hold where `should not` forbade it.
//
// The mood picks a word here; it does not invert a requirement. That is Mood.Holds's job, one layer down,
// and the reason a report says `should not, filename matches "*_test.go"; it does` rather than claiming
// the rule asked for a name that does not match.
func broke(mood kernel.Mood) string {
	if mood.Negated() {
		return "it does"
	}
	return "it does not"
}

// layerClause renders the requirement a layer policy's clause stated: the mood as its own verb, then the
// layers it named, quoted and comma-separated — `may only depend on layers "domain", "db"` — or `may only
// depend on no layers`, which is the sealed layer and the one reading of an empty list that is still English.
//
// It is spelled here and not read off the clause for the reason this whole package exists: how a failure
// reads is one layer's decision, and a domain module's own String is for a log line. The layers are quoted
// like a filter's pattern, because they are the words a reader has to go and find in their own test.
func layerClause(mood kernel.Mood, named []string) string {
	verb := "may only depend on"
	if mood.Negated() {
		verb = "may not depend on"
	}
	if len(named) == 0 {
		return verb + " no layers"
	}
	quoted := make([]string, 0, len(named))
	for _, layer := range named {
		quoted = append(quoted, `"`+layer+`"`)
	}
	return verb + " layers " + strings.Join(quoted, ", ")
}

// clauses renders the selectors that described a population, in the order the user chained them onto the
// rule. They are combined with AND, and the comma is how the library's own types already spell that, from
// matching.Filter.String up through a violation's own rendering.
func clauses(selectors []matching.Filter) string {
	rendered := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		rendered = append(rendered, selector.String())
	}
	return strings.Join(rendered, ", ")
}

// alternatives renders the selectors that described a set of external modules, in the order the user chained
// them onto the rule. They are combined with OR — a module cannot be two modules at once — and `or` is how the
// library's own types already spell that, from the condition's rendering down to the violation's.
func alternatives(selectors []matching.Filter) string {
	rendered := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		rendered = append(rendered, selector.String())
	}
	return strings.Join(rendered, " or ")
}

// plural counts a noun the way a report needs it — `1 violation`, `3 violations` — because a heading that
// says "1 violations" is the first thing a reader distrusts. Every noun this layer counts takes a plain
// `s`, and one that does not would be a phrasing decision belonging in this package anyway.
func plural(count int, noun string) string {
	counted := strconv.Itoa(count) + " " + noun
	if count == 1 {
		return counted
	}
	return counted + "s"
}
