package processor

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v4/internal/shared"
)

//////
// Consts, vars and types.
//////

// IProcessor defines processor lifecycle, metrics, configuration, and
// execution. Implementations must not mutate Run's input slice or the values it
// points to.
type IProcessor[ProcessingData any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[ProcessingData]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[ProcessingData])

	// GetCounterInterrupted returns the counter for paused runs interrupted by
	// context cancellation or deadline.
	GetCounterInterrupted() *expvar.Int

	// SetAsync controls whether a stage runs the processor in a goroutine. It
	// changes scheduling only; Run still records lifecycle status and counters.
	//
	// WARN: The output of the processing will not be forwarded!
	SetAsync(async bool)

	// GetAsync reports whether a stage should run the processor in a goroutine.
	GetAsync() bool

	// Run transforms processingData without mutating it or pointed-to values.
	// It records context termination while paused as an interruption and
	// transform errors as failures.
	Run(ctx context.Context, processingData []ProcessingData) (processedOut []ProcessingData, err error)
}
