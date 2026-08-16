---
layout: default
title: The files family
nav_order: 4
description: File-level rules in ArchUnitGo — what depends on what, what depends on which third-party module, cycles, naming, placement and your own predicate.
---

# The files family

The file is this library's primary node: a rule of this family selects files and then says something about
what they depend on, how they are named or where they live. It is the family to reach for first, and the one
the other four are variations on.

```go
rule := archunit.ProjectFiles(nil).
	InFolder("internal/api/**").
	ShouldNot().
	DependOnFiles().
	InFolder("internal/db/**")
```

`archunit.ProjectFiles` is the entry point, `archunit.Files` is the same one under a shorter name, and both
take an optional `*ProjectLocator`.

## Scope

The scope verbs are chainable and combined with AND, so each one narrows what the rule is about. Every one of
them takes an exclusion.

| Verb | Selects the files whose |
|---|---|
| `WithName` | filename matches the pattern |
| `InFolder` | folder — the identifier without the filename — matches the pattern |
| `InPath` | whole identifier matches the pattern |
| `InFile` | identifier *is* this string, taken literally |

| Exclusion | Reads its patterns against |
|---|---|
| `Except` | whatever the selector in front of it reads |
| `ExceptWithName` | the filename |
| `ExceptInFolder` | the folder |
| `ExceptInPath` | the whole identifier |

To see what a scope resolves to before you judge it, ask the scope rather than the rule. `SelectFiles` hands
back the identifiers, sorted:

```go
files, err := archunit.ProjectFiles(nil).
	InFolder("internal/api/**").
	ExceptWithName("*_gen.go").
	SelectFiles(nil)
```

## The predicates

`Should` and `ShouldNot` open the mood stage, and these are the predicates behind them:

| Predicate | Says that a selected file | Moods | Reports |
|---|---|---|---|
| `DependOnFiles` | depends on the files the object names | both | `FileDependencyViolation` |
| `DependOnExternalModules` | depends on the third-party modules the object names | both | `FileExternalDependencyViolation` |
| `HaveNoCycles` | is in no dependency circle | `Should` only | `FileCycleViolation` |
| `HaveName` | is named as the pattern says | both | `FileNamingViolation` |
| `BeInFolder` | sits where the pattern says | both | `FileNamingViolation` |
| `BeInPath` | has the identifier the pattern says | both | `FileNamingViolation` |
| `AdhereTo` | satisfies a function you wrote | both | `FileAdherenceViolation` |

Every one of them ends in `Check`, and each violation carries the offending file, the requirement as a
compiled pattern and what was found instead — never a sentence. [Running a rule](running.md) is where the
prose gets built.

## A boundary between two folders

`DependOnFiles` opens an object stage: the files the rule is about depending on. Its verbs are the scope's
three pattern verbs, chainable and combined with AND, and each takes the same four exclusions:

```go
rule := archunit.ProjectFiles(nil).
	InFolder("internal/api/**").
	Except("**/generated").
	ShouldNot().
	DependOnFiles().
	InFolder("internal/db/**").
	ExceptInFolder("internal/db/dto/**")
```

That reads as *the api does not touch the database, except for its data-transfer types* — a boundary with one
documented door in it. In the positive mood the same chain is a requirement: every selected file has to
depend on at least one file the object matches, which is how *every handler uses the service layer* is
written.

| Object verb | Matches against |
|---|---|
| `WithName` | the filename of the file depended on |
| `InFolder` | its folder |
| `InPath` | its whole identifier |

## Third-party modules

`DependOnExternalModules` is the same shape for the code outside your project, and its one object verb is
`Matching`, which is read against the import path:

```go
rule := archunit.ProjectFiles(nil).
	InFolder("internal/domain/**").
	ShouldNot().
	DependOnExternalModules().
	Matching("*.*/**").
	Except("gopkg.in/yaml*/**")
```

`Matching` is repeatable and **combined with OR** — the one chain in this library that widens rather than
narrows, because *any of these modules* is what a third-party policy means. `*.*/**` is the glob for
*anything with a dot in its first segment*, which is every module path that is not a standard library
package.

## Cycles

```go
rule := archunit.ProjectFiles(nil).InFolder("internal/**").Should().HaveNoCycles()
```

One `FileCycleViolation` per cycle, each carrying a `Circuit` — the chain of dependencies that leaves a file,
comes back to it and touches nothing twice on the way, printable as `a.go -> b.go -> a.go`. The predicate is
offered on the positive mood alone: the negation would ask a project to contain a circle somewhere, which is
not a rule anybody means.

## Naming and placement

```go
naming := archunit.ProjectFiles(nil).InFolder("internal/api/**").Should().HaveName("*_handler.go")
placement := archunit.ProjectFiles(nil).WithName("*_test.go").ShouldNot().BeInFolder("internal/db/**")
```

`HaveName` reads the filename, `BeInFolder` the folder and `BeInPath` the whole identifier — the predicate
forms of the three scope verbs, matching against exactly the same parts.

## Your own predicate

`AdhereTo` is the escape hatch: a question about one file, answered yes or no, and the words to report it by.
`Should` requires every selected file to answer yes; `ShouldNot` forbids any of them from doing so.

```go
rule := archunit.ProjectFiles(nil).
	InFolder("internal/**").
	Should().
	AdhereTo(func(file archunit.FileInfo) bool {
		return file.NonBlankLineCount <= 400
	}, "be at most 400 lines long")
```

The function is a `FilePredicate` and the value it is handed is a `FileInfo`, so it can ask about the text of
a file as well as its place:

| Field | Is |
|---|---|
| `Path` | the file's identifier |
| `Name` | its name without the extension |
| `Extension` | that extension |
| `Directory` | its folder |
| `Source` | its whole source text |
| `NonBlankLineCount` | how many of its lines carry something |

The second argument is the requirement in your own words, and it is what the failure message says the file
should have done — so write it as the predicate's own sentence, *be at most 400 lines long*, and the report
reads as one.

## Next

The other four families are the same grammar over a different node: [layers](layers.md) name their sets of
files up front, [slices](slices.md) cut them out of identifiers, [metrics](metrics.md) count rather than
judge, and the [dependency graph](graph.md) draws instead of judging at all.
