---
layout: default
title: Dependency-graph reports
nav_order: 8
description: The dependency graph of a Go project as data or as a diagram — nine modifiers, thirteen terminals, six formats, and no rule at all.
---

# Dependency-graph reports

This family judges nothing. It describes a report — what to draw, how far, under what labels — and then hands
it back as data or as a document:

```go
snapshot, err := archunit.ProjectGraph(nil).
	FocusOn("internal/api/**", 1).
	CollapseToFolderDepth(2).
	Titled("what the api layer touches").
	Snapshot()
```

`archunit.ProjectGraph` is the entry point and `archunit.DependencyGraph` the other name the family gives it.
There is no mood, no predicate and no violation type here, and that is the grammar rather than an omission: a
report is not a rule, so there is nothing for it to disagree with.

The default report is one node per file of the project's own code, with one arrow per dependency between two
of them.

## The nine modifiers

Every one is optional, chainable and order-independent.

| Modifier | Does |
|---|---|
| `IncludingExternalDependencies` | adds the code outside the project — the standard library and the modules it depends on |
| `IncludingSelfDependencies` | keeps a node's dependency on itself, which is dropped by default |
| `FocusOn` | narrows to the files a pattern names plus their neighborhood, that many hops out in **both** directions |
| `ReachableFrom` | narrows to those files and everything they depend on, transitively |
| `DependentsOf` | narrows to those files and everything that depends on them, transitively |
| `CollapseToFolderDepth` | draws each folder at that depth as one node |
| `CollapseByPattern` | draws every node a pattern names as one node under a label you choose |
| `Titled` | says what the report is called |
| `WithCheckOptions` | says how the project is read — test files, ignored import kinds, where it is |

`IncludingExternalDependencies` and `IncludingSelfDependencies` are the two verbs that widen the report; the
three that narrow it are three different questions. `FocusOn` is *what is around this code*, and depth 0 is the named files alone. `ReachableFrom`
follows the arrows forwards with no bound — *what does this pull in*, which is what a binary actually reaches.
`DependentsOf` follows them backwards — *who would notice if this changed*, which is the answer worth having
before deleting a folder.

```go
snapshot, err := archunit.ProjectGraph(nil).
	DependentsOf("internal/db/**").
	Titled("who would notice if the database changed").
	Snapshot()
```

Patterns here are matched against the whole identifier, so `internal/api/**` is that folder and everything
below it. They match identifiers and never the labels a collapse draws, which is what keeps focusing and
collapsing in one chain order-independent: the neighbors of the *files* named, drawn as folders.

The four modifiers that take a pattern each take `Except`, which qualifies the one the chain wrote most
recently — the one word in this family that is not order-independent, because an exclusion belongs to the
clause it was typed in:

```go
snapshot, err := archunit.ProjectGraph(nil).
	IncludingExternalDependencies().
	FocusOn("app/**", 1).
	Except("**/generated/**").
	CollapseByPattern("api", "internal/api/**").
	CollapseByPattern("third party", "**").
	Snapshot()
```

`CollapseByPattern` is the modifier for a diagram whose boxes are the architecture rather than the directory
tree: two folders that are one component, every dependency module as a single *third party* node, a legacy
corner as one box nobody wants to look inside. The label is asked for rather than derived, because a box has to
be called something and `internal/{api,web}/**` is not a name anybody wants to read — and giving those groups
the same names as [a layer policy's](layers.md) layers is what makes a report and a rule describe one
architecture. Groups are asked before `CollapseToFolderDepth`, so the two compose.

## The thirteen terminals

`Snapshot` hands the report back as data, and the other twelve hand it back as a document — six formats, each
as a string or as a file:

| Format | As a string | As a file |
|---|---|---|
| Graphviz | `ToDot` | `ExportAsDot` |
| Mermaid | `ToMermaid` | `ExportAsMermaid` |
| D2 | `ToD2` | `ExportAsD2` |
| Comma-separated values | `ToCSV` | `ExportAsCSV` |
| JSON | `ToJSON` | `ExportAsJSON` |
| A self-contained web page | `ToHTML` | `ExportAsHTML` |

```go
err := archunit.ProjectGraph(nil).
	CollapseToFolderDepth(2).
	Titled("the modules of this project").
	ExportAsHTML("build/architecture.html")
```

Every one of the twelve is `Snapshot` followed by one rendering function, which is why a new format is one
function this chain does not have to know about, and a modifier added here is understood by every format the
day it lands. A query that described a report with no node in it is `ErrEmptySnapshot` rather than an empty
diagram, for the same reason zero matches is a violation everywhere else.

Note that the terminals of this family take no options bag — the six exporters take the path they write to
and nothing else, and the other seven take no argument at all. The check options are a modifier here,
`WithCheckOptions`, because a report has no `Check` to pass them to.

## Reading a snapshot

A `GraphSnapshot` is immutable and ordered: nodes by label, dependencies by source and then target.

| Method | Hands back |
|---|---|
| `Nodes` | the `GraphNode` values that survived the query |
| `Edges` | the `GraphEdge` values drawn between them |
| `Summary` | the `GraphSummary`: the counts |
| `Title` | what the report is called |
| `Empty` | whether it holds no node at all |

A `GraphNode` has a `Label` — a file identifier, an import path, or the group a collapse merged it into — and
`IsExternal`, which says whether it is somebody else's code. A `GraphEdge` has its `SourceLabel` and
`TargetLabel`, an `IsSelfDependency` flag, an `IsExternal` flag, the import kinds behind it, and a `Count`:
how many of the project's raw dependencies this one arrow stands for. That count is what makes a collapsed
diagram honest — forty files merged onto two folders is one arrow, and an arrow that does not say *312
dependencies* invites the reader to think the two folders are barely coupled.

A `GraphSummary` is a snapshot in numbers, and every field is one a caller legitimately wants alone:
`Nodes`, `Edges`, `Dependencies` — the raw dependencies behind those edges, which does not shrink when a
collapse merges arrows — `ExternalNodes` and `ExternalEdges`. A test asserting that this project depends on
nothing outside it reads one of them.

## Next

The graph is extracted once per project and memoised, because a suite asks about one project many times —
which is why [running a rule](running.md) has a word to say about `archunit.ClearGraphCache`.
