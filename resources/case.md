# Overview

As a senior software developer, I'm designing and implementing an ETL solution.

## Software language

It should be written in Golang.

## Architecture

### Adapter - a Data source

An `Adapter` is an abstraction of a Data Source. Example of adatpers would be ElasticSearch, Redis, MongoDB, and SQL. All adapter should implement the same interface. The data - in, or out should be of generic type. The `Adapter` struct should have a `name`, `description` and any additional information required to connect, read and write data.

### Processor - Data transformation

A `Processor` is responsible to transform the data. Processing can be synchronous, or asynchronous. The `Processor` struct should have a `name`, `description`, a `concurrent` flag, a `critical` flag, and a `function` which process the data. If any error occurs:

- If the `critical` flag is set, the pipeline should stop and return the error
- Should print the error message to `stderr`
- Should increment the Error Metric

### Pipeline

- A pipeline should be able to run processors in a specific order.
- A pipeline orchestrate one or more processors in an order - if specified, synchronously and asynchronously
- A pipeline ingest data from an adapter, and output data to an adapter.
- A pipeline should have the ability to be paused, resumed, or canceled
- A pipeline should be able to provide its status which includes:
  - Current state: `running`, `paused`, `canceled`, `completed`
  - Current processor
  - Current progress

#### Diagram

An example of the pipeline:

```
Adapter -> [Processor 1, Processor 2, Processor 3] -> [Processor 4] -> [Processor 5] -> Adapter
```
