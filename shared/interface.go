package shared

import "github.com/thalesfsp/status"

// IMeta defines what a `Processor` must do.
type IMeta[C any] interface {
	// GetDescription returns the `Description` of the processor.
	GetDescription() string

	// GetName returns the `Name` of the processor.
	GetName() string

	// GetState returns the `State` of the processor.
	GetState() status.Status

	// SetState sets the `State` of the processor.
	SetState(state status.Status)
}
