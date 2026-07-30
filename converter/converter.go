package converter

import (
	"context"
	"expvar"
	"reflect"
	"sync"
	"time"

	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/etler/v4/internal/customapm"
	"github.com/thalesfsp/etler/v4/internal/logging"
	"github.com/thalesfsp/etler/v4/internal/metrics"
	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "converter"

// Convert converts `in`, returning the converted data and any error. It must
// be safe for concurrent use because stages invoke it from multiple goroutines.
type Convert[In, Out any] func(ctx context.Context, in In) (out Out, err error)

// Converter definition.
type Converter[In, Out any] struct {
	// Description of the processor.
	Description string `json:"description"`

	// Conversion function.
	Func Convert[In, Out] `json:"-" validate:"required"`

	// Logger is the internal logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the converter.
	Name string `json:"name" validate:"required"`

	// onFinished is the function that is called when a processor finishes its
	// execution.
	onFinished OnFinished[In, Out] `json:"-"`

	// Metrics.
	CounterCreated *expvar.Int `json:"counterCreated"`
	CounterRunning *expvar.Int `json:"counterRunning"`
	CounterFailed  *expvar.Int `json:"counterFailed"`
	CounterDone    *expvar.Int `json:"counterDone"`

	CreatedAt time.Time      `json:"createdAt"`
	Duration  *expvar.Int    `json:"duration"`
	Status    *expvar.String `json:"status"`

	// mu guards onFinished.
	mu sync.RWMutex
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

// GetName returns the `Name` of the converter.
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.onFinished
}

// SetOnFinished sets the `OnFinished` function.
func (c *Converter[In, Out]) SetOnFinished(onFinished OnFinished[In, Out]) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.onFinished = onFinished
}

// GetCreatedAt returns the created at time.
func (c *Converter[In, Out]) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetDuration returns the `CounterDuration` of the converter.
func (c *Converter[In, Out]) GetDuration() *expvar.Int {
	return c.Duration
}

// GetMetrics returns the converter's metrics.
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

	tracedContext, ender := customapm.Trace(
		ctx,
		Type,
		c.GetName(),
		status.Runnning,
		c.Logger,
		c.CounterRunning,
	)
	defer ender.End()

	c.GetStatus().Set(status.Runnning.String())

	if logging.Admits(c.GetLogger(), level.Debug) {
		c.GetLogger().PrintlnWithOptions(level.Debug, status.Runnning.String())
	}

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
			err,
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
	onFinished := c.GetOnFinished()
	if onFinished != nil {
		onFinished(ctx, c, in, out)
	}

	// Set duration.
	c.GetDuration().Set(time.Since(now).Milliseconds())

	// Print the converter's status.
	if logging.Admits(c.GetLogger(), level.Debug) {
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
	}

	return out, nil
}

//////
// Factory.
//////

// New returns a new converter.
func New[In, Out any](
	name, description string,
	fn Convert[In, Out],
	opts ...Func[In, Out],
) (IConverter[In, Out], error) {
	// Validate the raw name before constructing metrics because published
	// expvars cannot be removed.
	if err := validation.Validate(struct {
		Name string `validate:"required"`
	}{Name: name}); err != nil {
		return nil, err
	}

	var c IConverter[In, Out] = &Converter[In, Out]{
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

	// Apply options.
	for _, opt := range opts {
		if opt == nil {
			return nil, customerror.NewRequiredError("converter option")
		}

		c = opt(c)
		if isNilOptionResult(c) {
			return nil, customerror.NewRequiredError("converter option result")
		}
	}

	if err := validateOptionMetrics(c); err != nil {
		return nil, err
	}

	// Validation.
	if err := validation.Validate(c); err != nil {
		return nil, err
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	c.GetStatus().Set(status.Created.String())

	c.GetCounterCreated().Add(1)

	c.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return c, nil
}

// isNilOptionResult detects nil interfaces and interfaces holding nil values.
func isNilOptionResult[In, Out any](c IConverter[In, Out]) bool {
	if c == nil {
		return true
	}

	value := reflect.ValueOf(c)

	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// validateOptionMetrics ensures options retain the metrics dereferenced by
// converter lifecycle observability.
func validateOptionMetrics[In, Out any](c IConverter[In, Out]) error {
	switch {
	case c.GetStatus() == nil:
		return customerror.NewRequiredError("converter status metric")
	case c.GetCounterCreated() == nil:
		return customerror.NewRequiredError("converter created counter")
	case c.GetCounterRunning() == nil:
		return customerror.NewRequiredError("converter running counter")
	case c.GetCounterFailed() == nil:
		return customerror.NewRequiredError("converter failed counter")
	case c.GetCounterDone() == nil:
		return customerror.NewRequiredError("converter done counter")
	case c.GetDuration() == nil:
		return customerror.NewRequiredError("converter duration metric")
	default:
		return nil
	}
}

// Default returns a new converter with name and description set to "converter".
func Default[In, Out any](
	fn Convert[In, Out],
	opts ...Func[In, Out],
) (IConverter[In, Out], error) {
	return New(Type, Type, fn, opts...)
}

// MustDefault returns a new converter with name and description set to
// "converter", panicking on error.
func MustDefault[In, Out any](
	fn Convert[In, Out],
	opts ...Func[In, Out],
) IConverter[In, Out] {
	c, err := New(Type, Type, fn, opts...)
	if err != nil {
		panic(err)
	}

	return c
}
