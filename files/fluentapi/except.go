package fluentapi

import (
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
)

// Except takes the files these patterns name back out of the selector it follows: `project files, in
// folder "app/**", except "**/generated"`.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("app/**").
//		Except("**/generated").
//		ShouldNot().
//		DependOnFiles().
//		InFolder("internal/db/**")
//
// It is the companion every selector in this library has, and it exists so that *everything under `app/`
// except the generated folder* is one clause rather than an inverted rule. Without it that scope is
// written as a rule about `**/generated` that says the opposite of what the team means, which is the
// reading a stale glob hides in.
//
// The patterns are read against the same part of an identifier as the selector they qualify, because a
// bare exclusion is a second pattern of the same clause: after `in folder` an exclusion is a folder,
// after `with name` it is a name, after `in path` a whole identifier. That is why the example above says
// `**/generated` and not `**/generated/**` — a folder pattern already covers the files in it.
// ExceptWithName, ExceptInFolder and ExceptInPath are the same verb with the target said out loud, for
// an exclusion that is not about what its selector is about.
//
// It qualifies the selector the chain wrote most recently, and it is repeatable: several patterns in one
// call, or several calls, all veto. `except` before any selector is rejected — an exclusion with nothing
// in front of it names the part of no identifier — as is `except` with no pattern at all; both are
// reported by the terminal as a UserError, the way a pattern that will not compile is.
//
// An exclusion is not a mood. It always takes files out of the selection, so it reads the same whether
// the rule that follows is `should` or `should not`, and it never widens.
func (b FilesBuilder) Except(patterns ...string) FilesBuilder {
	return b.excepting("except", patterns, nil)
}

// ExceptWithName takes the files whose name matches one of these patterns out of the selector it
// follows, whatever that selector is about: `in folder "app/**", except with name "*_gen.go"`.
func (b FilesBuilder) ExceptWithName(patterns ...string) FilesBuilder {
	return b.excepting("except with name", patterns, matching.FilenameMatcher)
}

// ExceptInFolder takes the files in a folder matching one of these patterns out of the selector it
// follows: `with name "*.go", except in folder "**/generated"`.
func (b FilesBuilder) ExceptInFolder(patterns ...string) FilesBuilder {
	return b.excepting("except in folder", patterns, matching.FolderMatcher)
}

// ExceptInPath takes the files whose whole identifier matches one of these patterns out of the selector
// it follows: `in folder "app/**", except in path "app/legacy/*.go"`.
func (b FilesBuilder) ExceptInPath(patterns ...string) FilesBuilder {
	return b.excepting("except in path", patterns, matching.PathMatcher)
}

// Except takes the files these patterns name back out of the object verb it follows: `depend on files,
// in folder "internal/db/**", except "**/db/dto"`.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/api/**").
//		ShouldNot().
//		DependOnFiles().
//		InFolder("internal/db/**").
//		Except("**/db/dto")
//
// It is FilesBuilder.Except over the other half of the sentence, and everything that verb says holds
// here: the patterns are read against the target of the object verb they qualify, they veto rather than
// widen, and an exclusion before any object verb is rejected. This is how a boundary gets its one
// documented hole — the data-transfer types an api package is allowed to see of a db package — without
// the rule becoming two rules.
func (c FilesDependencyCondition) Except(patterns ...string) FilesDependencyCondition {
	return c.excepting("except", patterns, nil)
}

// ExceptWithName takes the files whose name matches one of these patterns out of the object verb it
// follows: `depend on files, in folder "internal/db/**", except with name "*_dto.go"`.
func (c FilesDependencyCondition) ExceptWithName(patterns ...string) FilesDependencyCondition {
	return c.excepting("except with name", patterns, matching.FilenameMatcher)
}

// ExceptInFolder takes the files in a folder matching one of these patterns out of the object verb it
// follows: `depend on files, with name "*.go", except in folder "**/dto"`.
func (c FilesDependencyCondition) ExceptInFolder(patterns ...string) FilesDependencyCondition {
	return c.excepting("except in folder", patterns, matching.FolderMatcher)
}

// ExceptInPath takes the files whose whole identifier matches one of these patterns out of the object
// verb it follows: `depend on files, in folder "internal/db/**", except in path "internal/db/dto/*.go"`.
func (c FilesDependencyCondition) ExceptInPath(patterns ...string) FilesDependencyCondition {
	return c.excepting("except in path", patterns, matching.PathMatcher)
}

// Except takes the modules these patterns name back out of the `matching` verb it follows: `depend on
// external modules, matching "github.com/deprecated/**", except "github.com/deprecated/logging"`.
//
//	rule := archunit.ProjectFiles(nil).
//		InFolder("internal/domain/**").
//		ShouldNot().
//		DependOnExternalModules().
//		Matching("*.*/**").
//		Except("github.com/google/uuid")
//
// This is the third-party policy with a carve-out: no framework in the domain except the one library the
// team decided is part of its vocabulary. The patterns are read against the whole import path, which is
// what `matching` is read against, and they veto rather than widen.
//
// One thing is worth knowing here and nowhere else in this library: `matching` is the one verb whose
// repetitions are combined with OR, and an exclusion qualifies the alternative it follows rather than
// the list. `matching "a/**", except "a/b", matching "c/**"` forbids `c/b` if `c/**` names it, because
// the exclusion belongs to the clause it was written in. Two alternatives with the same carve-out say it
// twice, which is also the only spelling that survives the reader.
func (c FilesExternalDependencyCondition) Except(patterns ...string) FilesExternalDependencyCondition {
	excepted, err := matching.Excepting(c.modules, c.rule.scope.factory, patterns, nil)
	if err != nil {
		rejected := c
		rejected.rule.scope = c.rule.scope.rejecting("except", strings.Join(patterns, ", "), err)
		return rejected
	}
	narrowed := c
	narrowed.modules = excepted
	return narrowed
}

// excepting is every `except` verb of the scope: hand the patterns to matching.Excepting, which attaches
// them to the last scope verb this builder was narrowed by, and hand back a new builder narrowed by the
// result. build is the target an exclusion names for itself, and nil is the plain form that inherits the
// qualified selector's own — so which part of an identifier a verb looks at is stated once per verb here,
// exactly as FilesBuilder.selecting states it for the scope verbs themselves.
//
// A pattern this library cannot understand, an exclusion with no selector to qualify and an exclusion
// with no pattern are all deferred to the terminal the way a scope verb's rejection is: the first thing
// the user has to fix is the one reported, the rule renders with the rejection visible, and no selector
// is quietly dropped or widened in the meantime.
func (b FilesBuilder) excepting(verb string, patterns []string, build func(matching.Pattern) matching.Filter) FilesBuilder {
	excepted, err := matching.Excepting(b.selectors, b.factory, patterns, build)
	if err != nil {
		return b.rejecting(verb, strings.Join(patterns, ", "), err)
	}
	narrowed := b
	narrowed.selectors = excepted
	return narrowed
}

// excepting is every `except` verb of the object, and it is the scope's own twin one stage later: the
// patterns are attached to the last object verb, and a rejection joins the scope so that the terminal
// reports it in the words of the verb the user typed.
func (c FilesDependencyCondition) excepting(verb string, patterns []string, build func(matching.Pattern) matching.Filter) FilesDependencyCondition {
	excepted, err := matching.Excepting(c.objects, c.rule.scope.factory, patterns, build)
	if err != nil {
		rejected := c
		rejected.rule.scope = c.rule.scope.rejecting(verb, strings.Join(patterns, ", "), err)
		return rejected
	}
	narrowed := c
	narrowed.objects = excepted
	return narrowed
}
