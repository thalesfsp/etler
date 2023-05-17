package processor

import (
	"context"
	"expvar"
	"time"

	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/etler/internal/customapm"
	"github.com/thalesfsp/etler/internal/logging"
	"github.com/thalesfsp/etler/internal/metrics"
	"github.com/thalesfsp/etler/internal/shared"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "processor"

// Processor definition.
type Processor[T any] struct {
	// Description of the processor.
	Description string `json:"description"`

	// Transform function.
	Func shared.Run[T] `json:"-"`

	// Logger is the pipeline logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the processor.
	Name string `json:"name"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[T] `json:"-"`

	// Metrics.
	CounterCreated *expvar.Int    `json:"counterCreated" validate:"required,gte=0"`
	CounterRunning *expvar.Int    `json:"counterRunning" validate:"required,gte=0"`
	CounterFailed  *expvar.Int    `json:"counterFailed" validate:"required,gte=0"`
	CounterDone    *expvar.Int    `json:"counterDone" validate:"required,gte=0"`
	Status         *expvar.String `json:"status" validate:"required,gte=0"`
}

//////
// Methods.
//////

// GetDescription returns the `Description` of the processor.
func (p *Processor[T]) GetDescription() string {
	return p.Description
}

// GetLogger returns the `Logger` of the processor.
func (p *Processor[T]) GetLogger() sypl.ISypl {
	return p.Logger
}

// GetName returns the `Name` of the processor.
func (p *Processor[T]) GetName() string {
	return p.Name
}

// GetCounterCreated returns the `CounterCreated` metric.
func (p *Processor[T]) GetCounterCreated() *expvar.Int {
	return p.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` metric.
func (p *Processor[T]) GetCounterRunning() *expvar.Int {
	return p.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` metric.
func (p *Processor[T]) GetCounterFailed() *expvar.Int {
	return p.CounterFailed
}

// GetCounterDone returns the `CounterDone` metric.
func (p *Processor[T]) GetCounterDone() *expvar.Int {
	return p.CounterDone
}

// GetStatus returns the `Status` metric.
func (p *Processor[T]) GetStatus() *expvar.String {
	return p.Status
}

// GetOnFinished returns the `OnFinished` function.
func (p *Processor[T]) GetOnFinished() OnFinished[T] {
	return p.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (p *Processor[T]) SetOnFinished(onFinished OnFinished[T]) {
	p.OnFinished = onFinished
}

// Run the transform function.
func (p *Processor[T]) Run(ctx context.Context, t []T) ([]T, error) {
	// originalIn is a copy of the input.
	originalIn := make([]T, len(t))

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

	// Update the status.
	p.GetStatus().Set(status.Runnning.String())

	//////
	// Pause the pipeline if needed.
	//////

	for shared.GetPaused() == 1 {
		// Update the status.
		p.GetStatus().Set(status.Paused.String())

		p.Logger.Debuglnf("Processor %s is paused. Waiting to be resumed...", p.GetName())

		select {
		case <-ctx.Done():
			// Update the status.
			p.GetStatus().Set(status.Failed.String())

			// Return if the context is done.
			return nil, customapm.TraceError(
				tracedContext,
				customerror.NewFailedToError(
					"process",
					customerror.WithError(ctx.Err()),
					customerror.WithField("processor", p.GetName()),
				),
				p.GetLogger(),
				p.GetCounterFailed(),
			)
		default:
			// If the context isn't done, check the status every second.
			time.Sleep(1 * time.Second)

			// If the status is no more paused, break the loop.
			if shared.GetPaused() != 1 {
				// Update the status.
				p.GetStatus().Set(status.Runnning.String())

				break
			}
		}
	}

	//////
	// Run processor.
	//////

	o, err := p.Func(tracedContext, t)
	if err != nil {
		// Update the status.
		p.GetStatus().Set(status.Failed.String())

		// Observability: logging, metrics, and tracing.
		return nil, customapm.TraceError(
			tracedContext,
			customerror.NewFailedToError(
				"process",
				customerror.WithError(err),
				customerror.WithField("processor", p.GetName()),
			),
			p.GetLogger(),
			p.GetCounterFailed(),
		)
	}

	// Observability: logging, metrics.
	p.GetStatus().Set(status.Done.String())

	p.GetCounterDone().Add(1)

	// Run onEvent callback.
	if p.GetOnFinished() != nil {
		p.GetOnFinished()(ctx, p, originalIn, t)
	}

	return o, nil
}

//////
// Factory.
//////

// New returns a new processor.
func New[T any](
	name string,
	description string,
	fn shared.Run[T],
	opts ...Func[T],
) (IProcessor[T], error) {
	p := &Processor[T]{
		Description: description,
		Func:        fn,
		Logger:      logging.Get().New(name).SetTags(Type, name),
		Name:        name,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),
		Status:         metrics.NewStringWithPattern(Type, name, status.Name),
	}

	// Apply options.
	for _, opt := range opts {
		opt(p)
	}

	if err := validation.Validate(p); err != nil {
		return nil, err
	}

	// Observability: logging, metrics.
	p.GetCounterCreated().Add(1)

	p.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return p, nil
}
