package stage

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v2/internal/shared"
	"github.com/thalesfsp/etler/v2/task"
)

// IStage defines what a `Stage` must do.
type IStage[ProcessedData, ConvertedOut any] interface {
	shared.IMeta

	shared.IMetrics

	// GetProgress returns the `CounterProgress` of the stage.
	GetProgress() *expvar.Int

	// GetProgressPercent returns the `ProgressPercent` of the stage.
	GetProgressPercent() *expvar.String

	// SetProgressPercent sets the `ProgressPercent` of the stage.
	SetProgressPercent()

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[ProcessedData, ConvertedOut]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[ProcessedData, ConvertedOut])

	// Run the stage function.
	Run(context.Context, task.Task[ProcessedData, ConvertedOut]) (task.Task[ProcessedData, ConvertedOut], error)
}
