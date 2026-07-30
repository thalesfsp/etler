package loader

import (
	"context"

	"github.com/thalesfsp/etler/v4/internal/shared"
)

// ILoader defines loader lifecycle, metrics, and execution.
type ILoader[In, Out any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In, Out]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In, Out])

	// Run loads data from in.
	Run(ctx context.Context, in In) (Out, error)
}
