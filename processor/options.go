package processor

import (
	"context"
)

//////
// Consts, vars and types.
//////

// Func configures a processor and returns the implementation to retain.
type Func[T any] func(p IProcessor[T]) IProcessor[T]

// OnFinished runs after a processor succeeds with its original input and
// processed output.
type OnFinished[T any] func(ctx context.Context, p IProcessor[T], originalIn []T, processedOut []T)

//////
// Built-toBeProcessed options.
//////

// WithAsync returns a Func that controls goroutine execution by a stage.
//
// WARN: The output of the processing will not be forwarded!
func WithAsync[T any](async bool) Func[T] {
	return func(p IProcessor[T]) IProcessor[T] {
		p.SetAsync(async)

		return p
	}
}

// WithOnFinished returns a Func that sets the completion callback.
func WithOnFinished[T any](onFinished OnFinished[T]) Func[T] {
	return func(p IProcessor[T]) IProcessor[T] {
		p.SetOnFinished(onFinished)

		return p
	}
}
