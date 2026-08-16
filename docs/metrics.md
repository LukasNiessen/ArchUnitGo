---
layout: default
title: The metrics family
nav_order: 7
description: The numbers a Go project adds up to — counts, Martin's package metrics, thresholds, the two zones, a metric of your own, and the HTML report.
---

# The metrics family

This family counts rather than judging what depends on what. A rule says where it looks, which number it is
about, and what that number has to be:

```go
rule := archunit.Metrics(nil).
	InFolder("internal/api/**").
	Count().
	LinesOfCode().
	ShouldBeBelow(400)
```

`archunit.Metrics` is the entry point, and it has no second spelling: `metrics` is what the family calls itself
in every port. A rule of this family ends in `Check` like every other rule in the library.

Left without a predicate, the same chain is a measurement rather than a rule — `Measure` hands back one
`Measurement` per subject, carrying the metric's own name, the subject it was read off and the value:

```go
measurements, err := archunit.Metrics(nil).InFolder("internal/api/**").Count().LinesOfCode().Measure(nil)
```

## Scope

Four scope verbs, chainable and combined with AND. Three of them describe files and `ForClassesMatching`
describes declared types, which is the one thing about this family's scope worth remembering: an exclusion is
about the same population as the verb it qualifies.

| Verb | Selects by | Excluded by |
|---|---|---|
| `WithName` | the filename | `ExceptWithName` |
| `InFolder` | the folder | `ExceptInFolder` |
| `InPath` | the whole identifier | `ExceptInPath` |
| `ForClassesMatching` | the declared type's own name | `ExceptClassesMatching` |

`Except` is the plain form, read against the same part as the selector in front of it.

## Which number

The group a scope is followed by decides what there is to ask for, so a rule says which *kind* of number it
means before it says which number — `count, method count` reads as one phrase.

**`Count`** is the eight numbers this library takes of a project as it is written. Six are about a file and two
about a class:

| Verb | Counts | Per |
|---|---|---|
| `LinesOfCode` | the lines that carry something | file |
| `Statements` | the statements | file |
| `Imports` | the imports | file |
| `Functions` | the functions declared at package level | file |
| `Classes` | the declared types | file |
| `Interfaces` | the declared types that are interfaces | file |
| `MethodCount` | the methods a type has | class |
| `FieldCount` | the fields a type declares | class |

**`Distance`** is Robert C. Martin's package metrics and the coupling factor beside them, each about a
component — a folder of the project, with the types it declares and the packages it depends on:

| Verb | Is |
|---|---|
| `Abstractness` | how much of the component is interfaces |
| `Instability` | how much of its coupling points outward |
| `DistanceFromMainSequence` | how far it sits from the line where the two balance |
| `NormalizedDistance` | the same distance on a nought-to-one scale |
| `CouplingFactor` | how much of the coupling it could have with the other selected components it has |

The eight LCOM formulas are calculated and tested and have no fluent verb yet, which the README's
[what is not implemented yet](https://github.com/LukasNiessen/ArchUnitGo#what-is-not-implemented-yet) states as
the one place it is stated.

## The six thresholds

**There are exactly six threshold predicates, and each spells its own mood**, which is why this family has no
mood stage. Five hold every number a rule measured to a figure:

| Predicate | Holds a measured value to |
|---|---|
| `ShouldBeBelow` | less than the figure |
| `ShouldBeAbove` | more than the figure |
| `ShouldBe` | exactly the figure |
| `ShouldBeBelowOrEqual` | at most the figure |
| `ShouldBeAboveOrEqual` | at least the figure |

There is no seventh. *Should equal*, *should be at most* and every other synonym of one of the five is
deliberately absent, because two spellings of one comparison mean every reader of a suite has to learn which
of them the author picked. A broken threshold is a `MetricsThresholdViolation`, one per subject, carrying the
subject, the number and the comparison it failed.

The sixth is `ShouldSatisfy`, for the comparisons no threshold expresses — a question the user writes, and the
words to report it by:

```go
rule := archunit.Metrics(nil).
	ForClassesMatching("*Service").
	Count().
	MethodCount().
	ShouldSatisfy(func(measurement archunit.Measurement, class archunit.MetricsClassInfo) bool {
		return measurement.Value <= 20 || class.Interface
	}, "have at most 20 methods unless it is an interface")
```

The function is a `MetricsSatisfaction` and it is handed both halves of what the library knows: the
`Measurement`, so a predicate can exempt one subject or read the figure, and the `MetricsClassInfo` the number
was read off — the zero value for a metric that is not about a class, since a file's lines of code and a
package's abstractness have no class to be about. It reports a `MetricsSatisfactionViolation`, and the
requirement you wrote is the sentence it says.

## The two zones

`Abstractness` and `Instability` are the two axes of a plane, and two of its corners are places a package
should not be. Both checks are on the `Distance` group itself, because each is about both axes at once:

```go
rule := archunit.Metrics(nil).InFolder("internal/**").Distance().ShouldNotBeInZoneOfPain()
```

| Check | Forbids the corner where a package is |
|---|---|
| `ShouldNotBeInZoneOfPain` | concrete and depended upon — rigid, and offering no interface to depend on instead |
| `ShouldNotBeInZoneOfUselessness` | abstract and depended on by nothing — an abstraction nobody uses |

Each corner is a quarter-circle rather than the point itself, so a package that is *nearly* all concrete and
*nearly* all depended upon is reported too. Both exist in the negative mood alone: the positive would demand
that every selected package be badly designed. A failure is a `MetricsZoneViolation`, carrying the component,
the zone and the two coordinates that put it there.

## A metric of your own

`CustomMetric` is the family's escape hatch, and the reason it does not have to be exhaustive: a name, the
words saying what the number means, and your own function for reading it off one class.

```go
rule := archunit.Metrics(nil).
	ForClassesMatching("*Service").
	CustomMetric("public surface", "how many methods and fields a type exposes",
		func(class archunit.MetricsClassInfo) float64 {
			return float64(class.MethodCount + class.FieldCount)
		}).
	ShouldBeBelow(20)
```

The function is a `MetricsClassMeasure` and it is handed an `archunit.MetricsClassInfo`: the class's name, its
identifier, the file it was declared in, whether it is an interface, how many fields and methods it has, and —
through `MetricsFieldInfo` and `MetricsMethodInfo` — which of its fields each of its methods reaches.
Everything after the verb is unchanged, because a custom metric is a metric and not a second kind of rule.

## The numbers as a page

Either group also closes without naming a metric at all, and then the chain is a report rather than a rule.
`ExportAsHTML` measures every number of the group over the one scope and writes them as one self-contained
page — the form to reach for when the numbers are for a person rather than for a threshold:

```go
err := archunit.Metrics(nil).InFolder("internal/**").Count().ExportAsHTML("build/metrics.html", nil)
```

For measurements you have already taken and grouped your own way, `archunit.NewMetricsExporter` is the same
page under your own title, timestamp and stylesheet:

```go
exporter := archunit.NewMetricsExporter(&archunit.MetricsReportOptions{Title: "the numbers of this project"})
err := exporter.ExportAsHTML(archunit.MetricsReportData{"lines of code": measurements}, "build/metrics.html")
```

`MetricsReportData` is a map from a heading to the measurements under it, and `MetricsReportOptions` is the
page's own `Title`, `Timestamp` and `Style`. The timestamp is a field rather than a clock the library reads: a
page that stamped itself would render different bytes on every run, so a report committed beside the code would
show up in every diff. A scope that selected nothing to measure is `ErrEmptyReport`.

## Next

[Dependency-graph reports](graph.md) are the other half of *reporting rather than judging*: the project's own
files, drawn.
