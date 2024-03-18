package pipeline

import (
	"context"

	"github.com/thalesfsp/etler/v2/task"
)

//////
// Consts, vars and types.
//////

// Func allows to specify message's options.
type Func[ProcessedData, ConvertedOut any] func(p IPipeline[ProcessedData, ConvertedOut]) IPipeline[ProcessedData, ConvertedOut]

// OnFinished is the function that is called when a processor finishes its
// execution.
type OnFinished[ProcessedData, ConvertedOut any] func(ctx context.Context, p IPipeline[ProcessedData, ConvertedOut], processedData task.Task[ProcessedData, ConvertedOut], convertedOut task.Task[ProcessedData, ConvertedOut])

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
