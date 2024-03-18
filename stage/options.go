package stage

import (
	"context"

	"github.com/thalesfsp/etler/v2/task"
)

//////
// Consts, vars and types.
//////

// Func allows to specify message's options.
type Func[ProcessedData, ConvertedOut any] func(p IStage[ProcessedData, ConvertedOut]) IStage[ProcessedData, ConvertedOut]

// OnFinished is the function that is called when a processor finishes its
// execution.
type OnFinished[ProcessedData, ConvertedOut any] func(ctx context.Context, s IStage[ProcessedData, ConvertedOut], tskIn task.Task[ProcessedData, ConvertedOut], tskOut task.Task[ProcessedData, ConvertedOut])

//////
// Built-in options.
//////

// WithOnFinished sets the OnFinished function.
func WithOnFinished[ProcessedData, ConvertedOut any](onFinished OnFinished[ProcessedData, ConvertedOut]) Func[ProcessedData, ConvertedOut] {
	return func(p IStage[ProcessedData, ConvertedOut]) IStage[ProcessedData, ConvertedOut] {
		p.SetOnFinished(onFinished)

		return p
	}
}
