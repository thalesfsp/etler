package shared

import "context"

// Run is a function that transforms the data (`in`). It returns the
// transformed data and any errors that occurred during processing.
type Run[C any] func(ctx context.Context, in []C) (out []C, err error)
