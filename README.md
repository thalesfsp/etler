# ETL Pipeline

This is an ETL (extract, transform, load) pipeline project that allows developers to build data pipelines using Go code. The pipeline can be used to read data from various sources, transform it, and write it to a destination.

## Features

- Support for various data sources and destinations using adapters
- Concurrent or sequential processing of data through stages
- Context propagation through stages for request-scoped data

## Getting Started

To use the pipeline in your project, you will need to define the stages that make up your pipeline. A stage is a function that transforms a slice of values of any type and returns the transformed slice and any errors that occurred during processing.

```go
type Stage[In any, Out any] func(in []In) (out []Out, err error)
```

You can then create a pipeline by calling the Pipeline function and passing in the input data, a flag to indicate whether the stages should be run concurrently or sequentially, and the stages to be run:

```go
out, err := Pipeline(in, concurrent, stage1, stage2, stage3)
```

## Adapters

In order to read from or write to a particular data source or destination, you will need to use an adapter. The pipeline provides a number of built-in adapters for common data sources and destinations, such as ElasticSearch, Redis, and AWS S3.

To use an adapter, you will need to create an instance of the adapter and pass it to the appropriate stage in your pipeline. For example, to read data from ElasticSearch and write it to Redis:

```go
client, err := elastic.NewClient(...)
if err != nil {
	// handle error
}
elasticAdapter := NewElasticSearchAdapter(client, "index", "type")

client := redis.NewClient(...)
redisAdapter := NewRedisAdapter(client, "key")

out, err := Pipeline(nil, false, elasticAdapter.Read, redisAdapter.Upsert)
```

## Custom Adapters

In addition to the built-in adapters, you can also create your own custom adapters for data sources and destinations that are not supported out of the box. To create a custom adapter, you will need to implement the Adapter interface:

```go
type Adapter[C any] interface {
	Read(ctx context.Context, query interface{}) ([]C, error)
	Upsert(ctx context.Context, data []C) error
}
```

You can then use your custom adapter just like any of the built-in adapters.
