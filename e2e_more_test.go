package etler_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/etler/v4/converters/passthru"
	"github.com/thalesfsp/etler/v4/pipeline"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/stage"
	"github.com/thalesfsp/etler/v4/task"
	"github.com/thalesfsp/status"
)

func e2eIdentityProcessor[T any](t *testing.T, name string) processor.IProcessor[T] {
	t.Helper()

	proc, err := processor.New(
		name,
		"returns its input unchanged",
		func(ctx context.Context, processingData []T) ([]T, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	return proc
}

func e2eIdentityStage[T any](t *testing.T, name string) stage.IStage[T, T] {
	t.Helper()

	stg, err := stage.New(
		name,
		"passes data through unchanged",
		passthru.Must[T](),
		e2eIdentityProcessor[T](t, name+"-identity"),
	)
	require.NoError(t, err)

	return stg
}

func e2eOffsetStage(t *testing.T, name string, offset int) stage.IStage[int, int] {
	t.Helper()

	offsetter, err := processor.New(
		name+"-offset",
		"adds a stage-specific offset",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))

			for i, value := range processingData {
				out[i] = value + offset
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		name,
		"produces a stage-specific result",
		passthru.Must[int](),
		offsetter,
	)
	require.NoError(t, err)

	return stg
}

func TestE2E_concurrentStages_returnsEveryResultAndFullProgress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := pipeline.New(
		"e2e-concurrent-stages",
		"runs independent stages concurrently",
		true,
		e2eOffsetStage(t, "e2e-concurrent-stage-10", 10),
		e2eOffsetStage(t, "e2e-concurrent-stage-20", 20),
		e2eOffsetStage(t, "e2e-concurrent-stage-30", 30),
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{1, 2})
	require.NoError(t, err)
	require.Len(t, out, 3)

	converted := make([][]int, 0, len(out))
	for _, stageResult := range out {
		converted = append(converted, stageResult.ConvertedData)
	}

	assert.ElementsMatch(t, [][]int{
		{11, 12},
		{21, 22},
		{31, 32},
	}, converted)
	assert.Equal(t, int64(3), p.GetProgress().Value())
	assert.Equal(t, "100%", p.GetProgressPercent().Value())
	assert.Equal(t, status.Done.String(), p.GetStatus().Value())
}

func TestE2E_converterChangesType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	toInt, err := converter.New(
		"e2e-string-to-int",
		"parses decimal strings",
		func(ctx context.Context, in string) (int, error) {
			return strconv.Atoi(in)
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		"e2e-type-changing-stage",
		"converts strings to integers",
		toInt,
		e2eIdentityProcessor[string](t, "e2e-string-identity"),
	)
	require.NoError(t, err)

	p, err := pipeline.New(
		"e2e-type-changing-pipeline",
		"changes the pipeline output type",
		false,
		stg,
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []string{"7", "0", "-3"})
	require.NoError(t, err)
	require.Len(t, out, 1)

	converted := out[0].ConvertedData
	assert.Equal(t, []int{7, 0, -3}, converted)
	assert.Equal(t, []string{"7", "0", "-3"}, out[0].ProcessingData)
}

func TestE2E_processorErrorFailsPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	boom := errors.New("e2e processor failure")

	double, err := processor.New(
		"e2e-double-before-failure",
		"transforms data before the failing processor",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))

			for i, value := range processingData {
				out[i] = value * 2
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	doubleStage, err := stage.New(
		"e2e-before-processor-error-stage",
		"transforms data before the failing stage",
		passthru.Must[int](),
		double,
	)
	require.NoError(t, err)

	failingInput := make(chan []int, 1)

	failing, err := processor.New(
		"e2e-failing-processor",
		"fails after observing the previous stage output",
		func(ctx context.Context, processingData []int) ([]int, error) {
			failingInput <- append([]int(nil), processingData...)

			return nil, boom
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		"e2e-processor-error-stage",
		"contains a failing processor",
		passthru.Must[int](),
		failing,
	)
	require.NoError(t, err)

	p, err := pipeline.New(
		"e2e-processor-error-pipeline",
		"surfaces processor failures",
		false,
		doubleStage,
		stg,
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{2, 4})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, []int{4, 8}, <-failingInput)
	assert.Equal(t, int64(1), p.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), p.GetStatus().Value())
	assert.Equal(t, int64(1), stg.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), stg.GetStatus().Value())
}

func TestE2E_converterErrorFailsPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	boom := errors.New("e2e converter failure")

	failing, err := converter.New(
		"e2e-failing-converter",
		"always fails",
		func(ctx context.Context, in int) (string, error) {
			return "", boom
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		"e2e-converter-error-stage",
		"contains a failing converter",
		failing,
		e2eIdentityProcessor[int](t, "e2e-converter-error-identity"),
	)
	require.NoError(t, err)

	p, err := pipeline.New(
		"e2e-converter-error-pipeline",
		"surfaces converter failures",
		false,
		stg,
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{1})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), boom.Error())
	assert.Equal(t, int64(1), p.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), p.GetStatus().Value())
	assert.Equal(t, int64(1), stg.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), stg.GetStatus().Value())
	assert.Equal(t, int64(1), failing.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), failing.GetStatus().Value())
}

func TestE2E_nilInputSurfacesTaskValidationError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := pipeline.New(
		"e2e-nil-input-pipeline",
		"rejects nil task data",
		false,
		e2eIdentityStage[int](t, "e2e-nil-input-stage"),
	)
	require.NoError(t, err)

	_, validationErr := task.New[int, int](nil)
	require.Error(t, validationErr)

	out, err := p.Run(ctx, nil)
	assert.Nil(t, out)
	require.Error(t, err)
	assert.EqualError(t, err, validationErr.Error())
}

func TestE2E_emptyNonNilInputProducesEmptyResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := pipeline.New(
		"e2e-empty-input-pipeline",
		"accepts an empty non-nil task",
		false,
		e2eIdentityStage[int](t, "e2e-empty-input-stage"),
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.NotNil(t, out[0].ProcessingData)
	assert.Empty(t, out[0].ProcessingData)
	assert.NotNil(t, out[0].ConvertedData)
	assert.Empty(t, out[0].ConvertedData)
	assert.Equal(t, status.Done.String(), p.GetStatus().Value())
}

func TestE2E_converterZeroValuesArePreserved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	zeroForEven, err := converter.New(
		"e2e-zero-value-converter",
		"returns the empty string for even inputs",
		func(ctx context.Context, in int) (string, error) {
			if in%2 == 0 {
				return "", nil
			}

			return strconv.Itoa(in), nil
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		"e2e-zero-value-stage",
		"preserves every converter output",
		zeroForEven,
		e2eIdentityProcessor[int](t, "e2e-zero-value-identity"),
	)
	require.NoError(t, err)

	p, err := pipeline.New(
		"e2e-zero-value-pipeline",
		"guards against zero-value data loss",
		false,
		stg,
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{1, 2, 3, 4})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Len(t, out[0].ConvertedData, 4)
	assert.Equal(t, []string{"1", "", "3", ""}, out[0].ConvertedData)
}

func TestE2E_asyncProcessorErrorFailsPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	boom := errors.New("e2e async processor failure")

	failingAsync, err := processor.New(
		"e2e-failing-async-processor",
		"fails asynchronously",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return nil, boom
		},
	)
	require.NoError(t, err)
	failingAsync.SetAsync(true)

	stg, err := stage.New(
		"e2e-async-error-stage",
		"joins and surfaces async failures",
		passthru.Must[int](),
		failingAsync,
		e2eIdentityProcessor[int](t, "e2e-async-error-identity"),
	)
	require.NoError(t, err)

	p, err := pipeline.New(
		"e2e-async-error-pipeline",
		"does not report done after an async failure",
		false,
		stg,
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{1, 2})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, int64(1), p.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), p.GetStatus().Value())
	assert.Equal(t, int64(1), stg.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), stg.GetStatus().Value())
	assert.Equal(t, int64(1), failingAsync.GetCounterFailed().Value())
	assert.Equal(t, status.Failed.String(), failingAsync.GetStatus().Value())
}

func TestE2E_pauseAndResumeBlocksThenCompletes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	witness := e2eIdentityProcessor[int](t, "e2e-pause-witness")

	stg, err := stage.New(
		"e2e-pause-stage",
		"waits while the pipeline is paused",
		passthru.Must[int](),
		witness,
	)
	require.NoError(t, err)

	p, err := pipeline.New(
		"e2e-pause-pipeline",
		"resumes a paused run",
		false,
		stg,
	)
	require.NoError(t, err)

	p.SetPause(true)
	defer p.SetPause(false)

	type runResult struct {
		tasks []task.Task[int, int]
		err   error
	}

	done := make(chan runResult, 1)

	go func() {
		out, runErr := p.Run(ctx, []int{5, 8})
		done <- runResult{tasks: out, err: runErr}
	}()

	require.Eventually(t, func() bool {
		return witness.GetStatus().Value() == status.Paused.String()
	}, 5*time.Second, 10*time.Millisecond)

	select {
	case result := <-done:
		t.Fatalf("pipeline completed while paused: tasks=%v err=%v", result.tasks, result.err)
	case <-time.After(100 * time.Millisecond):
	}

	p.SetPause(false)

	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Len(t, result.tasks, 1)
		assert.Equal(t, []int{5, 8}, result.tasks[0].ConvertedData)
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not complete after resume")
	}
}

func TestE2E_contextCancellationPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})

	blocking, err := processor.New(
		"e2e-cancellation-processor",
		"waits for context cancellation",
		func(ctx context.Context, processingData []int) ([]int, error) {
			close(started)
			<-ctx.Done()

			return nil, ctx.Err()
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		"e2e-cancellation-stage",
		"surfaces processor cancellation",
		passthru.Must[int](),
		blocking,
	)
	require.NoError(t, err)

	p, err := pipeline.New(
		"e2e-cancellation-pipeline",
		"propagates a canceled context",
		false,
		stg,
	)
	require.NoError(t, err)

	done := make(chan error, 1)

	go func() {
		_, runErr := p.Run(ctx, []int{1})
		done <- runErr
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("processor did not start")
	}

	cancel()

	select {
	case runErr := <-done:
		require.Error(t, runErr)
		assert.ErrorIs(t, runErr, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not return after context cancellation")
	}
}
