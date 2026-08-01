// Package task defines the data and metadata passed between pipeline stages.
package task

import (
	"time"

	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

const (
	// Name identifies the default task logger.
	Name = "task"

	// Type identifies task entities.
	Type = "task"
)

// Task encapsulates the work to be done plus some metadata.
type Task[ProcessingData, ConvertedData any] struct {
	// Logger records task-related events and is omitted from JSON.
	Logger sypl.ISypl `json:"-"`

	// ID is the task's generated UUID.
	ID string `json:"id,omitempty"`

	// CreatedAt is the task creation time in RFC 3339 format.
	CreatedAt string `json:"createdAt,omitempty"`

	// Tags contains caller-managed task labels.
	Tags []string `json:"tags,omitempty"`

	// ProcessingData contains the values stages process.
	ProcessingData []ProcessingData `json:"in" validate:"required"`

	// ConvertedData contains the values produced by the most recent stage.
	ConvertedData []ConvertedData `json:"out"`
}

//////
// Factory.
//////

// New creates a task for processingData with a UUID, an RFC 3339 creation time,
// and a non-nil empty ConvertedData slice. A nil processingData slice is
// rejected; an empty non-nil slice is accepted.
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

// MustNew creates a task or panics if processingData is invalid.
func MustNew[ProcessingData, ConvertedData any](
	processingData []ProcessingData,
) Task[ProcessingData, ConvertedData] {
	tsk, err := New[ProcessingData, ConvertedData](processingData)
	if err != nil {
		panic(err)
	}

	return tsk
}
