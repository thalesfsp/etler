package pipeline

import (
	"context"
	"expvar"
	"fmt"
	"sync"
	"time"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/etler/v4/internal/customapm"
	"github.com/thalesfsp/etler/v4/internal/logging"
	"github.com/thalesfsp/etler/v4/internal/metrics"
	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/etler/v4/stage"
	"github.com/thalesfsp/etler/v4/task"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type identifies pipeline entities.
const Type = "pipeline"

// Pipeline runs a configured sequence of stages and records lifecycle metrics.
type Pipeline[ProcessedData any, ConvertedOut any] struct {
	// ConcurrentStage determines whether all stages receive the original task
	// concurrently or each stage receives the preceding stage's output.
	ConcurrentStage bool `json:"concurrentStage"`

	// Logger records pipeline lifecycle events.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Description describes the pipeline.
	Description string `json:"description"`

	// Name identifies the pipeline.
	Name string `json:"name" validate:"required"`

	// onFinished is called after a pipeline succeeds.
	onFinished OnFinished[ProcessedData, ConvertedOut] `json:"-"`

	// Stages contains the stages run by the pipeline, in declaration order.
	Stages []stage.IStage[ProcessedData, ConvertedOut] `json:"stages" validate:"required,gt=0"`

	// Lifecycle and progress metrics.
	CounterCreated *expvar.Int `json:"counterCreated"`
	CounterRunning *expvar.Int `json:"counterRunning"`
	CounterFailed  *expvar.Int `json:"counterFailed"`
	CounterDone    *expvar.Int `json:"counterDone"`

	CreatedAt       time.Time      `json:"createdAt"`
	Duration        *expvar.Int    `json:"duration"`
	Progress        *expvar.Int    `json:"progress"`
	ProgressPercent *expvar.String `json:"progressPercent"`
	Status          *expvar.String `json:"status"`

	// mu guards onFinished.
	mu sync.RWMutex

	// pause is this pipeline's pause controller. Pausing one pipeline does
	// not affect any other.
	pause *shared.PauseController
	// pauseMu coordinates pause status and controller transitions.
	pauseMu           sync.Mutex
	statusBeforePause string
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

// GetCounterCreated returns the pipeline's created counter.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterCreated() *expvar.Int {
	return p.CounterCreated
}

// GetCounterRunning returns the pipeline's running counter.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterRunning() *expvar.Int {
	return p.CounterRunning
}

// GetCounterFailed returns the pipeline's failed counter.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterFailed() *expvar.Int {
	return p.CounterFailed
}

// GetCounterDone returns the pipeline's done counter.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCounterDone() *expvar.Int {
	return p.CounterDone
}

// GetStatus returns the pipeline's visible lifecycle status. It reports Paused
// whenever GetPaused reports Paused, even if a run reaches Done or Failed while
// paused.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetStatus() *expvar.String {
	return p.Status
}

// setStatus coordinates lifecycle status transitions with this pipeline's
// pause controller.
func (p *Pipeline[ProcessedData, ConvertedOut]) setStatus(next status.Status) {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()

	if p.pause.Paused() {
		p.statusBeforePause = next.String()
		p.GetStatus().Set(status.Paused.String())

		return
	}

	p.GetStatus().Set(next.String())
}

// onError updates pipeline failure observability while preserving the visible
// paused status.
func (p *Pipeline[ProcessedData, ConvertedOut]) onError(
	tracedContext context.Context,
	err error,
	message string,
) error {
	p.setStatus(status.Failed)

	return customapm.TraceError(
		tracedContext,
		customerror.NewFailedToError(
			message,
			customerror.WithError(err),
			customerror.WithField(Type, p.GetName()),
		),
		p.GetLogger(),
		p.GetCounterFailed(),
	)
}

// GetPaused returns Paused while this pipeline's pause controller is paused and
// Runnning otherwise. Its paused state always agrees with GetStatus.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetPaused() status.Status {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()

	if p.pause.Paused() {
		return status.Paused
	}

	return status.Runnning
}

// SetPause pauses or resumes this pipeline. Setting the current state is a
// no-op. Processors stop before their next execution; if a run finishes while
// paused, Status remains Paused until resume restores Done or Failed. Otherwise
// resume restores Runnning.
func (p *Pipeline[ProcessedData, ConvertedOut]) SetPause(state bool) {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()

	if state {
		if p.pause.Paused() {
			return
		}

		p.statusBeforePause = p.GetStatus().Value()
		p.GetStatus().Set(status.Paused.String())

		p.pause.Pause()

		return
	}

	if p.pause.Paused() {
		if p.GetStatus().Value() == status.Paused.String() {
			// Updates the pipeline's status.
			switch p.statusBeforePause {
			case status.Done.String(), status.Failed.String():
				p.GetStatus().Set(p.statusBeforePause)
			default:
				p.GetStatus().Set(status.Runnning.String())
			}
		}

		p.statusBeforePause = ""
		p.pause.Resume()
	}
}

// GetOnFinished returns the `OnFinished` function.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetOnFinished() OnFinished[ProcessedData, ConvertedOut] {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.onFinished
}

// SetOnFinished sets the `OnFinished` function.
func (p *Pipeline[ProcessedData, ConvertedOut]) SetOnFinished(onFinished OnFinished[ProcessedData, ConvertedOut]) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.onFinished = onFinished
}

// GetType returns the entity type.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetType() string {
	return Type
}

// GetCreatedAt returns the created at time.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetDuration returns the pipeline's most recent run duration in milliseconds.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetDuration() *expvar.Int {
	return p.Duration
}

// GetMetrics returns a snapshot of the pipeline's lifecycle metrics as strings.
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

// UpdateObservability marks a run done, records its duration, invokes the
// callback, and logs its metrics. tasksOut contains one result per stage in
// stage order.
func (p *Pipeline[ProcessedData, ConvertedOut]) UpdateObservability(
	ctx context.Context,
	now time.Time,
	originalTask task.Task[ProcessedData, ConvertedOut],
	tasksOut []task.Task[ProcessedData, ConvertedOut],
) {
	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	p.setStatus(status.Done)

	p.GetCounterDone().Add(1)

	p.GetDuration().Set(time.Since(now).Milliseconds())

	onFinished := p.GetOnFinished()
	if onFinished != nil {
		onFinished(ctx, p, originalTask, tasksOut)
	}

	if logging.Admits(p.GetLogger(), level.Debug) {
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
}

// GetProgress returns the number of stages completed in the current or most
// recent run.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetProgress() *expvar.Int {
	return p.Progress
}

// GetProgressPercent returns the completed-stage percentage.
func (p *Pipeline[ProcessedData, ConvertedOut]) GetProgressPercent() *expvar.String {
	return p.ProgressPercent
}

// SetProgressPercent recalculates the completed-stage percentage.
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

// Run executes the pipeline. Sequential mode feeds each stage's full task into
// the next; concurrent mode gives every stage the original task. Both modes
// return one task per stage in stage order. A pipeline instance must not run
// concurrently with itself; separate instances run independently.
func (p *Pipeline[ProcessedData, ConvertedOut]) Run(ctx context.Context, processingData []ProcessedData) ([]task.Task[ProcessedData, ConvertedOut], error) {
	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	tracedContext, ender := customapm.Trace(
		ctx,
		Type,
		p.GetName(),
		status.Runnning,
		p.GetLogger(),
		p.CounterRunning,
	)
	defer ender.End()

	now := time.Now()
	durationSet := false

	defer func() {
		if !durationSet {
			p.GetDuration().Set(time.Since(now).Milliseconds())
		}
	}()

	// Make this pipeline's pause controller visible to its processors.
	tracedContext = shared.ContextWithPause(tracedContext, p.pause)

	// Task initialization.
	tsk, err := task.New[ProcessedData, ConvertedOut](processingData)
	if err != nil {
		p.setStatus(status.Failed)

		return nil, customapm.TraceError(
			tracedContext,
			err,
			p.GetLogger(),
			p.GetCounterFailed(),
		)
	}

	// A paused pipeline keeps reporting paused — its processors are about to
	// block on the pause controller.
	p.setStatus(status.Runnning)

	if logging.Admits(p.GetLogger(), level.Trace) {
		p.GetLogger().PrintlnWithOptions(level.Trace, status.Runnning.String())
	}

	// Progress is relative to the current run.
	p.GetProgress().Set(0)

	p.SetProgressPercent()

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

			return nil, p.onError(tracedContext, errs, "run stage")
		}

		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		// Recompute the final percentage once: the per-stage goroutines'
		// read-format-set sequences can interleave and leave a stale value.
		p.SetProgressPercent()

		p.UpdateObservability(ctx, now, originalTask, stagesOut)
		durationSet = true

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

			return nil, p.onError(tracedContext, err, "run stage")
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
	durationSet = true

	return tasksOut, nil
}

//////
// Factory.
//////

// New creates a pipeline with at least one stage. concurrentStage selects
// whether stages receive the original task concurrently or run as a chain.
func New[ProcessedData, ConvertedOut any](
	name string,
	description string,
	concurrentStage bool,
	stages ...stage.IStage[ProcessedData, ConvertedOut],
) (IPipeline[ProcessedData, ConvertedOut], error) {
	// Validate the raw name before constructing metrics because published
	// expvars cannot be removed.
	if err := validation.Validate(struct {
		Name string `validate:"required"`
	}{Name: name}); err != nil {
		return nil, err
	}

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
