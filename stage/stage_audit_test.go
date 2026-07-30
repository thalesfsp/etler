package stage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/task"
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

// Regression: an async processor is only complete after it returns
// successfully, not when its goroutine is launched.
func TestStage_asyncProgress_waitsForSuccessfulCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	processorStarted := make(chan struct{})
	releaseProcessor := make(chan struct{})
	converterStarted := make(chan struct{})

	blockingAsync, err := processor.New(
		"blocking-async-progress-audit",
		"blocks until the test releases it",
		func(ctx context.Context, processingData []int) ([]int, error) {
			close(processorStarted)
			<-releaseProcessor

			return processingData, nil
		},
		processor.WithAsync[int](true),
	)
	require.NoError(t, err)

	var converterStartedOnce sync.Once

	stg, err := New(
		"stage-async-progress-audit",
		"async progress waits for completion",
		converter.MustDefault(
			func(ctx context.Context, in int) (int, error) {
				converterStartedOnce.Do(func() {
					close(converterStarted)
				})

				return in, nil
			},
		),
		blockingAsync,
	)
	require.NoError(t, err)

	runDone := make(chan error, 1)

	go func() {
		_, err := stg.Run(ctx, task.MustNew[int, int]([]int{1}))
		runDone <- err
	}()

	<-processorStarted
	<-converterStarted

	assert.Equal(t, int64(0), stg.GetProgress().Value(),
		"a blocked async processor must not count as complete")
	assert.NotEqual(t, "100%", stg.GetProgressPercent().Value(),
		"a blocked async processor must not report full progress")

	close(releaseProcessor)
	require.NoError(t, <-runDone)

	assert.Equal(t, int64(1), stg.GetProgress().Value())
	assert.Equal(t, "100%", stg.GetProgressPercent().Value())
}

// Regression: a failed async processor never contributes completed progress.
func TestStage_asyncProgress_failureIsNotComplete(t *testing.T) {
	boom := errors.New("boom-async-progress-audit")

	failingAsync, err := processor.New(
		"failing-async-progress-audit",
		"fails asynchronously",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return nil, boom
		},
		processor.WithAsync[int](true),
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-async-progress-failure-audit",
		"failed async work is incomplete",
		identityConverter(),
		failingAsync,
	)
	require.NoError(t, err)

	_, err = stg.Run(context.Background(), task.MustNew[int, int]([]int{1}))
	require.ErrorIs(t, err, boom, "the stage must surface the async processor failure")

	assert.Equal(t, int64(0), stg.GetProgress().Value(),
		"a failed async processor must not count as complete")
	assert.NotEqual(t, "100%", stg.GetProgressPercent().Value(),
		"a failed async processor must not report full progress")
}

// Race: the async goroutine must own a COPY of the data, not just a copy of
// the slice header — otherwise a later sync processor mutating the slice in
// place races on the shared backing array. Run with -race.
func TestStage_asyncSnapshot_isolatedFromInPlaceMutation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	asyncSum := make(chan int, 1)

	reader, err := processor.New(
		"async-reader",
		"reads every element",
		func(ctx context.Context, processingData []int) ([]int, error) {
			sum := 0

			for _, v := range processingData {
				sum += v
			}

			asyncSum <- sum

			return processingData, nil
		},
		processor.WithAsync[int](true),
	)
	require.NoError(t, err)

	inPlace, err := processor.New(
		"sync-in-place",
		"mutates the slice in place",
		func(ctx context.Context, processingData []int) ([]int, error) {
			for i := range processingData {
				processingData[i]++
			}

			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-async-snapshot-audit",
		"async isolation from in-place mutation",
		identityConverter(),
		reader, inPlace,
	)
	require.NoError(t, err)

	out, err := stg.Run(ctx, task.MustNew[int, int]([]int{1, 2, 3}))
	require.NoError(t, err)
	assert.Equal(t, []int{2, 3, 4}, out.ConvertedData)

	select {
	case sum := <-asyncSum:
		// The async processor saw either the pristine snapshot (sum 6); it
		// must never see a torn/mutated view of a shared array.
		assert.Equal(t, 6, sum)
	case <-time.After(5 * time.Second):
		t.Fatal("async processor never ran")
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

// Regression: every run replaces the duration metric, including failed runs.
func TestStage_failedRun_replacesPreviousDuration(t *testing.T) {
	boom := errors.New("boom-stage-duration-audit")

	runs := 0

	slowThenFail, err := processor.New(
		"slow-then-fail-stage-duration-audit",
		"succeeds slowly, then fails fast",
		func(ctx context.Context, processingData []int) ([]int, error) {
			runs++

			if runs == 1 {
				time.Sleep(200 * time.Millisecond)

				return processingData, nil
			}

			return nil, boom
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-duration-audit",
		"failed run replaces duration",
		identityConverter(),
		slowThenFail,
	)
	require.NoError(t, err)

	_, err = stg.Run(context.Background(), task.MustNew[int, int]([]int{1}))
	require.NoError(t, err)

	successfulDuration := stg.GetDuration().Value()
	require.Positive(t, successfulDuration)

	_, err = stg.Run(context.Background(), task.MustNew[int, int]([]int{1}))
	require.ErrorIs(t, err, boom)

	assert.Less(t, stg.GetDuration().Value(), successfulDuration,
		"the failed stage run must replace the previous successful duration")
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
