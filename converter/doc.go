// Package converter provides a framework for creating converters.
//
// # Conveters
//
// ## Overview
//
// The `conveters` package is a core component of the ETL project, providing a versatile and efficient mechanism for defining and executing data conversion tasks within a pipeline. A conveter is responsible for transforming data from one format to another, enabling seamless integration between different stages and ensuring data compatibility throughout the pipeline.
//
// ## What's a Conveter?
//
// A conveter is a self-contained unit of data conversion within a pipeline. It encapsulates a specific conversion logic that transforms input data of type `In` into output data of type `Out`. Conveters are highly reusable and can be easily plugged into different stages of the pipeline.
//
// The conveter package provides a generic interface `IConverter` and a concrete implementation `Converter` that can be instantiated with custom conversion functions.
//
// ## How It Works
//
// The core of the `conveters` package revolves around the `IConverter` interface and the `Converter` struct. The `IConverter` interface defines the contract for a conveter, specifying methods for running the conversion, managing metrics, and handling callbacks.
//
// To create a conveter, you instantiate a new `Converter` using the `New` factory function, specifying the conveter name, description, and the conversion function. The conversion function is defined as `Convert[In, Out]`, where `In` represents the input data type and `Out` represents the output data type.
//
// When the `Run` method is called on a conveter, it executes the following steps:
//
// 1. It sets up observability, including tracing, metrics, status, and logging, to monitor and track the conveter's execution.
//
// 2. It invokes the conversion function, passing the input data and receiving the converted output data. If an error occurs during the conversion, it is handled appropriately.
//
// 3. After the conversion is complete, it updates the observability metrics, such as counters for created, running, failed, and done conveters, as well as the duration and status.
//
// 4. If an `OnFinished` callback is defined, it is invoked with the original input data and the converted output data, allowing for custom post-conversion processing.
//
// 5. Finally, the conveter returns the converted output data.
//
// ## Features
//
// 1. **Generic and Reusable**: Conveters are implemented using Go's generic types, allowing for flexibility in handling various data types. They can be easily reused across different stages and pipelines.
//
// 2. **Customizable Conversion Logic**: The conversion logic is encapsulated in a custom function that can be defined when creating a conveter. This allows for tailored data transformations specific to the project's requirements.
//
// 3. **Observability**: The conveters package provides comprehensive observability features, including tracing, metrics, status tracking, and logging. This enables effective monitoring and debugging of the conversion process.
//
// 4. **Metrics**: Conveter metrics are exposed using the `expvar` package, allowing for easy integration with monitoring systems. Metrics include counters for created, running, failed, and done conveters, as well as duration and status.
//
// 5. **Callbacks**: The package supports an `OnFinished` callback function that is invoked after the conversion is complete. This callback receives the original input data and the converted output data, enabling custom post-conversion processing.
//
// 6. **Error Handling**: The conveters package includes robust error handling mechanisms, propagating errors that occur during the conversion process and providing informative error messages.
//
// 7. **Factory Functions**: The package provides factory functions `New` and `Default` for creating conveters with custom or default configurations. The `MustDefault` function is also available for creating a conveter that panics on error.
//
// 8. **Thorough Testing**: The codebase includes comprehensive unit tests to ensure the correctness and reliability of the conveters package. The tests cover various scenarios and validate the conveter's behavior and metrics.
package converter
