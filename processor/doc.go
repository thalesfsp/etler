// Package processor provides a framework for creating processors.
//
// # Processor
//
// ## Overview
//
// The `processor` package is a key component of the ETL project, providing a versatile and efficient mechanism for defining and executing data processing operations within a pipeline stage. Processors are responsible for transforming and enhancing the data flowing through the pipeline, allowing for complex data manipulations and business logic to be applied.
//
// ## What's a Processor?
//
// A processor is a self-contained unit of data transformation within a pipeline stage. It encapsulates a specific data processing operation, defined as a function that takes a context and an input slice of data, performs the necessary transformations, and returns the processed data along with any errors that occurred during processing.
//
// Processors are highly modular and reusable, allowing them to be easily combined and composed within stages to create sophisticated data processing workflows.
//
// ## How It Works
//
// The core of the `processor` package revolves around the `IProcessor` interface, which defines the contract for a processor. The `Processor` struct implements this interface, providing the necessary functionality for creating and running processors.
//
// To create a processor, you instantiate a new `Processor` using the `New` factory function, specifying the processor name, description, and the transform function. The transform function is defined using the `Transform` type, which takes a context and an input slice of data of type `ProcessingData`, and returns the processed data along with any errors.
//
// When the `Run` method is called on a processor, it executes the following steps:
//
// 1. It checks if the pipeline is paused. If paused, the processor waits until it is resumed or the context is done. This allows for graceful handling of pipeline pauses during processor execution.
//
// 2. Once the pipeline is resumed or if it was not paused, the processor invokes the transform function, passing the input data and the context. The transform function performs the necessary data transformations and returns the processed data.
//
// 3. If the transform function returns an error, the processor handles it gracefully, updating the relevant metrics and logging the error details.
//
// 4. After successful execution of the transform function, the processor updates its metrics, such as incrementing the done counter and setting the duration.
//
// 5. If an `OnFinished` callback function is provided, the processor invokes it, passing the original input data, the processed data, and the context. This allows for custom post-processing or logging after the processor finishes its execution.
//
// 6. Finally, the processor returns the processed data to the caller.
//
// Throughout the execution, the processor maintains comprehensive observability, including metrics, logging, and tracing, to monitor and debug the processor's performance and behavior.
//
// ## Features
//
// 1. **Modularity and Reusability**: Processors are designed to be modular and reusable, enabling easy composition and combination within pipeline stages to create complex data processing workflows.
//
// 2. **Flexibility**: Processors can encapsulate any data transformation logic, from simple arithmetic operations to complex business rules and data enrichment.
//
// 3. **Pause and Resume**: Processors support pipeline pausing and resuming. If the pipeline is paused during a processor's execution, the processor gracefully waits until it is resumed or the context is done, ensuring proper handling of pauses.
//
// 4. **Observability**: The processor package provides comprehensive observability features, including metrics, logging, and tracing, to monitor and debug the processor's execution.
//
// 5. **Metrics**: Processor metrics are exposed using the `expvar` package, allowing for easy integration with monitoring systems. Metrics include counters for created, running, failed, done, and interrupted processors, as well as duration.
//
// 6. **Logging**: The package utilizes the `sypl` library for structured logging, providing rich context and consistent log levels throughout the codebase. Log messages include relevant information such as processor status, counters, and duration.
//
// 7. **Tracing**: Tracing is implemented using the `customapm` package, which integrates with Elastic APM (Application Performance Monitoring) under the hood. This enables distributed tracing of the processor's execution, allowing developers to gain insights into the performance and behavior of their processors.
//
// 8. **Error Handling**: The processor package includes robust error handling mechanisms, with detailed error messages and proper propagation of errors during processor execution.
//
// 9. **OnFinished Callback**: Processors support an optional `OnFinished` callback function, which is invoked after the processor finishes its execution. This callback receives the original input data, the processed data, and the context, enabling custom post-processing or logging.
//
// 10. **Flexible Configuration**: Processors can be configured with various options, such as the `OnFinished` callback, using a functional options pattern. This allows for easy customization of processor behavior without modifying the core processor struct.
//
// 11. **Thorough Testing**: The codebase includes comprehensive unit tests, ensuring the reliability and correctness of the processor functionality. The tests cover various scenarios, including success cases, error handling, and pause/resume functionality.
//
// 12. **Well-Documented**: The code is thoroughly documented, with clear comments explaining the purpose and functionality of each component. The package also includes usage examples and test cases.
//
// 13. **Idiomatic Go**: The codebase follows idiomatic Go practices, leveraging the language's features and conventions for clean and efficient code.
//
// 14. **Typed Errors**: The package utilizes typed errors, providing more context and facilitating error handling and debugging.
//
// 15. **Customizable**: The processor package provides a high level of customization through the use of interfaces and generic types. Developers can easily create custom processors with specific transformation logic to meet their data processing requirements.
package processor
