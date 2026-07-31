# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Project Roadmap

- [ ] Create and Update should allow to pass more than one object per time (Bulk Create and Update)

## [4.0.0] - 2026-07-30

Major release. Module path is now `github.com/thalesfsp/etler/v4`.

Second audit pass: bug and race fixes, a large efficiency improvement, and
coverage raised to ~99%.

### Breaking changes
- **Module path** is `github.com/thalesfsp/etler/v4`. Consumers must update
  their imports.
- **`OnFinished` and `Async` are no longer exported fields.** Their accessors
  (`GetOnFinished`/`SetOnFinished`, `GetAsync`/`SetAsync`) are unchanged and are
  now the only way to reach them. With the fields exported, a caller could
  assign them directly and race with a running `Run`, which made the
  synchronisation bypassable. A consequence: the processor's `async` field no
  longer appears in marshalled JSON.
- **`Stage`, `Processor`, `Converter` and `Loader` gained unexported fields**
  (the mutexes above), so unkeyed composite literals of those structs no longer
  compile. Use the factories, as before. (`Pipeline` gained one too, but already
  had an unexported field, so unkeyed literals of it were never possible.)
- **The CSV loader now decodes directly into the target type** with
  [gocsv](https://github.com/gocarina/gocsv) — the same library the CSV
  *converter* already used for writing. Reading and writing are therefore
  symmetric, and plain `int`, `float64` and `bool` struct fields now work.
  Structs are mapped by `csv:"..."` tag instead of `json:"..."` (untagged
  fields still match their header case-insensitively), and the
  `json:"age,string"` workaround is no longer needed — plain `int`, `float64`
  and `bool` fields decode directly. The loader also no longer strips tab
  characters, which previously corrupted legitimately quoted content such as
  `"a\tb"`. Output types using gocsv's `csv[]` slice expansion are rejected.

  **Numeric conversion is gocsv's and is permissive, not range-safe:** `010`
  decodes as `8`, `1_0` as `10`, `127.9` into an integer as `127`, and
  out-of-range values may wrap or become infinite. This is a deliberate,
  documented limitation. Validating it requires gocsv's header-to-field
  mapping, which it builds but keeps unexported with no accessor; three
  mechanisms that tried to reconstruct it each failed and silently missed
  fields, so the guarantee was withdrawn rather than faked. Callers needing a
  syntax or range guarantee should decode into a `string` field and parse it
  themselves. Note that v3's JSON-based path rejected some of these inputs, so
  this is a narrowing in that one respect — traded for correct typed decoding
  of ordinary values, which v3 could not do at all without a `,string` tag.
- **Storage writes now use a stable id** derived from the item, instead of a
  fresh UUID per attempt, so retrying after a timeout reuses the same key rather
  than inserting under a new one. Whether that overwrites or is rejected is the
  backend's `Create` semantics — etler guarantees the key, not the upsert. Two
  structurally identical items therefore share an id; `WithIDFunc` and
  `WithIDFieldName` are available when they must be distinct. Stability assumes
  the item's own extension points are deterministic — a `fmt.Stringer` id or a
  custom `MarshalJSON` that varies between calls will produce a varying id. The storage
  constructors also take their own option type now, so entity options must be
  wrapped in `WithConverterOptions` / `WithProcessorOptions`.
- **`processor.New("")` returns an error.** `Name` is now required, matching
  every other entity.
- **Pause state is separate from lifecycle status.** `GetStatus` reports the
  run — `created` → `running` → `done`/`failed` — and never returns `paused`;
  `SetPause` no longer writes it at all. `GetPaused` is the only truth about the
  controller. Previously the two were conflated, which meant a run finishing
  while paused could have its terminal status hidden until resume, or lost
  entirely if it was never resumed.
- **`converters/storage.New` rejects a blank target**, matching
  `processors/storage`.

### Performance
Profiling attributed roughly 85% of all allocations to log messages that were
built in full and then discarded by the level filter. Log payload construction
on every run path is now gated behind the same emit rule sypl itself applies, so
that work is skipped when the message would be discarded. The gate errs toward
doing the work: it admits everything when `SYPL_LEVEL` or `SYPL_FILTER` is set,
since those raise the level at runtime, and it cannot see an output processor
that force-emits — the same limitation sypl's own opt-in fast gate documents. Span names use
concatenation instead of `fmt.Sprintf`, and the CSV loader no longer round-trips
its rows through a map and a JSON encode/decode pass.

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

Recycling the pooled transaction (the trace fix below) is what took the
single-call paths from ~20 µs to ~3 µs.

No metric, status, counter, or duration write was removed or weakened — only
discarded logging work is skipped.

### Fixed
- **`ExtractID` killed the process on a cyclic embedded pointer.** A
  self-referential exported embedded pointer recursed until the stack
  overflowed — a fatal error, not a recoverable panic. Traversal now records
  visited pointers and stops on a repeat. Embedded interfaces are still
  traversed.
- **`OnFinished` and `Async` were unsynchronised.** Their getters and setters
  read and wrote the field with no synchronisation (a data race), and `Run` read
  the callback twice, so a `SetOnFinished(nil)` landing between the nil check
  and the call made `Run` invoke a nil function and panic. Both are now guarded,
  and every call site snapshots the value once.
- **Resuming clobbered a terminal status.** `SetPause(false)` after a run
  finished overwrote `done` (or `failed`) with `running`. Pause and status
  transitions now happen together under one lock, so an observer never sees a
  paused controller reporting running.
- **Async processors counted as complete when launched, not when finished.** A
  stage with a blocked async processor reported 100% immediately, and still
  reported full progress if that processor later failed.
- **Failed runs reported the previous run's duration.** Duration is now written
  on every return path.
- **Traces were never reported.** Nothing ended the APM transaction the tracing
  helper created, so neither it nor its spans ever shipped, and every standalone
  call left one behind. The transaction is now ended by whoever created it, and
  never when it was inherited from the caller's context.
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

Concurrent `Run` calls on one `Pipeline` used to be a documented footgun; they
are now serialized instead. The completion callback runs after that
serialization is released — a callback that started a nested `Run` previously
deadlocked — so callbacks may overlap and must be concurrency-safe.

### Added
- **12 new runnable examples** (13 in total) across `etler`, `pipeline`,
  `stage`, `processor`, `converter`, `loader`, `loaders/csv`, `converters/csv`
  and `converters/passthru`, verified by `go test` so they cannot silently rot —
  including an end-to-end one at the package root. Browse them on
  [pkg.go.dev](https://pkg.go.dev/github.com/thalesfsp/etler/v4) or run
  `go test -run Example ./...`. The storage packages and `task` have none yet.
- A benchmark suite isolating per-item cost from fixed per-call cost.
- Ten end-to-end scenarios, including a regression guard for the zero-value
  data-loss bug, async failure, pause/resume, and context cancellation.
- README installation, quick start, concurrency contract, and a v3 upgrade guide.
- Total coverage 96.8% → 99.0%; `processors/storage`, `converters/storage`,
  `converters/csv` and `converters/passthru` are at 100%, `loaders/csv` at 96.0%.
- The suite is hermetic against `SYPL_LEVEL` and `SYPL_FILTER`: it passes whether
  or not either is exported.

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
