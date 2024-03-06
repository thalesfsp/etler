package pipeline

import (
	"context"

	"github.com/thalesfsp/etler/v2/internal/shared"
	"github.com/thalesfsp/status"
)

// IPipeline defines what a `Pipeline` must do.
type IPipeline[In, Out any] interface {
	shared.IMeta

	shared.IMetrics

	// GetPaused returns the Paused status.
	GetPaused() status.Status

	// SetPaused sets the Paused status.
	SetPaused()

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In, Out]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In, Out])

	// Run the pipeline.
	Run(ctx context.Context, in []In) (out []Out, err error)
}
