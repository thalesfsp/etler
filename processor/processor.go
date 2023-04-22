package processor

import (
	"context"

	"github.com/WreckingBallStudioLabs/etler/shared"
	"github.com/thalesfsp/status"
)

type IProcessor[In any, Out any] interface {
	shared.IMeta[In]

	// Run the transform function.
	Run(ctx context.Context, in []In) (out []Out, err error)
}

// Processor definition.
type Processor[In any, Out any] struct {
	// Name of the processor.
	Name string `json:"name"`

	// Description of the processor.
	Description string `json:"description"`

	// Transform function.
	Func shared.Run[In, Out] `json:"-"`

	// State of the processor.
	State status.Status `json:"state"`
}

// GetDescription returns the `Description` of the processor.
func (p *Processor[In, Out]) GetDescription() string {
	return p.Description
}

// GetName returns the `Name` of the processor.
func (p *Processor[In, Out]) GetName() string {
	return p.Name
}

// GetState returns the `State` of the processor.
func (p *Processor[In, Out]) GetState() status.Status {
	return p.State
}

// SetState sets the `State` of the processor.
func (p *Processor[In, Out]) SetState(state status.Status) {
	p.State = state
}

// Run the transform function.
func (p *Processor[In, Out]) Run(ctx context.Context, in []In) (out []Out, err error) {
	return p.Func(ctx, in)
}

// New returns a new processor.
func New[In any, Out any](
	name string,
	description string,
	fn shared.Run[In, Out],
) IProcessor[In, Out] {
	return &Processor[In, Out]{
		Description: description,
		Func:        fn,
		Name:        name,
		State:       status.Stopped,
	}
}
