package pipeline

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/etler/v4/task"
	"github.com/thalesfsp/status"
)

// IPipeline defines pipeline lifecycle, metrics, pause control, and execution.
// Concurrent Run calls on one implementation are serialized.
type IPipeline[ProcessedData, ConvertedOut any] interface {
	shared.IMeta

	shared.IMetrics

	// GetProgress returns the number of stages completed in the current or
	// most recent run.
	GetProgress() *expvar.Int

	// GetProgressPercent returns the completed-stage percentage.
	GetProgressPercent() *expvar.String

	// SetProgressPercent recalculates the completed-stage percentage.
	SetProgressPercent()

	// GetPaused returns Paused when execution is paused and Runnning otherwise.
	// Pause state is independent from GetStatus, which never returns Paused.
	GetPaused() status.Status

	// SetPause pauses or resumes execution; setting the current state is a
	// no-op.
	SetPause(state bool)

	// GetOnFinished returns the `OnFinished` function.
	GetOnFinished() OnFinished[ProcessedData, ConvertedOut]

	// SetOnFinished sets the `OnFinished` function.
	SetOnFinished(onFinished OnFinished[ProcessedData, ConvertedOut])

	// Run executes the stages and returns one task per stage in stage order.
	// Lifecycle, progress, and duration describe the active or most recently
	// completed serialized run. Completion callbacks may overlap and must be
	// concurrency-safe.
	Run(ctx context.Context, processedData []ProcessedData) ([]task.Task[ProcessedData, ConvertedOut], error)
}
