---
layout: default
title: ArchUnitGo
nav_order: 1
description: Architecture rules as ordinary Go unit tests — install it, write your first rule, read what a failure looks like.
---

# ArchUnitGo

Write the sentence your team already says out loud — *the api does not touch the database* — as a value,
and let `go test` tell you where the code disagrees.

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

That is the whole setup: no fixture, no registration, no configuration file. A rule is a value, `nil` means
the defaults everywhere, and the only thing that reads your project is the terminal at the end of the chain.

## Install

```sh
go get github.com/LukasNiessen/ArchUnitGo
```

Go 1.26 or newer. The only direct dependency is `golang.org/x/tools`, which is how the extractor talks to
the Go toolchain. There is no tagged release yet, so `go get` resolves to a pseudo-version of `main` and the
API may still move.

The package is `archunit` while the last element of the module path is `ArchUnitGo`, so give the import the
name it has:

```go
import archunit "github.com/LukasNiessen/ArchUnitGo"
```

## Your first rule

An architecture rule is a normal test in a normal package. Put it wherever your other tests live:

```go
package architecture_test

import (
	"testing"

	archunit "github.com/LukasNiessen/ArchUnitGo"
)

func TestNoFileDependsOnAnotherInACircle(t *testing.T) {
	archunit.AssertPasses(t, archunit.ProjectFiles(nil).Should().HaveNoCycles(), nil)
}
```

Three things are worth knowing before you write the second one:

- **A rule is a value.** Building one does no work. Only the terminal — `AssertPasses` here, or `Check` —
  reads the project, so a half-built rule can be stored in a variable and branched from.
- **`nil` means the defaults, everywhere.** The `*ProjectLocator` an entry point takes is `nil` for *the
  project this test is in*, found by walking up to the nearest `go.mod`. The `*AssertOptions` and
  `*CheckOptions` a terminal takes are `nil` for the defaults.
- **A failing rule is not an error.** `Check` returns `([]Violation, error)`: the violations are the rule's
  result and the error is the library or the environment failing. `AssertPasses` turns the first into
  `t.Error` and the second into a message saying the check could not be run at all.

## What a failure looks like

One `t.Error`, carrying the rule as it was written and then the violations, numbered from one:

```
project files, path without filename matches "internal/api/**", should not, depend on files, path without filename matches "internal/db/**"
1 violation:
  1. internal/api/handler.go: should not, depend on files, path without filename matches "internal/db/**"; it depends on internal/db/conn.go
```

The rule's own sentence comes first, so a test asserting several rules says which one broke before it says
what broke it. A selector renders as the part of an identifier it was matched against rather than as the verb
that spelled it — `InFolder` reads back as *path without filename matches* — because that is what you have to
compare your glob against. See [patterns and identifiers](patterns.md) for what those parts are.

## Where to go next

- [The grammar](grammar.md) — the stages every rule is built from, in every family.
- [Patterns and identifiers](patterns.md) — what a glob means and what it is matched against.
- [The files family](files.md) — what depends on what, how files are named, where they live.
- [The layers family](layers.md) — a named-layer policy as one rule.
- [The slices family](slices.md) — components cut out of identifiers, and diagrams.
- [The metrics family](metrics.md) — the numbers a project adds up to, and what they have to be.
- [Dependency-graph reports](graph.md) — the diagram rather than the rule.
- [Running a rule](running.md) — check options, logging, colors, suites, no test framework at all.
- [How it works](internals.md) — the pipeline, the layout, and where to change something.

Two things this site deliberately does not hold. The **API reference** is
[pkg.go.dev](https://pkg.go.dev/github.com/LukasNiessen/ArchUnitGo), generated from the doc comments in the
source, because that is where a Go reader looks for one and a second copy would be a second thing to go
stale. The list of **what is not implemented yet** is in the
[README](https://github.com/LukasNiessen/ArchUnitGo#what-is-not-implemented-yet), which is the one place it
is stated and tested.
