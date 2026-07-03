# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Project Roadmap

- [ ] Create and Update should allow to pass more than one object per time (Bulk Create and Update)

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
