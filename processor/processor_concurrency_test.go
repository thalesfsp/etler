package processor

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Race: Run and SetOnFinished must be safe to use concurrently. Run with
// -race.
func TestProcessor_onFinishedConcurrentSetAndRun(t *testing.T) {
	p, err := New(
		"onfinished-race",
		"concurrent callback updates",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	callback := func(ctx context.Context, p IProcessor[int], originalIn, processedOut []int) {}

	const iterations = 1000

	start := make(chan struct{})
	errs := make(chan error, 1)

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start

		for range iterations {
			if _, err := p.Run(context.Background(), []int{1}); err != nil {
				errs <- err

				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		<-start

		for range iterations {
			p.SetOnFinished(callback)
			p.SetOnFinished(nil)
		}
	}()

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestProcessor_onFinishedSetRunAndClear(t *testing.T) {
	p, err := New(
		"onfinished-normal",
		"callback arguments and clearing",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return []int{processingData[0] * 2}, nil
		},
	)
	require.NoError(t, err)

	var (
		calls       int
		gotOriginal []int
		gotOut      []int
	)

	p.SetOnFinished(func(ctx context.Context, gotProcessor IProcessor[int], originalIn, processedOut []int) {
		calls++
		gotOriginal = originalIn
		gotOut = processedOut
		assert.Same(t, p, gotProcessor)
	})

	assert.NotNil(t, p.GetOnFinished())

	out, err := p.Run(context.Background(), []int{2})
	require.NoError(t, err)
	assert.Equal(t, []int{4}, out)
	assert.Equal(t, 1, calls)
	assert.Equal(t, []int{2}, gotOriginal)
	assert.Equal(t, []int{4}, gotOut)

	p.SetOnFinished(nil)
	assert.Nil(t, p.GetOnFinished())

	assert.NotPanics(t, func() {
		_, err = p.Run(context.Background(), []int{3})
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}
