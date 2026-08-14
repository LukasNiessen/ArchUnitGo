package fluentapi

// BeInFolder is the predicate that requires every file the scope selected to live in a folder matching this
// pattern: `be in folder`.
//
//	rule := archunit.ProjectFiles(nil).WithName("*_handler.go").Should().BeInFolder("internal/api/**")
//
// It looks at a file's identifier without its last segment, which is the scope verb InFolder's own reading
// one stage later: `internal/api` is that folder alone, `internal/api/**` is it together with everything
// below it, and a file at the project root is in the folder `.`.
//
// Read as a sentence it is the rule that keeps a kind of file where it belongs — *every file called
// `*_handler.go` should be in folder `internal/api/**`* — which is why the scope is usually a name and the
// predicate a place. Selecting by folder and then demanding the same folder is a rule that cannot fail.
func (b FilesShouldBuilder) BeInFolder(pattern string) FilesNamingCondition {
	return b.rule.requiring("be in folder", pattern, b.rule.scope.factory.FolderMatcher)
}

// BeInFolder is the negated mood of the same predicate: `should not be in folder`, which forbids the place
// rather than requiring it.
//
//	rule := archunit.ProjectFiles(nil).WithName("*_test_helper.go").ShouldNot().BeInFolder("internal/api/**")
//
// It is the positive rule with assertion.Mood threaded into the same assertion — one violation per selected
// file that *is* in such a folder — and not a second implementation. Everything BeInFolder says about which
// part of an identifier is looked at holds here unchanged.
func (b FilesShouldNotBuilder) BeInFolder(pattern string) FilesNamingCondition {
	return b.rule.requiring("be in folder", pattern, b.rule.scope.factory.FolderMatcher)
}
