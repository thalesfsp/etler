// TODO: Add metrics, error handling, logging, context, APM, APM transaction, etc.

package pipeline

import (
	"context"
	"expvar"

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

	// Progress of the pipeline.
	Progress int `json:"progress"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[In, Out] `json:"-"`

	// Stages to be used in the pipeline.
	Stages []stage.IStage[In, Out] `json:"stages"`

	// Metrics.
	CounterCreated *expvar.Int    `json:"counterCreated"`
	CounterRunning *expvar.Int    `json:"counterRunning"`
	CounterFailed  *expvar.Int    `json:"counterFailed"`
	CounterDone    *expvar.Int    `json:"counterDone"`
	Status         *expvar.String `json:"status"`
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

// Run the pipeline.
func (p *Pipeline[In, Out]) Run(ctx context.Context, in []In) ([]Out, error) {
	//////
	// Observability: logging, metrics, and tracing.
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

	// Initialize the output.
	out := make([]Out, 0)

	// Validation.
	if in == nil {
		return out, customapm.TraceError(
			tracedContext,
			customerror.NewRequiredError(
				"input",
				customerror.WithField(Type, p.Name),
			),
			p.GetLogger(),
			p.GetCounterFailed(),
		)
	}

	// Update the status.
	p.GetStatus().Set(status.Runnning.String())

	//////
	// Run the pipeline.
	//////

	// Iterate through the stages, passing the output of each stage
	// as the input of the next stage.
	for _, s := range p.Stages {
		oS, err := s.Run(tracedContext, in)
		if err != nil {
			// Observability: logging, metrics.
			p.GetStatus().Set(status.Failed.String())

			// Returns whatever is in `out` and the error.
			return out, customapm.TraceError(
				tracedContext,
				customerror.New(
					"failed to run stage",
					customerror.WithError(err),
					customerror.WithField(Type, p.Name),
				),
				s.GetLogger(),
				s.GetCounterFailed(),
			)
		}

		out = append(out, oS...)
	}

	// Observability: logging, metrics.
	p.GetStatus().Set(status.Done.String())

	p.GetCounterDone().Add(1)

	if p.GetOnFinished() != nil {
		p.GetOnFinished()(ctx, p, in, out)
	}

	return out, nil
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
	p := &Pipeline[In, Out]{
		ConcurrentStage: false,
		Logger:          logging.Get().New(name).SetTags(Type, name),
		Name:            name,
		Progress:        0,
		Stages:          stages,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),
		Status:         metrics.NewStringWithPattern(Type, name, status.Name),
	}

	// Validation.
	if err := validation.Validate(p); err != nil {
		return nil, err
	}

	// Observability: logging, metrics.
	p.GetCounterCreated().Add(1)

	p.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return p, nil
}
