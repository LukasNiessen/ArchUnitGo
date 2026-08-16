---
layout: default
title: Running a rule
nav_order: 9
description: How a rule becomes a test — the assert helpers, check options, violations, colors, logging, the ignore directive and the empty-selection guard.
---

# Running a rule

A rule is a value and nothing happens until a terminal runs it. `Check` is the terminal every rule in every
family ends in, and everything on this page is about what happens around it.

## As a test

```go
func TestTheApiDoesNotTouchTheDatabase(t *testing.T) {
	rule := archunit.ProjectFiles(nil).
		InFolder("internal/api/**").
		ShouldNot().
		DependOnFiles().
		InFolder("internal/db/**")

	archunit.AssertPasses(t, rule, nil)
}
```

A rule that holds reports nothing. One that does not is a single `t.Error` carrying the rule as it was written
and then the violations, numbered from one.

`AssertPasses` needs `Error` and nothing else, which is what `TestingT` says: `*testing.T` satisfies it, and so
does any other framework's handle. There is no registration and no configuration anywhere in this library.

More than one rule is a map and one call:

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

Each rule is asserted in its own subtest named after the sentence you wrote, so a rule that does not hold fails
that subtest and the rules around it are asserted all the same. The subtests run in the sorted order of their
names, which makes the output the same on every run and lets `go test -run` select one of them. A suite with no
rules in it is a failure rather than a pass, for the same reason a rule that selected no file is.

`AssertAllPass` takes `TestingRunner` — `Error`, `Helper` and `Run` — so only `*testing.T` satisfies it. A
framework whose handle has no subtests still has `AssertPasses`.

## Without a test framework

The report layer is separate from the check, so nothing about this library needs `testing`:

```go
violations, err := rule.Check(nil)
if err != nil {
	return err
}
if result := archunit.NewResultFactory(nil).Result(violations); !result.Passed {
	fmt.Println(result.Message)
}
```

`Result` is the whole of what an adapter reads: `Passed`, and the `Message` to print. It never ends in a
newline, because the caller's own `t.Error`, log line or diff decides how the last line ends.
`archunit.NewViolationFactory` phrases one violation at a time, for a report of your own shape.

**A failing rule is not an error.** `Check` returns `([]Violation, error)`: the violations are the rule's
result, and the error is the library or the environment failing — a pattern that will not compile, a project
that will not load, a file that cannot be written. A rule that holds is an empty violation list and a `nil`
error.

## What a violation is

A `Violation` carries the thing that disagreed rather than a sentence about it. Its whole contract is `Kind`,
which returns a `ViolationKind` spelled the same way in every ArchUnit port, so a caller can group and count
without asserting on a concrete type:

```go
counted := map[archunit.ViolationKind]int{}
for _, violation := range violations {
	counted[violation.Kind()]++
}
```

The kinds are `archunit.KindEmptyTest`, `archunit.KindFileCycle`, `archunit.KindFileNaming`,
`archunit.KindFileDependency`, `archunit.KindFileExternalDependency`, `archunit.KindFileAdherence`,
`archunit.KindLayerDependency`, `archunit.KindMetricsZone`, `archunit.KindMetricsThreshold`,
`archunit.KindMetricsSatisfaction`, `archunit.KindSliceDependency` and `archunit.KindSliceDiagram`. For the
data itself, type switch on the concrete type — `archunit.FileDependencyViolation` and its siblings, each
listed on its own family's page.

## Check options

One options bag, and `nil` is always the defaults. Every default is a zero value, so a check is quiet, strict
about empty selections, free to reuse a cached graph, and looks at the production code under the host
platform's build constraints.

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

`AssertOptions` is the two bags each half already has — `Check` for how the rule is run and `Message` for how a
failure is written — so the assert helpers need no vocabulary of their own:

```go
archunit.AssertPasses(t, rule, &archunit.AssertOptions{
	Check:   archunit.CheckOptions{IncludeTestFiles: true},
	Message: archunit.MessageOptions{Palette: archunit.DefaultPalette(), MaxViolations: 10},
})
```

`MessageOptions` is the report's own two knobs. `Palette` is which color each part of a message is painted in,
and the zero palette paints nothing — plain text is the default because an escape sequence in a CI log is
noise, and the caller who wants color is the one who knows whether a terminal is there.
`archunit.DefaultPalette` is the palette to ask for; `Color` is a closed set — `archunit.ColorRed`,
`archunit.ColorGreen`, `archunit.ColorYellow`, `archunit.ColorBlue`, `archunit.ColorMagenta`,
`archunit.ColorCyan`, `archunit.ColorGray` and `archunit.ColorNone` — so nothing but the library ever writes an
escape sequence. `MaxViolations` is how many violations a report lists before it says how many it left out, and
the cut is never silent: a truncated list that looks complete is worse than a long one.

## The cache

Extraction is the expensive half of a check and every rule in a suite asks about the same project, so the
dependency graph is memoised per process. `ClearCache` on the options bag, or `archunit.ClearGraphCache`, is
for source that changed underneath the library — a test that writes a fixture project, generated code produced
between two checks.

## Logging

Logging is off by default and there is no way to turn it on globally: the destination is injected per check, a
`nil` `*LogOptions` logs nothing, and a bag with neither a writer nor a file in it logs nothing either. That is
what lets one test assert on a log while the rest of the suite runs beside it. `LogOptions` also holds the file
a CI job archives, and the four levels are `LogLevelDebug`, `LogLevelInfo` — the default — `LogLevelWarn` and
`LogLevelError`. A technical failure is still the error `Check` returns; a log line is never how this library
reports something.

`Logger` is the open log itself, for a caller assembling something of its own. Every rule this library offers
logs through one already.

## Keeping one import out of the graph

Some imports are not dependencies. The directive is written the way Go writes a machine-readable comment, so
`gofmt` leaves it where you put it:

```go
import (
	"database/sql"

	_ "github.com/lib/pq" //archunit:ignore
)
```

A bare directive is honoured by every rule. A scoped one — `//archunit:ignore layers` — is honoured only by a
check that names that scope in `CheckOptions.IgnoreScopes`, and counts as an ordinary dependency everywhere
else. The directive belongs to the one import it is written on, either trailing it or on a comment-only line
directly above it, so a directive above the whole import block belongs to nothing.

For the blank imports that register a driver, `CheckOptions.IgnoredImportKinds` drops a whole flavor of import
before any edge is emitted — an import kind set, which lives behind the surface in `common/extraction`.

## When a rule selects nothing

**Zero matches is a violation, not a pass.** A selector matching no file is almost always a stale glob or a
renamed folder, and such a rule is green forever — so the guard is wired into every terminal, in whichever of
its two shapes that terminal has room for:

- **A terminal that returns violations reports an `EmptyTestViolation`**, which is `Check` in every family.
- **A report terminal fails with an error**, because a report has no violation list to put a violation in:
  `ErrEmptySnapshot` for the graph family, `ErrEmptyReport` for the metrics family's `ExportAsHTML`, and
  `ErrNothingToDraw` for the slices family's `ToPlantUML` and `ExportAsPlantUML`.

`CheckOptions.AllowEmptyTests` opts out of both. The terminals that report emptiness in neither shape are
`Measure` and the scope terminals `SelectFiles`, `SelectLayerFiles` and `SelectSliceFiles`, deliberately: they
judge nothing, so whether an empty selection is a failure is not a question they can ask. `Measure` hands back
the numbers themselves, and no subject is no numbers; a scope terminal hands back what a selector resolved to,
which is the answer a reader wants most in the case where it resolved to nothing.

To see what a selector actually resolves to, ask the scope stage rather than the rule:

```go
files, err := archunit.ProjectFiles(nil).InFolder("internal/api/**").SelectFiles(nil)
membership, err := archunit.ProjectLayers(nil).Layer("api").DefinedByFolder("internal/api/**").SelectLayerFiles(nil)
sliced, err := archunit.ProjectSlices(nil).DefinedBy("internal/(**)/**").SelectSliceFiles(nil)
```

## Next

[How it works](internals.md) is where the pipeline behind all of this is laid out, and where to change
something.
