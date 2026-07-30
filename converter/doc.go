// Package converter wraps value conversions with lifecycle metrics, tracing,
// logging, and completion callbacks.
//
// Stages call converters concurrently for individual items, so converter
// implementations, conversion functions, and callbacks must be safe for
// concurrent use.
package converter
