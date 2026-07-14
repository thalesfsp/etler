package processor

import (
	"context"
	"expvar"
	"time"

	"github.com/thalesfsp/etler/v3/internal/customapm"
	"github.com/thalesfsp/etler/v3/internal/logging"
	"github.com/thalesfsp/etler/v3/internal/metrics"
	"github.com/thalesfsp/etler/v3/internal/shared"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "processor"

// Transform is a function that transforms (`processingData`) into
// (`processingData`), returning any errors that occurred during processing.
type Transform[ProcessedData any] func(ctx context.Context, processingData []ProcessedData) (processedOut []ProcessedData, err error)

// Processor definition.
type Processor[ProcessingData any] struct {
	// Transform function.
	Func Transform[ProcessingData] `json:"-" validate:"required"`

	// Logger is the internal logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the processor.
	Name string `json:"name"`

	// Description of the processor.
	Description string `json:"description"`

	// Async if set will run the processor in a go routine.
	//
	// WARN: The output of the processing will not be forwarded!
	Async bool `default:"false" json:"async"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[ProcessingData] `json:"-"`

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

// GetCounterInterrupted returns the `CounterInterrupted` metric.
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
	return p.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (p *Processor[ProcessingData]) SetOnFinished(onFinished OnFinished[ProcessingData]) {
	p.OnFinished = onFinished
}

// GetType returns the entity type.
func (p *Processor[ProcessingData]) GetType() string {
	return Type
}

// GetCreatedAt returns the created at time.
func (p *Processor[ProcessingData]) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetDuration returns the `CounterDuration` of the stage.
func (p *Processor[ProcessingData]) GetDuration() *expvar.Int {
	return p.Duration
}

// GetMetrics returns the stage's metrics.
func (p *Processor[ProcessingData]) GetMetrics() map[string]string {
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

// SetAsync if set will run the processor in a go routine.
//
// WARN: The output of the processing will not be forwarded!
func (p *Processor[ProcessingData]) SetAsync(async bool) {
	p.Async = async
}

// GetAsync returns if the processor is running in a go routine.
func (p *Processor[ProcessingData]) GetAsync() bool {
	return p.Async
}

// Run the transform function.
func (p *Processor[ProcessingData]) Run(ctx context.Context, processingData []ProcessingData) ([]ProcessingData, error) {
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

	p.GetStatus().Set(status.Runnning.String())

	p.GetLogger().PrintlnWithOptions(level.Trace, status.Runnning.String())

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
		p.GetLogger().Tracelnf("Processor %s is paused. Waiting to be resumed...", p.GetName())

		// Blocks until resumed, or until the context is done.
		if err := pc.Wait(tracedContext); err != nil {
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
	if p.GetOnFinished() != nil {
		p.GetOnFinished()(ctx, p, originalProcessingData, o)
	}

	// Update status.
	p.GetStatus().Set(status.Done.String())

	// Increment the done counter.
	p.GetCounterDone().Add(1)

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
func New[ProcessingData any](
	name, description string,
	fn Transform[ProcessingData],
	opts ...Func[ProcessingData],
) (IProcessor[ProcessingData], error) {
	p := &Processor[ProcessingData]{
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

	p.GetLogger().PrintlnWithOptions(level.Trace, status.Created.String())

	return p, nil
}
