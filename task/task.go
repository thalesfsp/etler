package task

import "github.com/thalesfsp/validation"

//////
// Consts, vars and types.
//////

// Type of the entity.
const Type = "stage"

// Task definition.
type Task[ProcessingData, ConvertedData any] struct {
	// ProcessingData is the input of the task.
	ProcessingData []ProcessingData `json:"in" validate:"required"`

	// ConvertedData is the output of the task.
	ConvertedData []ConvertedData `json:"out"`
}

//////
// Methods.
//////

// Initialize if empty, task's processing and converted data.
//
// TODO: Remove it.
func (t Task[ProcessingData, ConvertedData]) Initialize() {
	if t.ProcessingData == nil {
		t.ProcessingData = make([]ProcessingData, 0)
	}

	if t.ConvertedData == nil {
		t.ConvertedData = make([]ConvertedData, 0)
	}
}

//////
// Factory.
//////

// New returns a new stage.
func New[ProcessingData, ConvertedData any](
	processingData []ProcessingData,
) (Task[ProcessingData, ConvertedData], error) {
	tsk := Task[ProcessingData, ConvertedData]{
		ProcessingData: processingData,
	}

	tsk.Initialize()

	// Validation.
	if err := validation.Validate(&tsk); err != nil {
		return Task[ProcessingData, ConvertedData]{}, err
	}

	return tsk, nil
}
