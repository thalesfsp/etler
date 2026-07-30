package stage

import (
	"context"

	"github.com/thalesfsp/etler/v4/task"
)

//////
// Consts, vars and types.
//////

// Func configures a stage and returns the implementation to retain.
type Func[ProcessedData, ConvertedOut any] func(p IStage[ProcessedData, ConvertedOut]) IStage[ProcessedData, ConvertedOut]

// OnFinished runs after a stage succeeds with its original and final tasks.
type OnFinished[ProcessedData, ConvertedOut any] func(ctx context.Context, s IStage[ProcessedData, ConvertedOut], tskIn task.Task[ProcessedData, ConvertedOut], tskOut task.Task[ProcessedData, ConvertedOut])

//////
// Built-in options.
//////

// WithOnFinished returns a Func that sets the completion callback.
func WithOnFinished[ProcessedData, ConvertedOut any](onFinished OnFinished[ProcessedData, ConvertedOut]) Func[ProcessedData, ConvertedOut] {
	return func(p IStage[ProcessedData, ConvertedOut]) IStage[ProcessedData, ConvertedOut] {
		p.SetOnFinished(onFinished)

		return p
	}
}
