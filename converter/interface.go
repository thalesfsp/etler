package converter

import (
	"context"
	"io"

	"github.com/thalesfsp/etler/internal/shared"
)

// IConverter defines what a `Conveter` must do.
type IConverter[T any] interface {
	shared.IMeta

	shared.IMetrics

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[T]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[T])

	// Run the converter function.
	Run(ctx context.Context, r io.Reader) (T, error)
}
