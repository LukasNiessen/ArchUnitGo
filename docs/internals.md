---
layout: default
title: How it works
nav_order: 10
description: The pipeline behind an ArchUnitGo rule, the repository layout, the four dependency rules the library holds itself to, and where a new verb goes.
---

# How it works

The whole product is the fluent API; everything else is its implementation. This page is the short version of
what is behind it — enough to read the source, find the file a behaviour lives in, or add a verb.

## The pipeline

```
SOURCE  ->  EXTRACT  ->  PROJECT  ->  ASSERT  ->  REPORT
```

| Stage | Does | Knows Go? |
|---|---|---|
| **Source** | finds the user's project, from a locator or by walking up to the nearest `go.mod` | only in how the root is found |
| **Extract** | walks and parses the source, resolves every import to a target, emits the edges | **yes, entirely** |
| **Project** | relabels those edges into what the rule is about — files, slices, layers, components — and drops the rest | no |
| **Assert** | walks the projected structure and emits one violation per disagreement | no |
| **Report** | turns violations into a test failure, a message or a rendered diagram | only the test-framework glue |

The consequence worth internalising is that **almost all the Go-specific work is in Extract**. Everything
downstream of it is arithmetic over labelled edges, which is why the projections and the assertions can be
tested against a hand-built graph with no project on disk at all — and why a rule behaves the same on Windows
and on Linux.

It is also why this library is recognisably the same product as its siblings: the four stages, the vocabulary
and the fluent grammar are the shared design, and only Extract is written per language.

## The layout

```
common/         the kernel — everything shared
  extraction/   the dependency-graph extractor  <- one of the two packages that know Go
  projection/   reshaping the graph, plus cycles
  assertion/    the violation vocabulary
  fluentapi/    Checkable, CheckOptions
  archerror/    the technical and user error types
  matching/     globs, regular expressions, match targets
  logging/      the log a check writes
files/          file-level dependency and naming rules
layers/         named-layer policy
slices/         component and diagram rules
metrics/        numeric code-quality rules      <- metrics/extraction is the other one
graph/          dependency-graph reports
archtest/       violation formatting and test-framework glue
archunit.go     the public surface — re-exports, nothing else
```

`common/extraction` and `metrics/extraction` are those two: the first walks the imports of the whole project,
the second reads what is inside one file, and nothing downstream of either knows what an import or a
statement is.

Every domain module has the same internal shape, which is the single most useful thing to know about this
repository: reading one module teaches you how to navigate all of them.

```
files/
  fluentapi/     the builder chain the user types   <- the only public part
  assertion/     pure: structure -> violations
  projection/    module-specific reshaping          (optional)
  extraction/    module-specific gathering          (optional)
  calculation/   pure formulas                      (optional)
```

`assertion`, `projection` and `calculation` are pure: no filesystem, no clock, no globals. That is what makes
the interesting half of the library testable without a project to point it at.

## The four dependency rules

The library's own architecture is four rules, and they are the rules it enforces on itself:

1. `common` depends on nothing but the standard library and the Go analysis toolchain.
2. Domain modules depend on `common`. **They must not depend on each other.**
3. The testing layer depends on `common` and on the domain modules' violation types. Nothing else.
4. The public surface depends on everything, and nothing depends on it.

Rule 2 is the one that decays first — one module reaching into another for *just one helper* — which is why
they are written down as a suite through this library's own public API in
[architecture_test.go](https://github.com/LukasNiessen/ArchUnitGo/blob/main/architecture_test.go). That file is
also the best worked example this repository has: it is a real policy over a real codebase, and it fails when
somebody breaks it.

## Two invariants of the data model

- **Identifiers are project-relative and forward-slashed, everywhere.** Every pattern, every violation and
  every label is that spelling, which is why the same rule holds on every platform.
- **Every file gets a self-edge.** That is how a file with no dependencies at all still appears as a node.
  Projections filter self-edges out by default, and the node projection is the one that depends on them.

## Adding a verb

In order, each step landing in a predictable place:

1. **Write the sentence first** and read it aloud. If it is not something an architect who does not write Go
   would understand, the verb is wrong — that is the acceptance test, before any code.
2. **Pick the module.** A new node vocabulary is a new module; anything else belongs in an existing one.
3. **Define the violation type** in `<module>/assertion`. Data only, never a sentence.
4. **Write the gather function** beside it. Pure, and handling both moods through the mood flag rather than in
   two code paths.
5. **Add a projection** in `<module>/projection`, or reuse one from `common`.
6. **Add the fluent stages** in `<module>/fluentapi` — the predicate on both mood builders, and the terminal
   implementing `Checkable`.
7. **Wire the empty-test guard** into that terminal.
8. **Teach the violation factory** how to phrase it, in the testing layer.
9. **Test at both levels**: the pure parts against a fixture graph, and one integration test through the
   public API.

If a step has no obvious home, the rule is probably in the wrong module.

[AGENTS.md](https://github.com/LukasNiessen/ArchUnitGo/blob/main/AGENTS.md) is the authority on all of this —
the architecture, the naming, the grammar and the error taxonomy — for this library and for every other
ArchUnit port. It is what to read before changing anything.

## Why the documents are tested

Prose goes stale in the commit that changes the code, so the documents of this repository are held to the
source by tests like everything else:

- `README.md` is parsed, and every Go identifier it names is looked up in this module's own syntax tree.
- These pages are held to the same guard, plus their own: each family page has to name every verb its module
  exports, and every link between pages has to resolve.
- The workflow files are held to the checks they are supposed to run, as text.

A document nobody can fail is a document that goes stale, and a documentation site is the easiest place in a
repository for that to happen quietly.

## The API reference

There is deliberately no reference section on this site. Every exported name of this library carries its own
doc comment, and [pkg.go.dev](https://pkg.go.dev/github.com/LukasNiessen/ArchUnitGo) renders them — which is
where a Go reader looks, and one copy fewer to go stale.
