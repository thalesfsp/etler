package stage

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/etler/v4/task"
)

// IStage defines stage lifecycle, metrics, and execution. A single
// implementation must not be run concurrently with itself.
type IStage[ProcessedData, ConvertedOut any] interface {
	shared.IMeta

	shared.IMetrics

	// GetProgress returns the number of processors completed in the current or
	// most recent run.
	GetProgress() *expvar.Int

	// GetProgressPercent returns the completed-processor percentage.
	GetProgressPercent() *expvar.String

	// SetProgressPercent recalculates the completed-processor percentage.
	SetProgressPercent()

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[ProcessedData, ConvertedOut]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[ProcessedData, ConvertedOut])

	// Run processes and converts a task.
	Run(
		ctx context.Context,
		task task.Task[ProcessedData, ConvertedOut],
	) (task.Task[ProcessedData, ConvertedOut], error)
}
