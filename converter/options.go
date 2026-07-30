package converter

import (
	"context"
)

//////
// Consts, vars and types.
//////

// Func configures a converter and returns the implementation to retain.
type Func[In, Out any] func(p IConverter[In, Out]) IConverter[In, Out]

// OnFinished runs after a conversion succeeds. It may be called concurrently.
type OnFinished[In, Out any] func(ctx context.Context, c IConverter[In, Out], originalIn In, convertedOut Out)

//////
// Built-in options.
//////

// WithOnFinished returns a Func that sets the completion callback.
func WithOnFinished[In, Out any](onFinished OnFinished[In, Out]) Func[In, Out] {
	return func(p IConverter[In, Out]) IConverter[In, Out] {
		p.SetOnFinished(onFinished)

		return p
	}
}
