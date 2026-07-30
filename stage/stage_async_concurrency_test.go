package stage

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v3/processor"
	"github.com/thalesfsp/etler/v3/task"
)

// Race: Stage.Run and Processor.SetAsync must be safe to use concurrently.
// Run with -race.
func TestStage_asyncConcurrentSetAndRun(t *testing.T) {
	proc, err := processor.New(
		"async-race",
		"concurrent async updates",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-async-race",
		"reads async concurrently",
		identityConverter(),
		proc,
	)
	require.NoError(t, err)

	const iterations = 1000

	start := make(chan struct{})
	errs := make(chan error, 1)

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start

		for range iterations {
			if _, err := stg.Run(context.Background(), task.MustNew[int, int]([]int{1})); err != nil {
				errs <- err

				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		<-start

		for range iterations {
			proc.SetAsync(true)
			proc.SetAsync(false)
		}
	}()

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}
