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

## Issue #9 — Extraction: classify internal vs external dependencies

- WHY: the issue's literal ask — set `External`, keep the raw target — already landed with issue #8, which
  could not resolve an import without deciding it. So this issue is the one that gives the decision a name
  and a file of its own (`targetIndex.classify` in `classify_target.go`, unit-tested against a hand-built
  index with no toolchain run), and closes the one hole the toolchain's answer alone cannot: an import path
  under the project's own module that `go list` reported no package for — `<module>/internal/nope`, a package
  half-written, renamed or deleted, or a folder holding no Go source — was being classified as an external
  module, so every rule about third-party dependencies fired, naming the project's own path as the offender.
  It is now the project's own code with no node to point at, and yields no edge, exactly like the
  walk-excluded package of issue #8.
- WHY: "keep the raw module name as the target" is read as the raw *import path*, unaltered — the target of
  an external edge is still `golang.org/x/tools/go/packages`, not the module `golang.org/x/tools`, and no
  `Module` field joined `Edge`. Three reasons: the siblings keep the import specifier as the file wrote it
  and in TypeScript that specifier *is* the package name, so the import path is the faithful port; trimming
  it would throw away which package of a module was imported, which `in path`-style rules want; and a
  module-wide rule already reads naturally as `golang.org/x/tools/**`, since issue #2 made a trailing `/**`
  match the folder itself as well as everything under it.
- WHY: the module's own path comes from the toolchain — `packages.NeedModule` and the first package with
  `Module.Main` — rather than from parsing `go.mod` with `golang.org/x/mod/modfile`, which issue #7's note
  anticipated and `.golangci.yml`'s depguard already allows. It is the same `go list` invocation that
  reported the package paths, so the two answers cannot disagree; it stays right when the toolchain was
  pointed at another module file (`-modfile`, a workspace); it needs no second dependency; and `NeedModule`
  adds no work, the field is in the JSON `go list` already prints. `x/mod` stays indirect.
- WHY: a module nested inside the project is external even when it declares a path under its parent's, and a
  `go.mod` between the project root (exclusive) and the folder an import names is what decides it. Such a
  module is versioned, distributed and resolved on its own, none of its files are in this project's build,
  and issue #8 already documented it as external. It is the only question in the classification the
  filesystem rather than the toolchain answers.
- WHY: the consequence, and it is a deliberate trade: a *separate* module whose path is under the main
  module's but whose source is not inside the project — a `/v2` published from another repository, or a
  `replace` pointing outside — is now read as the project's own code with no node, so its edges are dropped
  instead of appearing as external. Telling that case from a missing package needs the `require` list out of
  `go.mod`, which is a second source of truth about the project for a rare layout; the common case it would
  buy is the exotic one, while the case it costs — an import of a package that is not there — happens in any
  refactor.
- WHY: an import path that is under the module path but climbs out of the project (`<module>/../elsewhere`)
  is external, not owned. It is not legal in a module, but a file may write it, so the suffix is normalised
  before it is used as a folder — which is also what keeps the nested-module search inside the project.
- WHY: file stem is `classify_target.go`, one concept, as in issues #1, #4, #6 and #7 — no sibling stem names
  this step. `targetIndex` and `classify` are unexported: the classification is a step inside `ExtractGraph`,
  the `External` flag on `Edge` is its whole public result, and downstream rules read that flag.
- WHY: still no fluent-API integration test — no builder chain exists yet, as in issues #1 to #8. The level
  above the unit tests is three extractions: two fixture projects, one whose import points at a package that
  is not there and one holding a nested module, plus
  `TestExtractGraphClassifiesThisRepositoriesDependencies`, which extracts this repository and asserts that
  no external target is this library's own module path under another name.

## Issue #10 — Extraction: self-edges and parallel-edge merging

- WHY: both halves of the issue's literal ask already landed — `SelfEdge` and `NewGraph`'s merge with issue
  #1, `ExtractGraph` emitting one self-edge per node with issue #8 — so this issue is the one that pins them,
  the same way issue #9 pinned a classification issue #8 could not resolve an import without making. What it
  found is that the *shape* of a self-edge was only guaranteed for edges built by `SelfEdge`: a file that
  imports its own package resolves, via the package's own file list, to an edge to itself, so
  `internal/api/handler.go -> itself [plain]` was landing in the graph beside self-edges carrying nothing.
  Verified before the fix, and four of the new tests fail without it. Two shapes of self-edge is a real
  hazard because downstream code drops a self-edge on `Source == Target` alone and never reads the other two
  fields, so the difference would only ever surface as a report disagreeing with itself.
- WHY: the fix is `Edge.canonical()`, called by `NewGraph` in place of the inline struct it used to build, so
  the canonical form is established in the same one place the identifiers are normalised and the parallel
  edges merged — not in `ExtractGraph`, which is only one of the ways edges reach a graph. An edge from a
  node to itself becomes exactly `SelfEdge(node)`: no import kinds, not external. Importing your own package
  is illegal Go, so nothing is lost; the file's edges to the *other* files of that package are real
  dependencies and are kept.
- WHY: `Edge.merge`'s union of `External` was deliberately left alone, though canonicalising the self-edge
  before merging is now what stops a merge from re-externalising one. Issue #1 documented that union and
  `TestEdgeMergeUnionsExternality` covers it; the only remaining way to reach it is an external import path
  that spells a node identifier exactly, which is not worth reopening a passing decision for.
- WHY: `Graph.SelfEdges()` and `Graph.Dependencies()` are the issue's second sentence — "projections filter
  self-edges out by default; node projection depends on them" — given a name at the level that owns the
  invariant. `common/projection` does not exist yet and inventing a `MapFunction` here would be issue #11's
  work; these two are queries on `Graph`, not projections, and they are what a projection will be written
  over. They are documented and tested as a partition, and `nodes.Add(dependencies...)` rebuilds the graph.
- WHY: no `Graph.Files()` beside them. `Nodes()` already lists every identifier a graph mentions, external
  import paths included, and the project's own files are `SelfEdges()` read for their `Source` — a third
  overlapping way to list nodes is what the empty-test guard would then have to choose between.
- WHY: both new methods inherit the receiver's order instead of going back through `NewGraph`. A subsequence
  of a graph sorted by (source, target) is sorted, and a subset of a set with unique keys has unique keys, so
  re-merging would be a map round-trip that can only reproduce its input — and `Dependencies()` deliberately
  does *not* satisfy the self-edge-per-node property, which is `ExtractGraph`'s promise rather than
  `Graph`'s. Both doc comments say so.
- WHY: no new file. `canonical` sits in `edge.go` next to `SelfEdge` and `merge`, the two things it makes
  honest, and the two views sit in `graph.go` next to `Nodes` and `Find`; no sibling stem names either, and
  splitting one invariant across two files is how it stops being read as one.
- WHY: still no fluent-API integration test — no builder chain exists yet, as in issues #1 to #9. The level
  above the unit tests is four extractions of real projects:
  `TestExtractGraphGivesEveryNodeOfEveryEdgeASelfEdge` (every node an edge mentions is in the self-edges,
  and no external target is), `TestExtractGraphMergesAFilesImportOfItsOwnPackageIntoItsSelfEdge` (the case
  above, end to end), `TestExtractGraphMergesParallelImportsOfOneOfItsOwnPackages` (the internal half of the
  merge — two imports of a two-file package are four edges before it and two after) and
  `TestExtractGraphSplitsIntoOneNodePerFileAndTheDependenciesBetweenThem`.

## Issue #11 — Extraction: graph cache and clear-graph-cache

- WHY: the memo is a new function, `CachedGraph(root, options)`, in front of `ExtractGraph` rather than
  caching inside `ExtractGraph` itself. `ExtractGraph` is documented and tested as "runs the Go toolchain
  once over the whole project", and every existing test calls it; making that one function stateful would
  have changed what a dozen passing tests mean. The two doors now read as what they are — the extraction,
  and the memo a check goes through — and `ExtractGraph`'s doc points at the memo.
- WHY: the cache key is built from the **resolved project root**, not from the `ProjectLocator` the issue
  names. A locator is a starting point for an upward search, so `nil` (the working directory), the project
  root and a directory three levels inside it are three spellings of one project; keying on the locator
  would extract it three times and lose the point of the cache. `resolveProjectRoot` runs before the key is
  built, which also makes a relative root and a symlinked one one entry — and it is the same resolution
  `ExtractGraph` does internally, so the two cannot disagree.
- WHY: `graphCacheKey` sorts the folder exclusions and the build tags before quoting them. Neither list's
  order changes what is extracted — an exclusion list is a set and `-tags=a,b` is the same build as
  `-tags=b,a` — so an order-sensitive key would only ever miss. It resolves the options first, so a nil bag,
  the zero bag and one whose exclusions are nil are one key, while a caller who excluded nothing on purpose
  keeps their own; every string is `%q`, so a folder named `a b` cannot forge a boundary between entries.
- WHY: the "hard to forget" half of the issue is a test, not only a named function. `graph_cache_test.go`
  reflects over `SourceOptions`, varies each field in turn and fails if the key does not move — so a field
  added to that bag without reaching the key fails a test rather than passing review, and a field of a kind
  the test cannot vary fails loudly instead of silently passing.
- WHY: a failed extraction is not cached, and the graph handed out is a copy. An extraction fails because of
  the environment, so the next rule deserves the same chance rather than a memoised error; and a `Graph` is
  a slice, so handing out the cached one would let any reader write through into every later reader's graph.
  Both are covered by tests that fail without them.
- WHY: the lock is not held across the extraction. Two goroutines asking about one project at once extract
  it twice and store the same answer, which costs one redundant run in a case a synchronous test suite does
  not reach, while locking for the whole extraction would make one project's rules wait on another
  project's toolchain run.
- WHY: `CheckOptions.ClearCache` is honored in one new method, `CheckOptions.ExtractGraph(locator)`, which is
  the same kind of translator as issue #7's `CheckOptions.SourceOptions()` and the only edit outside the
  issue's package. Without it every terminal would locate, clear and extract in three steps of its own, and
  the one that forgot the middle step would silently ignore the user's flag. It clears the whole cache rather
  than this project's entry: the reason to clear is that the source moved, and one project is cached once per
  set of options it was asked about. It clears *before* the lookup rather than instead of it, so the check
  that cleared still fills the cache for the rules after it in the suite.
- WHY: the public global is `extraction.ClearGraphCache()`, and there is no root-package re-export of it yet.
  `archunit.go` is the public surface, it does not exist, and creating it for one function would be claiming a
  deliverable another issue owns; `ClearGraphCache` is exported from a public package in the meantime, and
  re-exporting it is one line when that surface lands.
- WHY: file stem is `graph_cache.go`, one concept, as in issues #1, #4, #6, #7 and #9 — no sibling stem names
  this step. The cache type is unexported (`graphCache`), with the two exported functions over one
  package-level instance: a cache exists to be shared by every rule in a suite, and rules are built and
  checked independently of each other, so there is nothing for it to hang from. That global carries the
  `//nolint:gochecknoglobals` the harness sanctions, with its reason on it.
- WHY: still no fluent-API integration test through a builder chain — none exists, as in issues #1 to #10.
  The level above the unit tests is `common/fluentapi`'s four new tests, which do what a terminal will do
  with a rule (`options.ExtractGraph(rule.locator)`) over a fixture project that gains a file between two
  checks: the second check shares the first one's graph, `ClearCache` reads the source again,
  `IncludeTestFiles` is a different analysis rather than a cache hit, and a locator naming no project is a
  `UserError`.

## Issue #12 — Extraction: per-line ignore directive

- WHY: the Go spelling is `//archunit:ignore`, not the issue's `# archunit: ignore`. Every directive comment
  in this language is a `//` line with no space after the slashes and a `tool:verb` word — `//go:build`,
  `//go:generate`, `//nolint:gochecknoglobals` — and gofmt is what enforces that shape, so a directive
  written any other way would be reformatted out from under the user. The parser is lenient about the
  whitespace it accepts, so `// archunit: ignore` transliterated straight from a sibling port still works,
  but the canonical form is the one the language uses. A `/* */` comment is never a directive: it cannot be
  the last thing on the line an import is on, so accepting one would accept a spelling every other Go tool
  disagrees about.
- WHY: "optionally scoped to named modules" is implemented as *named scopes on the check*, end to end: a
  bare directive is honored by every analysis, and `//archunit:ignore layers` is honored only by a check
  whose `CheckOptions.IgnoreScopes` names `layers`. The other reading of the issue — naming the modules the
  ignore applies to — does not arise in Go, where one import spec is one import path and the directive is
  already on that line; there is nothing left to name. Scopes follow the `IgnoredImportKinds` precedent and
  are wired through today rather than being a field nothing reads.
- WHY: an unanswered or misspelled scope leaves the import **in** the graph. That is the loud direction: the
  rule then reports a dependency the user thought they had suppressed, whereas honoring an unknown scope
  would silently swallow real violations for a typo.
- WHY: the scopes live on `IgnoreDirective` as one comma-separated `Scopes string`, not a `[]string`.
  `ImportInfo` has to stay comparable — `extract_imports_test.go` compares imports with `!=`, and the
  directive is a field of it — which is the same reason `Edge` carries an `ImportKindSet` bit set instead of
  a list. `Names()` is the accessor that hands back the list.
- WHY: the directive is *not* read from `ast.ImportSpec.Doc` / `.Comment`. Both are empty for shapes a user
  will actually write — a trailing comment on a lone `import "fmt" //archunit:ignore`, and a trailing
  comment inside a block when the next line is also a comment — so `newIgnoreDirectives` indexes every
  comment by the line it is on, marks the lines that hold code, and answers "trailing" and "directly above"
  from positions. Both blind spots are cases in `ignore_directive_test.go`.
- WHY: a directive belongs to one import, never to a block or a file. It counts when it trails the import's
  own line or sits on a comment line above it, with a blank line or a line of code ending the block; a
  directive above `import (`, or beside the opening parenthesis, applies to nothing. Leaving a whole file
  out is what `SourceOptions.ExcludedFolders` is for, and a directive that quietly ignored a block would be
  the kind of invisible suppression the previous note avoids. It does reach across an ordinary comment
  above the import, so a reason can be written above the directive; and a second `//` on the directive line
  starts a reason, so prose is never read as a list of scope names.
- WHY: `CheckOptions.IgnoreScopes` is the fourth knob translated in `CheckOptions.SourceOptions()`, and it
  sits with the language-neutral knobs rather than the Go-specific ones: the directive convention is a
  family convention, and only its comment syntax is Go's. It reaches `graphCacheKey` too — two checks
  answering to different scopes are two analyses of one project, and the cache would otherwise hand the
  second one the first one's graph.
- WHY: file stem is `ignore_directive.go`, one concept, as in issues #1, #4, #6, #7, #9 and #11 — no sibling
  stem names this convention. The parser and the per-file index live there with the type; the *decision* to
  honor a directive is `SourceOptions.IgnoresImport`, next to `IgnoresImportKind`, because both are the same
  question the extractor asks of one import.
- WHY: still no fluent-API integration test through a builder chain — none exists, as in issues #1 to #11.
  The level above the unit tests is `common/fluentapi`'s two new tests, which do what a terminal will do
  (`options.ExtractGraph(locator)`) over a fixture whose file marks two of its imports: the bare directive
  is honored by a default check, the scoped one only by the check that answers to its name.
- WHY: the per-file index reads `fileSet.PositionFor(position, false)`, not `fileSet.Position(position)`.
  `Position` applies `//line` directives, and a column-1 one is legal inside an import block — generated Go
  (goyacc, committed cgo output) has them. Every question the index answers is about *physical* adjacency:
  trailing this import, the line above holds code, a blank line ended the block. Rewritten numbering can
  shift or collide two segments, losing a directive or attaching one to an import the file never marked;
  `TestExtractImportsReadsDirectivesByPhysicalLine` is the file where the adjusted lines swap the two.

## Issue #13 — Projection: project edges, nodes and cycles

- WHY: `MapFunction` is `func(extraction.Edge) (MappedEdge, bool)`. The data model's "`Edge -> MappedEdge |
  nothing`" has no union type in Go, and `(value, ok)` is how every optional answer in the language is
  spelled — a `*MappedEdge` would put a pointer to a two-string struct on the heap for every edge of the
  graph and make "nothing" a nil dereference away.
- WHY: `ProjectEdges` drops an edge whose two **labels** are equal, not one whose raw `Source == Target`.
  That is the data model's "projections filter self-edges out by default" applied after relabelling
  rather than before, and the difference is the whole point of a slicing projection: two files of one
  slice depending on each other is a real file-level dependency and no dependency at all between slices,
  so the drop has to be by label. It subsumes the raw self-edge, because any label that is a function of
  the identifier gives a self-edge two equal labels.
- WHY: `ProjectToNodes(graph, mapper)` takes the graph and the hook, not the `[]ProjectedEdge` a reader
  of the sibling ports might expect. Node projection is the half that "depends on the self-edges", and by
  the time `ProjectEdges` has returned they are gone — so a signature over projected edges would silently
  lose every file that depends on nothing, which for a rule about naming or placement is the entire
  population. Both functions go through one unexported `projectAll`, so the `MapFunction` is called, and
  the merge decided, in exactly one place.
- WHY: a projected self-edge appears in neither `Incoming` nor `Outgoing`: it names the node and is not a
  dependency, so every edge reachable from a `ProjectedNode` is an edge `ProjectEdges` returned too. The
  edges *inside* a slice therefore have no public door yet — when a cohesion metric wants them,
  `projectAll` is the door to open, and inventing an options bag for it before that issue lands would be
  a knob nothing reads.
- WHY: `ProjectCycles` returns one entry per **strongly connected component**, not one per simple cycle,
  and each entry holds every projected edge between labels of that component. A component of five labels
  can hold dozens of simple cycles and enumerating them is exponential, while every edge inside a
  component provably lies on one — for `a -> b` inside it there is a path from `b` back to `a` — and
  breaking any single one of them breaks the component. So the component is what a report names, and its
  labels are the source labels of its edges — inside a component every label has an outgoing edge that
  stays in it, so an assertion reading them off misses none.
- WHY: a projected self-edge is not a cycle. `a -> a` is a strongly connected component of one label and
  is skipped with every other single-label component, which keeps one convention across the whole PROJECT
  stage: a node depending on itself is not a dependency, so it cannot be a cyclic one either.
- WHY: `cycles` is a subpackage of `projection` (`AGENTS.md`'s `projection/ ... plus cycles/`) and
  `tarjan_scc.go` is the sibling stem. Tarjan runs on labels and sorted successors, and `visit` recurses
  by name off a `tarjanSearch` receiver rather than closing over eight locals; sorted input is what makes
  the components a function of the projection alone, which a reproducible report needs. Recursion depth is
  the longest path in the projection, well inside Go's growable stacks.
- WHY: `ProjectedEdge` and `ProjectedNode` keep unexported fields behind accessors, unlike
  `extraction.Edge`'s exported ones, and `CumulatedEdges`/`Incoming`/`Outgoing` clone on read. Both types
  carry slices, so exported fields would let a reader write through into a value that has already been
  reported — the same reason `matching.Filter` hides its exclusions and `graphCache` hands out copies.
- WHY: `NewProjectedEdge` puts its raw edges through `extraction.NewGraph`. It copies them away from the
  caller's slice, and it means the cumulated edges of a projection have the kernel's one edge order and
  one self-edge shape rather than a second ordering invented here.
- WHY: labels are used verbatim — no `NormalizeIdentifier`. A label is the vocabulary the rule speaks, and
  a layer called `API` is not a path; identifier-shaped labels arrive already normalised from the
  extractor. An empty label drops the edge, which is the same defence `NewGraph` applies to an edge
  without an identifier.
- WHY: a nil `MapFunction` projects nothing rather than defaulting to `PerEdge` or returning a
  `UserError`. Projection is pure and has no `error` to travel in, and an empty projection is exactly what
  `GatherEmptyTestViolations` was built to report — the loud direction, where guessing at `PerEdge` would
  quietly judge a rule against a vocabulary nobody asked for.
- WHY: two `per <thing>` factories, `PerEdge` and `PerInternalEdge`, and no `PerExternalEdge`. A rule
  about third-party modules is a rule about which edges *leave* the project, and `PerEdge` keeps them with
  `extraction.Edge.External` still on the raw edges the projection cumulates; a third factory would be a
  second place deciding what external means. `PerInternalEdge` is written over `PerEdge` so the identity
  labelling is stated once.
- WHY: file stems are `project_edges.go` (the sibling stem, and where the package doc lives),
  `map_function.go`, `project_nodes.go`, `cycles/project_cycles.go` and `cycles/tarjan_scc.go`. The hook
  and its factories are one concept and share a file; `project_nodes.go` is the stem for a function the
  issue names `project to nodes`, because `ProjectToNodes` is the English and `project_to_nodes.go` is not
  a stem any sibling has.
- WHY: still no fluent-API integration test — no builder chain exists, as in issues #1 to #12. The level
  above the unit tests is three projections of this repository, extracted the way a check will do it:
  `TestProjectEdgesProjectsThisRepositoryByFolder` (the `common/projection -> common/extraction` folder
  dependency, and the raw edges under it naming the two files that make it),
  `TestProjectToNodesGivesThisRepositoryOneNodePerFolder` (every folder is a node, `common/matching`
  included, which depends on nothing of the library's own) and
  `TestProjectCyclesFindsNoCycleAmongTheFoldersOfThisRepository`, which is this library dogfooding the
  rule the cycle projection exists to serve.

## Issue #14 — Projection: the per-edge MapFunctions

- WHY: `PerEdge` **changed meaning** and now drops the self-edges, which issue #13's version kept. The
  issue names four factories and spells `per edge` out as "everything except self-edges", and that
  parenthetical is what makes `identity` a fourth function rather than a second name for the same one —
  the only difference available between the two is the self-edge. So `Identity` is what issue #13 shipped
  as `PerEdge` (every edge, labels verbatim, self-edges and external edges included), and the three
  `per <thing> edge` factories are strictly nested inside it. Nothing else here has two names for one
  behaviour, and the issue that owns these factories is the place to reconcile that.
- WHY: the family's one rule is therefore "every `per <thing> edge` factory is about *dependencies*, so
  none of them keeps the edge that only names a node" — `PerExternalEdge` ⊆ `PerEdge` ⊆ `Identity`, and
  `PerInternalEdge` ⊆ `PerEdge` too. The alternative was to have `PerInternalEdge` keep the self-edges
  (a self-edge is internal by construction, so it partitions the whole graph with `PerExternalEdge`
  rather than only its dependencies), and that is the trap: a reader who has been told `per edge` drops
  self-edges will assume `per internal edge` is a subset of it. Nesting is statable in one sentence, so
  nesting won.
- WHY: the consequence, and it is the whole cost of the change: `ProjectToNodes` wants `Identity`, not
  `PerEdge`. Node projection is the half written over the self-edges, so projected through any
  `per <thing> edge` factory it loses every file that depends on nothing — for a rule about naming or
  placement, most of the population. `project_nodes.go`'s doc says so in as many words and
  `TestOnlyIdentityGivesAFileThatDependsOnNothingANode` fails if either half of the split is undone.
  Seven `ProjectToNodes(…, PerEdge())` calls in the existing tests moved to `Identity()`; their
  assertions are unchanged.
- WHY: `Identity` therefore puts every external module in a node projection as well, so "the project's
  own files as nodes" has no factory in `common` — it is `PerInternalEdge` plus the self-edges, which is
  neither of the two, and inventing a fifth factory for a rule that has not landed would be the
  speculative knob the earlier notes keep refusing. When the file-level node rules land, that mapper is
  `files/projection`'s to write, which is what a module's own projection package is for.
- WHY: `PerExternalEdge` exists now, reversing issue #13's note that said it would not. That note's
  reasoning — `PerEdge` keeps the external edges and `extraction.Edge.External` is still on the raw ones,
  so a second factory would be a second place deciding what external means — is answered by writing it
  as the exact complement of `PerInternalEdge`: both read the one flag the extractor set, neither
  re-decides anything, and `TestPerInternalEdgeAndPerExternalEdgePartitionWhatPerEdgeKeeps` is what
  holds them to being two halves of one whole. Making a rule about third-party modules filter
  `PerEdge`'s output by hand was the thing that would have spread the decision.
- WHY: all four are factories returning a `MapFunction` rather than functions of the right signature
  that a caller could pass directly. `PerEdge` as a plain `func(extraction.Edge) (MappedEdge, bool)`
  would read better at a call site by one pair of parentheses, but a `slice by <thing>` factory has to
  take arguments, so the family would then be spelled two ways; and the factory form is where a knob
  lands if one of them ever needs one.
- WHY: the tests for the factories moved out of `project_edges_test.go` into `map_function_test.go`,
  beside the file they test, as `AGENTS.md` asks. The four are one table with the predicate each is
  supposed to satisfy, so the family's rule above is checked in a loop rather than four times by hand —
  and the table is a function returning the slice rather than a package-level `var`, which needs no
  `//nolint:gochecknoglobals` (the linter is excluded in `_test.go` files, so the directive would itself
  be a `nolintlint` finding).
- WHY: `TestProjectEdgesDropsTheGraphsSelfEdges` now projects through `Identity`. It is a test about
  `ProjectEdges` dropping an edge whose labels are equal, and `PerEdge` no longer hands it one to drop —
  through `PerEdge` it would have passed without exercising anything.
- WHY: no new file. The four factories join the hook in `map_function.go`, for the reason issue #13 gave
  for putting the first two there: the hook and its factories are one concept.
- WHY: still no fluent-API integration test — no builder chain exists, as in issues #1 to #13. The level
  above the unit tests is `TestTheMapFunctionsSplitThisRepositoriesDependencies`, which extracts this
  repository the way a check will and projects it through each factory in turn: the external half holds
  `common/extraction/extract_graph.go -> golang.org/x/tools/go/packages` and nothing internal, the
  internal half holds `common/projection/map_function.go -> common/extraction/edge.go` and nothing that
  leaves the project, and the identity projection has a node for every file the graph gives a self-edge.

## Issue #15 — Projection: cycle detection — Tarjan and Johnson

- WHY: the Tarjan half already landed with issue #13 (`tarjan_scc.go`, and `ProjectCycles` over it), so
  this issue is the Johnson half — and it **reverses** issue #13's note, which said the elementary
  circuits would not be listed because listing them is exponential. That reasoning is sound and it is
  answered rather than overruled: the enumeration is bounded (see below), so the exponential case ends in
  a truncated answer that says it is truncated instead of in a test suite that never finishes.
- WHY: `ProjectCircuits` is therefore a **second door beside `ProjectCycles`, not a replacement for it**.
  They answer one question at two resolutions — which parts of the graph are cyclic, and which cycles
  there are — and both are wanted: the component is linear and so always safe to ask for, while the
  circuits are what a report names (`api -> db -> api` is the sentence; the component is a haystack of
  every edge around it). Replacing `ProjectCycles` would also have rewritten the meaning of a dozen
  passing tests, which the scope rule forbids. The package doc now names both doors and each function's
  doc points at the other.
- WHY: the vocabulary splits on Johnson's own word. A `Circuit` is one elementary circuit and a "cycle"
  stays the strongly connected component `ProjectCycles` returns, so the two are nameable in one sentence
  and neither name has to grow an adjective. The fluent API's `should have no cycles` is unaffected —
  that is a mood plus a predicate, and it will read whichever of the two it wants.
- WHY: `ProjectCircuits` returns `([]Circuit, bool)` and is **bounded by default** —
  `CircuitOptions.MaxCircuits`, defaulting to `DefaultMaxCircuits` = 1000, negative meaning unbounded.
  The count of elementary circuits is exponential in the size of a component (twenty labels that all
  depend on each other hold more than 10^17 of them), so an unbounded enumeration is not something a
  pure function called from a unit test can promise. The bool is `complete`, in the `(value, ok)` shape
  `MapFunction` already chose: a silent cap would read as "these are all your cycles", which is the
  quiet direction this library keeps refusing. A rule only needs to know that there is one, so `false`
  costs it nothing; a report built from a truncated answer can say so.
- WHY: `MaxCircuits` is the bag's only knob. A `MaxLength` beside it is the obvious second one and
  nothing asks for it yet — that is the speculative knob the earlier notes keep declining.
- WHY: Johnson is implemented with the per-starting-label restriction from the paper, which calls
  `tarjanSCC` again on the shrinking subgraph (`strongComponentOf`). That is what makes the enumeration
  report each circuit exactly once rather than once per rotation, and it is why `ProjectCircuits` can
  hand `johnsonCircuits` a whole component and get the circuits *inside* it: the search restricts itself
  further at every start. The outer decomposition into components is kept anyway — it is the issue's
  structure, and it keeps each start's Tarjan run on a smaller graph.
- WHY: `blockedOn` (Johnson's `B`) is a `map[string][]string` with an append-if-absent, not a set of
  maps. The unblocking order decides which branches the search takes next and so which circuits come out
  first — and *which ones a truncated enumeration keeps*. Go's map iteration order is deliberately
  random, so a set would have made a truncated report unreproducible;
  `TestJohnsonCircuitsIsAFunctionOfTheGraphAlone` runs a limited enumeration eight times for that reason.
- WHY: `cyclicComponents` was lifted out of `ProjectCycles` unchanged, which is the only edit to code
  that was already passing its tests. Both doors need "the components of two or more labels, in a
  reproducible order", and stating that rule twice is how the two doors would drift apart. No behaviour
  changed; `ProjectCycles`' own tests are untouched.
- WHY: no `NewCircuit` constructor, though `NewProjectedEdge` beside it is exported. A circuit is not
  three independent fields but a closed chain, and a hand-built one that is not closed would be a value
  the type promises cannot exist. An assertion's fixture is a hand-built projection put through
  `ProjectCircuits`, which is the shape a rule sees anyway.
- WHY: file stems are `johnson_circuits.go` for the pure algorithm over labels and `project_circuits.go`
  for the public `Circuit`/`CircuitOptions`/`ProjectCircuits` over projected edges — the same split as
  `tarjan_scc.go` and `project_cycles.go`. No sibling stem names Johnson; `<algorithm>_<what>` is the
  shape `AGENTS.md` gives for `tarjan_scc`, and Johnson's paper is titled "Finding all the elementary
  circuits of a directed graph".
- WHY: the unit tests check Johnson against `naiveCircuits`, a short exponential enumerator in the test
  file, on six fixtures — rather than against hand-counted expectations only. The correctness claim
  here ("every elementary circuit, exactly once, no rotations") is the kind that a hand-written
  expectation can agree with while being wrong in the same way, and the naive walk is slow enough to be
  obviously right. The hand-counted cases are kept beside it, including the complete graph on four
  labels, whose 20 circuits are a closed form.
- WHY: one fixture, `"a label blocked on twice"`, was found by brute-force enumerating every directed
  graph on five labels and looking for the one that makes the search record a dead end against the same
  blocked label twice. Nothing smaller reaches it, and without it the append-if-absent in `blockOn` was
  the one line in the package no test executed.
- WHY: the transitive half of the blocking rule has its own fixture, `releasedDeadEndPairs` (a <-> b,
  b <-> d, a -> c -> d), because `deadEndAdjacency` does not reach it: there every release is `unblock`'s
  own `s.blocked[label] = false`, so deleting `blockOn` or `unblock`'s recursion changed no answer. In the
  new one, d is blocked on the root's first branch and the root's second branch needs it released, so both
  deletions lose `a -> c -> d -> b -> a` — verified by making each mutation in turn.
- WHY: `TestJohnsonCircuitsIsAFunctionOfTheGraphAlonePastADeadEnd` runs *both* dead-end fixtures rather
  than moving to the new one. Only the new one has a `blockedOn` list whose contents reach the answer,
  which is what the test is about — but the chain fixture is the case where a label is blocked and released
  twice over, and dropping it would have traded one kind of coverage for the other.
- WHY: `TestProjectCircuitsIsBoundedByDefault` runs on the complete projection over seven labels (2365
  elementary circuits) through `ProjectCircuits(edges, nil)`, and `TestCircuitOptionsWithDefaults` spells
  the default out as `1000` instead of naming `DefaultMaxCircuits`. Both are the same point: a test that
  compares against the constant under test agrees with any value it is given, and a test that passes its
  own limit never walks the default path. `completeProjection` is `completeAdjacency`'s twin one layer up,
  over projected edges rather than an adjacency.
- WHY: still no fluent-API integration test — no builder chain exists, as in issues #1 to #14. The level
  above the unit tests is two extractions of this repository, projected the way a check will:
  `TestProjectCircuitsFindsNoCycleAmongTheFoldersOfThisRepository` and
  `TestProjectCircuitsFindsEveryFileLevelCycleOfThisRepository`, the second at the vocabulary a file-level
  `should have no cycles` rule will use.

## Issue #16 — Files API: entry point and selectors

- WHY: `archunit.go` exists now, which is another issue's deliverable in name. An entry point that cannot be
  reached from the public surface is not an entry point: `AGENTS.md`'s own first example and
  `common/extraction/locate_project.go`'s doc comment both say `archunit.ProjectFiles(...)`, and there is
  nowhere else for the integration test to run through. It is kept to what this issue needs — the two files
  entry points, the three type aliases a user needs to name a rule (`ProjectLocator`, `CheckOptions`,
  `FilesBuilder`) and issue #11's parked one-line re-export of `ClearGraphCache`. Everything else the surface
  will re-export still belongs to the issue that builds it.
- WHY: the locator is one nilable parameter, `ProjectFiles(locator *extraction.ProjectLocator)`, not a
  variadic and not two constructors. Go has no optional parameter, and this is the spelling
  `locate_project.go` has documented since issue #7 (`archunit.ProjectFiles(nil)`); it is also the same
  nil-means-defaults contract as `*CheckOptions` and `*SourceOptions`, so a reader learns it once.
- WHY: the two `govet.unusedresult.funcs` entries the config parked for this moment are uncommented —
  `…/ArchUnitGo.ProjectFiles` and `.Files`. Verified empirically that a bare `archunit.ProjectFiles(nil)`
  statement is now reported; the entries are package-level functions, which is the only thing that flag can
  guard, as the comment above them says. No chain method was added and no default was removed.
- WHY: what the selectors *mean* lives in `files/projection.SelectFiles(graph, selectors...)`, not in the
  builder. It is the module's half of the PROJECT stage, it is pure, and it is what lets the meaning of
  `project files, in folder "internal/**", with name "*.go"` be tested against a hand-built graph. The AND
  the issue asks for is stated there, once, so no coming predicate re-derives it.
- WHY: `SelectFiles` reads `Graph.SelfEdges()` for the file population rather than `Graph.Nodes()`. Issue
  #10's note already named this: the project's own files are the self-edges read for their `Source`. It is
  what keeps an import path the project depends on unselectable — `in path "**"` cannot select `fmt` — and
  what keeps a file that depends on nothing selectable. Three tests fail if the two are swapped.
- WHY: `FilesBuilder.SelectFiles(options)` is a public method the fluent grammar has no stage for. Without
  it the issue lands as a bag of compiled filters that nothing can resolve, and the SOURCE-and-EXTRACT-plus-
  scope half it performs is what every predicate this module gains will be written over — better one door,
  tested, than the same three steps assembled inside each terminal. It deliberately does *not* wire the
  empty-test guard: selecting nothing is only a failure once a rule judges something, so the guard stays with
  the terminal, which is what `Selectors()` is exported to report with.
- WHY: a pattern a scope verb cannot compile is kept on the builder as a deferred `err` and returned by
  `SelectFiles`, not panicked on and not returned from the verb. A fluent method has nowhere to put an error
  and `.golangci.yml`'s `forbidigo` bans `panic`. This is the wrap point issue #6's note predicted:
  `UserError.Operation` is the verb as the user typed it (`in folder`), `Subject` the pattern, and
  `matching.ErrInvalidPattern` travels as the cause. The first rejection wins — it is the one to fix — and a
  rejected pattern does not join the selectors, because a zero `Filter` matches nothing and the rule would
  then report every file as unselected instead of reporting the typo. The error is returned before the
  project is read.
- WHY: `common/fluentapi` is imported as `kernel` inside `files/fluentapi`. Both packages are called
  `fluentapi` — that is `AGENTS.md`'s per-module shape, and the unaliased import compiles and lints clean —
  but `*fluentapi.CheckOptions` written inside package `fluentapi` reads as a self-reference. `kernel` is the
  word `AGENTS.md` uses for `common/`, and every sibling module's fluent API will want the same alias.
- WHY: `Selectors()` is exported and clones on read; there is no `Err()` beside it. The filters are what
  `assertion.EmptyTestViolation` carries (issue #4 chose `[]matching.Filter` over the pattern strings for
  exactly this reason) and the only way to observe a stored half-built rule; the deferred error has one
  reader, the terminal, which is in this package.
- WHY: `String()` renders the selectors by their own descriptions — `project files, path without filename
  matches "internal/**"` — rather than replaying the verbs the user typed. Keeping the verb would mean a
  parallel field beside every filter, and issue #4 already settled that a `Filter` is the reporting unit
  because it carries both the pattern as typed and the part of an identifier it looks at. A rejected pattern
  is rendered too, so a builder cannot print as the rule the user thought they wrote.
- WHY: the four verbs are the four matchers issue #3 built, and none of them re-decides which part of an
  identifier it looks at: `selecting(verb, pattern, compile)` takes the factory method, so `with name` →
  `FilenameMatcher`, `in folder` → `FolderMatcher`, `in path` → `PathMatcher`, `in file` →
  `ExactFileMatcher`. The `RegexFactory` is a field on the builder rather than one built per verb, so glob
  syntax is chosen once per rule — which is also where `defined by regex`-style syntax choices will hang.
- WHY: no mood, no predicate, no terminal, no `files/assertion`. This issue is the entry point and the
  selectors; there is nothing to judge yet, and `Checkable` arrives with the first predicate.
- WHY: file stems are `files/fluentapi/project_files.go` (the entry point the chain starts at) and
  `files/projection/select_files.go`. No sibling stem names either; `project_files` is the sentence and
  `select_files` is what the projection does, and naming the latter `project_files.go` too would put two
  different concepts under one name in one module.
- WHY: **the first fluent-API integration test in the repository**, which every note since issue #1 has said
  was waiting for a builder chain. `archunit_test.go` dogfoods on this repository through the public surface:
  a chain with no locator selects the files of `common/matching`, `Files` and `ProjectFiles` build the same
  rule, a stored rule is branched from twice (the `AGENTS.md` example, verbatim), `IncludeTestFiles` reaches
  the selection, and a folder no file is in selects nothing rather than everything. `files/fluentapi` has a
  second one against a fixture project on disk, and the unit tests below both run against hand-built graphs.
- WHY: `TestProjectFilesAndFilesAreOneEntryPoint` resolves both names against the fixture project *on disk*
  rather than against `fixtureGraph()`, and `archunit_test.go` gained
  `TestTheLocatorReachesTheProjectThroughEitherEntryPoint`. The locator is an argument nothing else observes
  — `String()` does not render it and a hand-built graph never reads it — so an entry point that dropped it
  would silently analyse the auto-detected project. Verified by making each entry point return
  `ProjectFiles(nil)` in turn: both mutations now fail.
- WHY: `TestABranchDoesNotWriteIntoTheRuleItGrewFrom` branches from a base of *three* verbs. A value receiver
  alone does not make a builder immutable — the copy shares the selectors' backing array — but with one or two
  selectors `append` has no spare capacity, so the aliasing bug is invisible; at three it is, and the test was
  confirmed to fail with `slices.Clone` removed from `selecting`.
- WHY: `ProjectFiles` copies the `*ProjectLocator` it is handed, because a builder that is immutable in
  every stage but the argument it started from is the worse of the two contracts: a caller reusing one
  locator struct to build a rule per directory would otherwise leave every stored rule, and every branch
  of it, pointing at the last directory, with no error and no visible symptom.
  `TestTheLocatorAnEntryPointWasGivenCannotBeChangedAfterwards` writes the locator after the rule is built
  and both entry points still resolve the project they were given; it fails with the copy removed.
