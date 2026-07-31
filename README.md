# Overview

ETLer is a typed Extract, Transform, Load (ETL) framework for Go. It provides
pipelines, stages, processors, converters, loaders, and storage adapters that
can be composed without giving up ordinary Go functions and interfaces.

A pipeline runs stages either sequentially or concurrently. Within a stage,
synchronous processor output feeds the next processor; async processors run on
snapshots and do not feed their output forward. The stage converts the resulting
items concurrently and waits for all async processors before it returns.

## Key Features

- **Composable, typed building blocks:** loaders retrieve or decode data,
  processors transform slices, converters map individual values, stages combine
  processors with a converter, and pipelines coordinate stages.
- **Two pipeline modes:** sequential pipelines feed each stage's task into the
  next stage; concurrent pipelines give every stage the original task. Both
  return one task per stage in declaration order.
- **Custom functions and implementations:** the core packages expose generic
  interfaces and functional options. The repository includes CSV, pass-through,
  and storage implementations; callers can supply their own functions and
  interface implementations.
- **Lifecycle observability:** pipelines, stages, processors, converters, and
  loaders record status, counters, and duration with `expvar`, emit structured
  logs through `sypl`, and create Elastic APM transactions and spans.
- **Retry-aware storage keys:** the built-in storage ID paths use an ID field or
  a SHA-256 hash of the item's JSON, so stable input produces the same key on a
  retry. This is not unconditional: `WithIDFunc` defines its own stability, and
  a nondeterministic `MarshalJSON` changes the content-derived key. The storage
  backend's `Create` semantics determine whether a repeated key overwrites,
  rejects, or otherwise handles the write.
- **Checked examples:** the repository has 18 runnable examples across 12
  packages: `etler`, `converter`, `converters/csv`,
  `converters/passthru`, `converters/storage`, `loader`, `loaders/csv`,
  `pipeline`, `processor`, `processors/storage`, `stage`, and `task`.

## Installation

```bash
go get github.com/thalesfsp/etler/v4
```

Requires Go 1.25 or newer.

## Quick start

This sequential pipeline has one stage, one synchronous processor, and a
pass-through converter:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/thalesfsp/etler/v4/converters/passthru"
	"github.com/thalesfsp/etler/v4/pipeline"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/stage"
)

func main() {
	double, err := processor.New("double", "doubles each value",
		func(_ context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))
			for i, v := range in {
				out[i] = v * 2
			}

			return out, nil
		})
	if err != nil {
		log.Fatalln(err)
	}

	convert, err := passthru.New[int]()
	if err != nil {
		log.Fatalln(err)
	}

	stg, err := stage.New("stage-1", "doubles", convert, double)
	if err != nil {
		log.Fatalln(err)
	}

	p, err := pipeline.New("my-pipeline", "doubles every value", false, stg)
	if err != nil {
		log.Fatalln(err)
	}

	tasks, err := p.Run(context.Background(), []int{1, 2, 3})
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(tasks[len(tasks)-1].ProcessingData)
	// Output: [2 4 6]
}
```

Browse the examples on
[pkg.go.dev](https://pkg.go.dev/github.com/thalesfsp/etler/v4) or run
`go test -run Example ./...`.

## Concurrency contract

The runtime relies on these rules:

- **Converters must be safe for concurrent use.** A stage converts its items
  concurrently, so the same conversion function and completion callback may be
  called from multiple goroutines.
- **Processors must treat input as immutable.** A stage feeds each synchronous
  result to the next processor. Async processors get a new slice containing a
  shallow copy of the element values, so pointers and other references inside
  those values still refer to shared data.
- **A `Stage` instance must not run concurrently with itself.** Distinct stages
  can run independently only when they do not share processors, a converter, or
  published metric names.
- **Concurrent `Run` calls on one `Pipeline` are serialized.** Lifecycle,
  progress, and duration are shared across those runs and update at different
  transition points: status becomes `running` before task validation, progress
  resets after validation, and duration is replaced before a successful
  completion callback or as a failed run returns. The callback runs after
  serialization is released, so callbacks from two runs may overlap and must be
  concurrency-safe.
- **Published metrics can be shared across otherwise independent instances.**
  With `ETLER_METRICS_PUBLISH=true`, entities with the same type and name reuse
  the same global `expvar` values.

## Metrics publishing

Every entity owns its lifecycle metrics whether or not they are globally
published. By default those values are available through the entity's getters
but are not registered in the process-wide `expvar` registry.

Set `ETLER_METRICS_PUBLISH` to the exact string `true` before constructing an
entity to publish its values under stable names such as
`etler.pipeline.my-pipeline.done.counter`. Global registration is opt-in because
`expvar` entries cannot be removed. A later entity with the same type and name
reuses the same published objects; its construction sets the shared status to
`created` and increments the shared created counter, while other shared values
retain their prior contents.

## Upgrading from v3

Change imports to `github.com/thalesfsp/etler/v4` and use Go 1.25 or newer.
Then make each applicable migration below:

- **Replace direct field access.** `OnFinished` is no longer exported on
  `Pipeline`, `Stage`, `Processor`, `Converter`, or `Loader`; use
  `GetOnFinished` and `SetOnFinished`. `Processor.Async` is also unexported; use
  `GetAsync` and `SetAsync`. The accessors keep their v3 signatures. Because
  `Async` is unexported, it no longer appears in processor JSON.
- **Replace unkeyed entity literals with constructors.** `Stage`, `Processor`,
  `Converter`, and `Loader` gained unexported mutex fields, so their unkeyed
  composite literals no longer compile. Use their `New` factories.
- **Update CSV field mapping and validate numeric input explicitly.** The v4
  loader maps with gocsv and `csv:"..."` tags instead of converting rows through
  JSON and `json:"..."` tags. Default matching is case-sensitive but not
  byte-exact: `Name` also matches a header with surrounding whitespace or any
  U+200B, U+200C, U+200D, or U+FEFF zero-width characters removed. A
  process-wide `gocsv.SetHeaderNormalizer` applies to field keys and headers and
  can make matching case-insensitive. Add explicit `csv` tags wherever the
  accepted header spelling must be unambiguous.

  Plain `int`, `float64`, and `bool` fields now decode directly, so remove the
  old `json:"age,string"` workaround. Numeric conversion remains gocsv's
  permissive, not range-safe conversion: `010` decodes as `8`, `1_0` as `10`,
  `127.9` into an integer as `127`, and out-of-range values may wrap or become
  infinite. Decode into a `string` and parse it yourself when syntax or range
  matters. Tabs in quoted values are now preserved; callers that relied on v3
  sanitizing them must strip them explicitly.

  Loading and writing run the same validation, so a type the converter writes is
  a type the loader reads. They differ in mechanics: writing calls
  `gocsv.MarshalString` through gocsv's writer factory, while loading strips one
  leading UTF-8 BOM and constructs `encoding/csv.NewReader` directly, so a
  host-configured gocsv reader factory does not affect the loader. The loader does use gocsv's tag name, tag separator, field
  combiner, header normalizer, and duplicate-header settings;
  `gocsv.FailIfUnmatchedStructTags` does not affect it.

  **Cyclic types are rejected on both paths, before gocsv sees them.** A
  self-referential type makes gocsv recurse until the process dies of a stack
  overflow, which no `recover` can catch, so `loaders/csv.Load` and
  `converters/csv.New` both reject one. The converter fails at construction
  rather than at conversion, and would otherwise die even on an empty input
  slice, because the recursion is driven by the type, not the values.

  **`csv[]` expansion is supported where gocsv handles it, on both paths.** A direct `[]T` field
  with a positive count, or a `[N]T` field whose count is at most `N`, decodes
  normally. Three forms are rejected: the tag on a pointer-wrapped field, which
  gocsv ignores entirely; a count that is non-numeric or non-positive, which
  expands to nothing; and a count larger than the array it applies to, which
  fails inside gocsv. The first two discard columns with no error. The message
  says which applies.

  **Configure gocsv's package-level settings before a type is used for the first
  time.** gocsv caches each row type's field mapping keyed only by the type, with
  no way to invalidate it, and the cache is shared by reading and writing — so
  the first load *or* conversion anywhere in the process freezes that mapping.
  Changing `gocsv.TagName` or the header normalizer afterwards does not re-resolve
  it: the old mapping stays in force, the new headers match nothing, and the
  affected fields come back zero-valued with **no error**. Treat those settings as
  process-wide startup configuration.

  Cycle rejection walks a deliberate *superset* of gocsv's own traversal: it may
  reject a type gocsv could technically load, but it can never miss one that
  kills the process. A field type implementing `MarshalCSV`, `MarshalText`,
  `UnmarshalCSV`, or `UnmarshalCSVWithFields` is a leaf, and only one pointer
  level is followed, so `**T` is a leaf. A type implementing only
  `encoding.TextUnmarshaler` is **not** a leaf, because gocsv descends into it.
  Anonymous struct fields are traversed; anonymous slice and array fields are
  not, because gocsv drops them.

  A `csv:"-"` tag does **not** exempt a field from the cycle check. gocsv applies
  its process-wide header normalizer before deciding whether to skip the field,
  and that normalizer cannot be read back, so honouring the tag could let a
  cyclic type through into an unrecoverable crash.

  **Field matching is gocsv's, and two cases resolve silently.** When two fields
  can answer to the same header — an embedded field and an outer field of the
  same name, or two fields carrying the same `csv` tag — gocsv fills whichever is
  declared first and leaves the other zero. Declaration order decides it, not
  Go's shadowing rules, so an outer field that shadows an embedded one in normal
  Go can still lose to it here. Neither case reports an error; give colliding
  fields distinct `csv` tags.

  A panic raised by your own `UnmarshalCSV` or `UnmarshalText` is caught by the
  same guard that catches gocsv's, so it is reported as a gocsv panic naming the
  output type.
- **Wrap options passed to storage constructors.** Storage constructors now
  take their own option types. Use
  `converters/storage.WithConverterOptions(opt)` or
  `processors/storage.WithProcessorOptions(opt)` for options that configure the
  embedded entity.
- **Account for deterministic storage IDs.** Resolution order is
  `WithIDFunc`, then the configured or default `ID`/`Id` field, then a SHA-256
  hash of the item's JSON. Supported ID fields are strings and signed or
  unsigned integers except `uintptr`, after pointer/interface dereferencing.
  A `fmt.Stringer`-only ID is ignored, `String()` is never called, and resolution
  falls through to the JSON content hash. Equal JSON content shares an ID. For
  distinct keys, give each logical record a stable unique scalar field and
  select it with `WithIDFieldName`, or provide a deterministic `WithIDFunc`.

  The built-in field and hash paths are stable for stable input. A custom
  `WithIDFunc` or nondeterministic `MarshalJSON` can produce a different key on
  retry. Etler guarantees only the resolved key; idempotence still depends on
  the backend's `Create` behavior.
- **Give every processor a name.** `processor.New("")` now returns an error.
- **Do not rely on overlapping runs of one pipeline.** Concurrent `Run` calls on
  one `Pipeline` are now serialized. If intentional overlap is required, use
  separate pipeline instances and do not share a `Stage` instance between them.
  `OnFinished` runs after serialization is released, so callbacks can still
  overlap.
- **Read pause state separately from lifecycle status.** `GetStatus` reports
  `created`, `running`, `done`, or `failed` and never reports `paused`.
  `GetPaused` is the pause-controller state, and `SetPause` changes only that
  controller. A paused pipeline can legitimately report a running lifecycle
  while its next processor waits for resume.
- **Pass a nonblank converter storage target.**
  `converters/storage.New` now rejects an empty or whitespace-only target,
  matching the processor storage implementation.
- **Remove calls to `Pipeline.UpdateObservability`.** The exported method was
  removed with no public replacement. `Pipeline.Run` still updates lifecycle
  status, counters, progress, and duration internally and, after a successful
  run releases serialization, invokes `OnFinished`. Custom execution paths must
  maintain their own observability and callback behavior.

See [CHANGELOG.md](CHANGELOG.md) for the release record.

## Architectural Modularity and Flexibility

The core interfaces (`IPipeline`, `IStage`, `IProcessor`, `IConverter`, and
`ILoader`) separate orchestration from data-specific work. Generic function
types provide the default implementations, while functional options configure
callbacks and execution behavior. This keeps ordinary transformations small and
testable while allowing callers to replace a complete component when its
contract is preserved.
