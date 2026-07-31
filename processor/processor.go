package processor

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

// Type identifies processor entities.
const Type = "processor"

// Transform transforms `processingData`, returning the processed data and any
// error. It must not mutate its input slice or pointed-to values; stages reuse
// results and async processors receive only a shallow copy.
type Transform[ProcessedData any] func(ctx context.Context, processingData []ProcessedData) (processedOut []ProcessedData, err error)

// Processor wraps a slice transform with lifecycle observability.
type Processor[ProcessingData any] struct {
	// Func transforms a slice without mutating its input or pointed-to values.
	Func Transform[ProcessingData] `json:"-" validate:"required"`

	// Logger records processor lifecycle events.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name identifies the processor.
	Name string `json:"name" validate:"required"`

	// Description describes the processor.
	Description string `json:"description"`

	// async tells a stage to run this processor in a goroutine.
	//
	// WARN: The output of the processing will not be forwarded!
	async bool `default:"false"`

	// onFinished is called after a processor succeeds.
	onFinished OnFinished[ProcessingData] `json:"-"`

	// Lifecycle metrics. CounterInterrupted counts paused runs whose wait ends
	// because their context is cancelled or reaches its deadline.
	CounterCreated     *expvar.Int `json:"counterCreated"`
	CounterDone        *expvar.Int `json:"counterDone"`
	CounterFailed      *expvar.Int `json:"counterFailed"`
	CounterInterrupted *expvar.Int `json:"counterInterrupted"`
	CounterRunning     *expvar.Int `json:"counterRunning"`

	CreatedAt time.Time      `json:"createdAt"`
	Duration  *expvar.Int    `json:"duration"`
	Status    *expvar.String `json:"status"`

	// mu guards async and onFinished.
	mu sync.RWMutex
}

//////
// Methods.
//////

// GetDescription returns the `Description` of the processor.
func (p *Processor[ProcessingData]) GetDescription() string {
	return p.Description
}

// GetLogger returns the `Logger` of the processor.
func (p *Processor[ProcessingData]) GetLogger() sypl.ISypl {
	return p.Logger
}

// GetName returns the `Name` of the processor.
func (p *Processor[ProcessingData]) GetName() string {
	return p.Name
}

// GetCounterCreated returns the `CounterCreated` metric.
func (p *Processor[ProcessingData]) GetCounterCreated() *expvar.Int {
	return p.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` metric.
func (p *Processor[ProcessingData]) GetCounterRunning() *expvar.Int {
	return p.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` metric.
func (p *Processor[ProcessingData]) GetCounterFailed() *expvar.Int {
	return p.CounterFailed
}

// GetCounterInterrupted returns the counter for paused runs interrupted by
// context cancellation or deadline.
func (p *Processor[ProcessingData]) GetCounterInterrupted() *expvar.Int {
	return p.CounterInterrupted
}

// GetCounterDone returns the `CounterDone` metric.
func (p *Processor[ProcessingData]) GetCounterDone() *expvar.Int {
	return p.CounterDone
}

// GetStatus returns the `Status` metric.
func (p *Processor[ProcessingData]) GetStatus() *expvar.String {
	return p.Status
}

// GetOnFinished returns the `OnFinished` function.
func (p *Processor[ProcessingData]) GetOnFinished() OnFinished[ProcessingData] {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.onFinished
}

// SetOnFinished sets the `OnFinished` function.
func (p *Processor[ProcessingData]) SetOnFinished(onFinished OnFinished[ProcessingData]) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.onFinished = onFinished
}

// GetType returns the entity type.
func (p *Processor[ProcessingData]) GetType() string {
	return Type
}

// GetCreatedAt returns when the processor was constructed.
func (p *Processor[ProcessingData]) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetDuration returns the most recent successful transform duration in
// milliseconds. A failed transform does not replace it.
func (p *Processor[ProcessingData]) GetDuration() *expvar.Int {
	return p.Duration
}

// GetMetrics returns selected processor metrics as strings.
func (p *Processor[ProcessingData]) GetMetrics() map[string]string {
	return map[string]string{
		"createdAt":          p.GetCreatedAt().String(),
		"counterCreated":     p.GetCounterCreated().String(),
		"counterDone":        p.GetCounterDone().String(),
		"counterFailed":      p.GetCounterFailed().String(),
		"counterInterrupted": p.GetCounterInterrupted().String(),
		"counterRunning":     p.GetCounterRunning().String(),
		"duration":           p.GetDuration().String(),
		"status":             p.GetStatus().String(),
	}
}

// SetAsync marks the processor for goroutine execution by a stage. Async
// scheduling does not change Run's lifecycle status or counters. Async
// processors receive a shallow copy, so transforms must not mutate input or
// pointed-to values.
//
// WARN: The output of the processing will not be forwarded!
func (p *Processor[ProcessingData]) SetAsync(async bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.async = async
}

// GetAsync reports whether a stage should run the processor in a goroutine.
func (p *Processor[ProcessingData]) GetAsync() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.async
}

// Run waits if its context carries a paused pipeline controller, then invokes
// the transform and completion callback. A processor running without such a
// controller does not pause. Context termination while waiting is recorded as
// an interruption; transform errors are recorded as failures.
func (p *Processor[ProcessingData]) Run(ctx context.Context, processingData []ProcessingData) ([]ProcessingData, error) {
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

	p.GetStatus().Set(status.Runnning.String())

	if logging.Admits(p.GetLogger(), level.Trace) {
		p.GetLogger().PrintlnWithOptions(level.Trace, status.Runnning.String())
	}

	// Store as reference to be used in the OnFinished function.
	originalProcessingData := processingData

	//////
	// Pause if the owning pipeline is paused.
	//////

	// The pipeline injects its pause controller via the context. A processor
	// running standalone (no controller in the context) never pauses.
	if pc := shared.PauseFromCtx(tracedContext); pc != nil && pc.Paused() {
		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		// Update the status.
		p.GetStatus().Set(status.Paused.String())

		// Notifiy user.
		if logging.Admits(p.GetLogger(), level.Trace) {
			p.GetLogger().Tracelnf("Processor %s is paused. Waiting to be resumed...", p.GetName())
		}

		// Blocks until resumed, or until the context is done.
		if err := pc.Wait(tracedContext); err != nil {
			//////
			// Observability: tracing, metrics, status, logging, etc.
			//////

			p.GetStatus().Set(status.Interrupted.String())

			return nil, customapm.TraceError(
				tracedContext,
				customerror.NewFailedToError(
					"process",
					customerror.WithError(err),
					customerror.WithField(Type, p.GetName()),
				),
				p.GetLogger(),
				p.GetCounterInterrupted(),
			)
		}

		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		p.GetStatus().Set(status.Runnning.String())
	}

	//////
	// Run processor.
	//////

	now := time.Now()

	o, err := p.Func(tracedContext, processingData)
	if err != nil {
		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		return nil, shared.OnErrorHandler(
			tracedContext,
			p,
			p.GetLogger(),
			err,
			"process",
			Type,
			p.GetName(),
		)
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	// Run onEvent callback with the original input and the processed output.
	onFinished := p.GetOnFinished()
	if onFinished != nil {
		onFinished(ctx, p, originalProcessingData, o)
	}

	// Update status.
	p.GetStatus().Set(status.Done.String())

	// Increment the done counter.
	p.GetCounterDone().Add(1)

	// Set duration.
	p.GetDuration().Set(time.Since(now).Milliseconds())

	// Print the stage's status.
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
			sypl.WithField("status", p.GetStatus().String()),
		)
	}

	return o, nil
}

//////
// Factory.
//////

// New creates a processor. It applies options in order, retains each returned
// implementation, and then validates the result. Nil options and results are
// rejected. Structural validation runs before lifecycle metric getters; the
// final implementation must expose non-nil lifecycle metrics. The transform
// must not mutate its input slice or pointed-to values.
func New[ProcessingData any](
	name, description string,
	fn Transform[ProcessingData],
	opts ...Func[ProcessingData],
) (IProcessor[ProcessingData], error) {
	// Validate the raw name before constructing metrics because published
	// expvars cannot be removed.
	if err := validation.Validate(struct {
		Name string `validate:"required"`
	}{Name: name}); err != nil {
		return nil, err
	}

	var p IProcessor[ProcessingData] = &Processor[ProcessingData]{
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
		if opt == nil {
			return nil, customerror.NewRequiredError("processor option")
		}

		p = opt(p)
		if isNilOptionResult(p) {
			return nil, customerror.NewRequiredError("processor option result")
		}
	}

	// Validation.
	if err := validation.Validate(p); err != nil {
		return nil, err
	}

	if err := validateOptionMetrics(p); err != nil {
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

// isNilOptionResult detects nil interfaces and interfaces holding nil values.
func isNilOptionResult[ProcessingData any](p IProcessor[ProcessingData]) bool {
	if p == nil {
		return true
	}

	value := reflect.ValueOf(p)

	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// validateOptionMetrics ensures options retain the metrics dereferenced by
// processor lifecycle observability.
func validateOptionMetrics[ProcessingData any](p IProcessor[ProcessingData]) error {
	switch {
	case p.GetStatus() == nil:
		return customerror.NewRequiredError("processor status metric")
	case p.GetCounterCreated() == nil:
		return customerror.NewRequiredError("processor created counter")
	case p.GetCounterRunning() == nil:
		return customerror.NewRequiredError("processor running counter")
	case p.GetCounterFailed() == nil:
		return customerror.NewRequiredError("processor failed counter")
	case p.GetCounterDone() == nil:
		return customerror.NewRequiredError("processor done counter")
	case p.GetCounterInterrupted() == nil:
		return customerror.NewRequiredError("processor interrupted counter")
	case p.GetDuration() == nil:
		return customerror.NewRequiredError("processor duration metric")
	default:
		return nil
	}
}
