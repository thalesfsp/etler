package converter

import (
	"context"

	"github.com/thalesfsp/etler/v4/internal/shared"
)

// IConverter defines converter lifecycle, metrics, and execution.
// Implementations must be safe for concurrent use because stages call Run from
// multiple goroutines.
type IConverter[In, Out any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In, Out]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In, Out])

	// Run converts one input value.
	Run(ctx context.Context, in In) (Out, error)
}
