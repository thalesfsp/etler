package shared

import (
	"context"
	"sync/atomic"
)

// Paused is a flag that indicates if the pipeline is paused.
//
//nolint:revive
var Paused int32 = 0

// SetPaused sets the `Paused` flag. It's concurrency-safe.
func SetPaused(val int32) {
	atomic.StoreInt32(&Paused, val)
}

// GetPaused returns the `Paused` flag. It's concurrency-safe.
func GetPaused() int32 {
	return atomic.LoadInt32(&Paused)
}

// Run is a function that transforms the data (`in`). It returns the
// transformed data and any errors that occurred during processing.
type Run[In any] func(ctx context.Context, in []In) (out []In, err error)
