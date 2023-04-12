// TODO: Add metrics, error handling, logging, context, APM, APM transaction, etc.

package pipeline

import (
	"context"
	"sync"

	"github.com/thalesfsp/etler/adapter"
	"github.com/thalesfsp/etler/processor"
	"github.com/thalesfsp/etler/shared"
	"github.com/thalesfsp/etler/state"
)

// Number is a simple struct to be used in the tests.
type Number struct {
	// Numbers to be processed.
	Numbers []int `json:"numbers"`
}

// Stage definition.
type Stage[C any] struct {
	// Concurrent determines whether the stage should be run concurrently.
	Concurrent bool `json:"concurrent"`

	// Processors to be run in the stage.
	Processors []processor.IProcessor[C] `json:"processors"`
}

// IPipeline defines what a `Pipeline` must do.
type IPipeline[C any] interface {
	shared.IMeta[C]

	Run(ctx context.Context, in []C) (out []C, err error)
}

// Pipeline definition.
type Pipeline[C any] struct {
	// Description of the processor.
	Description string `json:"description"`

	// Name of the processor.
	Name string `json:"name"`

	// Adapters to be used in the pipeline.
	Adapters map[string]adapter.IAdapter[C] `json:"adapters"`

	// Control the pipeline.
	Control chan string `json:"-"`

	// Progress of the pipeline.
	Progress int `json:"progress"`

	// Stages to be used in the pipeline.
	Stages []Stage[C] `json:"stages"`

	// State of the pipeline.
	State state.State `json:"state"`
}

// GetDescription returns the `Description` of the processor.
func (p *Pipeline[C]) GetDescription() string {
	return p.Description
}

// GetName returns the `Name` of the processor.
func (p *Pipeline[C]) GetName() string {
	return p.Name
}

// GetState returns the `State` of the processor.
func (p *Pipeline[C]) GetState() state.State {
	return p.State
}

// SetState sets the `State` of the processor.
func (p *Pipeline[C]) SetState(state state.State) {
	p.State = state
}

// Run the pipeline.
func (p *Pipeline[C]) Run(ctx context.Context, in []C) (out []C, err error) {
	// Set the input of the first stage to be the input of the pipeline.
	out = in

	// Iterate through the stages, passing the output of each stage
	// as the input of the next stage.
	for _, s := range p.Stages {
		if s.Concurrent {
			var wg sync.WaitGroup

			for _, p := range s.Processors {
				wg.Add(1)

				// Start a goroutine to run the stage.
				//
				// TODO: Make it boundaded.
				go func(ctx context.Context, p processor.IProcessor[C]) {
					// Process the data.
					out, err = p.Run(ctx, out)

					wg.Done()
				}(ctx, p)
			}

			wg.Wait()
		} else {
			// Process the data sequentially.
			for _, p := range s.Processors {
				out, err = p.Run(ctx, out)
				if err != nil {
					return out, err
				}
			}
		}
	}

	return out, err
}

// New returns a new pipeline.
func New[C any](
	name string,
	description string,
	stages []Stage[C],
) IPipeline[C] {
	return &Pipeline[C]{
		Control:  make(chan string),
		Name:     name,
		Progress: 0,
		Stages:   stages,
		State:    state.Stopped,
	}
}
