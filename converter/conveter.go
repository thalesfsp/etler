package converter

import (
	"expvar"

	"github.com/thalesfsp/etler/v2/internal/logging"
	"github.com/thalesfsp/etler/v2/internal/metrics"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "converter"

// Converter definition.
type Converter[T any] struct {
	// Description of the processor.
	Description string `json:"description"`

	// Logger is the pipeline logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the stage.
	Name string `json:"name" validate:"required"`

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
func (c *Converter[T]) GetDescription() string {
	return c.Description
}

// GetLogger returns the `Logger` of the processor.
func (c *Converter[T]) GetLogger() sypl.ISypl {
	return c.Logger
}

// GetName returns the `Name` of the stage.
func (c *Converter[T]) GetName() string {
	return c.Name
}

// GetType returns the entity type.
func (c *Converter[T]) GetType() string {
	return Type
}

// GetCounterCreated returns the `CounterCreated` of the processor.
func (c *Converter[T]) GetCounterCreated() *expvar.Int {
	return c.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` of the processor.
func (c *Converter[T]) GetCounterRunning() *expvar.Int {
	return c.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` of the processor.
func (c *Converter[T]) GetCounterFailed() *expvar.Int {
	return c.CounterFailed
}

// GetCounterDone returns the `CounterDone` of the processor.
func (c *Converter[T]) GetCounterDone() *expvar.Int {
	return c.CounterDone
}

// GetStatus returns the `Status` metric.
func (c *Converter[T]) GetStatus() *expvar.String {
	return c.Status
}

// GetOnFinished returns the `OnFinished` function.
func (c *Converter[T]) GetOnFinished() OnFinished[T] {
	return c.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (c *Converter[T]) SetOnFinished(onFinished OnFinished[T]) {
	c.OnFinished = onFinished
}

//////
// Factory.
//////

// New returns a new stage.
func New[T any](name, description string) (*Converter[T], error) {
	c := &Converter[T]{
		Logger:      logging.Get().New(name).SetTags(Type, name),
		Name:        name,
		Description: description,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),
		Status:         metrics.NewStringWithPattern(Type, name, status.Name),
	}

	// Validation.
	if err := validation.Validate(c); err != nil {
		return nil, err
	}

	// Observability: logging, metrics.
	c.GetCounterCreated().Add(1)

	c.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return c, nil
}
