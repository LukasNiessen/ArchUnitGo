package fluentapi

// IncludingExternalDependencies draws the standard library and the modules this project depends on as nodes
// too, instead of keeping the report to the project's own code.
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		IncludingExternalDependencies().
//		CollapseToFolderDepth(1).
//		Snapshot()
//
// Off unless asked for, because the two reports answer different questions: how a project is arranged is
// about its own files, and a diagram in which `fmt` and `net/http` are nodes mostly draws somebody else's
// code. This is the report that asks what the project depends on rather than how it is put together, and it
// is the one to reach for when the interesting fact is that a domain folder imports a database driver.
//
// External nodes are never collapsed to folder depth — an import path is not a folder of this project — but
// `collapse by pattern` does claim them, which is how a report draws every dependency module as one node.
//
// The modifier is a participle and it is chainable, order-independent and idempotent: including twice
// includes once.
func (b GraphBuilder) IncludingExternalDependencies() GraphBuilder {
	including := b.modifying()
	including.query.IncludeExternalDependencies = true
	return including
}

// IncludingSelfDependencies draws a node's dependency on itself, which after a collapse says that the files
// inside a folder depend on each other.
//
//	snapshot, err := archunit.ProjectGraph(nil).
//		CollapseToFolderDepth(2).
//		IncludingSelfDependencies().
//		Snapshot()
//
// Off unless asked for, and only ever a question after a collapse: a file does not depend on itself, so an
// uncollapsed report has no such dependency to draw and this modifier changes nothing. Collapsed, it is the
// cohesion report — a folder with a loud self-dependency is one whose files belong together, and a folder
// with none is a folder that is only a folder.
//
// The count on a self-dependency is what makes it worth drawing: `internal/api -> internal/api [312
// dependencies]` and `[2 dependencies]` are very different facts about the same box on a diagram.
//
// The modifier is a participle and it is chainable, order-independent and idempotent.
func (b GraphBuilder) IncludingSelfDependencies() GraphBuilder {
	including := b.modifying()
	including.query.IncludeSelfDependencies = true
	return including
}
