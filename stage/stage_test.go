package stage

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/etler/processor"
	"github.com/thalesfsp/status"
)

func TestNew(t *testing.T) {
	double, err := processor.New(
		context.Background(),
		"double",
		"doubles the input",
		func(ctx context.Context, in []int) ([]int, error) {
			out := make([]int, len(in))

			for i, v := range in {
				out[i] = v
				out[i] *= 2
			}

			return out, nil
		},
		processor.WithOnFinished(func(ctx context.Context, p processor.IProcessor[int], originalIn []int, processedIn []int) {
			fmt.Println(p.GetName(), "finished")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	stg1, err := New(
		context.Background(),
		"stage-1",
		func(ctx context.Context, tu int) (int, error) {
			return tu, nil
		},
		// Add as many as you want.
		double,
	)
	if err != nil {
		t.Fatal(err)
	}

	out, err := stg1.Run(context.Background(), []int{1, 2, 3, 4, 5})
	assert.NoError(t, err)

	assert.Equal(t, []int{2, 4, 6, 8, 10}, out)

	assert.Equal(t, int64(1), double.GetCounterCreated().Value())
	assert.Equal(t, int64(1), double.GetCounterRunning().Value())
	assert.Equal(t, int64(0), double.GetCounterFailed().Value())
	assert.Equal(t, int64(1), double.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), double.GetStatus().Value())

	assert.Equal(t, int64(1), stg1.GetCounterCreated().Value())
	assert.Equal(t, int64(1), stg1.GetCounterRunning().Value())
	assert.Equal(t, int64(0), stg1.GetCounterFailed().Value())
	assert.Equal(t, int64(1), stg1.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), stg1.GetStatus().Value())
}
