package converter

import (
	"context"
	"io"
)

//////
// Consts, vars and types.
//////

// Func allows to specify message's options.
type Func[T any] func(p IConverter[T]) IConverter[T]

// OnFinished is the function that is called when a processor finishes its
// execution.
type OnFinished[T any] func(ctx context.Context, c IConverter[T], r io.Reader, processed []T)

//////
// Built-in options.
//////

// WithOnFinished sets the OnFinished function.
func WithOnFinished[T any](onFinished OnFinished[T]) Func[T] {
	return func(p IConverter[T]) IConverter[T] {
		p.SetOnFinished(onFinished)

		return p
	}
}
