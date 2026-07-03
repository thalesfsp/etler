package stage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v3/processor"
	"github.com/thalesfsp/etler/v3/task"
	"github.com/thalesfsp/status"
)

// v3: the stage must JOIN async processors before completing — when Run
// returns, every async processor has finished. Deterministic by design (no
// sleeps needed).
func TestStage_waitsForAsyncProcessors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var asyncDone atomic.Bool

	slowAsync, err := processor.New(
		"slow-async-audit",
		"finishes late and flags completion",
		func(ctx context.Context, processingData []int) ([]int, error) {
			time.Sleep(300 * time.Millisecond)

			asyncDone.Store(true)

			return processingData, nil
		},
		processor.WithAsync[int](true),
	)
	require.NoError(t, err)

	identity, err := processor.New(
		"identity-join-audit",
		"identity",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-join-audit",
		"joins async processors",
		identityConverter(),
		slowAsync, identity,
	)
	require.NoError(t, err)

	out, err := stg.Run(ctx, task.MustNew[int, int]([]int{1}))
	require.NoError(t, err)
	assert.Equal(t, []int{1}, out.ConvertedData)

	assert.True(t, asyncDone.Load(),
		"Run must not return before async processors have finished")
	assert.Equal(t, status.Done.String(), stg.GetStatus().Value())
}

// v3: an async processor failure FAILS the stage — errors are no longer
// silently reduced to a status flip.
func TestStage_asyncProcessorError_failsStage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	boom := errors.New("boom-async-audit")

	failingAsync, err := processor.New(
		"failing-async-audit",
		"always fails, asynchronously",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return nil, boom
		},
		processor.WithAsync[int](true),
	)
	require.NoError(t, err)

	identity, err := processor.New(
		"identity-async-err-audit",
		"identity",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-async-err-audit",
		"async failure fails the stage",
		identityConverter(),
		failingAsync, identity,
	)
	require.NoError(t, err)

	_, err = stg.Run(ctx, task.MustNew[int, int]([]int{1}))
	require.Error(t, err, "an async processor failure must fail the stage")
	assert.Contains(t, err.Error(), "boom-async-audit")

	assert.Equal(t, status.Failed.String(), stg.GetStatus().Value())
	assert.Equal(t, int64(1), stg.GetCounterFailed().Value())
}
