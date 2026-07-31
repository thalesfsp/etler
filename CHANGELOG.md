# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Project Roadmap

- [ ] Create and Update should allow to pass more than one object per time (Bulk Create and Update)

## [4.0.0] - 2026-07-30

Major release. Module path is now `github.com/thalesfsp/etler/v4`.

The release hardens lifecycle concurrency, fixes tracing and storage failure
paths, replaces the CSV loader's JSON round trip with direct gocsv decoding, and
brings statement coverage to 99.4%.

### Breaking changes

- **Module path** is `github.com/thalesfsp/etler/v4`. Consumers must update
  imports. The module declares Go 1.25.
- **`OnFinished` and `Processor.Async` are no longer exported fields.**
  `OnFinished` is private on `Pipeline`, `Stage`, `Processor`, `Converter`, and
  `Loader`; use `GetOnFinished`/`SetOnFinished`. Use
  `GetAsync`/`SetAsync` for processor scheduling. The accessor signatures are
  unchanged. Direct writes previously bypassed the locks used by `Run`.
  `Processor.Async` also no longer appears in marshalled JSON.
- **`Stage`, `Processor`, `Converter` and `Loader` gained unexported fields**
  for those locks, so unkeyed composite literals no longer compile. Use the
  `New` factories. (`Pipeline` already had an unexported field in v3.)
- **`Pipeline.UpdateObservability` was removed.** It was an exported internal
  completion helper, not part of `IPipeline`. Remove direct calls; `Run` still
  writes lifecycle status, counters, progress, and duration and invokes
  `OnFinished` after successful serialized execution is released. There is no
  public replacement for custom execution paths.
- **Concurrent `Run` calls on one `Pipeline` are now serialized.** Consumers
  that intentionally overlapped them must use separate pipeline instances and
  must not share a `Stage` instance between those concurrent pipelines.
  `OnFinished` runs after serialization is released, so callbacks can still
  overlap.
- **The CSV loader now decodes directly into the target type** with
  [gocsv](https://github.com/gocarina/gocsv). Structs are mapped by
  `csv:"..."` tag instead of `json:"..."`. Default matching is case-sensitive
  but not byte-exact: gocsv also matches after trimming surrounding header
  whitespace or removing U+200B, U+200C, U+200D, and U+FEFF zero-width
  characters. A host-configured gocsv header normalizer applies to field keys
  and headers and can make matching case-insensitive. Consumers should add
  explicit `csv:"..."` tags wherever accepted header spelling must be
  unambiguous. The
  `json:"age,string"` workaround is no longer needed — plain `int`, `float64`
  and `bool` fields decode directly. The loader also no longer strips tab
  characters, which previously corrupted legitimately quoted content such as
  `"a\tb"`. gocsv's `csv[]` slice expansion is supported where gocsv handles it:
  a direct `[]T` field with a positive count, or a `[N]T` field whose count is at
  most `N`, decodes normally. Four forms are rejected — the tag on an anonymous
  slice or array field, which gocsv drops whole so the tag never takes effect;
  the tag on a pointer-wrapped field, which gocsv ignores outright; a count that
  is non-numeric or non-positive; and a count larger than its array, which fails
  inside gocsv. The first three lose data with no error — an invalid count
  collapses a primitive slice into one JSON column when writing and leaves it
  empty when loading. The message distinguishes them. A tag on an anonymous
  *struct* is inert rather than lossy and is accepted.

  gocsv's package-level tag and header settings must be configured before a row
  type is used for the first time. gocsv caches each type's field mapping keyed
  only by the type, cannot invalidate it, and shares the cache between reading
  and writing, so the first load or conversion freezes that mapping process-wide;
  changing `TagName` or the header normalizer afterwards leaves the old mapping
  in force and the affected fields come back zero-valued with no error.

  Cyclic types are rejected on both the read and the write path, because they
  make gocsv recurse until the process dies. Depending on the shape that is a
  stack overflow or memory exhaustion, and neither is recoverable.
  The walk is a deliberate *superset* of gocsv's own traversal — it may reject a
  type gocsv could technically load, but it can never miss a cycle that kills the
  process. It does not bound how much work gocsv then does on an acyclic type,
  and cannot: etler's check memoises and accepts a wide or deep row type
  instantly, while gocsv's own field walk does not memoise and re-expands the
  type along every distinct path, so cost grows exponentially with nesting depth.
  Measured through `Load` with one data row: nineteen levels of two-way nesting,
  one name per field, produced over half a million columns; two comma-separated
  names at four levels took 7 ms and at five exhausted memory in about 300 ms,
  because gocsv folds the growing name set back into itself at each level rather
  than multiplying once; and a `csv[]` count on a slice is materialised eagerly,
  so an absurd count allocates proportionally, array counts being bounded by the
  array. None of this is reachable from CSV data — it is fixed by the Go type and
  its tags, and ordinary row types are unaffected. A field whose type implements `MarshalCSV`, `MarshalText`,
  `UnmarshalCSV`, or `UnmarshalCSVWithFields` is a leaf, and only one pointer
  level is followed. A type implementing only `encoding.TextUnmarshaler` is
  **not** a leaf, because gocsv descends into it. Anonymous struct fields are
  traversed; anonymous slice and array fields are not, because gocsv drops them.
  A `csv:"-"` tag does not exempt a field from either check: gocsv applies its
  process-wide header normalizer before deciding to skip, and that normalizer
  cannot be read back, so honouring the tag could let a cyclic type through into a
  crash. The same reasoning applies to the `csv[]` check, which therefore also
  ignores `-` and can reject a `-`-tagged field gocsv would have dropped
  harmlessly.

  Both paths run the same *shape* validation: valid `csv[]` expansion works on
  both, and the four lossy forms are rejected on both. They were briefly allowed
  to diverge, and the converter then emitted a phantom trailing column for an
  over-count array and JSON-dumped a pointer-wrapped one into a single column,
  with no error. This is not a round-trip guarantee — gocsv's marshal and
  unmarshal interfaces are directional, so a field implementing only `MarshalCSV`
  writes fine and fails to load back.

  Loading and writing still differ in the mechanics. The converter writes with
  `gocsv.MarshalString` and uses gocsv's writer factory.
  Loading strips one leading UTF-8 BOM, forces `encoding/csv.NewReader`, and
  therefore ignores gocsv's reader factory. Its gocsv unmarshaller still uses
  the package-level tag name, tag separator, field combiner, header normalizer,
  and duplicate-header settings; `FailIfUnmatchedStructTags` does not affect
  this loader.

  **Numeric conversion is gocsv's and is permissive, not range-safe:** `010`
  decodes as `8`, `1_0` as `10`, `127.9` into an integer as `127`, and
  out-of-range values may wrap or become infinite. Callers needing a syntax or
  range guarantee should decode into a `string` field and parse it themselves.
- **Storage writes no longer default to a fresh UUID.** Without `WithIDFunc`,
  they use the configured or default `ID`/`Id` field, then a SHA-256 hash of the
  item's JSON. Stable field values or deterministic JSON therefore reuse a key
  on retry. Whether that overwrites or is rejected is the backend's `Create`
  semantics — etler guarantees the resolved key, not the upsert. Equal JSON
  content shares an id. For distinct keys, give each logical record a stable
  unique scalar field and select it with `WithIDFieldName`, or provide a
  deterministic `WithIDFunc`. A `fmt.Stringer`-only field is ignored, its
  `String` method is never called, and resolution falls through to the content
  hash. A nondeterministic `WithIDFunc` or `MarshalJSON` can produce a different
  key.
  The storage constructors also take their own option type now, so entity
  options must be wrapped in `WithConverterOptions` /
  `WithProcessorOptions`.
- **`processor.New("")` returns an error.** `Name` is now required, matching
  every other entity.
- **Pause state is separate from lifecycle status.** `Pipeline.GetStatus` reports
  the run — `created` → `running` → `done`/`failed` — and never returns `paused`;
  `SetPause` changes only the controller and `GetPaused` reports that controller.
  A paused controller and a running lifecycle are a valid combination while a
  new run waits before its next processor. Terminal status remains visible even
  when the controller stays paused. This is about the pipeline's own status:
  `Processor.GetStatus` still reports `paused` while that processor waits, and
  `interrupted` if its context ends during the wait.
- **`converters/storage.New` rejects a blank target**, matching
  `processors/storage`.

### Performance

Debug and trace payload construction on entity run paths is now guarded by the
same output-level rule used by sypl, so discarded messages do not pay to build
their fields. The guard deliberately admits every message when `SYPL_LEVEL` or
`SYPL_FILTER` is set because those values can raise the effective level at
runtime. It cannot detect output processors that force a message through the
level filter. Span names use string concatenation instead of `fmt.Sprintf`, and
the CSV loader decodes directly instead of building maps and round-tripping them
through JSON.

Measured on an Apple M4 Pro, Go 1.26.5, median of three runs of
`go test -run '^$' -bench . -benchtime 100x -count 3 .`; the raw output is
committed at [`benchmarks/v4.0.0.txt`](benchmarks/v4.0.0.txt) alongside the
`v3.0.3` baseline it is compared against.

| Benchmark | Time | Memory | Allocations |
| --- | --- | --- | --- |
| Pipeline, 2 stages × 1k items | 19.0 → 5.7 ms (−70%) | 30.0 → 6.3 MiB (−79%) | 542k → 112k (−79%) |
| Stage, 1k items | 9.7 → 2.9 ms (−70%) | 14.9 → 3.1 MiB (−79%) | 271k → 56k (−79%) |
| Stage, single item | 145 → 10.9 µs (−92%) | 68 → 8 KiB (−88%) | 762 → 93 (−88%) |
| Processor, 1k items | 71.6 → 2.6 µs (−96%) | 44 → 11 KiB (−75%) | 236 → 13 (−94%) |
| Converter, single call | 58.5 → 2.1 µs (−96%) | 36 → 3 KiB (−90%) | 235 → 12 (−95%) |
| CSV loader, 1k rows | 1.09 → 0.36 ms (−67%) | 802 → 370 KiB (−54%) | 18.9k → 10.1k (−47%) |

Every figure above is the median of the three runs in the committed files, and
can be recomputed from them.

The logging guard does not remove status, counter, progress, or duration writes
from the normal internal run paths. The exported
`Pipeline.UpdateObservability` method that exposed completion writes was
removed, as listed under breaking changes.

### Fixed

- **The CSV loader hung forever on some cyclic output types, and silently
  dropped columns in others.** Its guard predicted gocsv's recursion with its own
  rules rather than gocsv's, and the two disagreed. It stopped descending on the
  unmarshal-side interfaces while gocsv stops on the marshal-side set, so a type
  implementing only `encoding.TextUnmarshaler` was a leaf here and a recursion
  target there. A cycle behind such a type passed validation and then hung inside
  gocsv — `Load` never returned — and a `csv[]` field nested inside one was never
  seen, so its columns were dropped with no error. The same gap rejected shapes
  gocsv handles perfectly: cycles behind `MarshalText`, `UnmarshalCSVWithFields`,
  a double pointer, or `*[]T`. The guard now walks a deliberate superset of
  gocsv's traversal, so it can over-reject but never miss a shape that kills the
  process. A cycle behind a `csv:"-"` field remains rejected on purpose, for the
  reason given above.
- **The CSV converter killed the process on a cyclic input type.** The read path
  was hardened against those shapes; the write path was not. `MarshalString` was
  called with no guard, and marshalling walks the same recursion, so a
  self-referential type ended the process — by stack overflow or by exhausting
  memory, neither recoverable —
  even for an empty input slice, since the recursion follows the type rather than
  the values. `converters/csv.New` now rejects the type at construction, and a
  panic during marshalling becomes an error. The walk is shared with the loader
  through `internal/csvshape` so the two paths cannot drift apart again.
- **A `csv:"-"` field could defeat the cycle guard entirely.** The guard compared
  the raw tag against `-`, while gocsv compares it after applying its
  process-wide header normalizer. A normalizer that rewrote `-` therefore made
  gocsv descend into a field the guard had skipped, and the process died. Because
  that normalizer cannot be read back, the tag no longer exempts a field at all.
- **A processor that filtered everything out failed its stage.** The idiomatic
  `var out []T` plus `append` returns nil when it keeps nothing, and the
  concurrent converter rejects nil input, so an ordinary empty result surfaced as
  `items cannot be nil`. Nothing documented a non-nil requirement. A nil
  processor result is now treated as an empty one.
- **`Duration` included the completion callback** on `Converter`, `Loader`,
  `Processor`, and `Stage`: a no-op converter with a 200 ms `OnFinished` reported
  202 ms. All four now record duration before invoking the callback, matching
  `Pipeline`.
- **`ExtractID` killed the process on a cyclic embedded pointer.** A
  self-referential exported embedded pointer recursed until the stack
  overflowed — a fatal error, not a recoverable panic. Traversal now records
  visited pointers and stops on a repeat. Embedded interfaces are still
  traversed.
- **`ExtractID` also hung on a self-referential interface field.** An `any` field
  holding a pointer to itself alternated interface → pointer → interface with no
  cycle check of its own. Being a spin rather than a stack overflow, nothing ever
  returned and no `recover` could help, and it was reachable from both storage
  entities' id resolution. Such a field now resolves to no id, matching the
  pure-pointer cycle.
- **A header-only CSV loaded as a nil slice.** A file with a header and no data
  rows is valid input and an empty result, but `Load` returned nil, and
  `task.New` rejects a nil `ProcessingData`, so loading zero rows failed a
  pipeline with a validation error naming an internal field. `Load` now returns
  an empty non-nil slice for both `[]T` and `[]*T`; error paths still return the
  zero value.
- **Nil processors and stages were accepted, then panicked.** `stage.New` and
  `pipeline.New` checked only that their slice was non-empty, so a nil — or a
  typed-nil, which is non-nil as an interface value — element passed construction
  and became a nil pointer dereference inside `Run`. Both now reject such
  elements and name the offending position.
- **A rejected construction published metrics anyway.** `expvar` entries cannot
  be removed, so every rejected construction permanently leaked a registry entry
  under that entity name, and a later valid entity with the same name inherited
  the leaked objects — a retry loop over a bad config grew the registry without
  bound. All five entities now validate before minting metrics, on every
  rejection path but one: an empty required collection, a nil or typed-nil
  element, a nil converter, a nil function, a nil option, and a blank name. The
  remaining case is not preflightable — an option that itself returns a nil or
  structurally invalid entity can only be detected by executing it, which
  requires the entity, and its metrics, to already exist.
- **`OnFinished` and `Async` were unsynchronised.** Their getters and setters
  read and wrote the field with no synchronisation (a data race), and `Run` read
  the callback twice, so a `SetOnFinished(nil)` landing between the nil check
  and the call made `Run` invoke a nil function and panic. Both are now guarded,
  and every call site snapshots the value once.
- **Resuming clobbered a terminal status.** `SetPause(false)` after a run
  finished overwrote `done` (or `failed`) with `running`. Pause-controller
  transitions and lifecycle status are now independent: `SetPause` never writes
  lifecycle status, and starting a run publishes `running` even when the
  controller is paused. Observers must use `GetPaused` and `GetStatus`
  separately.
- **Async processors counted as complete when launched, not when finished.** A
  stage with a blocked async processor reported 100% immediately, and still
  reported full progress if that processor later failed.
- **Failed pipeline and stage runs reported the previous run's duration.** Those
  two run paths now replace duration on every return. Failed converter,
  processor, and loader calls still retain their most recent successful
  duration.
- **Standalone trace transactions were never reported.** Etler ended the span
  but not a transaction it created for a call without an inherited transaction.
  Spans under an inherited transaction remained reportable when that
  transaction's owner ended it. The returned ender now ends the span and any
  transaction created by that trace, but never an inherited transaction.
- **A nil storage panicked on first use.** `storage.New(nil, …)` returned a
  usable-looking entity because the storage was only captured inside a validated
  closure. Both storage factories now reject nil — including a typed nil inside
  a non-nil interface — at construction.
- **`shared.Unmarshal` silently succeeded on a non-pointer destination.** It
  passed `&v`, a pointer to the interface, so `encoding/json` accepted a
  non-pointer and discarded the result.
- **The CSV converter ignored cancellation**, doing the full marshal and
  reporting success for an already-cancelled context.
- **`processor.New` accepted an empty name**, then registered metrics under
  `etler.processor..done.counter` with empty log tags. `Name` is now required,
  matching every other entity. Callers passing `""` now get an error.

### Documented

Contracts the design relied on but never stated: converters must be safe for
concurrent use (a stage fans conversion out per item), and processors must not
mutate their input slice or the values it points to.

### Added

- **17 new runnable examples** (18 in total) across `etler`, `converter`,
  `converters/csv`, `converters/passthru`, `converters/storage`, `loader`,
  `loaders/csv`, `pipeline`, `processor`, `processors/storage`, `stage`, and
  `task`, verified by `go test` so they cannot silently rot. Browse them on
  [pkg.go.dev](https://pkg.go.dev/github.com/thalesfsp/etler/v4) or run
  `go test -run Example ./...`.
- A benchmark suite isolating per-item cost from fixed per-call cost.
- Ten new end-to-end tests (13 in total), including regression guards for the
  zero-value data-loss bug, async failure, pause/resume, and context
  cancellation.
- README installation, quick start, concurrency contract, and a v3 upgrade guide.
- Current statement coverage is 99.4%; `processor`, `processors/storage`,
  `converters/storage`, `converters/csv`, `converters/passthru`, `loaders/csv`,
  `task`, `internal/logging`, `internal/metrics`, `internal/shared`, and
  `internal/csvshape` report 100%.

## [3.0.0] - 2026-07-03

Major release. Module path is now `github.com/thalesfsp/etler/v3`.

### Breaking changes
- **Per-pipeline pause**: pausing is no longer a process-wide global. `SetPause`
  affects only that pipeline; its processors receive the pause controller via
  the context. Processors running standalone never pause.
- **Async processors are joined**: a stage does not complete while its async
  processors still run, and an async failure now fails the stage (previously a
  silent status flip). Async outputs are still not forwarded.
- **Per-stage results**: sequential pipelines return one task per stage, in
  stage order (previously only the final task). The final task is the last
  element. Intermediate stages' converted data is no longer discarded.
- **`pipeline.OnFinished` signature**: now receives `[]task.Task` (per-stage
  results) instead of a single task.
- **`IPipeline`** exposes `GetProgress`, `GetProgressPercent`, and
  `SetProgressPercent` (parity with `IStage`) — implementors must add them.
- **Metrics are no longer auto-published** to the global expvar registry
  (unbounded growth for apps creating many pipelines). Set
  `ETLER_METRICS_PUBLISH=true` to publish under stable, UUID-free names;
  same-named entities then share metrics.
- **`converter.Default`** returns the error instead of panicking (use
  `MustDefault` for the panicking variant).
- `task.Type` constant is now `"task"` (was `"stage"`).
- Requires Go 1.22+.

### Fixed
- All fixes from the v2 audit (silent zero-value data loss, async data races,
  swallowed concurrent-mode errors, broken `SetPause`, `OnFinished` receiving
  the input instead of the output, CSV loader panic on empty input,
  `ExtractID` reflection panics, progress accumulation across runs).

### Added
- Coverage gate: the build fails below 90% total test coverage (currently
  ~97%).
- golangci-lint v2 configuration; lint re-enabled in CI.

## [1.0.0] - 2023-02-08
### Added
- First release.
