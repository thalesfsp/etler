# Overview

ETLer is a comprehensive and highly customizable Extract, Transform, Load (ETL) framework designed for modern data pipelines. Built with Go, ETLer provides a set of powerful tools and abstractions to streamline the process of extracting data from various sources, transforming it to meet specific requirements, and loading it into target systems.

In today's data-driven world, ETL pipelines have become essential for organizations to harness the full potential of their data. ETLer aims to simplify and accelerate the development of these pipelines, enabling developers to focus on the core logic of data transformations while providing a robust and efficient runtime environment.
ETL pipelines play a crucial role in modern data architectures, particularly in the realm of artificial intelligence (AI) and 
machine learning (ML). As the volume, variety, and velocity of data continue to grow, the ability to efficiently extract, transform, and load data becomes paramount.

ETL pipelines enable organizations to:

- Consolidate data from disparate sources into a unified format
- Cleanse and validate data to ensure accuracy and consistency
- Transform data to meet the specific requirements of downstream systems
- Load data into target systems, such as data warehouses or AI/ML platforms
- Automate data workflows to reduce manual intervention and improve efficiency
- By leveraging ETL pipelines, organizations can ensure that their AI and ML models have access to high-quality, up-to-date, and relevant data, ultimately leading to more accurate insights and better decision-making.

## Key Features

- Modularity and Reusability: ETLer is designed with modularity and reusability at its core. The framework provides a set of building blocks, such as pipelines, stages, processors, converters, and loaders, which can be easily composed and reused across different ETL workflows. This modular approach enables developers to create complex data processing pipelines by combining and customizing these components to suit their specific requirements.

- Flexible Pipeline Definition: ETLer allows developers to define multi-stage data processing pipelines using a declarative and intuitive syntax. Stages can be configured to run in both synchronous and concurrent modes, providing flexibility in execution and optimizing performance based on the specific needs of the pipeline.

- Powerful Data Transformations:Transformations can be easily combined and chained together within stages to create complex data processing logic. The framework also allows developers to implement custom processors and converters using Go functions, providing unlimited flexibility in data manipulation.

- Efficient Data Loading: Loaders allows to efficiently load data from various sources, including files, databases, APIs, and message queues. Loaders are designed to handle different data formats and protocols, making it easy to integrate with diverse data sources.

- Comprehensive Observability: ETLer prioritizes observability and provides built-in features for monitoring, logging, and tracing pipeline execution. The framework exposes pipeline/stage/processor metrics using Golang's built-in, battle-tested `expvar` package (global registration is opt-in via `ETLER_METRICS_PUBLISH=true`), allowing easy integration with monitoring systems. Structured logging is powered by the `sypl` package, providing rich context and consistent log levels across the pipeline. Distributed tracing is supported through the `customapm` package which is powered under-the-hood by Elastic APM, enabling deep insights into pipeline performance and behavior.

- Error Handling and Resilience: ETLer includes robust error handling mechanisms to ensure pipeline resilience and fault tolerance. Errors that occur during pipeline execution are propagated and handled gracefully, with detailed error messages and proper error reporting. Storage writes derive a stable id from each item, so a retry reuses the same key rather than inserting a second record — on a backend whose `Create` upserts, that means no duplicate.

- Testing and Documentation: ETLer emphasizes the importance of testing and documentation. The framework includes a comprehensive suite of unit tests to ensure the reliability and correctness of pipeline components. The codebase is documented, with comments explaining the purpose and functionality of components facilitating adoption, and enable developers to effectively leverage the framework.

## Installation

```bash
go get github.com/thalesfsp/etler/v4
```

Requires Go 1.25 or newer.

## Quick start

A pipeline is made of stages; each stage runs its processors in order, then
converts every item.

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

Most packages ship runnable examples — see them on
[pkg.go.dev](https://pkg.go.dev/github.com/thalesfsp/etler/v4) or run
`go test -run Example ./...`.

### Concurrency contract

Three rules the design relies on:

- **Converters must be safe for concurrent use.** A stage converts its items
  concurrently, so the conversion function is called from many goroutines.
- **Processors must not mutate their input slice, or the values it points to.**
  A stage reuses the returned slice as the next processor's input, and async
  processors receive only a shallow copy.
- **Concurrent `Run` calls on one `Pipeline` are serialized.** Its lifecycle,
  progress and duration describe the active or most recently completed run.
  Completion callbacks run after that serialization is released, so two runs'
  callbacks may overlap — a callback must be safe for concurrent use.
  Separate instances are independent, with one exception: under
  `ETLER_METRICS_PUBLISH=true`, two entities of the same type sharing a name
  deliberately share the same published `expvar`s.

## Upgrading from v3

The import path changes to `github.com/thalesfsp/etler/v4`, and the module now
requires Go 1.25. Beyond that:

- `OnFinished` and `Async` are no longer exported struct fields — use the
  `GetOnFinished`/`SetOnFinished` and `GetAsync`/`SetAsync` accessors, whose
  signatures are unchanged.
- The CSV loader now maps struct fields by `csv:"..."` tag instead of
  `json:"..."`, matching the CSV converter. Untagged fields still match their
  header case-insensitively. Plain `int`, `float64` and `bool` fields decode
  directly — remove the old `json:"age,string"` workaround.
  **Numeric conversion is gocsv's and is permissive, not range-safe:** `010`
  decodes as `8`, `1_0` as `10`, `127.9` into an integer as `127`, and
  out-of-range values may wrap or become infinite. If you need a syntax or
  range guarantee, decode into a `string` field and parse it yourself. Output
  types using gocsv's `csv[]` slice expansion are not supported.
- The storage constructors take their own option type. Entity options that used
  to be passed directly must now be wrapped:
  `storage.New[Item](s, "target", storage.WithConverterOptions(opt))` — and
  `WithProcessorOptions` for the processor equivalent.
- Storage writes derive a **stable** id from each item instead of a fresh UUID
  per attempt, so a retry reuses the same key. Whether that overwrites or is
  rejected is the backend's `Create` semantics — etler guarantees the key, not
  the upsert. Stability assumes your item's own extension points are
  deterministic: a `fmt.Stringer` id or a custom `MarshalJSON` that varies
  between calls will produce a varying id. Two structurally identical items therefore share an id; pass
  `WithIDFunc` or `WithIDFieldName` if they must be distinct.
- `processor.New("")` returns an error; a name is required.

See [CHANGELOG.md](CHANGELOG.md) for the full list.

## Architectural Modularity and Flexibility

ETLer is designed with architectural modularity and flexibility in mind, leveraging Go's powerful features to create a highly extensible and customizable ETL framework.

The framework is built around a set of core interfaces, such as IPipeline, IStage, IProcessor, IConverter, and ILoader, which define the contracts for the various components of an ETL pipeline. These interfaces provide a clear separation of concerns and allow for easy composition and integration of custom implementations.

The framework provides optmized and default implementations for each component, such as Pipeline, Stage, Processor, Converter, and Loader, which can be used as-is or extended to meet specific requirements.

The use of Go's generic types enables the framework to handle various data types seamlessly. ETLer leverages generics to define type-safe data processing functions, making the framework highly adaptable to different data scenarios without sacrificing type safety.

ETLer also embraces the functional options pattern for configuration and customization. This pattern allows developers to configure pipeline components using a fluent and expressive syntax, providing a clean and flexible way to customize behavior without modifying the core structures.

The modular and flexible architecture of ETLer empowers developers to create highly customized and optimized ETL pipelines tailored to their specific data processing needs. The framework provides a solid foundation for building scalable and maintainable ETL solutions while offering the freedom to extend and adapt the components to fit unique requirements.