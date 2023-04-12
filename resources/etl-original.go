// TODO: Add metrics, error handling, logging, etc.

package etler

import (
	"fmt"
	"sync"
)

// StageFunc is a function that transforms a slice of values of any type and returns
// the transformed slice and any errors that occurred during processing.
type StageFunc[C any] func(in []C) (out []C, err error)

// Pipeline creates a pipeline that processes a slice of values of any type
// by passing it through a series of stages. Stages can be run concurrently
// or sequentially, depending on the value of the 'concurrent' flag.
func Pipeline[C any](in []C, concurrent bool, stages ...StageFunc[C]) (out []C, err error) {
	// Set the input of the first stage to be the input of the pipeline.
	out = in

	var wg sync.WaitGroup

	// Iterate through the stages, passing the output of each stage
	// as the input of the next stage.
	for _, s := range stages {
		if concurrent {
			wg.Add(1)
			// Start a goroutine to run the stage.
			go func(stage StageFunc[C]) {
				// Process the data.
				out, err = stage(out)
				wg.Done()
			}(s)
		} else {
			// Process the data sequentially.
			out, err = s(out)
			if err != nil {
				return out, err
			}
		}
	}

	wg.Wait()

	return out, err
}

func main() {
	// Define the stages of the pipeline.
	double := func(in []int) ([]int, error) {
		out := make([]int, len(in))
		for i, v := range in {
			out[i] = v * 2
		}
		return out, nil
	}

	square := func(in []int) ([]int, error) {
		out := make([]int, len(in))
		for i, v := range in {
			out[i] = v * v
		}
		return out, nil
	}

	// Run the pipeline with some input data.
	out, err := Pipeline([]int{1, 2, 3}, true, double, square)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(out) // Output: [4 16 36]
}
