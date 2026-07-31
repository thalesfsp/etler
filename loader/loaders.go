package loader

import (
	"context"
	"expvar"
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

// Type identifies loader entities.
const Type = "loader"

// Load retrieves or decodes data from in.
type Load[In, Out any] func(ctx context.Context, in In) (out Out, err error)

// Loader wraps a load function with lifecycle observability.
type Loader[In, Out any] struct {
	// Description describes the loader.
	Description string `json:"description"`

	// Func loads data from one input.
	Func Load[In, Out] `json:"-" validate:"required"`

	// Logger records loader lifecycle events.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name identifies the loader.
	Name string `json:"name" validate:"required"`

	// onFinished is called after a load succeeds.
	onFinished OnFinished[In, Out] `json:"-"`

	// Lifecycle metrics.
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

// GetDescription returns the loader's description.
func (c *Loader[In, Out]) GetDescription() string {
	return c.Description
}

// GetLogger returns the loader's logger.
func (c *Loader[In, Out]) GetLogger() sypl.ISypl {
	return c.Logger
}

// GetName returns the loader's name.
func (c *Loader[In, Out]) GetName() string {
	return c.Name
}

// GetType returns the entity type.
func (c *Loader[In, Out]) GetType() string {
	return Type
}

// GetCounterCreated returns the loader's created counter.
func (c *Loader[In, Out]) GetCounterCreated() *expvar.Int {
	return c.CounterCreated
}

// GetCounterRunning returns the loader's running counter.
func (c *Loader[In, Out]) GetCounterRunning() *expvar.Int {
	return c.CounterRunning
}

// GetCounterFailed returns the loader's failed counter.
func (c *Loader[In, Out]) GetCounterFailed() *expvar.Int {
	return c.CounterFailed
}

// GetCounterDone returns the loader's done counter.
func (c *Loader[In, Out]) GetCounterDone() *expvar.Int {
	return c.CounterDone
}

// GetStatus returns the `Status` metric.
func (c *Loader[In, Out]) GetStatus() *expvar.String {
	return c.Status
}

// GetOnFinished returns the `OnFinished` function.
func (c *Loader[In, Out]) GetOnFinished() OnFinished[In, Out] {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.onFinished
}

// SetOnFinished sets the `OnFinished` function.
func (c *Loader[In, Out]) SetOnFinished(onFinished OnFinished[In, Out]) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.onFinished = onFinished
}

// GetCreatedAt returns when the loader was constructed.
func (c *Loader[In, Out]) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetDuration returns the most recent successful load duration in milliseconds.
// A failed load does not replace it.
func (c *Loader[In, Out]) GetDuration() *expvar.Int {
	return c.Duration
}

// GetMetrics returns a snapshot of the loader's metrics as strings.
func (c *Loader[In, Out]) GetMetrics() map[string]string {
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

// Run invokes the load function and updates lifecycle observability.
func (c *Loader[In, Out]) Run(ctx context.Context, in In) (Out, error) {
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

// New creates a loader. It applies options in order, retains each returned
// implementation, and then validates the result. Nil options, results, and
// embedded implementations at any anonymous-field depth are rejected.
// Structural validation runs before lifecycle metric getters; the final
// implementation must expose non-nil lifecycle metrics.
func New[In, Out any](
	name, description string,
	fn Load[In, Out],
	opts ...Func[In, Out],
) (ILoader[In, Out], error) {
	// Validate the raw name before constructing metrics because published
	// expvars cannot be removed.
	if err := validation.Validate(struct {
		Name string `validate:"required"`
	}{Name: name}); err != nil {
		return nil, err
	}

	var c ILoader[In, Out] = &Loader[In, Out]{
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
			return nil, customerror.NewRequiredError("loader option")
		}

		c = opt(c)
		if shared.HasNilEmbeddedImplementation(c) {
			return nil, customerror.NewRequiredError("loader option result")
		}
	}

	// Validation.
	if err := validation.Validate(c); err != nil {
		return nil, err
	}

	if err := validateOptionMetrics(c); err != nil {
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

// validateOptionMetrics ensures options retain the metrics dereferenced by
// loader lifecycle observability.
func validateOptionMetrics[In, Out any](c ILoader[In, Out]) error {
	switch {
	case c.GetStatus() == nil:
		return customerror.NewRequiredError("loader status metric")
	case c.GetCounterCreated() == nil:
		return customerror.NewRequiredError("loader created counter")
	case c.GetCounterRunning() == nil:
		return customerror.NewRequiredError("loader running counter")
	case c.GetCounterFailed() == nil:
		return customerror.NewRequiredError("loader failed counter")
	case c.GetCounterDone() == nil:
		return customerror.NewRequiredError("loader done counter")
	case c.GetDuration() == nil:
		return customerror.NewRequiredError("loader duration metric")
	default:
		return nil
	}
}
