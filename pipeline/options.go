package pipeline

import (
	"context"
)

//////
// Consts, vars and types.
//////

// Func allows to specify message's options.
type Func[In, Out any] func(p IPipeline[In, Out]) IPipeline[In, Out]

// OnFinished is the function that is called when a processor finishes its
// execution.
type OnFinished[In, Out any] func(ctx context.Context, p IPipeline[In, Out], in []In, processedOut []Out)

//////
// Built-in options.
//////

// WithOnFinished sets the OnFinished function.
func WithOnFinished[In, Out any](onFinished OnFinished[In, Out]) Func[In, Out] {
	return func(p IPipeline[In, Out]) IPipeline[In, Out] {
		p.SetOnFinished(onFinished)

		return p
	}
}
