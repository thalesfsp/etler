package pipeline

import (
	"context"

	"github.com/thalesfsp/etler/v4/task"
)

//////
// Consts, vars and types.
//////

// Func configures a pipeline and returns the implementation to retain.
type Func[ProcessedData, ConvertedOut any] func(p IPipeline[ProcessedData, ConvertedOut]) IPipeline[ProcessedData, ConvertedOut]

// OnFinished runs after a pipeline succeeds. originalTask is built from the
// input data; tasksOut contains one result per stage in stage order.
type OnFinished[ProcessedData, ConvertedOut any] func(ctx context.Context, p IPipeline[ProcessedData, ConvertedOut], originalTask task.Task[ProcessedData, ConvertedOut], tasksOut []task.Task[ProcessedData, ConvertedOut])

//////
// Built-ProcessedData options.
//////

// WithOnFinished returns a Func that sets the completion callback.
func WithOnFinished[ProcessedData, ConvertedOut any](onFinished OnFinished[ProcessedData, ConvertedOut]) Func[ProcessedData, ConvertedOut] {
	return func(p IPipeline[ProcessedData, ConvertedOut]) IPipeline[ProcessedData, ConvertedOut] {
		p.SetOnFinished(onFinished)

		return p
	}
}
