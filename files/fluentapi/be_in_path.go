package fluentapi

// BeInPath is the predicate that requires every file the scope selected to have an identifier matching this
// pattern, folder and name at once: `be in path`.
//
//	rule := archunit.ProjectFiles(nil).WithName("*_test.go").Should().BeInPath("internal/**/*_test.go")
//
// It looks at the whole identifier, which is the scope verb InPath's own reading one stage later, so it is
// the predicate for a convention that ties a name to a place — `internal/**/handler*.go` — and the one to
// reach for when neither the name alone nor the folder alone says what the rule means. When only the place
// matters, BeInFolder says so more plainly; when only the name does, HaveName does.
func (b FilesShouldBuilder) BeInPath(pattern string) FilesNamingCondition {
	return b.rule.requiring("be in path", pattern, b.rule.scope.factory.PathMatcher)
}

// BeInPath is the negated mood of the same predicate: `should not be in path`, which forbids the identifier
// rather than requiring it.
//
//	rule := archunit.ProjectFiles(nil).InFolder("internal/**").ShouldNot().BeInPath("**/legacy/**")
//
// It is the positive rule with assertion.Mood threaded into the same assertion — one violation per selected
// file whose identifier *does* match — and not a second implementation. Everything BeInPath says about which
// part of an identifier is looked at holds here unchanged.
func (b FilesShouldNotBuilder) BeInPath(pattern string) FilesNamingCondition {
	return b.rule.requiring("be in path", pattern, b.rule.scope.factory.PathMatcher)
}
