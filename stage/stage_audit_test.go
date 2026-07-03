package stage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v2/converter"
	"github.com/thalesfsp/etler/v2/processor"
	"github.com/thalesfsp/etler/v2/task"
	"github.com/thalesfsp/status"
)

// identityConverter returns an int identity converter.
func identityConverter() converter.IConverter[int, int] {
	return converter.MustDefault(
		func(ctx context.Context, in int) (int, error) {
			return in, nil
		},
	)
}

// Race + correctness: an async processor followed by a sync one. The async
// goroutine must run the async processor (not whatever the loop variable
// points at later) and must read a stable snapshot of the data (no data race
// with the sync processor updating it). Run with -race.
func TestStage_asyncThenSync_runsCorrectProcessor_noRace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	asyncRan := make(chan []int, 1)

	asyncProc, err := processor.New(
		"async-marker",
		"records that it ran and with which input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			asyncRan <- processingData

			return processingData, nil
		},
		processor.WithAsync[int](true),
	)
	require.NoError(t, err)

	syncDouble, err := processor.New(
		"sync-double",
		"doubles the input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))

			for i, v := range processingData {
				out[i] = v * 2
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-async-audit",
		"async then sync",
		identityConverter(),
		asyncProc, syncDouble,
	)
	require.NoError(t, err)

	out, err := stg.Run(ctx, task.MustNew[int, int]([]int{1, 2}))
	require.NoError(t, err)

	// The sync processor output must flow to the converter.
	assert.Equal(t, []int{2, 4}, out.ConvertedData)

	// The async processor must have run — with the data as it was when it was
	// scheduled (the original input, since it is the first processor).
	select {
	case got := <-asyncRan:
		assert.Equal(t, []int{1, 2}, got,
			"async processor must receive the data snapshot from its position in the chain")
	case <-time.After(5 * time.Second):
		t.Fatal("async processor never ran (wrong processor captured by the goroutine?)")
	}
}

// Bad path: a failing sync processor must mark the stage failed AND increment
// the stage's failed counter.
func TestStage_processorError_updatesFailureMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	boom := errors.New("boom-stage-processor-audit")

	failing, err := processor.New(
		"failing-proc-audit",
		"always fails",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return nil, boom
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-proc-err-audit",
		"failing processor",
		identityConverter(),
		failing,
	)
	require.NoError(t, err)

	_, err = stg.Run(ctx, task.MustNew[int, int]([]int{1}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-stage-processor-audit")

	assert.Equal(t, status.Failed.String(), stg.GetStatus().Value())
	assert.Equal(t, int64(1), stg.GetCounterFailed().Value(),
		"a processor failure must count as a stage failure")
}

// Bad path: a failing converter must fail the stage with the cause preserved.
func TestStage_converterError_propagatesCause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	boom := errors.New("boom-stage-converter-audit")

	identity, err := processor.New(
		"identity-conv-err-audit",
		"identity",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	failingConv := converter.MustDefault(
		func(ctx context.Context, in int) (int, error) {
			return 0, boom
		},
	)

	stg, err := New(
		"stage-conv-err-audit",
		"failing converter",
		failingConv,
		identity,
	)
	require.NoError(t, err)

	_, err = stg.Run(ctx, task.MustNew[int, int]([]int{1}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-stage-converter-audit")

	assert.Equal(t, status.Failed.String(), stg.GetStatus().Value())
	assert.Equal(t, int64(1), stg.GetCounterFailed().Value())
}

// Bug: zero values must survive the conversion step at the stage level. The
// conversion previously dropped zero values silently (data loss).
func TestStage_zeroValues_notDropped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	identity, err := processor.New(
		"identity-zero-audit",
		"identity",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-zero-audit",
		"zero values must survive",
		identityConverter(),
		identity,
	)
	require.NoError(t, err)

	out, err := stg.Run(ctx, task.MustNew[int, int]([]int{0, 1, 0, 2}))
	require.NoError(t, err)

	assert.Equal(t, []int{0, 1, 0, 2}, out.ConvertedData,
		"zero values must not be silently dropped by the converter step")
}

// Edge case: a stage without processors must not be created (validation).
func TestStage_new_withoutProcessors_returnsError(t *testing.T) {
	stg, err := New[int, int](
		"stage-no-procs-audit",
		"no processors",
		identityConverter(),
	)
	assert.Nil(t, stg)
	assert.Error(t, err)
}

// Edge case: a stage without a converter must not be created (validation).
func TestStage_new_withoutConverter_returnsError(t *testing.T) {
	identity, err := processor.New(
		"identity-no-conv-audit",
		"identity",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New[int, int](
		"stage-no-conv-audit",
		"no converter",
		nil,
		identity,
	)
	assert.Nil(t, stg)
	assert.Error(t, err)
}

// OnFinished must receive the original task and the final (converted) task.
func TestStage_onFinished_receivesFinalTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	double, err := processor.New(
		"double-onfinished-audit",
		"doubles the input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))

			for i, v := range processingData {
				out[i] = v * 2
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-onfinished-audit",
		"onfinished data check",
		identityConverter(),
		double,
	)
	require.NoError(t, err)

	var gotOriginal, gotFinal task.Task[int, int]

	stg.SetOnFinished(func(ctx context.Context, s IStage[int, int], original task.Task[int, int], final task.Task[int, int]) {
		gotOriginal = original
		gotFinal = final
	})

	_, err = stg.Run(ctx, task.MustNew[int, int]([]int{1, 2}))
	require.NoError(t, err)

	assert.Equal(t, []int{1, 2}, gotOriginal.ProcessingData)
	assert.Equal(t, []int{2, 4}, gotFinal.ProcessingData)
	assert.Equal(t, []int{2, 4}, gotFinal.ConvertedData)
}
