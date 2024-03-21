# Pipeline

## Overview

The `pipeline` package is a core component of the ETL project, providing a powerful and flexible mechanism for defining and executing data processing pipelines. It is designed to be modular, extensible, and production-ready, with a focus on observability, concurrency, and adherence to best practices.

## What's a Pipeline?

A pipeline is a series of stages that define a sequence of data processing operations. Each stage consists of a converter and one or more processors, which transform and enhance the data as it flows through the pipeline. Pipelines are highly configurable and can be tailored to meet specific data processing requirements.

## How It Works

The pipeline package revolves around the `IPipeline` interface, which defines the contract for a pipeline. The `Pipeline` struct implements this interface and provides the core functionality for creating and running pipelines.

To create a pipeline, you instantiate a new `Pipeline` using the `New` factory function, specifying the pipeline name, description, concurrency mode, and a variadic list of stages. Each stage is defined using the `stage.IStage` interface, which encapsulates a converter and a set of processors.

When the `Run` method is called on a pipeline, it iterates through the stages, passing the output of each stage as the input to the next. The pipeline supports both synchronous and concurrent execution modes, allowing for efficient processing of large datasets.

## Features

1. **Modular and Extensible**: The pipeline package is designed with modularity and extensibility in mind. It leverages interfaces and generic types to enable easy integration of custom converters and processors.

2. **Concurrent Execution**: Pipelines support concurrent execution of stages, enabling parallel processing of data for improved performance.

3. **Observability**: The package provides comprehensive observability features, including metrics, logging, and tracing, to monitor and debug pipeline execution.

4. **Metrics**: Pipeline metrics are exposed using the `expvar` package, allowing for easy integration with monitoring systems. Metrics include counters for created, running, failed, and done pipelines, as well as duration and progress tracking.

5. **Logging**: The package utilizes the `sypl` library for structured logging, providing rich context and consistent log levels throughout the codebase.

6. **Tracing**: The pipeline package integrates with the `customapm` package, which utilizes Elastic APM (Application Performance Monitoring) under the hood. This enables distributed tracing of the pipeline execution, allowing developers to gain deep insights into the performance and behavior of their pipelines. The tracing data can be visualized, analyzed, and correlated with other application metrics.

7. **Error Handling**: The pipeline package includes robust error handling mechanisms, with detailed error messages and proper propagation of errors throughout the pipeline.

8. **Pause and Resume**: Pipelines support pausing and resuming execution, allowing for graceful handling of system events or maintenance tasks.

9. **Progress Tracking**: The package provides progress tracking capabilities, including absolute progress and percentage completion, enabling real-time monitoring of pipeline execution.

10. **Flexible Configuration**: Pipelines can be configured with various options, such as concurrency mode, on-finished callbacks, and custom loggers, using a functional options pattern.

11. **Thorough Testing**: The codebase includes comprehensive unit tests, ensuring the reliability and correctness of the pipeline functionality.

12. **Production-Ready**: The pipeline package is production-ready, with a focus on performance, reliability, and scalability. It has been deployed and is in use by companies like Adobe.

13. **Well-Documented**: The code is thoroughly documented, with clear comments explaining the purpose and functionality of each component. The package also includes usage examples and test cases.

14. **Idiomatic Go**: The codebase follows idiomatic Go practices, leveraging the language's features and conventions for clean and efficient code.

15. **Typed Errors**: The package utilizes typed errors, providing more context and facilitating error handling and debugging.

## Architectural Modularity and Flexibility

The pipeline package is designed with architectural modularity and flexibility in mind. It leverages Go's interfaces and generic types to provide a highly extensible and customizable pipeline framework.

The `IPipeline` interface defines the contract for a pipeline, allowing for easy integration of custom pipeline implementations. The `stage.IStage` interface enables the creation of custom stages with specific converters and processors.

The use of generic types for `ProcessedData` and `ConvertedOut` allows pipelines to handle various data types, making the package adaptable to different data processing scenarios.

The functional options pattern, used in the `New` factory function and various configuration methods, provides a clean and flexible way to customize pipeline behavior without modifying the core pipeline struct.

## Concurrency

Concurrency is a powerful feature of the pipeline package, enabling efficient parallel processing of data. Pipelines support both synchronous and concurrent execution modes, controlled by the `ConcurrentStage` flag.

When `ConcurrentStage` is set to `true`, the pipeline executes stages concurrently using the `concurrentloop` package. This allows for parallel processing of data across stages, improving overall performance. In this mode, the same input data is fed to all stages concurrently, and the results are merged in a slice, at the end of the pipeline.

When `ConcurrentStage` is set to `false`, the pipeline executes stages synchronously, processing data sequentially through each stage. This mode is useful for scenarios where data dependencies or ordering are important. In this mode, the output of one stage becomes the input of the next stage, and so on.

The package handles concurrency safely, ensuring proper synchronization and avoiding race conditions. Errors that occur during concurrent execution are propagated and handled appropriately.

## Applied Best Practices

The pipeline package adheres to best practices and idiomatic Go programming principles:

- **Interface-Driven Design**: The package heavily relies on interfaces, such as `IPipeline` and `stage.IStage`, to provide abstraction and extensibility. This allows for easy integration of custom implementations and facilitates testing.

- **Functional Options**: The package utilizes the functional options pattern for pipeline configuration, providing a clean and flexible way to customize pipeline behavior.

- **Error Handling**: The package follows Go's error handling conventions, returning errors from functions and methods when necessary. Errors are propagated and handled appropriately throughout the pipeline.

- **Testing**: The package includes comprehensive unit tests, covering various scenarios and edge cases. The tests ensure the correctness and reliability of the pipeline functionality.

- **Naming Conventions**: The codebase follows Go's naming conventions, using descriptive and meaningful names for variables, functions, and types.

- **Code Organization**: The package is well-organized, with separate files for different components and concerns. This promotes code readability and maintainability.

By applying these best practices, the pipeline package maintains a high level of code quality, reliability, and ease of use.
