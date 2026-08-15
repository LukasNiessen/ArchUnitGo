# AGENTS.md

## What this repository is

**ArchUnitGo** is the Go member of the ArchUnitEverything family: one architecture-testing library
per language, all recognisably the same product. Users write architecture rules as ordinary unit
tests.

```go
func TestApiDoesNotTouchTheDatabase(t *testing.T) {
	rule := archunit.ProjectFiles().
		InFolder("internal/api/**").
		ShouldNot().
		DependOnFiles().
		InFolder("internal/db/**")

	archunit.AssertPasses(t, rule)
}
```

The siblings are [ArchUnitTS](https://github.com/LukasNiessen/ArchUnitTS) and
[ArchUnitPython](https://github.com/LukasNiessen/ArchUnitPython), which are shipped and in
production, plus ArchUnitRuby, ArchUnitNET and ArchUnitRust, which are being built alongside this one.
Someone who knows any of them should be able to read this one without a map.

## How to read this file

This is a guideline, not a specification. It exists so that code written by different people, in
different languages, by different agents, comes out looking like it came from one library.

**Follow it by default. Where Go makes a convention awkward, diverge — knowingly, and leave a
line in the commit message or a comment saying why.** Fighting the host language to match a sibling
is the one failure mode worse than divergence. Idiomatic Go wins whenever the two genuinely
conflict; most of the time they do not.

It is also not exhaustive, deliberately. If something is not covered here, pick whatever a reader of
the other ArchUnit libraries would find least surprising, and get on with it.

## The mental model

ArchUnit turns a codebase into a directed dependency graph, lets the user describe a subset of that
graph in a sentence, and reports every place the code disagrees with the sentence.

Everything in the library is one of four things: something that **builds** the graph, something that
**reshapes** it into the vocabulary the user is talking about, something that **judges** the
reshaped graph, or something that **lets the user say the sentence**. The fluent API is the product.
The other three are its implementation.

A rule is a value, not an action. Building one does no work. Only the terminal `check` touches the
filesystem.

## The pipeline

```
SOURCE  ->  EXTRACT  ->  PROJECT  ->  ASSERT  ->  REPORT
```

| Stage | Does what | Go-specific? |
|---|---|---|
| **Source** | The user's project, found from a project root or a build file. | Only in how the root is found. |
| **Extract** | Walk and parse the source, resolve every import to a target, emit `Edge` values. | **Yes, entirely.** This is the only stage that knows Go. |
| **Project** | Relabel raw edges into what the rule is about: files, slices, layers. Drop the rest. | No. |
| **Assert** | Walk the projected structure, emit a `Violation` per disagreement. Pure, in-memory. | No. |
| **Report** | Turn violations into a test failure, a message, or a rendered diagram. | Only the test-framework glue. |

The consequence worth internalising: **almost all the Go-specific work is in Extract.** If you
find yourself rewriting projection or assertion logic in a language-specific way, something has
gone wrong — go and look at how a sibling does it.

## Layout

```
common/         the kernel — everything shared
  extraction/   the dependency-graph extractor  <- the only Go-specific code
  projection/   reshaping the graph, plus cycles/
  assertion/    Violation, EmptyTestViolation
  fluentapi/    Checkable, CheckOptions
  error/        TechnicalError, UserError
  matching/     globs, regex, match targets  <- was util/; see the note below
  logging/      the log a check writes       <- the other half of that note
files/          file-level dependency and naming rules
layers/         named-layer policy
slices/         component and diagram rules
metrics/        numeric code-quality rules
graph/          dependency-graph reports
testing/        violation formatting and test-framework glue
archunit.go     the public surface — re-exports, nothing else
```

**There is no `common/util`, and there will not be one.** The sibling libraries have one and this table
used to say so, but `revive`'s `package-naming` rule in `.golangci.yml` rejects the name, citing
[go.dev/blog/package-names](https://go.dev/blog/package-names#bad-package-names) — and in Go the
directory *is* the package, so the directory has to carry a real name too. Logging landed as `logging/`
the same way pattern matching landed as `matching/`, and path helpers get their own named package when
they land too. This is the "idiomatic Go wins over a sibling convention" rule below, applied once so that
nobody has to rediscover it against a failing lint run.

Every domain module has the same internal shape. This is the single most useful convention in the
whole file, because it means reading any one module teaches you how to navigate all of them.

```
files/
  fluentapi/     the builder chain the user types   <- the only public part
  assertion/     pure: structure -> violations
  projection/    module-specific reshaping          (optional)
  extraction/    module-specific gathering          (optional)
  calculation/   pure formulas                      (optional)
```

`assertion`, `projection` and `calculation` are **pure** — no filesystem, no clock, no globals. That
is what lets you test them against hand-built fixture graphs before the extractor works at all, and
you should.

Four dependency rules, which are just the library's own architecture rules:

1. `common` depends on nothing but the standard library and the Go analysis toolchain.
2. Domain modules depend on `common`. **They must not depend on each other.**
3. `testing` depends on `common` and on the domain modules' violation types. Nothing else.
4. The public surface depends on everything, and nothing depends on it.

Rule 2 is the one that decays first. `files` reaching into `slices` for "just one helper" is the
classic failure — the helper belongs in `common/projection` instead.

## The data model

Six types carry everything. Keep the names; adapt the shapes to whatever Go wants.

```
Edge          { source, target, external, importKinds }
Graph         = list of Edge

MappedEdge    { sourceLabel, targetLabel }
ProjectedEdge { sourceLabel, targetLabel, cumulatedEdges }
ProjectedNode { label, incoming, outgoing }

MapFunction   = Edge -> MappedEdge | nothing      ("nothing" means drop this edge)

Filter        { regexp, target, matching, exclusions }
Violation     the base type; every rule family adds its own
CheckOptions  { allowEmptyTests, logging, clearCache, ...language knobs }
Checkable     { check(options?) -> list of Violation }
```

Things that are easy to get wrong and expensive to fix later:

- **Identifiers must be normalised and stable.** Pick one convention — separators normalised, and
  either project-relative throughout or absolute throughout — and never mix. Patterns match against
  these strings, so inconsistency shows up as baffling rule failures.
- **Every file gets a self-edge.** That is how a file with no dependencies still appears as a node.
  Projections filter self-edges out by default; node projection depends on them.
- **Parallel edges are merged**, unioning their import kinds. Downstream code may assume
  `(source, target)` is unique.
- **Globs are sugar, regex is the substrate.** Every user pattern compiles to a regex at build time
  in one place. Nothing downstream ever sees a glob.
- **Violations carry data, not prose.** Message construction lives in `testing`, so one place
  controls formatting, numbering and colour.
- **Zero matches is a violation, not a pass.** A selector matching no files is almost always a typo,
  and silently passing is the worst possible outcome. `EmptyTestViolation` by default;
  `allowEmptyTests` to opt out. This is the highest-value defensive thing in the library — every
  terminal needs it.
- `ImportKind` is **expected to differ per language**. Here it is `ImportKindPlain`, `ImportKindAliased`, `ImportKindBlank` (`_`), `ImportKindDot`. This is the one
  place a language-specific vocabulary is welcome in the shared model.

`Checkable` is the seam the whole library hangs from. Every terminal implements it, every consumer
programs against it and nothing else. Adding a rule should never require touching the testing layer.

## The fluent API — how it should sound

This is the part users see and the part that most needs to feel the same in every language.

Every rule is an English sentence, read left to right:

```
ENTRY          SCOPE          MOOD         PREDICATE      OBJECT          TERMINAL
project files  in folder      should not   depend on      in folder       check
               "**/api/**"                 files          "**/db/**"
```

> *project files, in folder `**/api/**`, should not depend on files in folder `**/db/**`.*

**The acceptance test for any new API method: read the whole chain aloud. If it is not a sentence an
architect who does not write Go would understand, the name is wrong.** Not the implementation —
the name.

| Stage | How many | Returns |
|---|---|---|
| Entry | exactly 1 | a scope builder |
| Scope | 0..n, chainable, combined with AND | a scope builder |
| Mood | exactly 1, except where the predicate carries it | a positive or negated predicate builder |
| Predicate | exactly 1 | a terminal, or an object builder if the predicate is relational |
| Object | 1..n, chainable | a terminal |
| Exclusion | 0..n after any selector, qualifying the one it follows | whatever the selector returned |
| Terminal | exactly 1 | violations, or a rendered artifact |

The one exception to the mood stage is the pair of layer predicates below: `may only depend on layers`
and `may not depend on layers` are their own polarity, so a mood before them would read as `should not,
may not depend on layers`. The mood is still what makes the two one piece of logic — it travels on the
clause the predicate builds, where the allowlist is `should` and the blocklist `should not` — and every
other family, existing or not yet written, has the stage.

Word choice is fixed. Casing is yours. Exported identifiers are `PascalCase`, unexported ones `camelCase`. `depend on files` becomes `DependOnFiles`.

**Entry points** are noun phrases — `project files` (alias `files`), `project slices`,
`project layers` (alias `layers`), `project graph` (alias `dependency graph`), `metrics`. Each takes
an optional project locator; omitted means auto-detect. Never make it required.

**Scope verbs** are prepositional, describing where or what — `with name`, `in folder`, `in path`,
`in file`, `for classes matching`, `defined by`, `defined by regex`, `defined by folder`, `layer`,
`where layer`. The last two are the layer policy's pair: `layer(name)`, closed by a `defined by` verb,
declares who exists, and `where layer(name)` picks the one a clause is about.

**Exclusions** are the `except` family, and every selector takes one — a scope verb, an object verb, a
layer declaration, a pattern modifier. `except` alone reads its patterns against the same part of an
identifier as the selector it follows; `except with name`, `except in folder`, `except in path` and
`except classes matching` name a target of their own, and a family offers exactly the ones its own
selectors already name. An exclusion qualifies the selector in front of it and nothing else, it is
repeatable, and `except` with no selector in front of it is a user error rather than a rule about
everything. This is what keeps *everything under `app/`, but not the generated folder* one clause
instead of an inverted rule about the generated folder.

**Mood** is exactly two words, `should` and `should not`. No synonyms, ever.

**Predicates** are bare infinitives, so that `should` + predicate reads as English — `have no
cycles`, `have name`, `be in folder`, `be in path`, `adhere to`, `depend on files`, `depend on
external modules`, `adhere to diagram`, `contain dependency`, `may only depend on layers`,
`may not depend on layers`.

**Threshold predicates**, metrics only, exactly these six — `should be below`, `should be above`,
`should be`, `should be below or equal`, `should be above or equal`, `should satisfy`. Do not add
`should equal`, `should be at most`, `should be less than` or any other synonym. Synonyms are how a
fluent API stops sounding like one language.

**Modifiers** are present participles, chainable, order-independent and always optional —
`ignoring ...`, `including ...`, `focus on`, `reachable from`, `dependents of`,
`collapse to folder depth`, `collapse by pattern`, `titled`, `with check options`.

**Terminals** — `check(options?)` returning violations is the universal one, plus `to <format>`,
`export as <format>(path)` and `summary()`.

Two structural rules behind the grammar:

- **Mood is a boolean flag threaded into the assertion function, not a duplicated set of negative
  code paths.** The positive and negated builders are two thin types over one shared assertion.
  This is why the negative API costs almost nothing to maintain, and why forking the logic would
  double the bug surface.
- **Builders are immutable and return new instances.** A half-built rule can be stored and branched
  from:

  ```
  base  = project files, in folder "src/**"
  ruleA = base, should, have no cycles
  ruleB = base, should not, depend on files, in folder "**/legacy/**"
  ```

## Naming

| Thing | Convention |
|---|---|
| Directories | Lowercase, singular, unabbreviated, no separators. `fluentapi` is one word. lowercase, exactly as written above |
| Files | One concept per file, named after the concept: `extract_graph.go`, with `extract_graph_test.go` beside it |
| `extract <thing>` | gathers data from source |
| `project <thing>` | reshapes a graph |
| `per <thing>` | a `MapFunction` factory — `per edge`, `per internal edge` |
| `slice by <thing>` | a slicing `MapFunction` factory |
| `gather <thing> violations` | the pure assertion functions |
| `matches <thing>` | a boolean predicate |
| `<thing> matcher` | a `Filter` factory — `filename matcher`, `folder matcher` |
| `<Thing>Violation` | a violation subtype — always this order, never `Violating<Thing>` |
| `<Thing>Options` | an options bag |
| `<Thing>Factory` | a collection of constructors |
| `<Thing>Info` | extracted descriptive data — `ClassInfo`, `FileInfo` |
| `<Thing>Builder` / `<Thing>Condition` | a fluent stage / a fluent terminal |

**Keep the file stems the same as the siblings.** `extract_graph`, `project_edges`, `tarjan_scc`,
`regex_factory`. It looks like a small thing and it is worth a lot: a reviewer who knows one port
finds the corresponding file in another in one guess.

**Options bags, always.** No terminal takes more than one parameter beyond its required argument.
Every optional knob goes in a named options type with defaults. That is what lets options be added
later without breaking callers.

**Two error types**, and the distinction is about who is at fault. `TechnicalError` — the library or
the environment failed. `UserError` — the API was used wrongly. **A failing architecture rule is
neither**; it is a `Violation` in a returned list. Never raise for a rule failure. That is the
assert helper's job, at the very edge.

## Go specifics

- **Module path** `github.com/LukasNiessen/ArchUnitGo`. The public surface is package `archunit`
  at the repository root; it re-exports and does nothing else.
- **Directory is package.** Package name always matches the directory name. `fluentapi` stays one
  word, so the package is `fluentapi`, not `fluentApi` or `fluent_api`.
- **The `testing` directory is the one name worth reconsidering.** The siblings all call it
  `testing`, but a package of that name shadows the stdlib `testing` in any file that imports both —
  which is exactly the file that needs it. `archtest` is the sane Go answer. Either is fine;
  decide once, early, and do not mix.
- **Extraction** uses `go/packages` and `go/ast`, and locates the project by walking up to `go.mod`.
  Let the toolchain resolve imports; do not do path arithmetic on import strings.
- **Options bags** are structs. `Check(opts ...CheckOption)` is tempting, but `Check(*CheckOptions)`
  with a nil-means-defaults contract matches the siblings and reads better in the docs. Pick that.
- **Errors** are `TechnicalError` and `UserError`, both implementing `error`, wrapped with `%w`.
  A failing rule is a `[]Violation` — never an `error`.
- **Testing** is the stdlib `testing` package. `AssertPasses(t, rule)` calls `t.Helper()` and
  `t.Error` with the formatted violations. Go has no matcher-registration hook, so that helper *is*
  the integration; there is nothing to auto-register.
- **Node vocabulary.** Go engineers reason in packages more than in files. Files stay the primary
  node so rules match the siblings — but package-level selectors are a natural and welcome addition
  on top, not a replacement.
- Everything is synchronous. Go has no async idiom to mirror here.
- Avoid `interface{}`/`any` in the public API. Generics only where they genuinely remove duplication.

## Working here

- **Commit straight to `main`.** No branching ceremony, no required review, no pull requests. There
  are no users yet and one maintainer. Speed is the point. (This does *not* apply to ArchUnitTS and
  ArchUnitPython, which have real users and full process.)
- **Test the pure parts against fixture graphs**, and add one integration test per rule through the
  fluent API against a sample project. The first is where the coverage comes from; the second proves
  the wiring.
- **Dogfood.** Once the files module works, the library should enforce its own architecture rules on
  itself in its own test suite. The siblings do this and it is the best documentation the project
  has.
- Issues in this repository cover a large chunk of the planned work. They are starting points, not
  specifications. If one conflicts with reality in Go, say so and we adjust it.

## Adding a new rule

In order, and each step lands in a predictable place:

1. **Write the sentence first**, and check it reads as English. If it does not, the rule or the verb
   is wrong. Do not proceed.
2. **Pick the module.** New node vocabulary means a new module; otherwise it belongs in an existing
   one.
3. **Define the violation type** in `<module>/assertion`. Data only.
4. **Write `gather <thing> violations`** in `<module>/assertion`. Pure. Handle both moods via the
   flag.
5. **Add a projection** in `<module>/projection`, or reuse one from `common`.
6. **Add the fluent stages** in `<module>/fluentapi` — the predicate on both builders, and the
   terminal implementing `Checkable`.
7. **Wire the empty-test guard** into the terminal.
8. **Teach the violation factory** how to phrase it, in `testing`.
9. **Test at both levels.**

If a step has no obvious home, the rule is probably in the wrong module.
