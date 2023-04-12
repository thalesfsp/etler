package processor

import (
	"context"

	"github.com/thalesfsp/etler/shared"
	"github.com/thalesfsp/etler/state"
)

// IProcessor defines what a `Processor` must do.
type IProcessor[C any] interface {
	shared.IMeta[C]

	// Run the transform function.
	Run(ctx context.Context, in []C) (out []C, err error)
}

// Processor definition.
type Processor[C any] struct {
	// Name of the processor.
	Name string `json:"name"`

	// Description of the processor.
	Description string `json:"description"`

	// Transform function.
	Func shared.Run[C] `json:"-"`

	// State of the processor.
	State state.State `json:"state"`
}

// GetDescription returns the `Description` of the processor.
func (p *Processor[C]) GetDescription() string {
	return p.Description
}

// GetName returns the `Name` of the processor.
func (p *Processor[C]) GetName() string {
	return p.Name
}

// GetState returns the `State` of the processor.
func (p *Processor[C]) GetState() state.State {
	return p.State
}

// SetState sets the `State` of the processor.
func (p *Processor[C]) SetState(state state.State) {
	p.State = state
}

// Run the transform function.
func (p *Processor[C]) Run(ctx context.Context, in []C) (out []C, err error) {
	return p.Func(ctx, in)
}

// New returns a new processor.
func New[C any](name string, description string, fn shared.Run[C]) IProcessor[C] {
	return &Processor[C]{
		Description: description,
		Func:        fn,
		Name:        name,
		State:       state.Stopped,
	}
}
