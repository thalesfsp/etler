package loader

import (
	"context"
)

//////
// Consts, vars and types.
//////

// Func configures a loader and returns the implementation to retain. An option
// must return a usable implementation; returning one whose embedded interface
// is nil will panic when it is used, as it would anywhere in Go.
type Func[In, Out any] func(p ILoader[In, Out]) ILoader[In, Out]

// OnFinished runs after a load succeeds with its input and output.
type OnFinished[In, Out any] func(ctx context.Context, c ILoader[In, Out], originalIn In, convertedOut Out)

//////
// Built-in options.
//////

// WithOnFinished returns a Func that sets the completion callback.
func WithOnFinished[In, Out any](onFinished OnFinished[In, Out]) Func[In, Out] {
	return func(p ILoader[In, Out]) ILoader[In, Out] {
		p.SetOnFinished(onFinished)

		return p
	}
}
