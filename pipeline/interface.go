package pipeline

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v3/internal/shared"
	"github.com/thalesfsp/etler/v3/task"
	"github.com/thalesfsp/status"
)

// IPipeline defines what a `Pipeline` must do.
type IPipeline[ProcessedData, ConvertedOut any] interface {
	shared.IMeta

	shared.IMetrics

	// GetProgress returns the `Progress` of the pipeline.
	GetProgress() *expvar.Int

	// GetProgressPercent returns the `ProgressPercent` of the pipeline.
	GetProgressPercent() *expvar.String

	// SetProgressPercent sets the `ProgressPercent` of the pipeline.
	SetProgressPercent()

	// GetPaused returns the Paused status.
	GetPaused() status.Status

	// SetPause the pipeline.
	SetPause(state bool)

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[ProcessedData, ConvertedOut]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[ProcessedData, ConvertedOut])

	// Run the pipeline.
	Run(ctx context.Context, processedData []ProcessedData) ([]task.Task[ProcessedData, ConvertedOut], error)
}
