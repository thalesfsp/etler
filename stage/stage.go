package stage

import (
	"context"
	"expvar"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/etler/v2/internal/customapm"
	"github.com/thalesfsp/etler/v2/internal/logging"
	"github.com/thalesfsp/etler/v2/internal/metrics"
	"github.com/thalesfsp/etler/v2/processor"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "stage"

// Stage definition.
type Stage[In, Out any] struct {
	// Description of the processor.
	Description string `json:"description"`

	// Conversor to be used in the stage.
	Conversor concurrentloop.MapFunc[In, Out] `json:"-" validate:"required"`

	// Logger is the pipeline logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the stage.
	Name string `json:"name" validate:"required"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[In, Out] `json:"-"`

	// Processors to be run in the stage.
	Processors []processor.IProcessor[In] `json:"processors" validate:"required,gt=0"`

	// Progress of the stage.
	Progress int `json:"progress"`

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
func (s *Stage[In, Out]) GetDescription() string {
	return s.Description
}

// GetLogger returns the `Logger` of the processor.
func (s *Stage[In, Out]) GetLogger() sypl.ISypl {
	return s.Logger
}

// GetName returns the `Name` of the stage.
func (s *Stage[In, Out]) GetName() string {
	return s.Name
}

// GetCounterCreated returns the `CounterCreated` of the processor.
func (s *Stage[In, Out]) GetCounterCreated() *expvar.Int {
	return s.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` of the processor.
func (s *Stage[In, Out]) GetCounterRunning() *expvar.Int {
	return s.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` of the processor.
func (s *Stage[In, Out]) GetCounterFailed() *expvar.Int {
	return s.CounterFailed
}

// GetCounterDone returns the `CounterDone` of the processor.
func (s *Stage[In, Out]) GetCounterDone() *expvar.Int {
	return s.CounterDone
}

// GetStatus returns the `Status` metric.
func (s *Stage[In, Out]) GetStatus() *expvar.String {
	return s.Status
}

// GetOnFinished returns the `OnFinished` function.
func (s *Stage[In, Out]) GetOnFinished() OnFinished[In, Out] {
	return s.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (s *Stage[In, Out]) SetOnFinished(onFinished OnFinished[In, Out]) {
	s.OnFinished = onFinished
}

// GetType returns the entity type.
func (s *Stage[In, Out]) GetType() string {
	return Type
}

// Run the transform function.
func (s *Stage[In, Out]) Run(ctx context.Context, in []In) ([]Out, error) {
	//////
	// Observability: logging, metrics, and tracing.
	//////

	tracedContext, span := customapm.Trace(
		ctx,
		Type,
		s.GetName(),
		status.Runnning,
		s.Logger,
		s.CounterRunning,
	)
	defer span.End()

	// Update the status.
	s.GetStatus().Set(status.Runnning.String())

	//////
	// Stage's processors.
	//////

	// Initialize the output.
	out := make([]Out, 0)

	// Store in as reference to be used as the input of the next stage.
	retroFeedIn := in

	// NOTE: It process the data sequentially.
	for _, proc := range s.Processors {
		// Re-use the output of the previous stage as the input of the
		// next stage ensuring that the data is processed sequentially.
		rFI, err := proc.Run(tracedContext, retroFeedIn)
		if err != nil {
			// Observability: logging, metrics.
			s.GetStatus().Set(status.Failed.String())

			// Returns whatever is in `out` and the error.
			//
			// Don't need tracing, it's already traced.
			return out, customapm.TraceError(
				tracedContext,
				customerror.New(
					"failed to run processor",
					customerror.WithError(err),
					customerror.WithField(Type, s.Name),
				),
				s.GetLogger(),
				s.GetCounterFailed(),
			)
		}

		// Update the input with the output.
		retroFeedIn = rFI
	}

	//////
	// Stage's conversor.
	//////

	mapOut, errs := concurrentloop.Map(tracedContext, retroFeedIn, s.Conversor)
	if errs != nil {
		// Observability: logging, metrics.
		s.GetStatus().Set(status.Failed.String())

		// Returns whatever is in `out` and the error.
		//
		// Observability: logging, metrics, and tracing.
		return out, customapm.TraceError(
			tracedContext,
			customerror.NewFailedToError(
				"convert",
				customerror.WithError(errs),
				customerror.WithField(Type, s.Name),
			),
			s.GetLogger(),
			s.GetCounterFailed(),
		)
	}

	// Observability: logging, metrics.
	s.GetStatus().Set(status.Done.String())

	s.GetCounterDone().Add(1)

	// Run onEvent callback.
	if s.GetOnFinished() != nil {
		s.GetOnFinished()(ctx, s, in, mapOut)
	}

	return mapOut, nil
}

//////
// Factory.
//////

// New returns a new stage.
func New[In, Out any](
	name string,
	conversor concurrentloop.MapFunc[In, Out],
	processors ...processor.IProcessor[In],
) (IStage[In, Out], error) {
	s := &Stage[In, Out]{
		Conversor:  conversor,
		Logger:     logging.Get().New(name).SetTags(Type, name),
		Name:       name,
		Processors: processors,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),
		Status:         metrics.NewStringWithPattern(Type, name, status.Name),
	}

	// Validation.
	if err := validation.Validate(s); err != nil {
		return nil, err
	}

	// Observability: logging, metrics.
	s.GetCounterCreated().Add(1)

	s.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return s, nil
}
