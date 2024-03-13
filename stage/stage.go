package stage

import (
	"context"
	"expvar"
	"fmt"
	"time"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/internal/customapm"
	"github.com/thalesfsp/etler/v2/internal/logging"
	"github.com/thalesfsp/etler/v2/internal/metrics"
	"github.com/thalesfsp/etler/v2/processor"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "stage"

// Stage definition.
type Stage[In, Out any] struct {
	// Description of the stage.
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

	// CreatedAt is the time when the stage was created.
	CreatedAt time.Time `json:"createdAt"`

	// Metrics.
	CounterCreated *expvar.Int `json:"counterCreated"`
	CounterDone    *expvar.Int `json:"counterDone"`
	CounterFailed  *expvar.Int `json:"counterFailed"`
	CounterRunning *expvar.Int `json:"counterRunning"`

	Duration        *expvar.Int    `json:"duration"`
	Progress        *expvar.Int    `json:"progress"`
	ProgressPercent *expvar.String `json:"progressPercent"`
	Status          *expvar.String `json:"status"`
}

//////
// Methods.
//////

// GetDescription returns the `Description` of the stage.
func (s *Stage[In, Out]) GetDescription() string {
	return s.Description
}

// GetLogger returns the `Logger` of the stage.
func (s *Stage[In, Out]) GetLogger() sypl.ISypl {
	return s.Logger
}

// GetName returns the `Name` of the stage.
func (s *Stage[In, Out]) GetName() string {
	return s.Name
}

// GetCounterCreated returns the `CounterCreated` of the stage.
func (s *Stage[In, Out]) GetCounterCreated() *expvar.Int {
	return s.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` of the stage.
func (s *Stage[In, Out]) GetCounterRunning() *expvar.Int {
	return s.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` of the stage.
func (s *Stage[In, Out]) GetCounterFailed() *expvar.Int {
	return s.CounterFailed
}

// GetCounterDone returns the `CounterDone` of the stage.
func (s *Stage[In, Out]) GetCounterDone() *expvar.Int {
	return s.CounterDone
}

// GetDuration returns the `CounterDuration` of the stage.
func (s *Stage[In, Out]) GetDuration() *expvar.Int {
	return s.Duration
}

// GetProgress returns the `CounterProgress` of the stage.
func (s *Stage[In, Out]) GetProgress() *expvar.Int {
	return s.Progress
}

// GetProgressPercent returns the `ProgressPercent` of the stage.
func (s *Stage[In, Out]) GetProgressPercent() *expvar.String {
	return s.ProgressPercent
}

// SetProgressPercent sets the `ProgressPercent` of the stage.
func (s *Stage[In, Out]) SetProgressPercent() {
	currentProgress := s.GetProgress().Value()
	totalProgress := len(s.Processors)
	percentage := float64(currentProgress) / float64(totalProgress) * 100

	s.GetProgressPercent().Set(fmt.Sprintf("%d%%", int(percentage)))
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

// GetCreatedAt returns the created at time.
func (s *Stage[In, Out]) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetMetrics returns the stage's metrics.
func (s *Stage[In, Out]) GetMetrics() map[string]string {
	return map[string]string{
		"createdAt":       s.GetCreatedAt().String(),
		"counterCreated":  s.GetCounterCreated().String(),
		"counterDone":     s.GetCounterDone().String(),
		"counterFailed":   s.GetCounterFailed().String(),
		"counterRunning":  s.GetCounterRunning().String(),
		"duration":        s.GetDuration().String(),
		"progress":        s.GetProgress().String(),
		"progressPercent": s.GetProgressPercent().String(),
		"status":          s.GetStatus().String(),
	}
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

		//////
		// Observability: logging, metrics.
		//////

		// Increment the progress.
		s.GetProgress().Add(1)

		// Set the progress percentage.
		//
		// NOTE: MUST BE after increment the progress, as its internal calculation
		// depends on that.
		s.SetProgressPercent()
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

	//////
	// Observability: logging, metrics.
	//////

	// Update status.
	s.GetStatus().Set(status.Done.String())

	// Increment the done counter.
	s.GetCounterDone().Add(1)

	// Set duration.
	s.GetDuration().Set(time.Since(s.GetCreatedAt()).Milliseconds())

	// Run onEvent callback.
	if s.GetOnFinished() != nil {
		s.GetOnFinished()(ctx, s, in, mapOut)
	}

	// Print the stage's status.
	s.GetLogger().PrintWithOptions(
		level.Debug,
		"done",
		sypl.WithField("createdAt", s.GetCreatedAt().String()),
		sypl.WithField("counterCreated", s.GetCounterCreated().String()),
		sypl.WithField("counterDone", s.GetCounterDone().String()),
		sypl.WithField("counterFailed", s.GetCounterFailed().String()),
		sypl.WithField("counterRunning", s.GetCounterRunning().String()),
		sypl.WithField("duration", s.GetDuration().String()),
		sypl.WithField("progress", s.GetProgress().String()),
		sypl.WithField("progressPercent", s.GetProgressPercent().String()),
		sypl.WithField("status", s.GetStatus().String()),
	)

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
		Conversor: conversor,
		Logger:    logging.Get().New(name).SetTags(Type, name),

		CreatedAt:  time.Now(),
		Name:       name,
		Processors: processors,

		CounterCreated: metrics.NewIntWithPattern(Type, name, status.Created),
		CounterDone:    metrics.NewIntWithPattern(Type, name, status.Done),
		CounterFailed:  metrics.NewIntWithPattern(Type, name, status.Failed),
		CounterRunning: metrics.NewIntWithPattern(Type, name, status.Runnning),

		Duration:        metrics.NewIntWithPattern(Type, name, "duration"),
		Progress:        metrics.NewIntWithPattern(Type, name, "progress"),
		ProgressPercent: metrics.NewStringWithPattern(Type, name, "progressPercent"),
		Status:          metrics.NewStringWithPattern(Type, name, status.Name),
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
