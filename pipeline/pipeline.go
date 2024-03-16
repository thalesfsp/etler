// TODO: Add metrics, error handling, logging, context, APM, APM transaction, etc.

package pipeline

import (
	"context"
	"expvar"
	"fmt"
	"time"

	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/internal/customapm"
	"github.com/thalesfsp/etler/v2/internal/logging"
	"github.com/thalesfsp/etler/v2/internal/metrics"
	"github.com/thalesfsp/etler/v2/internal/shared"
	"github.com/thalesfsp/etler/v2/stage"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "pipeline"

// Pipeline definition.
type Pipeline[In any, Out any] struct {
	// Concurrent determines whether the stage should be run concurrently.
	ConcurrentStage bool `json:"concurrentStage"`

	// Logger is the pipeline logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Description of the processor.
	Description string `json:"description"`

	// Name of the processor.
	Name string `json:"name" validate:"required"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[In, Out] `json:"-"`

	// Stages to be used in the pipeline.
	Stages []stage.IStage[In, Out] `json:"stages"`

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
}

//////
// Methods.
//////

// GetDescription returns the `Description` of the pipeline.
func (p *Pipeline[In, Out]) GetDescription() string {
	return p.Description
}

// GetLogger returns the `Logger` of the pipeline.
func (p *Pipeline[In, Out]) GetLogger() sypl.ISypl {
	return p.Logger
}

// GetName returns the `Name` of the pipeline.
func (p *Pipeline[In, Out]) GetName() string {
	return p.Name
}

// GetCounterCreated returns the `CounterCreated` of the processor.
func (p *Pipeline[In, Out]) GetCounterCreated() *expvar.Int {
	return p.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` of the processor.
func (p *Pipeline[In, Out]) GetCounterRunning() *expvar.Int {
	return p.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` of the processor.
func (p *Pipeline[In, Out]) GetCounterFailed() *expvar.Int {
	return p.CounterFailed
}

// GetCounterDone returns the `CounterDone` of the processor.
func (p *Pipeline[In, Out]) GetCounterDone() *expvar.Int {
	return p.CounterDone
}

// GetStatus returns the `Status` metric.
func (p *Pipeline[In, Out]) GetStatus() *expvar.String {
	return p.Status
}

// GetPaused returns the Paused status.
func (p *Pipeline[In, Out]) GetPaused() status.Status {
	if shared.GetPaused() == 1 {
		return status.Paused
	}

	return status.Runnning
}

// SetPaused sets the Paused status.
func (p *Pipeline[In, Out]) SetPaused() {
	shared.SetPaused(1)
}

// GetOnFinished returns the `OnFinished` function.
func (p *Pipeline[In, Out]) GetOnFinished() OnFinished[In, Out] {
	return p.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (p *Pipeline[In, Out]) SetOnFinished(onFinished OnFinished[In, Out]) {
	p.OnFinished = onFinished
}

// GetType returns the entity type.
func (p *Pipeline[In, Out]) GetType() string {
	return Type
}

// GetCreatedAt returns the created at time.
func (p *Pipeline[In, Out]) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetDuration returns the `CounterDuration` of the stage.
func (p *Pipeline[In, Out]) GetDuration() *expvar.Int {
	return p.Duration
}

// GetMetrics returns the stage's metrics.
func (p *Pipeline[In, Out]) GetMetrics() map[string]string {
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

// GetProgress returns the `CounterProgress` of the stage.
func (p *Pipeline[In, Out]) GetProgress() *expvar.Int {
	return p.Progress
}

// GetProgressPercent returns the `ProgressPercent` of the stage.
func (p *Pipeline[In, Out]) GetProgressPercent() *expvar.String {
	return p.ProgressPercent
}

// SetProgressPercent sets the `ProgressPercent` of the stage.
func (p *Pipeline[In, Out]) SetProgressPercent() {
	currentProgress := p.GetProgress().Value()
	totalProgress := len(p.Stages)
	percentage := float64(currentProgress) / float64(totalProgress) * 100

	p.GetProgressPercent().Set(fmt.Sprintf("%d%%", int(percentage)))
}

// Run the pipeline.
func (p *Pipeline[In, Out]) Run(ctx context.Context, in []In) ([]Out, error) {
	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	tracedContext, span := customapm.Trace(
		ctx,
		Type,
		p.GetName(),
		status.Runnning,
		p.Logger,
		p.CounterRunning,
	)
	defer span.End()

	// Validation.
	if in == nil {
		return nil, customapm.TraceError(
			tracedContext,
			customerror.NewRequiredError(
				"input",
				customerror.WithField(Type, p.Name),
			),
			p.GetLogger(),
			p.GetCounterFailed(),
		)
	}

	p.GetStatus().Set(status.Runnning.String())

	p.GetLogger().PrintlnWithOptions(level.Debug, status.Runnning.String())

	now := time.Now()

	//////
	// Run the pipeline.
	//////

	// Store in as reference to be used as the input of the next stage.
	retroFeedIn := make([]Out, 0)

	// Iterate through the stages, passing the output of each stage
	// as the input of the next stage.
	for _, s := range p.Stages {
		oS, err := s.Run(tracedContext, in)
		if err != nil {
			//////
			// Observability: tracing, metrics, status, logging, etc.
			//////

			return nil, shared.OnErrorHandler(
				tracedContext,
				p,
				p.GetLogger(),
				"run stage",
				Type,
				p.GetName(),
			)
		}

		retroFeedIn = oS

		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		// Increment the progress.
		s.GetProgress().Add(1)

		// Set the progress percentage.
		//
		// NOTE: MUST BE after increment the progress, as its internal calculation
		// depends on that.
		s.SetProgressPercent()
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	p.GetStatus().Set(status.Done.String())

	p.GetCounterDone().Add(1)

	if p.GetOnFinished() != nil {
		p.GetOnFinished()(ctx, p, in, retroFeedIn)
	}

	p.GetDuration().Set(time.Since(now).Milliseconds())

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

	return retroFeedIn, nil
}

//////
// Factory.
//////

// New returns a new pipeline.
func New[In, Out any](
	name string,
	description string,
	concurrentStage bool,
	stages ...stage.IStage[In, Out],
) (IPipeline[In, Out], error) {
	// WARN: Currently disabled.
	concurrentStage = false

	p := &Pipeline[In, Out]{
		ConcurrentStage: concurrentStage,
		Stages:          stages,
		Logger:          logging.Get().New(name).SetTags(Type, name),

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

	p.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return p, nil
}
