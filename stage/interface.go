package stage

import (
	"context"

	"github.com/thalesfsp/etler/internal/shared"
)

// IStage defines what a `Stage` must do.
type IStage[In, Out any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In, Out]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In, Out])

	// Run the stage function.
	Run(ctx context.Context, in []In) (out []Out, err error)
}
