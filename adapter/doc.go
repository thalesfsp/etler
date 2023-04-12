// Package adapter abstracts a data source. An `Adapter` is an abstraction of a
// Data Source. Example of adatpers would be ElasticSearch, Redis, MongoDB, and
// SQL. All adapters should implement the same interface (IAdapter). The data -
// in, or out should be generic. Adapters should have a `name`, `description`.
// It should be able to read and write data.
package adapter
