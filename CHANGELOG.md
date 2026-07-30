# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Project Roadmap

- [ ] Create and Update should allow to pass more than one object per time (Bulk Create and Update)

## [3.1.0] - 2026-07-29

Second audit pass: bug and race fixes, a large efficiency improvement, and
coverage raised to 99.4%. No breaking API changes.

### Performance
Profiling attributed roughly 85% of all allocations to log messages that were
built in full and then discarded by the level filter. Log payload construction
on every run path is now gated behind the same emit rule sypl itself applies, so
that work only happens when a message will actually be written. Span names use
concatenation instead of `fmt.Sprintf`, APM correlation fields are skipped when
the tracer is inactive, and the CSV loader drops a regexp in favour of
`strings.ReplaceAll` and pre-sizes its rows.

Measured (median of three runs):

| Benchmark | Time | Memory | Allocations |
| --- | --- | --- | --- |
| Pipeline, 2 stages × 1k items | 19.0 → 7.1 ms | 31.5 → 6.6 MB | 542k → 112k |
| Stage, 1k items | 9.9 → 3.9 ms | 15.7 → 3.2 MB | 271k → 56k |
| Stage, single item | 145 → 34 µs | 69 → 28 KB | 762 → 106 |
| Converter, single call | 58 → 17 µs | 36 → 21 KB | 235 → 21 |
| CSV loader, 1k rows | 1.09 → 0.93 ms | 821 → 738 KB | 18.9k → 12.9k |

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
Three contracts the design already relied on but never stated: converters must
be safe for concurrent use (a stage fans conversion out per item), processors
must not mutate their input slice or the values it points to, and a single
pipeline or stage instance must not be `Run` concurrently with itself.

### Added
- A benchmark suite isolating per-item cost from fixed per-call cost.
- Ten end-to-end scenarios, including a regression guard for the zero-value
  data-loss bug, async failure, pause/resume, and context cancellation.
- Total coverage 96.8% → 99.4%; `processors/storage`, `converters/storage`,
  `converters/csv` and `converters/passthru` are at 100%, `loaders/csv` at 96.9%.

### Compatibility notes
No exported symbol, signature, or interface method changed. Two things are worth
calling out:
- `processor.New("")` now returns an error instead of succeeding.
- `Stage`, `Processor`, `Converter` and `Loader` gained unexported fields (the
  mutexes above), so an unkeyed composite literal of those structs no longer
  compiles. Use the factories, as before.

### Known limitations (deferred — they need a breaking change)
- Storage writes generate a fresh UUID per attempt, so retrying after a timeout
  can duplicate rows. A fix requires a caller-supplied or content-derived ID.
- The CSV loader routes values through JSON as strings, so plain numeric and
  boolean struct fields need a `,string` tag on the JSON field.

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
