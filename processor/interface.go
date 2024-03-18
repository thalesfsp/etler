package processor

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v2/internal/shared"
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

	// Get
	GetCounterInterrupted() *expvar.Int

	// Run the transform function.
	Run(ctx context.Context, processingData []ProcessingData) (processedOut []ProcessingData, err error)
}
