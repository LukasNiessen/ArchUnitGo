---
layout: default
title: The slices family
nav_order: 6
description: Slices in ArchUnitGo — components cut out of file identifiers, one rule per pair, and the whole architecture checked against a component diagram.
---

# The slices family

A slice is a name cut out of a file's identifier. Nothing declares the slices of a project: the capture in the
slicing pattern does, so a slicing is a way of talking about a codebase rather than a list of its parts.

```go
rule := archunit.ProjectSlices(nil).
	DefinedBy("internal/(**)/**").
	ShouldNot().
	ContainDependency("api", "db")
```

`internal/(**)/**` says *the slices of this project are its folders under internal*, so
`internal/api/handler.go` is in the slice `api` and a file the pattern does not match is in no slice at all.
That is the difference from [a layer policy](layers.md), where every layer is named before any file is read —
and the reason the two families exist side by side.

`archunit.ProjectSlices` is the entry point. It has no shorter alias, because `slices` alone is a standard
library package and a chain starting with it would read as one.

## The slicing

The scope of this family is **exactly one verb**, not the usual chain of them:

| Verb | Reads its pattern as |
|---|---|
| `DefinedBy` | a glob, whose one capture — `(**)` or `(*)` — names the slice |
| `DefinedByRegex` | Go's own regular expression syntax, whose one capturing group names the slice |

`internal/(**)/**` and `internal/([^/]+)/.*` are the same slicing said twice. Both need exactly one capture,
because a slice with two names is not a name; see [patterns and identifiers](patterns.md) for the rest of the
syntax.

A second slicing is a user error rather than a narrower rule — two slicings would be two vocabularies for the
same project — so a chain that spells one twice reports `ErrSlicedTwice`, and a chain that spells none reports
`ErrNoSlicing`.

## One pair of slices

`ContainDependency` takes the two slices as they come out of the capture, in either mood:

```go
slicing := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**")

noDatabase := slicing.ShouldNot().ContainDependency("api", "db")
viaDomain := slicing.Should().ContainDependency("api", "domain")
```

Both rules are values over one stored slicing, which is what makes a suite of them cheap to write. `Check`
reports `SliceDependencyViolation` values, carrying the two slices and the concrete file dependencies behind
the pair, because *api depends on db* is one fact however many imports made it true.

## The whole architecture at once

Forty pairwise rules nobody keeps up to date are better written as the drawing everybody already has.
`AdhereToDiagram` takes the diagram as text and `AdhereToDiagramInFile` reads it from a file beside the code:

```go
rule := archunit.ProjectSlices(nil).
	DefinedBy("internal/(**)/**").
	Should().
	AdhereToDiagramInFile("docs/architecture.puml")
```

```go
rule := archunit.ProjectSlices(nil).
	DefinedBy("internal/(**)/**").
	Should().
	AdhereToDiagram(`
		@startuml
		' the architecture we agreed on
		component [api]
		component [domain]
		component [db]
		[api] --> [domain]
		[api] --> [db]
		@enduml
	`)
```

The predicate is on the positive mood alone: *should not adhere to the diagram* is not a rule anybody means.

The dialect is the component-diagram subset of PlantUML and no more of it — component declarations, arrows,
comments, and the frame. `component [api]`, `component api` and `[api]` each declare a component; `[api] -->
[db]` and `[api] -> [db]` each draw a dependency, and a `: label` after an arrow is read and dropped. A line
outside that grammar is refused with its number and its text rather than skipped, because a diagram whose
arrows are quietly half-read is worse than no diagram: it becomes rules nobody wrote. A styling directive a
drawing needs for its looks can be commented out with `'` for this library's benefit. A text with no component
in it at all is `ErrEmptyDiagram`.

A project and a drawing can disagree in three ways, and one check reports every one of them it finds:

| `SliceDiagramFinding` | Says |
|---|---|
| `FindingUndrawnDependency` | the project has a dependency between two slices that the diagram does not draw |
| `FindingUndeclaredSlice` | the project has a slice the diagram does not declare |
| `FindingAbsentComponent` | the diagram declares a component the project has no slice for |

They are one violation type, `SliceDiagramViolation`, under one kind: a reader checking a project against a
drawing wants one list of the ways the two do not match. Only the first carries the file dependencies, because
the other two report that something is not there at all.

Two modifiers switch off one finding each, and they are chainable in either order:

| Modifier | Leaves out |
|---|---|
| `IgnoringOrphanSlices` | the slices no dependency reaches, which an architect drawing the architecture rather than the folder tree may reasonably not draw |
| `IgnoringExternalSlices` | the components this project has no slice for, which is what a drawing of a whole system full of sibling modules is made of |

Neither touches the dependencies, and a slice that is an end of an arrow and missing from the drawing is a
hole in it whatever the modifiers say.

## Drawing the project as it is

The reverse of the rule is a terminal on the slicing itself — a drawing states what a project *is*, and no
rule about it has been written yet:

```go
diagram, err := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").ToPlantUML(nil)
```

```go
err := archunit.ProjectSlices(nil).
	DefinedBy("internal/(**)/**").
	ExportAsPlantUML("docs/architecture.puml", nil)
```

`ToPlantUML` hands back the document as a string and `ExportAsPlantUML` writes it to a file, in exactly the
dialect `AdhereToDiagram` reads. So the way to start using this family on a codebase nobody has drawn yet is
to export the diagram, read it, delete the arrows that should not be there, and check the rest in as the
architecture. A slicing that found no slice at all is `ErrNothingToDraw` rather than an empty frame.

## Asking what a slice holds

`SelectSliceFiles` resolves the slicing and hands back the files of each slice by name, which is how a rule
about a slice that does not exist — a typo, a renamed folder — is found in one line:

```go
files, err := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").SelectSliceFiles(nil)
```

## Next

[Metrics](metrics.md) is the family that counts instead of judging what depends on what, and
[dependency-graph reports](graph.md) is the one that draws the project's files rather than its components.
