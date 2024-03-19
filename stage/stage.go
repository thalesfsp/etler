package stage

import (
	"context"
	"expvar"
	"fmt"
	"time"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/internal/customapm"
	"github.com/thalesfsp/etler/v2/internal/logging"
	"github.com/thalesfsp/etler/v2/internal/metrics"
	"github.com/thalesfsp/etler/v2/internal/shared"
	"github.com/thalesfsp/etler/v2/processor"
	"github.com/thalesfsp/etler/v2/task"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "stage"

// Stage definition.
type Stage[ProcessingData, ConvertedData any] struct {
	// Description of the stage.
	Description string `json:"description"`

	// Conversor to be used tsk the stage.
	Conversor concurrentloop.MapFunc[ProcessingData, ConvertedData] `json:"-" validate:"required"`

	// Logger is the internal logger.
	Logger sypl.ISypl `json:"-" validate:"required"`

	// Name of the stage.
	Name string `json:"name" validate:"required"`

	// OnFinished is the function that is called when a processor finishes its
	// execution.
	OnFinished OnFinished[ProcessingData, ConvertedData] `json:"-"`

	// Processors to be run tsk the stage.
	Processors []processor.IProcessor[ProcessingData] `json:"processors" validate:"required,gt=0"`

	// Metrics.
	CounterCreated *expvar.Int `json:"counterCreated"`
	CounterDone    *expvar.Int `json:"counterDone"`
	CounterFailed  *expvar.Int `json:"counterFailed"`
	CounterRunning *expvar.Int `json:"counterRunning"`

	CreatedAt       time.Time      `json:"createdAt"`
	Duration        *expvar.Int    `json:"duration"`
	Progress        *expvar.Int    `json:"progress"`
	ProgressPercent *expvar.String `json:"progressPercent"`
	Status          *expvar.String `json:"status"`
}

//////
// Methods.
//////

// GetDescription returns the `Description` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetDescription() string {
	return s.Description
}

// GetLogger returns the `Logger` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetLogger() sypl.ISypl {
	return s.Logger
}

// GetName returns the `Name` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetName() string {
	return s.Name
}

// GetCounterCreated returns the `CounterCreated` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetCounterCreated() *expvar.Int {
	return s.CounterCreated
}

// GetCounterRunning returns the `CounterRunning` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetCounterRunning() *expvar.Int {
	return s.CounterRunning
}

// GetCounterFailed returns the `CounterFailed` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetCounterFailed() *expvar.Int {
	return s.CounterFailed
}

// GetCounterDone returns the `CounterDone` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetCounterDone() *expvar.Int {
	return s.CounterDone
}

// GetProgress returns the `CounterProgress` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetProgress() *expvar.Int {
	return s.Progress
}

// GetProgressPercent returns the `ProgressPercent` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetProgressPercent() *expvar.String {
	return s.ProgressPercent
}

// SetProgressPercent sets the `ProgressPercent` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) SetProgressPercent() {
	currentProgress := s.GetProgress().Value()
	totalProgress := len(s.Processors)
	percentage := float64(currentProgress) / float64(totalProgress) * 100

	s.GetProgressPercent().Set(fmt.Sprintf("%d%%", int(percentage)))
}

// GetStatus returns the `Status` metric.
func (s *Stage[ProcessingData, ConvertedData]) GetStatus() *expvar.String {
	return s.Status
}

// GetOnFinished returns the `OnFinished` function.
func (s *Stage[ProcessingData, ConvertedData]) GetOnFinished() OnFinished[ProcessingData, ConvertedData] {
	return s.OnFinished
}

// SetOnFinished sets the `OnFinished` function.
func (s *Stage[ProcessingData, ConvertedData]) SetOnFinished(onFinished OnFinished[ProcessingData, ConvertedData]) {
	s.OnFinished = onFinished
}

// GetType returns the entity type.
func (s *Stage[ProcessingData, ConvertedData]) GetType() string {
	return Type
}

// GetCreatedAt returns the created at time.
func (s *Stage[ProcessingData, ConvertedData]) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetDuration returns the `CounterDuration` of the stage.
func (s *Stage[ProcessingData, ConvertedData]) GetDuration() *expvar.Int {
	return s.Duration
}

// GetMetrics returns the stage's metrics.
func (s *Stage[ProcessingData, ConvertedData]) GetMetrics() map[string]string {
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
func (s *Stage[ProcessingData, ConvertedData]) Run(ctx context.Context, tsk task.Task[ProcessingData, ConvertedData]) (task.Task[ProcessingData, ConvertedData], error) {
	//////
	// Observability: tracing, metrics, status, logging, etc.
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

	s.GetLogger().PrintlnWithOptions(level.Debug, status.Runnning.String())

	now := time.Now()

	//////
	// Run stage.
	//////

	// Store as reference to be used in the OnFinished function.
	originalTask := tsk

	// Store as reference to be used as the input of the next processor.
	retroFeedIn := originalTask.ProcessingData

	// NOTE: It process the data sequentially.
	for _, proc := range s.Processors {
		// Re-use the output of the previous stage as the input of the
		// next stage ensuring that the data is processed sequentially.
		rFI, err := proc.Run(tracedContext, retroFeedIn)
		if err != nil {
			//////
			// Observability: tracing, metrics, status, logging, etc.
			//////

			s.GetStatus().Set(status.Failed.String())

			// Returns whatever is tsk `out` and the error.
			//
			// Don't need tracing, it's already traced.
			return task.Task[ProcessingData, ConvertedData]{}, err
		}

		// Update the input with the output.
		retroFeedIn = rFI

		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

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

	convertedData, errs := concurrentloop.Map(tracedContext, retroFeedIn, s.Conversor)
	if errs != nil {
		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////
		s.GetStatus().Set(status.Failed.String())

		//////
		// Observability: tracing, metrics, status, logging, etc.
		//////

		return task.Task[ProcessingData, ConvertedData]{}, shared.OnErrorHandler(
			tracedContext,
			s,
			s.GetLogger(),
			"convert",
			Type,
			s.GetName(),
		)
	}

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	s.GetStatus().Set(status.Done.String())

	s.GetCounterDone().Add(1)

	s.GetDuration().Set(time.Since(s.GetCreatedAt()).Milliseconds())

	//////
	// Updates task's data.
	//////

	tsk.ProcessingData = retroFeedIn

	tsk.ConvertedData = convertedData

	if s.GetOnFinished() != nil {
		s.GetOnFinished()(ctx, s, originalTask, tsk)
	}

	// Set duration.
	s.GetDuration().Set(time.Since(now).Milliseconds())

	// Print the stage's status.
	s.GetLogger().PrintWithOptions(
		level.Debug,
		status.Done.String(),
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

	return tsk, nil
}

//////
// Factory.
//////

// New returns a new stage.
func New[ProcessingData, ConvertedData any](
	name string,
	description string,
	conversor concurrentloop.MapFunc[ProcessingData, ConvertedData],
	processors ...processor.IProcessor[ProcessingData],
) (IStage[ProcessingData, ConvertedData], error) {
	s := &Stage[ProcessingData, ConvertedData]{
		Logger:     logging.Get().New(name).SetTags(Type, name),
		Processors: processors,
		Conversor:  conversor,

		CreatedAt:   time.Now(),
		Name:        name,
		Description: description,

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

	//////
	// Observability: tracing, metrics, status, logging, etc.
	//////

	s.GetStatus().Set(status.Created.String())

	s.GetCounterCreated().Add(1)

	s.GetLogger().PrintlnWithOptions(level.Debug, status.Created.String())

	return s, nil
}
