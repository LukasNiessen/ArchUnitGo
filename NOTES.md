# NOTES

Deviations from the issue text, `AGENTS.md` or sibling convention. One line each.

## Issue #1 — Kernel: Edge, Graph and ImportKind

- WHY: `importKinds` is its own type `ImportKindSet` (a `uint8` bit set) instead of a list field —
  merging parallel edges is then a single `|`, and it keeps `Edge` comparable so it can be a map key,
  which is what `NewGraph` merges with.
- WHY: file stems are `edge.go`, `graph.go`, `import_kind.go`, `identifier.go` rather than the
  sibling stems listed in `AGENTS.md` (`extract_graph.go` and friends) — those name the extractor,
  which this issue does not build; these name the concepts the issue does ask for.
- WHY: identifier normalisation sits in `common/extraction/identifier.go`, not in `common/util` with
  the path helpers — the issue scopes the kernel to `common/extraction`, and identifiers must be
  minted in exactly one place, which is here.
- WHY: `Graph` is `type Graph []Edge` (a list of `Edge`, as the data model says) with the invariants
  — normalised identifiers, merged parallel edges, stable order — established by `NewGraph`/`Add`
  rather than by a struct wrapper, so downstream code just ranges over it.
- WHY: `RelativeIdentifier` does lexical prefix arithmetic on normalised identifiers instead of
  calling `filepath.Rel` — `filepath` treats `\` as a separator only on Windows, so `Rel` gave
  host-dependent answers for the same input.
- WHY: no fluent-API integration test in this issue — nothing public exists to route a rule through
  yet. The kernel is covered by unit tests against hand-built edges and a fixture graph.

## Issue #2 — Kernel: pattern matching

- WHY: pattern matching lives in `common/matching`, not in `common/util` as the `AGENTS.md` layout
  table says — `revive`'s `package-naming` rule in this repository's own `.golangci.yml` rejects
  `util` as a package name, and "directory is package" means the directory has to move with it. Only
  pattern matching moved; `common/util` is still free to appear for logging and path helpers.
- WHY: separators are normalised for globs and for match candidates, but never for a regex pattern —
  in a regular expression a backslash is an escape, so rewriting it to `/` would silently corrupt
  `\.` into `/.`. A regex pattern is documented as using forward slashes, which is the identifier
  convention anyway.
- WHY: `Filter.Matches` does not call `extraction.NormalizeIdentifier` on its input, only the cheap
  separator normalisation local to this package. Importing `extraction` from `matching` would invert
  the kernel's layering and set up an import cycle the moment the extractor wants to filter files;
  identifiers reaching a Filter come from a `Graph` and are already canonical.
- WHY: patterns are anchored at both ends, regex ones included (`^(?:…)$`, non-capturing so that
  `a|b` anchors as a whole). Globs are whole-identifier by nature and a half-anchored regex beside
  them would be the surprise.
- WHY: `a/**/b` matches `a/b` and a trailing `a/**` matches `a` itself. Without the second, a folder
  rule written as `internal/api/**` would not hold for the folder `internal/api`, which is exactly
  what `TargetPathWithoutFilename` extracts from a file in it.
- WHY: an invalid pattern is returned as an error wrapping the `ErrInvalidPattern` sentinel, not as a
  `UserError` — `common/error` does not exist yet. When it lands, these two constructors are the
  place to wrap.
- WHY: `TargetClassname` means "the declared name with package and path qualification stripped", the
  final dot-separated element. Go has no classes; the family's vocabulary is kept, and this is the
  nearest honest Go meaning.
- WHY: `Filter.Excluding` and `Filter.NotMatching` are listed in `govet.unusedresult.funcs` as the
  harness requires, but that entry is inert and a reader should not trust it: `unusedresult` only
  matches *package-level functions* (it splits the name at the last `.` and compares
  `fn.Pkg().Path()`), and reaches methods only through its separate `stringmethods` flag, for
  `func() string`. Verified empirically — a bare `f.Excluding(p)` statement is not reported. So an
  unterminated chain *method* is currently unguarded; the list still works for entry points such as
  `ProjectFiles`. Nothing was lost by adding the entries: `funcs: []` had already clobbered the
  analyzer's stdlib defaults, since setting the flag at all replaces them.
- WHY: no fluent-API integration test, for the same reason as issue #1. The level above the unit
  tests is `TestFilterSelectsNodesOfAFixtureGraph`, which runs filters over the nodes of a
  hand-built `extraction.Graph` — the shape every rule will use them in.
