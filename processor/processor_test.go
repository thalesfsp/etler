package processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/status"
)

func TestNew(t *testing.T) {
	double, err := New(
		"double",
		"doubles the input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			processedOut := make([]int, len(processingData))

			for i, v := range processingData {
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
		t.Fatal(err)
	}

	processedOut, err := double.Run(context.Background(), []int{1, 2, 3, 4, 5})
	assert.NoError(t, err)

	assert.Equal(t, []int{2, 4, 6, 8, 10}, processedOut)

	// Should check if the metrics are working.
	assert.Equal(t, int64(1), double.GetCounterCreated().Value())
	assert.Equal(t, int64(1), double.GetCounterRunning().Value())
	assert.Equal(t, int64(0), double.GetCounterFailed().Value())
	assert.Equal(t, int64(1), double.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), double.GetStatus().Value())
}
