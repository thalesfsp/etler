// Package loader provides a framework for creating loaders.
//
// # Loader
//
// ## Overview
//
// The `loader` package is a key component of the ETL project, providing a versatile and efficient mechanism for loading and transforming data from various sources. A loader represents a specific data loading step, consisting of a loading function that retrieves data from a source and applies any necessary transformations.
//
// ## What's a Loader?
//
// A loader is a self-contained unit of data loading within a pipeline. It encapsulates a loading function that fetches data from a specific source, such as a file, database, or API, and performs any required transformations on the data before passing it to the next stage in the pipeline.
//
// Loaders are highly modular and reusable, allowing them to be easily integrated into different data processing workflows.
//
// ## How It Works
//
// The core of the `loader` package revolves around the `ILoader` interface, which defines the contract for a loader. The `Loader` struct implements this interface, providing the necessary functionality for creating and running loaders.
//
// To create a loader, you instantiate a new `Loader` using the `New` factory function, specifying the loader name, description, and loading function. The loading function is defined using the `Load` type, which takes an input of type `In` and returns an output of type `Out`, along with any errors that may have occurred during the loading process.
//
// When the `Run` method is called on a loader, it executes the following steps:
//
// 1. It invokes the loading function, passing the input data (`in`) to it. The loading function retrieves the data from the specified source and applies any necessary transformations.
//
// 2. If the loading function encounters an error, the loader handles it gracefully using the `shared.OnErrorHandler` function, which logs the error and updates the loader's metrics and status accordingly.
//
// 3. If the loading function completes successfully, the loader updates its metrics, such as incrementing the done counter and setting the duration.
//
// 4. Finally, the loader returns the transformed data as the output of type `Out`.
//
// Throughout the execution, the loader maintains comprehensive observability, including metrics, logging, and tracing, to monitor and debug the loader's performance and behavior.
//
// ## Features
//
// 1. **Modularity and Reusability**: Loaders are designed to be modular and reusable, allowing for easy integration into various data processing workflows.
//
// 2. **Flexible Loading Functions**: The `Load` type allows for the creation of custom loading functions that can retrieve data from different sources and apply transformations specific to the use case.
//
// 3. **Observability**: The loader package provides comprehensive observability features, including metrics, logging, and tracing, to monitor and debug the loader's execution.
//
// 4. **Metrics**: Loader metrics are exposed using the `expvar` package, allowing for easy integration with monitoring systems. Metrics include counters for created, running, failed, and done loaders, as well as duration and status.
//
// 5. **Logging**: The package utilizes the `sypl` library for structured logging, providing rich context and consistent log levels throughout the codebase. Log messages include relevant information such as loader status, counters, duration, and more.
//
// 6. **Tracing**: Tracing is implemented using the `customapm` package, which integrates with Elastic APM (Application Performance Monitoring) under the hood. This enables distributed tracing of the loader's execution, allowing developers to gain deep insights into the performance and behavior of their loaders.
//
// 7. **Error Handling**: The loader package includes robust error handling mechanisms, with detailed error messages and proper propagation of errors throughout the loader's execution.
//
// 8. **Flexible Configuration**: Loaders can be configured with various options, such as custom on-finished callbacks, using a functional options pattern.
//
// 9. **Thorough Testing**: The codebase includes comprehensive unit tests, ensuring the reliability and correctness of the loader functionality. The tests cover various scenarios and validate the loader's behavior and metrics.
//
// 10. **Well-Documented**: The code is thoroughly documented, with clear comments explaining the purpose and functionality of each component. The package also includes usage examples and test cases.
//
// 11. **Idiomatic Go**: The codebase follows idiomatic Go practices, leveraging the language's features and conventions for clean and efficient code.
//
// 12. **Customizable**: The loader package provides a high level of customization through the use of interfaces and generic types. Developers can easily create custom loading functions to handle different data sources and transformations.
//
// ## Architectural Modularity and Flexibility
//
// The loader package is designed with architectural modularity and flexibility in mind. It leverages Go's interfaces and generic types to provide a highly extensible and customizable loader framework.
//
// The `ILoader` interface defines the contract for a loader, allowing for easy integration of custom loader implementations. The `Load` type enables the creation of custom loading functions that can handle various data sources and transformations.
//
// The use of generic types for `In` and `Out` allows loaders to handle different input and output data types, making the package adaptable to diverse data loading scenarios.
//
// The functional options pattern, used in the `New` factory function and configuration methods, provides a clean and flexible way to customize loader behavior without modifying the core loader struct.
package loader
