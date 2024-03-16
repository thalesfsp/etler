package converter

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
const Type = "converter"

// Convert is a function that converts the data (`in`). It returns the
// converted data and any errors that occurred during conversion.
type Convert[In, Out any] func(ctx context.Context, in In) (out Out, err error)

// Converter definition.
type Converter[In, Out any] struct {
	// Description of the processor.
	Description string `json:"description"`

	// Conversion function.
	Func Convert[In, Out] `json:"-"`

	// Logger is the pipeline logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the stage.
	Name string `json:"name" validate:"required"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[In, Out] `json:"-"`

	// Metrics.
	CounterCreated *expvar.Int `json:"counterCreated"`
	CounterRunning *expvar.Int `json:"counterRunning"`
	CounterFailed  *expvar.Int `json:"counterFailed"`
	CounterDone    *expvar.Int `json:"counterDone"`

	CreatedAt time.Time      `json:"createdAt"`
	Duration  *expvar.Int    `json:"duration"`
	Status    *expvar.String `json:"status"`
}

//////
// Methods.
//////

// GetDescription returns the `Description` of the processor.
func (c *Converter[In, Out]) GetDescription() string {
	return c.Description
}

// GetLogger returns the `Logger` of the processor.
func (c *Converter[In, Out]) GetLogger() sypl.ISypl {
	return c.Logger
}

// GetName returns the `Name` of the stage.
func (c *Converter[In, Out]) GetName() string {
	return c.Name
}

// GetType returns the entity type.
func (c *Converter[In, Out]) GetType() string {
	return Type
}

// GetCounterCreated returns the `CounterCreated` of the processor.
func (c *Converter[In, Out]) GetCounterCreated() *expvar.Int {
	return c.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` of the processor.
func (c *Converter[In, Out]) GetCounterRunning() *expvar.Int {
	return c.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` of the processor.
func (c *Converter[In, Out]) GetCounterFailed() *expvar.Int {
	return c.CounterFailed
}

// GetCounterDone returns the `CounterDone` of the processor.
func (c *Converter[In, Out]) GetCounterDone() *expvar.Int {
	return c.CounterDone
}

// GetStatus returns the `Status` metric.
func (c *Converter[In, Out]) GetStatus() *expvar.String {
	return c.Status
}

// GetOnFinished returns the `OnFinished` function.
func (c *Converter[In, Out]) GetOnFinished() OnFinished[In, Out] {
	return c.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (c *Converter[In, Out]) SetOnFinished(onFinished OnFinished[In, Out]) {
	c.OnFinished = onFinished
}

// GetCreatedAt returns the created at time.
func (c *Converter[In, Out]) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetDuration returns the `CounterDuration` of the stage.
func (c *Converter[In, Out]) GetDuration() *expvar.Int {
	return c.Duration
}

// GetMetrics returns the stage's metrics.
func (c *Converter[In, Out]) GetMetrics() map[string]string {
	return map[string]string{
		"createdAt":      c.GetCreatedAt().String(),
		"counterCreated": c.GetCounterCreated().String(),
		"counterDone":    c.GetCounterDone().String(),
		"counterFailed":  c.GetCounterFailed().String(),
		"counterRunning": c.GetCounterRunning().String(),
		"duration":       c.GetDuration().String(),
		"status":         c.GetStatus().String(),
	}
}

// Run the conversion function.
func (c *Converter[In, Out]) Run(ctx context.Context, in In) (Out, error) {
	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	tracedContext, span := customapm.Trace(
		ctx,
		Type,
		c.GetName(),
		status.Runnning,
		c.Logger,
		c.CounterRunning,
	)
	defer span.End()

	c.GetStatus().Set(status.Runnning.String())

	c.GetLogger().PrintlnWithOptions(level.Debug, status.Runnning.String())

	//////
	// Run conversor.
	//////

	now := time.Now()

	out, err := c.Func(tracedContext, in)
	if err != nil {
		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		return *new(Out), shared.OnErrorHandler(
			tracedContext,
			c,
			c.GetLogger(),
			"process",
			Type,
			c.GetName(),
		)
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	// Update status.
	c.GetStatus().Set(status.Done.String())

	// Increment the done counter.
	c.GetCounterDone().Add(1)

	// Run onEvent callback.
	if c.GetOnFinished() != nil {
		// TODO: Fix this.
		c.GetOnFinished()(ctx, c, in, out)
	}

	// Set duration.
	c.GetDuration().Set(time.Since(now).Milliseconds())

	// Print the stage's status.
	c.GetLogger().PrintWithOptions(
		level.Debug,
		status.Done.String(),
		sypl.WithField("createdAt", c.GetCreatedAt().String()),
		sypl.WithField("counterCreated", c.GetCounterCreated().String()),
		sypl.WithField("counterDone", c.GetCounterDone().String()),
		sypl.WithField("counterFailed", c.GetCounterFailed().String()),
		sypl.WithField("counterRunning", c.GetCounterRunning().String()),
		sypl.WithField("duration", c.GetDuration().String()),
		sypl.WithField("status", c.GetStatus().String()),
	)

	return out, nil
}

//////
// Factory.
//////

// New returns a new stage.
func New[In, Out any](
	name, description string,
	fn Convert[In, Out],
	opts ...Func[In, Out],
) (IConverter[In, Out], error) {
	c := &Converter[In, Out]{
		Func:   fn,
		Logger: logging.Get().New(name).SetTags(Type, name),

		CreatedAt:   time.Now(),
		Name:        name,
		Description: description,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),

		Duration: metrics.NewIntWithPattern(Type, name, "duration"),
		Status:   metrics.NewStringWithPattern(Type, name, status.Name),
	}

	// Validation.
	if err := validation.Validate(c); err != nil {
		return nil, err
	}

	// Apply options.
	for _, opt := range opts {
		opt(c)
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	c.GetStatus().Set(status.Created.String())

	c.GetCounterCreated().Add(1)

	c.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return c, nil
}
