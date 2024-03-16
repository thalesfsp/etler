package converter

import (
	"context"

	"github.com/thalesfsp/etler/v2/internal/shared"
)

// IConverter defines what a `Conveter` must do.
type IConverter[In, Out any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In, Out]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In, Out])

	// Run the stage function.
	Run(ctx context.Context, in In) (Out, error)
}
