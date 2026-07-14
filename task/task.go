package task

import (
	"time"

	"github.com/thalesfsp/etler/v3/internal/shared"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Type of the entity.
const (
	Name = "task"
	Type = "task"
)

// Task encapsulates the work to be done plus some metadata.
type Task[ProcessingData, ConvertedData any] struct {
	// Logger of the job.
	Logger sypl.ISypl `json:"-"`

	// ID of the job.
	ID string `json:"id,omitempty"`

	// CreatedAt date.
	CreatedAt string `json:"createdAt,omitempty"`

	// Tags of the job, for example, processors adds their name to the tags
	// indicating that they have processed the job.
	Tags []string `json:"tags,omitempty"`

	// ProcessingData is the input of the task.
	ProcessingData []ProcessingData `json:"in" validate:"required"`

	// ConvertedData is the output of the task.
	ConvertedData []ConvertedData `json:"out"`
}

//////
// Factory.
//////

// New returns a new stage.
func New[ProcessingData, ConvertedData any](
	processingData []ProcessingData,
) (Task[ProcessingData, ConvertedData], error) {
	tsk := Task[ProcessingData, ConvertedData]{
		// Default level is set to `none`. Use `SYPL_LEVEL` to change that.
		Logger: sypl.NewDefault(Name, level.None),

		ID:        shared.GenerateUUID(),
		CreatedAt: time.Now().Format(time.RFC3339),

		ProcessingData: processingData,
		ConvertedData:  make([]ConvertedData, 0),
	}

	// Validation.
	if err := validation.Validate(&tsk); err != nil {
		return Task[ProcessingData, ConvertedData]{}, err
	}

	return tsk, nil
}

// MustNew returns a new stage or panics.
func MustNew[ProcessingData, ConvertedData any](
	processingData []ProcessingData,
) Task[ProcessingData, ConvertedData] {
	tsk, err := New[ProcessingData, ConvertedData](processingData)
	if err != nil {
		panic(err)
	}

	return tsk
}
