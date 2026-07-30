// Package pipeline composes stages into an observable ETL pipeline.
//
// Sequential pipelines feed each stage's full task into the next stage.
// Concurrent pipelines give every stage the original task. Both modes return
// one result per stage in declaration order.
//
// A pipeline can pause processors before their next execution. A single
// Pipeline instance must not be run concurrently with itself.
package pipeline
