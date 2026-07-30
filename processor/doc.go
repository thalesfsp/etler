// Package processor wraps slice transforms with lifecycle metrics, tracing,
// logging, pause handling, and completion callbacks.
//
// A processor honors a pipeline pause controller carried by its context before
// invoking its transform. A stage interprets the async flag, runs the processor
// in a goroutine, and does not forward its output. Transforms must not mutate
// their input slice or pointed-to values.
package processor
