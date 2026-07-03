package processor

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v3/internal/shared"
)

//////
// Consts, vars and types.
//////

// IProcessor defines what a `Processor` must do.
type IProcessor[ProcessingData any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[ProcessingData]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[ProcessingData])

	// GetCounterProcessed returns the `CounterProcessed` variable.
	GetCounterInterrupted() *expvar.Int

	// SetAsync if set will run the processor in a go routine.
	//
	// WARN: The output of the processing will not be forwarded!
	SetAsync(async bool)

	// GetAsync returns if the processor is running in a go routine.
	GetAsync() bool

	// Run the transform function.
	Run(ctx context.Context, processingData []ProcessingData) (processedOut []ProcessingData, err error)
}
