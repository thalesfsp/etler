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
type IProcessor[In any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In])

	// Get
	GetCounterInterrupted() *expvar.Int

	// Run the transform function.
	Run(ctx context.Context, in []In) (out []In, err error)
}
