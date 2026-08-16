---
layout: default
title: Patterns and identifiers
nav_order: 3
description: What a glob means in ArchUnitGo, what each selector matches it against, and how a file or a declared type is identified.
---

# Patterns and identifiers

Every rule selects code by matching a pattern against an identifier. Two things are worth getting right
before you write a glob: what the identifier looks like, and which part of it the verb you chose reads.

## Identifiers

A file is identified by its path relative to the project root, always with forward slashes:
`internal/api/handler.go`. A declared type — a *class*, in the family's vocabulary — is that path's package
followed by the type's own name: `internal/api.Handler`.

Identifiers are normalised, so `internal\api\handler.go` on Windows and `internal/api/handler.go` on Linux
are the same identifier and one pattern matches both. They are project-relative throughout: no rule ever
sees the absolute path of your checkout, and a pattern that starts with one matches nothing.

## Globs

Patterns are globs, and globs are sugar: every one of them compiles to an anchored regular expression in one
place, so nothing downstream of the fluent API ever sees a glob.

| Glob | Means |
|---|---|
| `*` | any run of characters inside one segment, never crossing `/` |
| `**` | any run of characters, crossing `/`; `a/**` matches `a` itself as well as `a/b/c` |
| `?` | exactly one character, never `/` |
| `[a-z]`, `[!abc]` | one character from a class, or not from it |
| `(**)`, `(*)` | in a slicing only: the part of the identifier to cut a slice name out of |

Everything else is literal. Matching is case-sensitive, patterns are anchored at both ends — `api/**` does
not match `internal/api/handler.go`, and `**/api/**` does — and there is no escape character, because
separators are normalised and the one character you would want to escape is not special anywhere.

## What each verb matches against

Which part of the identifier a pattern is read against is the selector's business, and a violation message
names that part rather than the verb, because the part is what you have to compare your glob to.

| Verb | Matches against | For `internal/api/handler.go` |
|---|---|---|
| `WithName` | the filename | `handler.go` |
| `InFolder` | everything but the filename | `internal/api` |
| `InPath` | the whole identifier | `internal/api/handler.go` |
| `InFile` | the whole identifier, taken literally | — |
| `ForClassesMatching` | the declared type's own name | `Handler` |

`InFile` is the one that is not a pattern: the identifier is taken literally, so a file whose name contains
`*`, `[` or `.` needs no defensive spelling. Chaining it twice selects nothing, because scope verbs are
combined with AND and no file is two files.

## When a glob is not enough

`DefinedByRegex` and `SliceByRegex` take Go's own `regexp` syntax instead, for the patterns a glob cannot
spell — the fluent verb, and the projection behind it a caller can hold. Every other verb, `DefinedBy`
included, reads a glob:

```go
byRegex := archunit.ProjectSlices(nil).DefinedByRegex(`internal/([^/]+)/.*`)
byGlob := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**") // the glob spelling of the same slicing
```

Those two describe the same slicing — the second is the first written in the sugar. A regular expression is
anchored at both ends as well, and it has to hold exactly one capturing group when it is a slicing, because
the capture is where the slice's name comes from.

The layers family has the general form under a name of its own: `DefinedBy` matches the whole identifier of
a file, so a layer whose members are named rather than placed — `internal/**/*_repository.go` — is spelled
with it, and `DefinedByFolder` is the folder-shaped case said out loud.

## The projections behind the slicing verbs

A slicing is a `MapFunction`: the thing that relabels one dependency of the graph as a dependency between two
slices, or drops it. The fluent verbs build one for you, and four of them are exported for a caller who wants
to hold one:

| Function | Slices a project by |
|---|---|
| `archunit.SliceByPattern` | the capture in a glob — the projection behind `DefinedBy` |
| `archunit.SliceByRegex` | the capture in a regular expression — the projection behind `DefinedByRegex` |
| `archunit.SliceByFileSuffix` | the last `_`-separated word of a filename, so `order_handler.go` is in the slice `handler` |
| `archunit.Identity` | nothing at all: every dependency under the identifiers it already carries |

`SliceByPattern` and `SliceByRegex` return an error for a pattern that will not compile, or one that does not
capture exactly one name. `SliceByFileSuffix` takes no argument, so it has nothing to get wrong.

## A pattern that matches nothing

**Zero matches is a violation, not a pass.** A selector matching no file is almost always a stale glob or a
renamed folder, and a rule about nothing is green forever — so every terminal refuses one. That guard, and
the way to ask a selector what it actually resolved to, are on [running a rule](running.md).
