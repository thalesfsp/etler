// Package stage groups processors with a converter.
//
// A stage runs synchronous processors in order and converts the resulting
// items concurrently. Async processors run on shallow input copies; their
// output is not forwarded, but the stage waits for them and propagates their
// errors. Converters must be concurrency-safe, processors must not mutate
// input or pointed-to values, and a single Stage must not be run concurrently
// with itself.
package stage
