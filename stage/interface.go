package stage

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v2/internal/shared"
)

// IStage defines what a `Stage` must do.
type IStage[In, Out any] interface {
	shared.IMeta

	shared.IMetrics

	// GetProgress returns the `CounterProgress` of the stage.
	GetProgress() *expvar.Int

	// GetProgressPercent returns the `ProgressPercent` of the stage.
	GetProgressPercent() *expvar.String

	// SetProgressPercent sets the `ProgressPercent` of the stage.
	SetProgressPercent()

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[In, Out]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[In, Out])

	// Run the stage function.
	Run(ctx context.Context, in []In) ([]Out, error)
}
