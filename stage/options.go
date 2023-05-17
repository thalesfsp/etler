package stage

import (
	"context"
)

//////
// Consts, vars and types.
//////

// Func allows to specify message's options.
type Func[In, Out any] func(p IStage[In, Out]) IStage[In, Out]

// OnFinished is the function that is called when a processor finishes its
// execution.
type OnFinished[In, Out any] func(ctx context.Context, p IStage[In, Out], in []In, processedOut []Out)

//////
// Built-in options.
//////

// WithOnFinished sets the OnFinished function.
func WithOnFinished[In, Out any](onFinished OnFinished[In, Out]) Func[In, Out] {
	return func(p IStage[In, Out]) IStage[In, Out] {
		p.SetOnFinished(onFinished)

		return p
	}
}
