package shared

import (
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
