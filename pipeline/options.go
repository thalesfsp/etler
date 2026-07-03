package pipeline

import (
	"context"

	"github.com/thalesfsp/etler/v3/task"
)

//////
// Consts, vars and types.
//////

// Func allows to specify message's options.
type Func[ProcessedData, ConvertedOut any] func(p IPipeline[ProcessedData, ConvertedOut]) IPipeline[ProcessedData, ConvertedOut]

// OnFinished is the function that is called when the pipeline finishes its
// execution. `originalTask` is the task built from the input data; `tasksOut`
// holds the per-stage results — one task per stage, in stage order (for
// sequential pipelines the final task is the last element).
type OnFinished[ProcessedData, ConvertedOut any] func(ctx context.Context, p IPipeline[ProcessedData, ConvertedOut], originalTask task.Task[ProcessedData, ConvertedOut], tasksOut []task.Task[ProcessedData, ConvertedOut])

//////
// Built-ProcessedData options.
//////

// WithOnFinished sets the OnFinished function.
func WithOnFinished[ProcessedData, ConvertedOut any](onFinished OnFinished[ProcessedData, ConvertedOut]) Func[ProcessedData, ConvertedOut] {
	return func(p IPipeline[ProcessedData, ConvertedOut]) IPipeline[ProcessedData, ConvertedOut] {
		p.SetOnFinished(onFinished)

		return p
	}
}
