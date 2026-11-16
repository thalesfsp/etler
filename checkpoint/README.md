# Checkpoint

## Overview

The `checkpoint` package stores per-chunk outcomes for `pipeline.RunChunks`.
A record says whether a chunk of a run is `done` or `failed`, with the last
error and the number of attempts. `RunChunks` skips chunks the store holds as
done, which is how a rerun resumes.

## Stores

- **`NewMemory()`**: process memory. Keeps finished chunks across calls in one
  process; loses them when it exits.
- **`NewFile(path)`**: one JSON file, rewritten through a temporary file and a
  rename on every save. Survives a process crash; it does not fsync, so it is
  not durable across power loss, and it does not lock across processes.
- **Your own**: implement `Load(ctx, runID)` and `Save(ctx, runID, record)`.
  A database table keyed by `(run_id, key)` with an upsert in `Save` is enough.
  `Save` receives a context that is never canceled, so an interrupted run can
  still record what it finished; apply your own timeout if you need one.
