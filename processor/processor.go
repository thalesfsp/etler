package processor

import (
	"context"
	"expvar"
	"time"

	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/internal/customapm"
	"github.com/thalesfsp/etler/v2/internal/logging"
	"github.com/thalesfsp/etler/v2/internal/metrics"
	"github.com/thalesfsp/etler/v2/internal/shared"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "processor"

// Transform is a function that transforms the data (`in`). It returns the
// transformed data and any errors that occurred during processing.
type Transform[In any] func(ctx context.Context, in []In) (out []In, err error)

// Processor definition.
type Processor[T any] struct {
	// Description of the processor.
	Description string `json:"description"`

	// Transform function.
	Func Transform[T] `json:"-"`

	// Logger is the pipeline logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the processor.
	Name string `json:"name"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[T] `json:"-"`

	// Metrics.
	CounterCreated     *expvar.Int `json:"counterCreated"`
	CounterDone        *expvar.Int `json:"counterDone"`
	CounterFailed      *expvar.Int `json:"counterFailed"`
	CounterInterrupted *expvar.Int `json:"counterInterrupted"`
	CounterRunning     *expvar.Int `json:"counterRunning"`

	CreatedAt time.Time      `json:"createdAt"`
	Duration  *expvar.Int    `json:"duration"`
	Status    *expvar.String `json:"status"`
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

// GetCounterInterrupted returns the `CounterInterrupted` metric.
func (p *Processor[T]) GetCounterInterrupted() *expvar.Int {
	return p.CounterInterrupted
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

// GetType returns the entity type.
func (p *Processor[T]) GetType() string {
	return Type
}

// GetCreatedAt returns the created at time.
func (p *Processor[T]) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetDuration returns the `CounterDuration` of the stage.
func (p *Processor[T]) GetDuration() *expvar.Int {
	return p.Duration
}

// GetMetrics returns the stage's metrics.
func (p *Processor[T]) GetMetrics() map[string]string {
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

// Run the transform function.
func (p *Processor[T]) Run(ctx context.Context, t []T) ([]T, error) {
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

	p.GetStatus().Set(status.Runnning.String())

	p.GetLogger().PrintlnWithOptions(level.Debug, status.Runnning.String())

	// originalIn is a copy of the input used for the callback.
	originalIn := make([]T, len(t))

	//////
	// Pause the pipeline if needed.
	//////

	// TODO: Change this to channel.
	for shared.GetPaused() == 1 {
		p.GetCounterCreated()

		// Update the status.
		p.GetStatus().Set(status.Paused.String())

		// Notifiy user.
		p.Logger.Debuglnf("Processor %s is paused. Waiting to be resumed...", p.GetName())

		select {

		// If the context is done, do nothing, return.
		case <-ctx.Done():
			// Observability: update the processor status, logging, metrics.
			return nil, shared.OnErrorHandler(
				tracedContext,
				p,
				p.GetLogger(),
				"process",
				Type,
				p.GetName(),
			)
		default:
			// If the context isn't done, check the status every second.
			time.Sleep(1 * time.Second)

			// If the status is no more paused, break the loop.
			if shared.GetPaused() != 1 {
				// Observability: update the processor status.
				p.GetStatus().Set(status.Runnning.String())

				break
			}
		}
	}

	//////
	// Run processor.
	//////

	now := time.Now()

	o, err := p.Func(tracedContext, t)
	if err != nil {
		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		return nil, shared.OnErrorHandler(
			tracedContext,
			p,
			p.GetLogger(),
			"process",
			Type,
			p.GetName(),
		)
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	// Update status.
	p.GetStatus().Set(status.Done.String())

	// Increment the done counter.
	p.GetCounterDone().Add(1)

	// Run onEvent callback.
	if p.GetOnFinished() != nil {
		p.GetOnFinished()(ctx, p, originalIn, t)
	}

	// Set duration.
	p.GetDuration().Set(time.Since(now).Milliseconds())

	// Print the stage's status.
	p.GetLogger().PrintWithOptions(
		level.Debug,
		status.Done.String(),
		sypl.WithField("createdAt", p.GetCreatedAt().String()),
		sypl.WithField("counterCreated", p.GetCounterCreated().String()),
		sypl.WithField("counterDone", p.GetCounterDone().String()),
		sypl.WithField("counterFailed", p.GetCounterFailed().String()),
		sypl.WithField("counterRunning", p.GetCounterRunning().String()),
		sypl.WithField("duration", p.GetDuration().String()),
		sypl.WithField("status", p.GetStatus().String()),
	)

	return o, nil
}

//////
// Factory.
//////

// New returns a new processor.
func New[T any](
	name, description string,
	fn Transform[T],
	opts ...Func[T],
) (IProcessor[T], error) {
	p := &Processor[T]{
		Func:   fn,
		Logger: logging.Get().New(name).SetTags(Type, name),

		CreatedAt:   time.Now(),
		Name:        name,
		Description: description,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),

		CounterInterrupted: metrics.NewIntWithPattern(Type, name, status.Interrupted),
		Duration:           metrics.NewIntWithPattern(Type, name, "duration"),
		Status:             metrics.NewStringWithPattern(Type, name, status.Name),
	}

	// Apply options.
	for _, opt := range opts {
		opt(p)
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
