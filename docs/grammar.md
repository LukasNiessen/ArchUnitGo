---
layout: default
title: The grammar
nav_order: 2
description: The stages every ArchUnitGo rule is built from — entry, scope, mood, predicate, object, terminal — and the fixed vocabulary each of them is spelled with.
---

# The grammar

Every rule is an English sentence, read left to right:

```
ENTRY          SCOPE          MOOD         PREDICATE      OBJECT          TERMINAL
project files  in folder      should not   depend on      in folder       check
               "**/api/**"                 files          "**/db/**"
```

> *project files, in folder `**/api/**`, should not depend on files in folder `**/db/**`.*

Learn the stages once and you can read a rule from any of the five families, because they all spell the
same ones. The vocabulary is deliberately small and it has no synonyms: one comparison, one spelling.

## The stages

| Stage | How many | Spelled |
|---|---|---|
| **Entry** | exactly 1 | `ProjectFiles`, `ProjectLayers`, `ProjectSlices`, `Metrics`, `ProjectGraph` |
| **Scope** | 0..n, chainable, AND | `WithName`, `InFolder`, `InPath`, `InFile`, `ForClassesMatching` |
| **Exclusion** | 0..n, after any selector that has one | `Except`, `ExceptWithName`, `ExceptInFolder`, `ExceptInPath`, `ExceptClassesMatching` |
| **Mood** | exactly 1 | `Should`, `ShouldNot` |
| **Predicate** | exactly 1 | `HaveNoCycles`, `HaveName`, `BeInFolder`, `BeInPath`, `AdhereTo`, `DependOnFiles`, `DependOnExternalModules`, `ContainDependency`, `AdhereToDiagram`, `MayOnlyDependOnLayers`, `MayNotDependOnLayers` |
| **Object** | 1..n, chainable | `InFolder` after `DependOnFiles`, `Matching` after `DependOnExternalModules` |
| **Modifier** | 0..n, order-independent | `IgnoringOrphanSlices`, `FocusOn`, `CollapseToFolderDepth`, `Titled`, `WithCheckOptions` |
| **Terminal** | exactly 1 | `Check`, `Measure`, `Snapshot`, `ToDot`, `ExportAsHTML` |

Each family fills the stages in with its own vocabulary, and its own page says which:
[files](files.md), [layers](layers.md), [slices](slices.md), [metrics](metrics.md),
[dependency graph](graph.md).

## Entry points

An entry point is a noun phrase, and it takes an optional `*ProjectLocator` — `nil` for the project the
test itself is in, found by walking up to the nearest `go.mod`. It is never a required argument.

| Entry point | Family | Also spelled |
|---|---|---|
| `archunit.ProjectFiles` | files | `archunit.Files` |
| `archunit.ProjectLayers` | layers | `archunit.Layers` |
| `archunit.ProjectSlices` | slices | — |
| `archunit.Metrics` | metrics | — |
| `archunit.ProjectGraph` | dependency-graph reports | `archunit.DependencyGraph` |

The two families without a second spelling have a reason for it. `slices` alone is the name of a standard
library package, so a chain starting with it would read as one; `metrics` is what the metrics family calls
its entry point in every port.

## Mood

**Mood is exactly two words**, `Should` and `ShouldNot`. There are no synonyms, ever — not *must*, not *may*,
not *is not*.

The two are one assertion with a flag rather than two code paths, which is why every predicate that has a
meaningful negation is on both moods, and why a predicate whose negation would report nothing is on one:
`HaveNoCycles` is offered on `Should` alone, because *should have cycles* is not a rule anybody means.

Two families spell the mood inside the predicate instead of before it, and neither has a mood stage:

- The layers family: `MayOnlyDependOnLayers` is the allowlist and `MayNotDependOnLayers` the blocklist, so
  a mood in front of them would read as *should not, may not depend on layers*.
- The metrics family: each of the six threshold predicates carries its own — `ShouldBeBelow`,
  `ShouldBeAbove`, `ShouldBe`, `ShouldBeBelowOrEqual`, `ShouldBeAboveOrEqual`, `ShouldSatisfy` — as do the
  two zone checks, `ShouldNotBeInZoneOfPain` and `ShouldNotBeInZoneOfUselessness`.

The [dependency-graph family](graph.md) has no mood because it has no predicate: it describes a report
rather than a rule.

## Exclusions

`Except` qualifies the selector in front of it and nothing else. It is repeatable, and its patterns are read
against the same part of an identifier as the selector it follows — unless a targeted form names a part of
its own:

```go
rule := archunit.ProjectFiles(nil).
	InFolder("app/**").
	Except("**/generated").
	ExceptWithName("*_gen.go").
	ShouldNot().
	DependOnFiles().
	InFolder("internal/db/**").
	ExceptInFolder("internal/db/dto/**")
```

That is what keeps *everything under `app/`, but not the generated folder* one clause, instead of an
inverted rule about the generated folder. `Except` with no selector in front of it is a user error rather
than a rule about everything, and each family offers exactly the targeted forms its own selectors name —
four in the files family, five in metrics, three in layers, and the plain verb in the graph module, where
every pattern is matched against the whole identifier already. The slices family is the one without it: it
has no exclusion at all, so a slicing is narrowed by the pattern that defines it and nothing else.

## Terminals

`Check(*CheckOptions) ([]Violation, error)` is the universal terminal: every rule in every family ends in
it, and it is the whole of `archunit.Checkable`, the seam the library hangs from. A helper that loops over a
list of rules never has to know which family they came from.

```go
violations, err := rule.Check(nil)
```

The other terminals hand back something other than a judgement, and a chain that ends in one is a report
rather than a rule: `Measure` for the numbers a metric read, `Snapshot` for a graph report as data, `ToDot`
and its siblings for it as a string, `ExportAsHTML` and its siblings for it as a file. The scope stage has a
terminal of its own too — `SelectFiles`, `SelectLayerFiles`, `SelectSliceFiles` — which answers *what does
this selector actually match?* without judging anything.

## Builders are values

Every stage is immutable and returns a new instance, so a scope is worth typing once and a half-built rule
is worth storing:

```go
api := archunit.ProjectFiles(nil).InFolder("internal/api/**")

noDatabase := api.ShouldNot().DependOnFiles().InFolder("internal/db/**")
small := api.Should().AdhereTo(func(file archunit.FileInfo) bool {
	return file.NonBlankLineCount <= 400
}, "be at most 400 lines long")
```

Nothing was read from disk by any line of that. Both rules are values, and either can be stored in a struct
field, returned from a helper or put in a map of the suite's rules — see [running a rule](running.md).

## Asking a chain what it says

Three methods let a chain describe itself, and this page is where they are documented once, which is why the
family pages do not repeat them. Each is on the stages that have something to answer with:

| Method | On | Hands back |
|---|---|---|
| `String` | every stage but `LayerPolicyBuilder` | the sentence the chain has become so far |
| `Mood` | the mood stage of the two families that have one, files and slices | `archunit.Should` or `archunit.ShouldNot` |
| `Selectors` | the scope stage of the two families whose scope compiles filters, files and metrics | the compiled patterns the scope selects with |

`String` is what a failure message quotes the rule as, and it includes the first thing the chain rejected —
so a pattern that will not compile says so when the chain is printed as well as when it is checked.
`LayerPolicyBuilder` is the one stage without it, the clause caught between `WhereLayer` and its predicate:
it has named a layer and not yet said anything about it, so it has no sentence of its own.

The other two are on the stages that exist to answer them. Only the files and slices families have a mood
stage — the layers and metrics families spell the mood inside the predicate and the graph family has none —
and of the scope stages only the files and metrics ones hand their compiled filters back, so a layers,
slices or graph chain has no `Selectors`.

## Adding a verb

The acceptance test for a new verb is not the implementation, it is the name: **read the whole chain aloud,
and if it is not a sentence an architect who does not write Go would understand, the name is wrong.**
[AGENTS.md](https://github.com/LukasNiessen/ArchUnitGo/blob/main/AGENTS.md) is where that rule and the rest
of the conventions live, and [how it works](internals.md) is the short version of where the code goes.
