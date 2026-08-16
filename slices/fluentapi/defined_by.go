package fluentapi

import "github.com/LukasNiessen/ArchUnitGo/common/matching"

// DefinedBy says what the slices of this project are, by a glob with one capture in it: `defined by
// "internal/(**)/**"`.
//
// The capture is the slice's name. Everything outside it says which files are sliced at all, and the part it
// encloses is the name cut out of each of them, so one pattern answers both questions at once:
//
//	byFolder := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**")   // internal/api/handler.go is in slice "api"
//	byTopFolder := archunit.ProjectSlices(nil).DefinedBy("(*)/**")          // cmd/server/main.go is in slice "cmd"
//	byTestedName := archunit.ProjectSlices(nil).DefinedBy("**/(*)_test.go") // handler_test.go is in slice "handler"
//
// The pattern is matched against the file's identifier: its path from the project root, in slash form, with
// the extension, so `internal/api/handler.go`. A file the pattern does not match is in no slice, and one that
// matches with an empty capture is in none either — there is no name to put it under. Inside the capture,
// `**` takes as little as it can, so `internal/(**)/**` is the folder under internal rather than the whole
// path down to the file.
//
// Exactly one capture is the rule, and a pattern with none or with two is reported by the terminal as a
// UserError wrapping matching.ErrOneCapture: with no group there is no name, and with two there is no saying
// which of them was meant. A second slicing verb is ErrSlicedTwice — a project has one slicing at a time,
// because the slicing is the vocabulary the rule is written in.
//
// Nothing is read here. Calling this verb hands back the builder the mood is asked of.
func (b SlicesBuilder) DefinedBy(pattern string) SlicesBuilder {
	return b.slicing("defined by", pattern, b.globs.CapturePattern)
}

// DefinedByRegex says what the slices of this project are, by a regular expression with one capturing group
// in it: `defined by regex "internal/([^/]+)/.*"`.
//
// It is the same verb as DefinedBy with the pattern read as Go's own regexp syntax, for the slicing a glob
// cannot express — an alternation, a character class, a name that is only part of a path segment:
//
//	byAlternation := archunit.ProjectSlices(nil).DefinedByRegex(`internal/(api|db)/.*`) // two slices, nothing else sliced
//	byNamePrefix := archunit.ProjectSlices(nil).DefinedByRegex(`([a-z]+)_[a-z]+\.go`)   // order_handler.go is in slice "order"
//
// The expression is anchored at both ends, as every pattern in this library is, so it has to describe the
// whole identifier and not just the part the name is cut from — `internal/([^/]+)/` names nothing, because no
// identifier ends at that slash. Writing `^` and `$` is harmless and says what is already true. Greediness is
// the caller's own here, unlike in DefinedBy, because in this syntax it is the caller who writes the
// quantifiers: `(.*)` names the longest run and `(.*?)` the shortest.
//
// Everything else holds exactly as it does for DefinedBy: exactly one capturing group, a file the expression
// does not match is in no slice, and a second slicing verb is ErrSlicedTwice. An expression that will not
// compile is reported by the terminal as a UserError naming this verb.
func (b SlicesBuilder) DefinedByRegex(expression string) SlicesBuilder {
	// The factory is built here rather than carried by the builder, unlike the glob factory DefinedBy reads
	// with, so that the zero SlicesBuilder stays the builder ProjectSlices(nil) returns: the zero RegexFactory
	// is glob syntax, so a regex-syntax factory in a field would be a builder only a constructor can produce,
	// and `var b SlicesBuilder; b.DefinedByRegex(...)` would silently compile a regular expression as a glob.
	regexes := matching.NewRegexFactory(&matching.RegexFactoryOptions{Syntax: matching.SyntaxRegex})
	return b.slicing("defined by regex", expression, regexes.CapturePattern)
}
