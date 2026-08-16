# ArchUnitGo

[![CI](https://github.com/LukasNiessen/ArchUnitGo/actions/workflows/ci.yml/badge.svg)](https://github.com/LukasNiessen/ArchUnitGo/actions/workflows/ci.yml)

Architecture rules as ordinary Go unit tests. Write the sentence your team already says out loud —
*the api does not touch the database* — as a value, and let `go test` tell you where the code
disagrees.

ArchUnitGo is the Go member of **ArchUnitEverything**, one architecture-testing library per language.
Siblings: [ArchUnitTS](https://github.com/LukasNiessen/ArchUnitTS) ·
[ArchUnitPython](https://github.com/LukasNiessen/ArchUnitPython).

**Documentation site:** <https://lukasniessen.github.io/ArchUnitGo/> — the same material as this file with
a page per family, built from [docs/](docs). The API reference is
[pkg.go.dev](https://pkg.go.dev/github.com/LukasNiessen/ArchUnitGo), generated from the doc comments in the
source. This file stays the short version, and it is the one place that states
[what is not implemented yet](#what-is-not-implemented-yet).

- [Install](#install)
- [Your first rule](#your-first-rule)
- [What a failure looks like](#what-a-failure-looks-like)
- [The grammar](#the-grammar)
- [Patterns and identifiers](#patterns-and-identifiers)
- [One example per family](#one-example-per-family)
  - [Files](#files) · [Layers](#layers) · [Slices](#slices) · [Metrics](#metrics) · [Dependency graph](#dependency-graph)
- [A whole suite](#a-whole-suite)
- [Without a test framework](#without-a-test-framework)
- [Check options](#check-options)
- [Keeping one import out of the graph](#keeping-one-import-out-of-the-graph)
- [When a rule selects nothing](#when-a-rule-selects-nothing)
- [What is not implemented yet](#what-is-not-implemented-yet)
- [The map](#the-map)

## Install

```sh
go get github.com/LukasNiessen/ArchUnitGo
```

Go 1.26 or newer. The only direct dependency is `golang.org/x/tools`, which is how the extractor talks
to the Go toolchain. There is no tagged release yet, so `go get` resolves to a pseudo-version of
`main` and the API may still move.

The package is `archunit` while the last element of the module path is `ArchUnitGo`, so give the
import the name it has:

```go
import archunit "github.com/LukasNiessen/ArchUnitGo"
```

## Your first rule

```go
package architecture_test

import (
	"testing"

	archunit "github.com/LukasNiessen/ArchUnitGo"
)

func TestTheApiDoesNotTouchTheDatabase(t *testing.T) {
	rule := archunit.ProjectFiles(nil).
		InFolder("internal/api/**").
		ShouldNot().
		DependOnFiles().
		InFolder("internal/db/**")

	archunit.AssertPasses(t, rule, nil)
}
```

That is the whole setup. There is nothing to register, nothing to configure and no fixture to build:

- **A rule is a value.** Building one does no work; only the terminal — `AssertPasses` here, or
  `Check` — reads the project. Half-built rules can be stored in a variable and branched from.
- **`nil` means the defaults, everywhere.** The `*ProjectLocator` an entry point takes is `nil` for
  *the project this test is in*, found by walking up to the nearest `go.mod`. The `*AssertOptions` and
  `*CheckOptions` a terminal takes are `nil` for the defaults.
- **A failing rule is not an error.** `Check` returns `([]Violation, error)`; the violations are the
  rule's result and the error is the library or the environment failing. `AssertPasses` turns the
  first into `t.Error` and the second into a message saying the check could not be run at all.

## What a failure looks like

One `t.Error`, carrying the rule as it was written and then the violations, numbered from one:

```
project files, path without filename matches "internal/api/**", should not, depend on files, path without filename matches "internal/db/**"
1 violation:
  1. internal/api/handler.go: should not, depend on files, path without filename matches "internal/db/**"; it depends on internal/db/conn.go
```

The rule's own sentence is the first line, so a test that asserts several rules says which one broke
before it says what broke it. A selector renders as the part of an identifier it matches rather than as the
verb that spelled it — `InFolder` reads back as *path without filename matches* — because that is what a
reader has to compare their glob against. Violations carry data rather than prose — the offending file, the
requirement as a compiled pattern, what was found instead — and all phrasing happens in one place, so
`AssertOptions.Message` is where colour (`archunit.DefaultPalette()`) and a violation limit live.

## The grammar

Every rule is an English sentence, read left to right, and every family spells the same stages:

| Stage | How many | Example |
|---|---|---|
| **Entry** | exactly 1 | `ProjectFiles(nil)`, `Layers(nil)`, `ProjectSlices(nil)`, `Metrics(nil)`, `ProjectGraph(nil)` |
| **Scope** | 0..n, chainable, AND | `InFolder("internal/api/**")`, `WithName("*_handler.go")` |
| **Exclusion** | 0..n, after any selector | `Except("**/generated")`, `ExceptWithName("*_gen.go")` |
| **Mood** | exactly 1 | `Should()`, `ShouldNot()` |
| **Predicate** | exactly 1 | `HaveNoCycles()`, `DependOnFiles()`, `AdhereTo(f, "…")` |
| **Object** | 1..n, chainable, AND | `InFolder("internal/db/**")` after `DependOnFiles()` |
| **Terminal** | exactly 1 | `Check(nil)`, `Measure(nil)`, `Snapshot()`, `ExportAsHTML(path)` |

The vocabulary is fixed, and deliberately small:

- **Mood is exactly two words**, `Should` and `ShouldNot`. No synonyms, ever. The two builders are one
  assertion with a flag, so every predicate that has a meaningful negation is on both.
- **Predicates are bare infinitives**, so that mood plus predicate reads as English: `HaveNoCycles`,
  `HaveName`, `BeInFolder`, `BeInPath`, `AdhereTo`, `DependOnFiles`, `DependOnExternalModules`,
  `ContainDependency`, `AdhereToDiagram`, `MayOnlyDependOnLayers`, `MayNotDependOnLayers`.
- **Threshold predicates are exactly six**, metrics only, and each spells its own mood: `ShouldBeBelow`,
  `ShouldBeAbove`, `ShouldBe`, `ShouldBeBelowOrEqual`, `ShouldBeAboveOrEqual`, `ShouldSatisfy`.
- **Modifiers are present participles**, chainable, order-independent and always optional:
  `IncludingExternalDependencies`, `FocusOn`, `ReachableFrom`, `DependentsOf`, `CollapseToFolderDepth`,
  `CollapseByPattern`, `Titled`, `WithCheckOptions`, `IgnoringOrphanSlices`, `IgnoringExternalSlices`.
- **`Except` qualifies the selector in front of it** and nothing else, it is repeatable, and its
  patterns are read against the same part of an identifier as that selector unless a targeted form —
  `ExceptWithName`, `ExceptInFolder`, `ExceptInPath`, `ExceptClassesMatching` — names a target of its
  own. That is what keeps *everything under `app/`, but not the generated folder* one clause instead
  of an inverted rule about the generated folder.
- **Builders are immutable**, so a scope is worth typing once:

  ```go
  api := archunit.ProjectFiles(nil).InFolder("internal/api/**")
  noDb := api.ShouldNot().DependOnFiles().InFolder("internal/db/**")
  small := api.Should().AdhereTo(under400Lines, "be at most 400 lines long")
  ```

## Patterns and identifiers

A file is identified by its path relative to the project root, always with forward slashes:
`internal/api/handler.go`. A declared type — `classes`, in the family's vocabulary — is
`internal/api.Handler`.

Patterns are globs, and globs are sugar for regex: every one of them compiles to an anchored regular
expression in one place, so nothing downstream ever sees a glob.

| Glob | Means |
|---|---|
| `*` | any run of characters inside one segment, never crossing `/` |
| `**` | any run of characters, crossing `/`; `a/**` matches `a` itself as well as `a/b/c` |
| `?` | exactly one character, never `/` |
| `[a-z]`, `[!abc]` | one character from a class, or not from it |
| `(**)`, `(*)` | in a slicing only: the part of the identifier to cut a slice name out of |

Everything else is literal, matching is case-sensitive, and there is no escape character — separators
are normalised, so `internal\api\**` and `internal/api/**` are the same glob on every platform.

Which part of the identifier a pattern is matched against is the selector's business, and the
violation message names it:

| Verb | Matches against | `internal/api/handler.go` |
|---|---|---|
| `WithName` | the filename | `handler.go` |
| `InFolder` | everything but the filename | `internal/api` |
| `InPath` | the whole identifier | `internal/api/handler.go` |
| `InFile` | the whole identifier, as a literal | — |
| `ForClassesMatching` | the declared name alone | `Handler` |

`DefinedByRegex` and `SliceByRegex` take Go's own `regexp` syntax instead, for the patterns a glob
cannot spell.

## One example per family

### Files

The file-level family: what depends on what, how files are named and where they live.

```go
// A boundary, with one documented door in it.
rule := archunit.ProjectFiles(nil).
	InFolder("internal/api/**").
	Except("**/generated").
	ShouldNot().
	DependOnFiles().
	InFolder("internal/db/**").
	ExceptInFolder("internal/db/dto/**")

// No file depends on another in a circle. `HaveNoCycles` is offered on the positive mood alone.
cycles := archunit.ProjectFiles(nil).Should().HaveNoCycles()

// Naming and placement.
naming := archunit.ProjectFiles(nil).InFolder("internal/api/**").Should().HaveName("*_handler.go")

// Third-party policy: the object verb `Matching` is repeatable and combined with OR, which is the one
// chain in this library that widens rather than narrows.
external := archunit.ProjectFiles(nil).
	InFolder("internal/domain/**").
	ShouldNot().
	DependOnExternalModules().
	Matching("*.*/**")

// The escape hatch: your own predicate over one file, and the words to report it by.
custom := archunit.ProjectFiles(nil).
	InFolder("internal/**").
	Should().
	AdhereTo(func(file archunit.FileInfo) bool {
		return file.NonBlankLineCount <= 400
	}, "be at most 400 lines long")
```

`FileInfo` carries `Path`, `Name`, `Extension`, `Directory`, `Source` and `NonBlankLineCount`, so a
predicate can ask about the text of a file as well as its place.

### Layers

A named-layer policy: declare who exists, then say what each layer may depend on. The whole policy is
one rule.

```go
rule := archunit.ProjectLayers(nil).
	Layer("api").DefinedByFolder("internal/api/**").
	Layer("domain").DefinedByFolder("internal/domain/**").
	Layer("db").DefinedByFolder("internal/db/**").
	WhereLayer("api").MayOnlyDependOnLayers("domain", "db").
	WhereLayer("domain").MayOnlyDependOnLayers()
```

There is no mood stage here: `MayOnlyDependOnLayers` is the allowlist and `MayNotDependOnLayers` the
blocklist, so a mood before them would read as *should not, may not*. Dependencies inside a layer are
always allowed, a dependency with an end in no declared layer is ignored, and
`MayOnlyDependOnLayers()` with nothing named is the sealed layer. Every layer has to be declared
before the first clause. A violation is one per pair of layers rather than one per import, carrying
the concrete file dependencies that connect them.

### Slices

A slice is a name cut out of an identifier by the capture in the slicing pattern. Nothing is declared:
`internal/(**)/**` says that this project's slices are its folders under `internal`, so
`internal/api/handler.go` is in the slice `api` and a file the pattern does not match is in no slice
at all.

```go
rule := archunit.ProjectSlices(nil).
	DefinedBy("internal/(**)/**").
	ShouldNot().
	ContainDependency("api", "db")
```

The other rule a slicing can be asked for is the whole architecture at once, against the PlantUML
component diagram somebody drew of it:

```go
rule := archunit.ProjectSlices(nil).
	DefinedBy("internal/(**)/**").
	Should().
	AdhereToDiagramInFile("docs/architecture.puml").
	IgnoringOrphanSlices()
```

...and the reverse, which draws the diagram out of the project as it is:

```go
err := archunit.ProjectSlices(nil).
	DefinedBy("internal/(**)/**").
	ExportAsPlantUML("docs/architecture.puml", nil)
```

`ProjectSlices` has no shorter alias, unlike `Files` and `Layers`: `slices` is the name of a standard
library package, and a chain starting with it would read as one.

### Metrics

The numbers a project's code adds up to, and what they have to be.

```go
// Eight counting metrics: LinesOfCode, Statements, Imports, Functions, Classes, Interfaces,
// MethodCount, FieldCount.
rule := archunit.Metrics(nil).
	InFolder("internal/**").
	Count().
	LinesOfCode().
	ShouldBeBelow(400)

// Five distance metrics about a package: Abstractness, Instability, DistanceFromMainSequence,
// NormalizedDistance, CouplingFactor — plus the two zone checks, ShouldNotBeInZoneOfPain and
// ShouldNotBeInZoneOfUselessness, where the corner and the mood are one verb.
zone := archunit.Metrics(nil).Distance().ShouldNotBeInZoneOfPain()
useless := archunit.Metrics(nil).InFolder("internal/**").Distance().ShouldNotBeInZoneOfUselessness()

// The numbers themselves, without a threshold: one Measurement per file, class or folder.
measurements, err := archunit.Metrics(nil).InFolder("internal/api/**").Count().LinesOfCode().Measure(nil)

// A metric of your own, judged by a comparison of your own.
surface := archunit.Metrics(nil).
	ForClassesMatching("*Service").
	CustomMetric("public surface", "how many methods and fields a type exposes",
		func(class archunit.MetricsClassInfo) float64 {
			return float64(class.MethodCount + class.FieldCount)
		}).
	ShouldSatisfy(func(measurement archunit.Measurement, class archunit.MetricsClassInfo) bool {
		return measurement.Value <= 20 || class.Interface
	}, "expose at most 20 methods and fields unless it is an interface")

// Either group also closes without naming a metric, and then the chain is a report rather than a rule:
// every number of the group over the one scope, as one self-contained HTML page.
err = archunit.Metrics(nil).InFolder("internal/**").Count().ExportAsHTML("build/metrics.html", nil)
```

`Metrics` is the family's only spelling of its entry point. Its four scope verbs — `WithName`,
`InFolder`, `InPath`, `ForClassesMatching` — are chainable and combined with AND, three describing
files and the last describing declared types; an exclusion is about the same population as the verb it
qualifies. `NewMetricsExporter` writes the same page from measurements you have already taken and
grouped your own way.

### Dependency graph

Not every chain is a rule. `ProjectGraph` describes a report, so it has no mood, no predicate and no
violations — its terminals hand back the diagram.

```go
snapshot, err := archunit.ProjectGraph(nil).
	FocusOn("internal/api/**", 1).
	CollapseToFolderDepth(2).
	Titled("what the api layer touches").
	Snapshot()

fmt.Println(snapshot.Summary()) // 12 nodes, 18 edges, 143 dependencies

err = archunit.DependencyGraph(nil).
	CollapseToFolderDepth(2).
	Titled("the modules of this project").
	ExportAsHTML("build/architecture.html")
```

Nine modifiers, all optional and order-independent: `IncludingExternalDependencies`,
`IncludingSelfDependencies`, `FocusOn`, `ReachableFrom`, `DependentsOf`, `CollapseToFolderDepth`,
`CollapseByPattern`, `Titled`, `WithCheckOptions`. The default report is one node per file of the
project's own code, and each modifier either narrows what the diagram draws, widens it —
`IncludingExternalDependencies` adds the code outside the project and `IncludingSelfDependencies` adds a
node's dependency on itself — or says how it is labeled. A query that describes nothing is refused rather
than drawn: every terminal here fails with `ErrEmptySnapshot` unless `AllowEmptyTests` says an empty report
was meant.

Thirteen terminals: `Snapshot()` for the report as data, `ToDot`, `ToMermaid`, `ToD2`, `ToCSV`,
`ToJSON` and `ToHTML` for it as a string, and `ExportAsDot`, `ExportAsMermaid`, `ExportAsD2`,
`ExportAsCSV`, `ExportAsJSON` and `ExportAsHTML` for it as a file. A snapshot renders the same bytes
for the same project every time, so a diagram committed beside the code is reviewable in a pull
request.

## A whole suite

More than one rule is a map and one call, each rule in its own named subtest:

```go
func TestTheArchitectureHolds(t *testing.T) {
	archunit.AssertAllPass(t, map[string]archunit.Checkable{
		"the api does not touch the database": archunit.ProjectFiles(nil).
			InFolder("internal/api/**").
			ShouldNot().
			DependOnFiles().
			InFolder("internal/db/**"),
		"no file depends on another in a circle": archunit.ProjectFiles(nil).Should().HaveNoCycles(),
	}, nil)
}
```

A rule that does not hold fails the subtest its author named it after, and the rules around it are
asserted all the same. The subtests run in the sorted order of their names, so
`go test -run 'TestTheArchitectureHolds/no_file_depends_on_another_in_a_circle'` selects one. A suite
with no rules in it is a failure rather than a pass, for the same reason a rule that selected no file
is.

`archunit.Checkable` is the seam the whole library hangs from — every terminal implements it, and
`Check(*CheckOptions) ([]Violation, error)` is all of it — so a helper that loops over a list of rules
never has to know which family they came from.

## Without a test framework

`Check` is the universal terminal, and the report layer is separate from it:

```go
violations, err := rule.Check(nil)
if err != nil {
	return err
}
if result := archunit.NewResultFactory(nil).Result(violations); !result.Passed {
	fmt.Println(result.Message)
}
```

`Violation`'s whole contract is `Kind()`, so a caller can group and count without asserting on a
concrete type — `archunit.KindFileDependency`, `archunit.KindEmptyTest` and the ten others — or type
switch on `archunit.FileDependencyViolation` and its siblings for the data itself.
`NewViolationFactory` phrases one violation at a time, for a report of your own shape.

## Check options

One options bag, and `nil` is always the defaults. Every default is a zero value, so a check is quiet,
strict about empty selections, free to reuse a cached graph, and looks at the production code under
the host platform's build constraints.

```go
violations, err := rule.Check(&archunit.CheckOptions{
	AllowEmptyTests:  false,             // a rule that selected nothing is a violation; see below
	IncludeTestFiles: true,              // hold _test.go files to the same rules
	BuildTags:        []string{"linux"}, // the constraints to analyze under
	IgnoreScopes:     []string{"layers"},
	ClearCache:       true,
	Logging:          &archunit.LogOptions{Writer: os.Stderr, Level: archunit.LogLevelDebug},
})
```

`AssertOptions` is the two bags each half already has — `Check` for how the rule is run, `Message` for
how a failure is written — so `archunit.AssertPasses(t, rule, &archunit.AssertOptions{...})` needs no
vocabulary of its own.

Extraction is the expensive half of a check and every rule in a suite asks about the same project, so
the graph is memoised per process. `ClearCache`, or `archunit.ClearGraphCache()`, is for source that
changed underneath the library — a test that writes a fixture project, generated code produced between
two checks.

Logging is off by default and there is no way to turn it on globally: the destination is injected per
check, so one test can assert on a log while the rest of the suite runs beside it. `LogOptions` also
holds the file a CI job archives, and the four levels are `LogLevelDebug`, `LogLevelInfo` (the
default), `LogLevelWarn` and `LogLevelError`. A technical failure is still the error `Check` returns;
a log line is never how this library reports something.

## Keeping one import out of the graph

Some imports are not dependencies. The directive is written the way Go writes a machine-readable
comment, so `gofmt` leaves it where you put it:

```go
import (
	"database/sql"

	_ "github.com/lib/pq" //archunit:ignore
)
```

A bare directive is honoured by every rule. A scoped one — `//archunit:ignore layers` — is honoured
only by a check that names that scope in `CheckOptions.IgnoreScopes`, and counts as an ordinary
dependency everywhere else. The directive belongs to the one import it is written on, either trailing
it or on a comment-only line directly above it.

## When a rule selects nothing

**Zero matches is a violation, not a pass.** A selector matching no file is almost always a stale glob or a
renamed folder, and such a rule is green forever — so the guard is wired into every terminal, in whichever of
its two shapes that terminal has room for:

- **A terminal that returns violations reports an `EmptyTestViolation`**, which is `Check` in every family.
- **A report terminal fails with an error**, because a report has no violation list to put a violation in:
  `ErrEmptySnapshot` for the graph family's `Snapshot` and every `To*` and `ExportAs*` built on it,
  `ErrEmptyReport` for the metrics family's `ExportAsHTML`, and `ErrNothingToDraw` for the slices family's
  `ToPlantUML` and `ExportAsPlantUML`.

`CheckOptions.AllowEmptyTests` opts out of both. The one terminal that reports emptiness neither way is
`Measure`, deliberately: it judges nothing, so whether an empty selection is a failure is not a question it
can ask — its result is the numbers themselves, and no subject is no numbers.

To see what a selector actually resolves to, ask the scope stage rather than the rule:

```go
files, err := archunit.ProjectFiles(nil).InFolder("internal/api/**").SelectFiles(nil)
membership, err := archunit.ProjectLayers(nil).Layer("api").DefinedByFolder("internal/api/**").SelectLayerFiles(nil)
slices, err := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").SelectSliceFiles(nil)
```

Every stage of a chain that can describe itself is a `fmt.Stringer`, and prints the sentence it has become
so far — including the first thing it rejected, so a pattern that will not compile says so when the chain is
printed as well as when it is checked. The one stage that cannot is `LayerPolicyBuilder`, the clause caught
between `WhereLayer` and its predicate: it has named a layer and not yet said anything about it, so there is
no sentence of its own for it to print.

## What is not implemented yet

Documented from the source, so this list is what is actually missing today:

- **No tagged release.** `go get` resolves to a pseudo-version of `main`, and names may still change.
- **Files are the node vocabulary.** There are no package-level selectors, and no rule about a
  declared type's own dependencies. The metrics family reaches beyond files — its distance metrics are
  about a folder and `ForClassesMatching` selects declared types — but a rule of the files family is
  always about files.
- **The LCOM family has no fluent verb.** The eight measures live in `metrics/calculation` as plain
  functions — `LCOM96a`, `LCOM96b`, `LCOM1`…`LCOM5`, `LCOMStar` — and are not on the public surface.
  Until a `cohesion` group lands, the way to hold code to one is `CustomMetric` with your own function.
- **The slices family has no `Except`.** The other four have it: four spellings in the files family,
  five in metrics, three in layers, and the plain verb in the graph module.
- **Two of the exported slicings have no `DefinedBy` verb.** `SliceByFileSuffix` and `Identity` are
  `MapFunction` values a caller can hold, but the fluent slicing verbs are `DefinedBy` and
  `DefinedByRegex` alone.
- **The import kinds are not on the public surface.** `CheckOptions.IgnoredImportKinds` takes an
  `extraction.ImportKindSet`, so dropping blank imports before extraction means importing
  `common/extraction` directly.
- **Diagrams are read in one format and written in seven.** `AdhereToDiagram` parses PlantUML
  component diagrams; the graph module renders DOT, Mermaid, D2, CSV, JSON and HTML, and the slices
  module renders PlantUML, but nothing reads any of the other six back.
- **Everything is synchronous** and analysis is single-project: a rule is about one module, found by
  walking up to the nearest `go.mod`.

## The map

The whole product is the fluent API; everything else is its implementation. The pipeline is
`SOURCE -> EXTRACT -> PROJECT -> ASSERT -> REPORT`, and the SOURCE-and-EXTRACT stage is the only part that
knows Go — `common/extraction` for the dependency graph, and `metrics/extraction` for what is inside one
file. Nothing downstream of either knows what an import or a statement is.

```
common/         the kernel — extraction, projection, assertion, matching, logging
files/          file-level dependency and naming rules
layers/         named-layer policy
slices/         component and diagram rules
metrics/        numeric code-quality rules
graph/          dependency-graph reports
archtest/       violation formatting and test-framework glue
archunit.go     the public surface — re-exports, nothing else
```

Every domain module has the same internal shape — `fluentapi/` is the only public part, with
`assertion/`, `projection/`, `extraction/` and `calculation/` behind it — so reading one module
teaches you how to navigate all of them.

[AGENTS.md](AGENTS.md) is the authority on the architecture, the naming and the shape of the API, for
this library and for every other ArchUnit port. It is what to read before changing anything here.

This repository enforces its own architecture on itself:
[architecture_test.go](architecture_test.go) is the four dependency rules of `AGENTS.md` written as a
suite through this public surface, and it is the library's best worked example.
