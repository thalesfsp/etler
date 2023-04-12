package shared

import "github.com/thalesfsp/etler/state"

// IMeta defines what a `Processor` must do.
type IMeta[C any] interface {
	// GetDescription returns the `Description` of the processor.
	GetDescription() string

	// GetName returns the `Name` of the processor.
	GetName() string

	// GetState returns the `State` of the processor.
	GetState() state.State

	// SetState sets the `State` of the processor.
	SetState(state state.State)
}
