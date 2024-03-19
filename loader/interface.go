package loader

import (
	"context"

	"github.com/thalesfsp/etler/v2/internal/shared"
)

// ILoader defines what a `Conveter` must do.
type ILoader[In, Out any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In, Out]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In, Out])

	// Run the stage function.
	Run(ctx context.Context, in In) (Out, error)
}
