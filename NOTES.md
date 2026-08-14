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

## Issue #3 — Kernel: RegexFactory and the matcher factories

- WHY: `RegexFactory` joins the pattern constructors in `common/matching/regex_factory.go` rather than
  taking a file of its own. The sibling stem names one thing — the place a user pattern becomes a
  compiled regex — and the factory is the string-facing front door to the constructors already there;
  splitting them would put the two halves of one concept in two files and churn passing tests.
- WHY: the five matchers exist at two levels, with the same names: `FilenameMatcher(Pattern) Filter`
  from issue #2, and `RegexFactory.FilenameMatcher(string) (Filter, error)` now. The methods do not
  re-decide which part of an identifier a selector looks at — each one is `matcher(pattern, build)`
  over the package-level factory, so that pairing is still stated once, in `filter.go`. The receiver
  is what distinguishes them at a call site: a compiled `Pattern` in, or the string the user typed.
- WHY: glob-versus-regex is a `PatternSyntax` on the factory, not a second set of `RegexFilename…`
  methods and not a per-call argument. Go has no `string | RegExp` union, ten methods would double
  every future selector, and the fluent API needs the choice as a value anyway — `defined by` and
  `defined by regex` differ only in which factory they carry.
- WHY: `ExactFileMatcher` has no package-level twin, and needed a third pattern constructor,
  `NewLiteralPattern` (`regexp.QuoteMeta` over the normalised string, source kept as written). Its
  exactness lives in the pattern, not in the match target — a `Filter`-level `ExactFileMatcher` would
  just be `PathMatcher` under a name that promises more. For the same reason the factory's syntax does
  not apply to it, though case sensitivity still does.
- WHY: `ExactFileMatcher` matches the whole identifier, so it wants `internal/api/handler.go` and not
  `handler.go`; a bare filename is `FilenameMatcher`'s job. Choosing on whether the argument contains
  a separator would be exactly the hidden matching branch this issue exists to remove, and a
  no-match is already caught by the empty-test guard.
- WHY: `Compile` is exported. A filter's exclusions must be compiled in the same syntax as the pattern
  they qualify, and without it every caller would reach past the factory to `NewGlobPattern` and
  decide the syntax a second time.
- WHY: no fluent-API integration test, for the same reason as issues #1 and #2. The level above the
  unit tests is `TestRegexFactorySelectsNodesOfAFixtureGraph` — user strings in, selected nodes of a
  hand-built `extraction.Graph` out, which is what a scope verb will do with the factory.

## Issue #4 — Kernel: Violation base type and EmptyTestViolation

- WHY: the "base type" is an interface with one method, `Kind() ViolationKind`, not an embeddable base
  struct. Go has no inheritance and violations share no state — only a contract — and the interface is
  deliberately left without an unexported method so a rule family in any module can implement it.
  `violation_test.go` is therefore an external `assertion_test` package: an in-package test cannot tell
  an open interface from a sealed one, so sealing `Violation` has to break a compile somewhere.
- WHY: `Kind()` returns a named `ViolationKind` string rather than a bare `string`. It is the key the
  testing layer will pick a phrasing by, so it is a stable cross-port spelling (`empty-test`,
  lower-case, hyphenated) and each rule family declares its own constant of that type — naming the
  type is what makes that convention statable in one place.
- WHY: `Kind()` is the interface's only method. A violation's data stays on the concrete type: rule 3
  in `AGENTS.md` has `testing` depending on the domain modules' violation types by design, so a
  generic `Details()`-style accessor would be a second, weaker way to read the same data.
- WHY: `GatherEmptyTestViolations` ships with the type, not with the first terminal. The type alone
  cannot answer *when* zero matches is a violation, and that decision must be made once for every rule
  family; the function is the `gather <thing> violations` form `AGENTS.md` names, over a match count so
  it is indifferent to what was being counted. There is no `Passed([]Violation) bool` helper — an empty
  list is the pass, and a boolean beside it is the thing the issue rules out.
- WHY: `allowEmptyTests` reaches the guard as a field on `EmptyTestOptions`, not as `*CheckOptions`.
  `fluentapi.Checkable` returns `[]Violation`, so `fluentapi` depends on `assertion`; taking the check
  options here would invert that. The terminal copies the one flag across.
- WHY: `EmptyTestViolation` carries `[]matching.Filter`, not the pattern strings. A `Filter` already
  holds the pattern as the user typed it *and* the part of an identifier it was matched against, which
  is what a reader needs to see which selector was wrong — and it keeps the violation free of prose.
- WHY: file stems are `violation.go` and `empty_test_violation.go`, one concept each, rather than the
  sibling extractor stems in `AGENTS.md`. `empty_test_violation.go` is ordinary source: Go only treats
  a `_test.go` *suffix* as a test file.
- WHY: the package doc says "color", not `AGENTS.md`'s "colour" — `misspell` in `.golangci.yml` is set
  to `locale: US`.
- WHY: no fluent-API integration test, for the same reason as issues #1 to #3. The level above the unit
  tests is `TestEmptyTestGuardOnAFixtureGraph`: a filter over the nodes of a hand-built
  `extraction.Graph`, and the guard where a terminal will call it.

## Issue #5 — Kernel: CheckOptions and the Checkable contract

- WHY: `Check` returns `([]assertion.Violation, error)`, not the data model's bare list of violations.
  A check loads and parses a real project, so a technical failure has to go somewhere; Go has no
  exception to throw as the siblings do, and `.golangci.yml`'s `forbidigo` bans `panic`. The contract is
  documented on the method: violations are rule failures, the error is only ever technical or misuse,
  and the two are mutually exclusive. It is what `TechnicalError`/`UserError` will travel in when
  `common/error` lands.
- WHY: `logging` is one `io.Writer` field rather than a nested `{enabled, level}` bag. Non-nil is
  "enabled" and the destination is injected, which is what `.golangci.yml`'s `forbidigo` note on
  `log.Print*` requires of issue #39; a bool beside the writer would be a second way to say off.
- WHY: the Go-specific toggles are exactly three — `IncludeTestFiles` (go/packages' `Tests`),
  `IgnoredImportKinds` (an `extraction.ImportKindSet`, so blank driver imports can be dropped without
  four bools) and `BuildTags`. Each is a knob the extractor cannot work without; anything else would be
  speculative before extraction lands.
- WHY: every default is the zero value, so a nil bag, the zero bag and an explicitly empty one are the
  same check. `WithDefaults` exists anyway, and terminals are told to go through it, so that a
  non-zero default can be added later without touching a terminal.
- WHY: all `CheckOptions` methods take a pointer receiver and tolerate a nil one — `recvcheck` rejects
  mixed receivers, and a nil-safe read is the whole point of the "nil means defaults" contract.
  `CheckOptions` is uncomparable because of `BuildTags`, so its tests compare with `reflect.DeepEqual`.
- WHY: `WithDefaults` clones `BuildTags` instead of returning a bare `*o`. A struct copy separates every
  other field, but the slice header points at the caller's array, so a terminal appending a tag to its
  resolved bag would write through to the user's own options — which a stored half-built rule shares —
  and to every sibling copy. Same reason `EmptyTestOptions` clones its selectors.
- WHY: `CheckOptions.EmptyTestOptions` translates the bag into `assertion.EmptyTestOptions` rather than
  the guard taking `*CheckOptions`. `fluentapi` depends on `assertion` for the violations `Check`
  returns, so the dependency cannot run the other way; this is where issue #4's "the terminal copies
  the one flag across" actually happens, once.
- WHY: no `CheckFunc` adapter and no `Passed([]Violation) bool` helper. The seam is one method and an
  empty list is the pass; both would be a second way to say what already exists.
- WHY: no fluent-API integration test, for the same reason as issues #1 to #4 — no builder chain exists
  yet. The level above the unit tests is `checkable_test.go`, which implements the contract from
  outside the package as `dependencyRule`, the terminal shape of `project files, in folder X, should
  not, depend on files, in folder Y`, over a hand-built `extraction.Graph`, and runs it through a
  consumer that sees nothing but `Checkable`.

## Issue #6 — Kernel: TechnicalError and UserError

- WHY: the package is `common/archerror`, not `common/error` as the `AGENTS.md` layout table says.
  "Directory is package" makes the directory name the identifier, and `error` is a predeclared *type*:
  a file importing it loses `error` at file scope, so `func Check() ([]Violation, error)` stops
  compiling — verified, `error (package name) is not a type` — and every consumer in the library would
  have to alias the import. `.golangci.yml`'s own `predeclared` linter rejects it too (`package name
  error has same name as predeclared identifier`). `errors` was the other candidate and shadows the
  stdlib package in exactly the files that need both. `archerror` is the same answer `AGENTS.md`
  already reaches for with `archtest`, applied once so nobody rediscovers it against a failing build.
- WHY: neither type carries a message string, and the constructors take a cause instead: the reason a
  failure gives is always another error, normally a sentinel declared beside the API that produced it
  (`matching.ErrInvalidPattern` is the first). The siblings pass prose to the constructor; in Go that
  would make `errors.Is` useless and give a caller two ways to ask why. The three fields are
  `Operation` (what was being done / the call at fault), `Subject` (the thing it was about, quoted as
  the user wrote it) and `Cause`.
- WHY: `Error()` builds prose, which `AGENTS.md` otherwise reserves for the testing layer. Go's `error`
  interface requires it, so the rule is kept where it can be: both types render through one unexported
  `describe`, and violations still carry data only. The two types differ solely in the headline —
  `could not <operation>` against the operation as the user typed it.
- WHY: pointer receivers, and the constructors return `*TechnicalError` / `*UserError` rather than
  `error`. `errors.As(err, &user)` with `var user *UserError` is the form every caller writes, and a
  value receiver would silently not satisfy it.
- WHY: no `IsUserError`/`IsTechnicalError` helpers and no shared marker interface. `errors.As` is the
  idiom, the tests use it directly, and a helper would be a second way to ask the one question the two
  types exist to answer.
- WHY: `matching`'s pattern constructors are *not* wrapped in a `UserError` here, though issue #2's note
  said this is where that would land. `UserError.Operation` names the call the user made (`in folder`),
  and `matching` cannot know it — only a fluent stage can. The wrap therefore belongs to the scope verbs
  when they land; `user_error_test.go`'s `inFolder` is that wrap point, written out in test code so the
  shape is on record. `matching.ErrInvalidPattern` stays exactly as it is and travels as the cause.
- WHY: file stems are `technical_error.go` and `user_error.go`, one concept each, rather than the sibling
  extractor stems in `AGENTS.md` — as in issues #1 and #4. The package doc, the `archunit: ` message
  prefix and `describe` sit in `technical_error.go` because they are the package's message policy and it
  is the file the package comment is on; splitting a shared three-line renderer into a third file would
  be worse.
- WHY: one line of `common/fluentapi/checkable.go`'s doc comment changed, which is the only edit outside
  the new package. It promised "once `common/error` lands, the error is a `TechnicalError` or a
  `UserError`"; that is now true and names the real package. No behaviour changed.
- WHY: no fluent-API integration test, for the same reason as issues #1 to #5. The level above the unit
  tests is two tests: `TestCheckTellsAFailureApartFromAFailingRule`, which runs a terminal through
  `fluentapi.Checkable` and reads the blame off the returned error — including the third outcome, a rule
  the code disagrees with, which is neither error — and
  `TestUserErrorIsWhereAFluentStageRejectsWhatTheUserTyped`, which is a scope verb in the shape every one
  of them will have.

## Issue #7 — Extraction: locate the project and enumerate source files

- WHY: file stems are `locate_project.go`, `source_options.go` and `extract_source_files.go`, one concept
  each, rather than the sibling `extract_graph.go` — as in issues #1, #4 and #6. `extract_graph` names the
  stage after this one, and taking the name now would leave the graph extractor nowhere to land.
- WHY: `LocateProject` returns a `string`, and that string is a *host path* — absolute, cleaned, in the
  host's own separators, symlinks left alone — not an identifier and not a one-field `Project` struct. The
  root is the only value in the library that has to stay a path: it is what `filepath.WalkDir` and
  `go/packages` are pointed at, while everything from the graph onwards is the project-relative
  identifiers of `identifier.go`. `FileInfo` carries both forms precisely so nothing downstream has to
  convert between them, and its doc says which one a pattern may be matched against.
- WHY: the module path in `go.mod` is deliberately *not* parsed, so `golang.org/x/mod` stays unused even
  though `.golangci.yml` allows it. Locating a project needs only the file's presence; the module path is
  what tells an internal import from an external one, which is the graph extractor's problem, and
  `modfile` is the way to read it when that issue lands.
- WHY: "no go.mod at or above the starting point" is a `UserError`, not a `TechnicalError`. Nothing failed
  — the library was pointed at something that is not a Go project — and only the caller can say where the
  project is, by running the test inside it or by passing a locator. A `TechnicalError` would tell them to
  file a bug. `os.Getwd` failing is the one genuinely technical outcome here, and a starting point that is
  a file or is missing is a `UserError` quoting the directory as the user typed it.
- WHY: the exclusion set has two halves, and only one is configurable. The Go toolchain's own rule — a name
  beginning with `.` or `_`, and any directory named `testdata` — is applied unconditionally, because a file
  there is not in the build and so cannot be a node in a graph the toolchain would ever produce; that is
  what covers VCS directories, editor state and caches without a list of names to maintain. Vendored
  dependencies and build output are a name list, `DefaultExcludedFolders`, because a project may legitimately
  keep Go source in `build/` and has to be able to say so.
- WHY: `SourceOptions.ExcludedFolders` *replaces* that default list rather than extending it, with nil meaning
  the defaults and a non-nil empty slice meaning "exclude nothing". Extending-only would make the defaults
  impossible to escape, and a second `KeepDefaults` flag would be a second way to say the same thing.
  `WithDefaults` therefore keeps the cloned slice non-nil even when it is empty — otherwise resolving a
  caller's empty list would hand the defaults back.
- WHY: `DefaultExcludedFolders` is a function returning a fresh slice, not the `//nolint:gochecknoglobals`
  package-level table the harness sanctions. The doc promises `append(DefaultExcludedFolders(), "generated")`
  as the way to extend it, and a shared global would let that append write through into every later caller's
  defaults.
- WHY: `SourceOptions` has exactly two knobs, and extra exclusions are not `[]matching.Filter`. Folder names
  are what the walk can act on — a name match lets it `fs.SkipDir` a whole subtree instead of walking it and
  filtering afterwards — and a path-pattern exclusion belongs to the `ignoring ...` modifier when it lands, on
  identifiers, where every other user pattern is already matched. So `extraction` does not import `matching`.
- WHY: one addition outside the issue's package, `CheckOptions.SourceOptions()` in `common/fluentapi`. It is
  the same translation as issue #5's `EmptyTestOptions`: `IncludeTestFiles` exists in both bags, and this is
  where it crosses, once, so no terminal assembles a second enumeration bag by hand. Nothing else changed
  there; the `common/extraction` package doc gained a paragraph naming the stage order.
- WHY: `ExtractSourceFiles` resolves the root when the root *itself* is a symlink, which is the one place
  the "symlinks left alone" rule above is broken, and only there. `filepath.WalkDir` lstats what it is
  pointed at, so a linked root arrives as a non-directory entry and the walk visits nothing; resolving
  unconditionally instead would rewrite every `Path` (macOS `/var` is a link to `/private/var`) and with
  it every identifier. No link met during the walk is followed, so a link to a parent cannot loop.
- WHY: no fluent-API integration test, for the same reason as issues #1 to #6. The level above the unit tests
  is dogfooding on this repository — `TestLocateProjectFindsThisRepository` walks up out of
  `common/extraction` to the real root, and `TestExtractSourceFilesEnumeratesThisRepository` enumerates it and
  looks for this library's own files, with nothing hand-built about either step.

## Issue #8 — Extraction: parse imports and resolve them to targets

- WHY: **an import of one of the project's own packages becomes one edge per file that package is built
  from.** A node is a file and an import names a package, so resolution has to bridge the two, and this is
  the bridge that keeps the graph in one vocabulary — the alternative, a folder or a package as the target,
  would put nodes of two kinds in one graph, give half of them no self-edge, and break every node
  projection. It is also what the language means: a package is compiled as a whole, so a file importing it
  does depend on every file in it. The cost is that `internal/api/handler.go` shows an edge to each of
  `internal/db/conn.go` and `internal/db/query.go` where the source named neither — that is the honest
  reading, and it is what makes `in folder "**/db/**"` and `in file "internal/db/conn.go"` both work.
- WHY: an import is internal or external by whether its path is one of the package paths `go list` reported
  for the project, rather than by stripping the module path off the import string. That is the "let the
  toolchain resolve imports" rule of `AGENTS.md`: vendored copies, `replace` directives, nested modules and
  the standard library all come out right without this library knowing any of those rules. `pkg.Imports` was
  the other candidate and needs `NeedDeps`, which loads metadata for every transitive dependency — the whole
  standard library included — to answer a question the initial packages already answer.
- WHY: a file is a node only if the walk enumerates it **and** the toolchain puts it in the build. The
  intersection is what makes both halves' knobs work: folder exclusions are the walk's and build constraints
  are the toolchain's, and `CheckOptions.BuildTags` already promised that a file a constraint excludes is
  absent from the graph. The consequence is deliberate: on a Mac, a `_windows.go` file is not a node.
- WHY: `packages.Load` is asked for `NeedName | NeedFiles | NeedForTest` and nothing else — no types, no
  syntax. Imports are read from the files this library enumerated, with `go/parser` in `ImportsOnly` mode,
  rather than from `pkg.Syntax`: `Syntax` lines up with `CompiledGoFiles`, which for a cgo package is
  toolchain-generated output in the build cache and for a test binary is a synthesized `_testmain.go`, so
  parsing it would mean mapping generated files back onto project nodes. Parsing the enumerated files
  instead means the set of files parsed is exactly the set of nodes, and it makes "a file that fails to
  parse is skipped, not fatal" precise: `ExtractImports` returns the imports it reached alongside the error,
  the extractor uses them, and the file keeps its self-edge.
- WHY: `NeedForTest`, and a package with `ForTest` set contributes nodes but no targets. With
  `IncludeTestFiles` on, the toolchain reports a package twice — once as everyone else sees it and once built
  with its own test files — and without that distinction `main.go` importing `internal/api` would get an edge
  to `internal/api/handler_test.go`, which no build outside that test binary contains.
- WHY: an import of one of the project's own packages whose every file the walk excluded yields no edge at
  all, rather than an external edge to its path. It is the project's own code, so calling it an external
  module would fire every "should not depend on external modules" rule on `build/` and `vendor/`. The package
  path is therefore recorded even when no file of it survives — that key's presence is what tells the
  project's own code from somebody else's.
- WHY: `BuildTags` and `IgnoredImportKinds` join `SourceOptions` rather than getting a `GraphOptions` bag of
  their own. One bag for the whole EXTRACT stage means a caller resolves "nil means defaults" once and hands
  the same value to `ExtractSourceFiles` and `ExtractGraph`, and `CheckOptions` keeps one translator method
  instead of two overlapping ones. The struct doc says which fields bear on the walk and which on the graph.
- WHY: `import "C"` is not special-cased; it becomes an external edge to `C`. It is a cgo directive rather
  than a package, but a file that uses it does depend on C code, and a branch for it would be a hidden
  matching rule of exactly the kind the glob-compiles-in-one-place rule exists to prevent.
- WHY: `resolveProjectRoot` was lifted out of `ExtractSourceFiles` unchanged and is now called once, in
  `ExtractGraph`, for both halves. The walk and the toolchain each turn the paths they get back into
  identifiers relative to the root, and those two sets of identifiers are then matched against each other, so
  resolving it twice would be two chances to disagree — on macOS a `t.TempDir` root under `/var` is one
  `EvalSymlinks` away from `/private/var`, and either answer works as long as both halves have the same one.
- WHY: file stems are `extract_graph.go` and `extract_imports.go`. This is the issue the sibling stem
  `extract_graph` was being saved for since issue #1.
- WHY: `golang.org/x/tools v0.49.0` is the library's first non-standard-library dependency, added for
  `go/packages`. The issue asks for it by name and `.golangci.yml`'s depguard already allows exactly
  `golang.org/x/tools/go/packages`, so the module boundary was drawn before the code was; `x/mod` and
  `x/sync` come with it as indirect requirements. It is the only import of it in the library, and it lives
  behind `loadProjectBuild` — nothing above extraction knows the toolchain exists.
- WHY: still no fluent-API integration test — no builder chain exists yet, as in issues #1 to #7. The level
  above the unit tests is `TestExtractGraphExtractsThisRepository`, which locates and extracts this
  repository the way a check will and reads real edges out of the result — `common/fluentapi/check_options.go`
  to two files of `common/extraction`, `common/extraction/extract_graph.go` to
  `golang.org/x/tools/go/packages` as external — plus `TestExtractImportsReadsThisFilesOwnFile`, on a file of
  this package.
