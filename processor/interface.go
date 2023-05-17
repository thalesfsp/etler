package processor

import (
	"context"

	"github.com/thalesfsp/etler/internal/shared"
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

	// Run the transform function.
	Run(ctx context.Context, in []In) (out []In, err error)
}
