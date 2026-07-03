package pipeline

import (
	"context"
	"expvar"
	"fmt"
	"time"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/etler/v3/internal/customapm"
	"github.com/thalesfsp/etler/v3/internal/logging"
	"github.com/thalesfsp/etler/v3/internal/metrics"
	"github.com/thalesfsp/etler/v3/internal/shared"
	"github.com/thalesfsp/etler/v3/stage"
	"github.com/thalesfsp/etler/v3/task"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "pipeline"

// Pipeline definition.
type Pipeline[ProcessedData any, ConvertedOut any] struct {
	// Concurrent determines whether the stage should be run concurrently.
	ConcurrentStage bool `json:"concurrentStage"`

	// Logger is the internal logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Description of the processor.
	Description string `json:"description"`

	// Name of the processor.
	Name string `json:"name" validate:"required"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[ProcessedData, ConvertedOut] `json:"-"`

	// Stages to be used ProcessedData the pipeline.
	Stages []stage.IStage[ProcessedData, ConvertedOut] `json:"stages" validate:"required,gt=0"`

	// Metrics.
	CounterCreated *expvar.Int `json:"counterCreated"`
	CounterRunning *expvar.Int `json:"counterRunning"`
	CounterFailed  *expvar.Int `json:"counterFailed"`
	CounterDone    *expvar.Int `json:"counterDone"`

	CreatedAt       time.Time      `json:"createdAt"`
	Duration        *expvar.Int    `json:"duration"`
	Progress        *expvar.Int    `json:"progress"`
	ProgressPercent *expvar.String `json:"progressPercent"`
	Status          *expvar.String `json:"status"`

	// pause is this pipeline's pause controller. Pausing one pipeline does
	// not affect any other.
	pause *shared.PauseController
}

//////
// Methods.
//////

// GetDescription returns the `Description` of the pipeline.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetDescription() string {
	return p.Description
}

// GetLogger returns the `Logger` of the pipeline.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetLogger() sypl.ISypl {
	return p.Logger
}

// GetName returns the `Name` of the pipeline.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetName() string {
	return p.Name
}

// GetCounterCreated returns the `CounterCreated` of the processor.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterCreated() *expvar.Int {
	return p.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` of the processor.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterRunning() *expvar.Int {
	return p.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` of the processor.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterFailed() *expvar.Int {
	return p.CounterFailed
}

// GetCounterDone returns the `CounterDone` of the processor.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterDone() *expvar.Int {
	return p.CounterDone
}

// GetStatus returns the `Status` metric.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetStatus() *expvar.String {
	return p.Status
}

// GetPaused returns the Paused status of THIS pipeline.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetPaused() status.Status {
	if p.pause.Paused() {
		return status.Paused
	}

	return status.Runnning
}

// SetPause sets the Paused status of THIS pipeline. Its processors pause
// before their next execution and resume immediately on unpause.
func (p *Pipeline[ProcessedData, ConvertedOut]) SetPause(state bool) {
	if state {
		p.GetStatus().Set(status.Paused.String())

		p.pause.Pause()

		return
	}

	// Updates the pipeline's status.
	p.GetStatus().Set(status.Runnning.String())

	p.pause.Resume()
}

// GetOnFinished returns the `OnFinished` function.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetOnFinished() OnFinished[ProcessedData, ConvertedOut] {
	return p.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (p *Pipeline[ProcessedData, ConvertedOut]) SetOnFinished(onFinished OnFinished[ProcessedData, ConvertedOut]) {
	p.OnFinished = onFinished
}

// GetType returns the entity type.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetType() string {
	return Type
}

// GetCreatedAt returns the created at time.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetDuration returns the `CounterDuration` of the stage.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetDuration() *expvar.Int {
	return p.Duration
}

// GetMetrics returns the stage's metrics.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetMetrics() map[string]string {
	return map[string]string{
		"createdAt":      p.GetCreatedAt().String(),
		"counterCreated": p.GetCounterCreated().String(),
		"counterDone":    p.GetCounterDone().String(),
		"counterFailed":  p.GetCounterFailed().String(),
		"counterRunning": p.GetCounterRunning().String(),
		"duration":       p.GetDuration().String(),
		"status":         p.GetStatus().String(),
	}
}

// UpdateObservability updates the observability of the pipeline. `tasksOut`
// is the per-stage results — one task per stage, in stage order.
func (p *Pipeline[ProcessedData, ConvertedOut]) UpdateObservability(
	ctx context.Context,
	now time.Time,
	originalTask task.Task[ProcessedData, ConvertedOut],
	tasksOut []task.Task[ProcessedData, ConvertedOut],
) {
	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	p.GetStatus().Set(status.Done.String())

	p.GetCounterDone().Add(1)

	p.GetDuration().Set(time.Since(now).Milliseconds())

	if p.GetOnFinished() != nil {
		p.GetOnFinished()(ctx, p, originalTask, tasksOut)
	}

	p.GetLogger().PrintWithOptions(
		level.Debug,
		status.Done.String(),
		sypl.WithField("createdAt", p.GetCreatedAt().String()),
		sypl.WithField("counterCreated", p.GetCounterCreated().String()),
		sypl.WithField("counterDone", p.GetCounterDone().String()),
		sypl.WithField("counterFailed", p.GetCounterFailed().String()),
		sypl.WithField("counterRunning", p.GetCounterRunning().String()),
		sypl.WithField("duration", p.GetDuration().String()),
		sypl.WithField("progress", p.GetProgress().String()),
		sypl.WithField("progressPercent", p.GetProgressPercent().String()),
		sypl.WithField("status", p.GetStatus().String()),
	)
}

// GetProgress returns the `CounterProgress` of the stage.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetProgress() *expvar.Int {
	return p.Progress
}

// GetProgressPercent returns the `ProgressPercent` of the stage.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetProgressPercent() *expvar.String {
	return p.ProgressPercent
}

// SetProgressPercent sets the `ProgressPercent` of the stage.
func (p *Pipeline[ProcessedData, ConvertedOut]) SetProgressPercent() {
	currentProgress := p.GetProgress().Value()

	totalProgress := len(p.Stages)
	if totalProgress == 0 {
		p.GetProgressPercent().Set("0%")

		return
	}

	percentage := float64(currentProgress) / float64(totalProgress) * 100

	p.GetProgressPercent().Set(fmt.Sprintf("%d%%", int(percentage)))
}

// Run the pipeline.
func (p *Pipeline[ProcessedData, ConvertedOut]) Run(ctx context.Context, processingData []ProcessedData) ([]task.Task[ProcessedData, ConvertedOut], error) {
	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	tracedContext, span := customapm.Trace(
		ctx,
		Type,
		p.GetName(),
		status.Runnning,
		p.GetLogger(),
		p.CounterRunning,
	)
	defer span.End()

	// Make this pipeline's pause controller visible to its processors.
	tracedContext = shared.ContextWithPause(tracedContext, p.pause)

	// Task initialization.
	tsk, err := task.New[ProcessedData, ConvertedOut](processingData)
	if err != nil {
		return nil, customapm.TraceError(
			tracedContext,
			err,
			p.GetLogger(),
			p.GetCounterFailed(),
		)
	}

	// A paused pipeline keeps reporting paused — its processors are about to
	// block on the pause controller.
	if !p.pause.Paused() {
		p.GetStatus().Set(status.Runnning.String())
	}

	p.GetLogger().PrintlnWithOptions(level.Trace, status.Runnning.String())

	// Progress is relative to the current run.
	p.GetProgress().Set(0)

	p.SetProgressPercent()

	now := time.Now()

	//////
	// Run the pipeline.
	//////

	// Store as reference to be used ProcessedData the OnFinished function.
	originalTask := tsk

	// Store as reference to be used as the input of the next stage.
	retroFeedIn := originalTask

	if p.ConcurrentStage {
		stagesOut, errs := concurrentloop.Map(tracedContext, p.Stages, func(ctx context.Context, s stage.IStage[ProcessedData, ConvertedOut]) (task.Task[ProcessedData, ConvertedOut], error) {
			stageOut, err := s.Run(tracedContext, originalTask)
			if err != nil {
				// The stage already traced, logged, and counted its own
				// failure. The pipeline-level handling happens once, below,
				// so the failure isn't double counted.
				return task.Task[ProcessedData, ConvertedOut]{}, err
			}

			//////
			// Observability: tracing, metrics, status, logging, etc.
			//////

			// Increment the progress.
			p.GetProgress().Add(1)

			// Set the progress percentage.
			//
			// NOTE: MUST BE after increment the progress, as its internal
			// calculation depends on that.
			p.SetProgressPercent()

			return stageOut, nil
		}, concurrentloop.WithRemoveZeroValues(false))
		if errs != nil {
			//////
			// Observability: tracing, metrics, status, logging, etc.
			//////

			return nil, shared.OnErrorHandler(
				tracedContext,
				p,
				p.GetLogger(),
				errs,
				"run stage",
				Type,
				p.GetName(),
			)
		}

		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		// Recompute the final percentage once: the per-stage goroutines'
		// read-format-set sequences can interleave and leave a stale value.
		p.SetProgressPercent()

		p.UpdateObservability(ctx, now, originalTask, stagesOut)

		return stagesOut, nil
	}

	// Iterate through the stages, passing the output of each stage
	// as the input of the next stage. Each stage's full task (including its
	// converted data) is collected and returned — one task per stage, in
	// stage order. The final task is the last element.
	tasksOut := make([]task.Task[ProcessedData, ConvertedOut], 0, len(p.Stages))

	for _, s := range p.Stages {
		rFI, err := s.Run(tracedContext, retroFeedIn)
		if err != nil {
			//////
			// Observability: tracing, metrics, status, logging, etc.
			//////

			return nil, shared.OnErrorHandler(
				tracedContext,
				p,
				p.GetLogger(),
				err,
				"run stage",
				Type,
				p.GetName(),
			)
		}

		// Update reference to be used as the input of the next stage.
		retroFeedIn = rFI

		tasksOut = append(tasksOut, rFI)

		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		// Increment the progress.
		p.GetProgress().Add(1)

		// Set the progress percentage.
		//
		// NOTE: MUST BE after increment the progress, as its internal calculation
		// depends on that.
		p.SetProgressPercent()
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	p.UpdateObservability(ctx, now, originalTask, tasksOut)

	return tasksOut, nil
}

//////
// Factory.
//////

// New returns a new pipeline.
func New[ProcessedData, ConvertedOut any](
	name string,
	description string,
	concurrentStage bool,
	stages ...stage.IStage[ProcessedData, ConvertedOut],
) (IPipeline[ProcessedData, ConvertedOut], error) {
	p := &Pipeline[ProcessedData, ConvertedOut]{
		ConcurrentStage: concurrentStage,
		Stages:          stages,
		Logger:          logging.Get().New(name).SetTags(Type, name),
		pause:           shared.NewPauseController(),

		CreatedAt:   time.Now(),
		Name:        name,
		Description: description,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),

		Duration:        metrics.NewIntWithPattern(Type, name, "duration"),
		Progress:        metrics.NewIntWithPattern(Type, name, "progress"),
		ProgressPercent: metrics.NewStringWithPattern(Type, name, "progressPercent"),
		Status:          metrics.NewStringWithPattern(Type, name, status.Name),
	}

	// Validation.
	if err := validation.Validate(p); err != nil {
		return nil, err
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	p.GetStatus().Set(status.Created.String())

	p.GetCounterCreated().Add(1)

	p.GetLogger().PrintlnWithOptions(level.Trace, status.Created.String())

	return p, nil
}
