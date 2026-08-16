---
layout: default
title: The layers family
nav_order: 5
description: A named-layer policy in ArchUnitGo — declaring the layers once, then saying what each of them may and may not depend on.
---

# The layers family

An N-layer policy written as file-level rules is N² pairwise rules, and a report about two folder globs reads
worse than a report about `api` and `db`. This family is the same vocabulary with the sets of files named up
front:

```go
rule := archunit.ProjectLayers(nil).
	Layer("api").DefinedByFolder("internal/api/**").
	Layer("domain").DefinedByFolder("internal/domain/**").
	Layer("db").DefinedByFolder("internal/db/**").
	WhereLayer("api").MayOnlyDependOnLayers("domain", "db").
	WhereLayer("db").MayNotDependOnLayers("api")
```

`archunit.ProjectLayers` is the entry point and `archunit.Layers` the shorter spelling of it. The chain says
nothing a pile of file-level rules could not — it exists because that pile is miserable to write and worse to
read.

## The two halves

A chain has a declaration half and a policy half, and both are chainable:

| Half | Opened by | Closed by |
|---|---|---|
| Who exists | `Layer(name)` | `DefinedByFolder` or `DefinedBy` |
| What they may do | `WhereLayer(name)` | `MayOnlyDependOnLayers` or `MayNotDependOnLayers` |

`DefinedByFolder` matches the folder of a file, which is what a layer usually is. `DefinedBy` matches the
whole identifier, which is what a layer whose members are *named* rather than placed needs —
`Layer("persistence").DefinedBy("internal/**/*_repository.go")` is a layer scattered across the tree.

The policy half is where a chain becomes checkable. A chain that declares three layers and no clause is not
yet a rule about anything, so it has no `Check`: `WhereLayer` is what turns the declaration into one, and
every clause hands back something that takes another `WhereLayer` — so a whole policy is one value.

## The two clauses

```go
rule := archunit.ProjectLayers(nil).
	Layer("api").DefinedByFolder("internal/api/**").
	Layer("domain").DefinedByFolder("internal/domain/**").
	WhereLayer("api").MayOnlyDependOnLayers("domain").
	WhereLayer("domain").MayOnlyDependOnLayers()
```

`MayOnlyDependOnLayers` is the allowlist: this layer may depend on the layers it names and on no other
declared layer. `MayNotDependOnLayers` is the blocklist: whatever else it does, not these. Named with nothing
at all, the allowlist is the **sealed layer** — *domain may only depend on layers, full stop* — which is how
the innermost ring of a hexagonal architecture is spelled.

There is no mood stage here, because the two clauses carry their own polarity: `should not, may not depend on
layers` is not a sentence anybody would type. The mood is still what makes them one piece of logic rather than
two, and [the grammar](grammar.md#mood) is where that is spelled out.

## The three semantic rules

A policy is easy to misread, so its semantics are exactly three rules and no more:

1. **Dependencies inside a layer are always allowed.** A layer is a set of files that belong together; a
   policy is about the arrows between the sets.
2. **A dependency with an end in no declared layer is ignored.** Declaring three layers of a project with
   thirty folders is a rule about those three, not a ban on the other twenty-seven — and a file excluded from
   a layer by `Except` is in no layer, so it is ignored the same way.
3. **A blocklist clause is asked before an allowlist one.** When both name the same pair, the blocklist wins,
   so a general allowlist can be written once and a single forbidden pair carved out of it.

## Exclusions

`Except` and its two targeted forms qualify the declaration in front of them, so a layer with a hole in it is
still one declaration:

```go
policy := archunit.ProjectLayers(nil).
	Layer("api").DefinedByFolder("internal/api/**").Except("**/generated").
	Layer("db").DefinedByFolder("internal/db/**").ExceptInPath("**/*_mock.go").
	WhereLayer("db").MayNotDependOnLayers("api")
```

| Exclusion | Reads its patterns against |
|---|---|
| `Except` | whatever the declaration in front of it reads — the folder after `DefinedByFolder`, the identifier after `DefinedBy` |
| `ExceptInFolder` | the folder |
| `ExceptInPath` | the whole identifier |

A file taken out of a layer this way is not moved to another one: it is in none, and every dependency it is an
end of is ignored by rule 2.

## What a violation says

`Check` reports `LayerDependencyViolation` values, **one per pair of layers** rather than one per import: the
two layers, the layers the broken clause named, and the concrete file dependencies behind the pair. A policy
broken by two hundred imports between the same two layers is one violation with two hundred dependencies in
it, because *api depends on db* is one fact about the architecture.

## Asking what a layer holds

`SelectLayerFiles` resolves the declarations and hands back the files of each layer by name, which is the
first thing to reach for when a policy passes and you doubt it:

```go
files, err := archunit.ProjectLayers(nil).
	Layer("api").DefinedByFolder("internal/api/**").
	Layer("db").DefinedByFolder("internal/db/**").
	SelectLayerFiles(nil)
```

A layer no file matched is an empty entry, and it is a violation when the policy is checked rather than a
quiet pass — see [running a rule](running.md).

## Next

[Slices](slices.md) are the family for the components a project has *without* declaring them: the names are
cut out of the identifiers rather than written down.
