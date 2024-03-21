// Package stage provides a framework for creating stages
//
// # Stage
//
// ## Overview
//
// The `stage` package is a fundamental building block of the ETL project, providing a flexible and powerful mechanism for defining and executing individual stages within a pipeline. A stage represents a specific data processing step, consisting of a converter and one or more processors, which work together to transform and enhance the data flowing through the pipeline.
//
// ## What's a Stage?
//
// A stage is a self-contained unit of data processing within a pipeline. It encapsulates a converter and a set of processors that operate on the data sequentially. The converter is responsible for transforming the data from one format to another, while the processors perform specific operations on the data, such as filtering, aggregating, or enriching.
//
// Stages are highly modular and reusable, allowing them to be easily combined and composed to create complex data processing workflows.
//
// ## How It Works
//
// At the core of the `stage` package is the `IStage` interface, which defines the contract for a stage. The `Stage` struct implements this interface, providing the necessary functionality for creating and running stages.
//
// To create a stage, you instantiate a new `Stage` using the `New` factory function, specifying the stage name, description, converter, and a variadic list of processors. The converter is defined using the `converter.IConverter` interface, while the processors are defined using the `processor.IProcessor` interface.
//
// When the `Run` method is called on a stage, it executes the following steps:
//
// 1. It iterates through the processors sequentially, passing the output of each processor as the input to the next one. This ensures that the data is processed in a sequential manner, allowing for data dependencies and ordering.
//
// 2. After all the processors have finished executing, the stage applies the converter to transform the processed data into the desired output format. The converter operates concurrently on the data using the `concurrentloop` package, enabling efficient parallel processing.
//
// 3. Finally, the stage returns the converted data as a `task.Task`, which encapsulates both the processed and converted data.
//
// Throughout the execution, the stage maintains comprehensive observability, including metrics, logging, and tracing, to monitor and debug the stage's performance and behavior.
//
// ## Features
//
// 1. **Modularity and Reusability**: Stages are designed to be modular and reusable, allowing for easy composition and combination to create complex data processing workflows.
//
// 2. **Sequential Processing**: The stage executes processors sequentially, ensuring that data dependencies and ordering are maintained. This is particularly useful when the output of one processor depends on the output of a previous processor.
//
// 3. **Concurrent Conversion**: The stage applies the converter concurrently to the processed data, leveraging the `concurrentloop` package for efficient parallel processing. This improves the overall performance of the stage.
//
// 4. **Observability**: The stage package provides comprehensive observability features, including metrics, logging, and tracing, to monitor and debug the stage's execution.
//
// 5. **Metrics**: Stage metrics are exposed using the `expvar` package, allowing for easy integration with monitoring systems. Metrics include counters for created, running, failed, and done stages, as well as duration, progress, and progress percentage.
//
// 6. **Logging**: The package utilizes the `sypl` library for structured logging, providing rich context and consistent log levels throughout the codebase. Log messages include relevant information such as stage status, counters, duration, and progress.
//
// 7. **Tracing**: Tracing is implemented using the `customapm` package, which integrates with Elastic APM (Application Performance Monitoring) under the hood. This enables distributed tracing of the stage's execution, allowing developers to gain deep insights into the performance and behavior of their stages.
//
// 8. **Error Handling**: The stage package includes robust error handling mechanisms, with detailed error messages and proper propagation of errors throughout the stage's execution.
//
// 9. **Progress Tracking**: The package provides progress tracking capabilities, including absolute progress and percentage completion, enabling real-time monitoring of the stage's execution.
//
// 10. **Flexible Configuration**: Stages can be configured with various options, such as custom converters, processors, and on-finished callbacks, using a functional options pattern.
//
// 11. **Thorough Testing**: The codebase includes comprehensive unit tests, ensuring the reliability and correctness of the stage functionality. The tests cover various scenarios, including the usage of different processors and converters.
//
// 12. **Well-Documented**: The code is thoroughly documented, with clear comments explaining the purpose and functionality of each component. The package also includes usage examples and test cases.
//
// 13. **Idiomatic Go**: The codebase follows idiomatic Go practices, leveraging the language's features and conventions for clean and efficient code.
//
// 14. **Customizable**: The stage package provides a high level of customization through the use of interfaces and generic types. Developers can easily create custom converters and processors to meet their specific data processing requirements.
//
// ## Architectural Modularity and Flexibility
//
// The stage package is designed with architectural modularity and flexibility in mind. It leverages Go's interfaces and generic types to provide a highly extensible and customizable stage framework.
//
// The `IStage` interface defines the contract for a stage, allowing for easy integration of custom stage implementations. The `converter.IConverter` and `processor.IProcessor` interfaces enable the creation of custom converters and processors, respectively.
//
// The use of generic types for `ProcessingData` and `ConvertedData` allows stages to handle various data types, making the package adaptable to different data processing scenarios.
//
// The functional options pattern, used in the `New` factory function and various configuration methods, provides a clean and flexible way to customize stage behavior without modifying the core stage struct.
//
// ## Applied Best Practices
//
// The stage package adheres to best practices and idiomatic Go programming principles:
//
// - **Interface-Driven Design**: The package heavily relies on interfaces, such as `IStage`, `converter.IConverter`, and `processor.IProcessor`, to provide abstraction and extensibility. This allows for easy integration of custom implementations and facilitates testing.
//
// - **Functional Options**: The package utilizes the functional options pattern for stage configuration, providing a clean and flexible way to customize stage behavior.
//
// - **Error Handling**: The package follows Go's error handling conventions, returning errors from functions and methods when necessary. Errors are propagated and handled appropriately throughout the stage's execution.
//
// - **Testing**: The package includes comprehensive unit tests, covering various scenarios and edge cases. The tests ensure the correctness and reliability of the stage functionality.
//
// - **Naming Conventions**: The codebase follows Go's naming conventions, using descriptive and meaningful names for variables, functions, and types.
//
// - **Code Organization**: The package is well-organized, with separate files for different components and concerns. This promotes code readability and maintainability.
//
// By applying these best practices, the stage package maintains a high level of code quality, reliability, and ease of use.
package stage
