package stage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/status"

	"github.com/thalesfsp/etler/v2/converters/passthru"
	"github.com/thalesfsp/etler/v2/processor"
	"github.com/thalesfsp/etler/v2/task"
)

func TestNew(t *testing.T) {
	// out is a buffer which holds string content.
	onFinishedTXTBuffer := strings.Builder{}

	double, err := processor.New(
		"double",
		"doubles the input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))

			for i, v := range processingData {
				out[i] = v
				out[i] *= 2
			}

			// Artificially add some delay.
			time.Sleep(300 * time.Millisecond)

			return out, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[int], originalIn []int, processedOut []int) {
			onFinishedTXTBuffer.WriteString(fmt.Sprintf("%s finished\n", p.GetName()))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	plusOne, err := processor.New(
		"plusOne",
		"adds 1 to the input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))

			for i, v := range processingData {
				out[i] = v
				out[i]++
			}

			// Artificially add some delay.
			time.Sleep(125 * time.Millisecond)

			return out, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[int], originalIn []int, processedOut []int) {
			onFinishedTXTBuffer.WriteString(fmt.Sprintf("%s finished\n", p.GetName()))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	stg1, err := New(
		"stage-1",
		"main stage",

		// Add as many as you want.
		passthru.Must[int](),

		// Add as many as you want.
		double, plusOne,
	)
	if err != nil {
		t.Fatal(err)
	}

	tskOut, err := stg1.Run(context.Background(), task.Task[int, int]{
		ProcessingData: []int{1, 2, 3, 4, 5},
	})
	assert.NoError(t, err)

	// Validates output.
	assert.Equal(t, []int{3, 5, 7, 9, 11}, tskOut.ConvertedData)

	// Validates processors metrics.
	assert.Equal(t, int64(1), double.GetCounterCreated().Value())
	assert.Equal(t, int64(1), double.GetCounterRunning().Value())
	assert.Equal(t, int64(0), double.GetCounterFailed().Value())
	assert.Equal(t, int64(1), double.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), double.GetStatus().Value())

	assert.Equal(t, int64(1), plusOne.GetCounterCreated().Value())
	assert.Equal(t, int64(1), plusOne.GetCounterRunning().Value())
	assert.Equal(t, int64(0), plusOne.GetCounterFailed().Value())
	assert.Equal(t, int64(1), plusOne.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), plusOne.GetStatus().Value())

	// Validates stage metrics.
	assert.Equal(t, int64(0), stg1.GetCounterFailed().Value())
	assert.Equal(t, int64(1), stg1.GetCounterCreated().Value())
	assert.Equal(t, int64(1), stg1.GetCounterDone().Value())
	assert.Equal(t, int64(1), stg1.GetCounterRunning().Value())

	assert.Equal(t, "100%", stg1.GetProgressPercent().Value())
	assert.Equal(t, int64(2), stg1.GetProgress().Value())
	assert.Equal(t, true, stg1.GetDuration().Value() > int64(100))
	assert.NotEmpty(t, stg1.GetCreatedAt())
	assert.Equal(t, status.Done.String(), stg1.GetStatus().Value())

	// Validates onFinished function.
	assert.Equal(t, true, strings.Contains(onFinishedTXTBuffer.String(), "double finished"))
	assert.Equal(t, true, strings.Contains(onFinishedTXTBuffer.String(), "plusOne finished"))
}
