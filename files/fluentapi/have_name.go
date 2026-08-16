package fluentapi

// HaveName is the predicate that requires every file the scope selected to be named as this pattern says:
// `have name`.
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/api/**").Should().HaveName("*.go")
//
// It looks at the last segment of a file's identifier, which is the scope verb WithName's own reading one
// stage later: `*_service.go` is every file whose name ends that way wherever it lives, and `handler.go` is
// that name exactly. A rule about where a file lives is BeInFolder or BeInPath.
//
// The two readings of the same words are worth keeping apart, because both are useful and only one is this
// one. `with name "*_service.go"` selects the files to talk about; `have name "*_service.go"` demands it of
// the files already selected — so `project files, in folder "internal/service/**", should, have name
// "*_service.go"` is a naming convention held over a folder, while chaining the scope verb instead would be
// a rule about nothing but the files that already comply.
func (b FilesShouldBuilder) HaveName(pattern string) FilesNamingCondition {
	return b.rule.requiring("have name", pattern, b.rule.scope.factory.FilenameMatcher)
}

// HaveName is the negated mood of the same predicate: `should not have name`, which forbids the name rather
// than requiring it.
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/**").ShouldNot().HaveName("*_deprecated.go")
//
// It is the positive rule with assertion.Mood threaded into the same assertion — one violation per selected
// file that *is* named that way — and not a second implementation. Everything HaveName says about which
// part of an identifier is looked at holds here unchanged.
func (b FilesShouldNotBuilder) HaveName(pattern string) FilesNamingCondition {
	return b.rule.requiring("have name", pattern, b.rule.scope.factory.FilenameMatcher)
}
