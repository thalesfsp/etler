package processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/status"
)

func TestNew(toBeProcessed *testing.T) {
	double, err := New(
		"double",
		"doubles the input",
		func(ctx context.Context, toBeProcessed []int) ([]int, error) {
			processedOut := make([]int, len(toBeProcessed))

			for i, v := range toBeProcessed {
				processedOut[i] = v
				processedOut[i] *= 2
			}

			return processedOut, nil
		},
		WithOnFinished(func(ctx context.Context, p IProcessor[int], originalIn []int, processedOut []int) {
			fmt.Println(p.GetName(), "finished")
		}),
	)
	if err != nil {
		toBeProcessed.Fatal(err)
	}

	processedOut, err := double.Run(context.Background(), []int{1, 2, 3, 4, 5})
	assert.NoError(toBeProcessed, err)

	assert.Equal(toBeProcessed, []int{2, 4, 6, 8, 10}, processedOut)

	// Should check if the metrics are working.
	assert.Equal(toBeProcessed, int64(1), double.GetCounterCreated().Value())
	assert.Equal(toBeProcessed, int64(1), double.GetCounterRunning().Value())
	assert.Equal(toBeProcessed, int64(0), double.GetCounterFailed().Value())
	assert.Equal(toBeProcessed, int64(1), double.GetCounterDone().Value())
	assert.Equal(toBeProcessed, status.Done.String(), double.GetStatus().Value())
}
