package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v3/converter"
	"github.com/thalesfsp/etler/v3/processor"
	"github.com/thalesfsp/etler/v3/stage"
	"github.com/thalesfsp/etler/v3/task"
	"github.com/thalesfsp/status"
)

// newIdentityStage returns a stage with an identity processor and an identity
// converter, suitable for int-based pipeline tests.
func newIdentityStage(t *testing.T, name string) stage.IStage[int, int] {
	t.Helper()

	identity, err := processor.New(
		name+"-identity",
		"returns the input unchanged",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		name,
		"identity stage",
		converter.MustDefault(
			func(ctx context.Context, in int) (int, error) {
				return in, nil
			},
		),
		identity,
	)
	require.NoError(t, err)

	return stg
}

// newFailingStage returns a stage whose single processor always fails with
// the given error.
func newFailingStage(t *testing.T, name string, failWith error) stage.IStage[int, int] {
	t.Helper()

	failing, err := processor.New(
		name+"-failing",
		"always fails",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return nil, failWith
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		name,
		"failing stage",
		converter.MustDefault(
			func(ctx context.Context, in int) (int, error) {
				return in, nil
			},
		),
		failing,
	)
	require.NoError(t, err)

	return stg
}

// Bad path: a stage failure in CONCURRENT mode must surface the underlying
// cause to the caller, not a generic error with the cause dropped.
func TestPipeline_concurrent_stageErrorCausePropagated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	boom := errors.New("boom-concurrent-audit")

	p, err := New("concurrent-err-audit", "propagates stage errors", true,
		newFailingStage(t, "stage-fail-concurrent", boom),
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{1, 2, 3})

	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-concurrent-audit",
		"the real cause of the stage failure must be in the returned error")
	assert.Equal(t, status.Failed.String(), p.GetStatus().Value())
}

// Bad path: a stage failure in SEQUENTIAL mode must surface the underlying
// cause and update failure metrics.
func TestPipeline_sequential_stageErrorCausePropagated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	boom := errors.New("boom-sequential-audit")

	p, err := New("sequential-err-audit", "propagates stage errors", false,
		newFailingStage(t, "stage-fail-sequential", boom),
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, []int{1, 2, 3})

	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-sequential-audit")
	assert.Equal(t, status.Failed.String(), p.GetStatus().Value())
	assert.GreaterOrEqual(t, p.GetCounterFailed().Value(), int64(1))
}

// Bug: SetPause(true) must actually pause the pipeline. The original
// implementation fell through and immediately unpaused itself.
func TestPipeline_SetPause_togglesPausedState(t *testing.T) {
	p, err := New("pause-audit", "pause toggling", false,
		newIdentityStage(t, "stage-pause-audit"),
	)
	require.NoError(t, err)

	// Always leave the global pause flag clean for other tests.
	defer p.SetPause(false)

	p.SetPause(true)
	assert.Equal(t, status.Paused, p.GetPaused(), "SetPause(true) must pause")
	assert.Equal(t, status.Paused.String(), p.GetStatus().Value())

	p.SetPause(false)
	assert.Equal(t, status.Runnning, p.GetPaused(), "SetPause(false) must resume")
	assert.Equal(t, status.Runnning.String(), p.GetStatus().Value())
}

// E2E: pausing the pipeline must actually hold processors back until resumed.
func TestPipeline_pause_holdsProcessing_untilResumed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ran := make(chan struct{})

	witness, err := processor.New(
		"pause-witness",
		"signals when it runs",
		func(ctx context.Context, processingData []int) ([]int, error) {
			close(ran)

			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		"stage-pause-e2e",
		"pause e2e stage",
		converter.MustDefault(
			func(ctx context.Context, in int) (int, error) { return in, nil },
		),
		witness,
	)
	require.NoError(t, err)

	p, err := New("pause-e2e-audit", "pause e2e", false, stg)
	require.NoError(t, err)

	defer p.SetPause(false)

	p.SetPause(true)

	done := make(chan error, 1)

	go func() {
		_, err := p.Run(ctx, []int{1})
		done <- err
	}()

	// While paused, the processor must NOT run.
	select {
	case <-ran:
		t.Fatal("processor ran while the pipeline was paused")
	case <-time.After(1500 * time.Millisecond):
		// Still paused, as expected.
	}

	p.SetPause(false)

	// After resuming, the pipeline must finish.
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("processor did not resume after unpausing")
	}

	require.NoError(t, <-done)
}

// Bug: zero values in the data must survive the conversion step. The
// conversion previously dropped zero values silently (data loss).
func TestPipeline_zeroValues_notDropped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := New("zero-values-audit", "zero values must survive", false,
		newIdentityStage(t, "stage-zero-audit"),
	)
	require.NoError(t, err)

	in := []int{0, 1, 0, 2}

	out, err := p.Run(ctx, in)
	require.NoError(t, err)
	require.Len(t, out, 1)

	assert.Equal(t, in, out[0].ConvertedData,
		"zero values must not be silently dropped by the pipeline")
	assert.Equal(t, in, out[0].ProcessingData)
}

// Edge cases: a nil input is rejected by task validation; an empty (non-nil)
// input runs to completion with empty output.
func TestPipeline_nilAndEmptyInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := New("empty-input-audit", "empty input", false,
		newIdentityStage(t, "stage-empty-input-audit"),
	)
	require.NoError(t, err)

	out, err := p.Run(ctx, nil)
	assert.Nil(t, out)
	assert.Error(t, err, "nil input must be rejected")

	out, err = p.Run(ctx, []int{})
	require.NoError(t, err, "empty (non-nil) input is a valid, empty run")
	require.Len(t, out, 1)
	assert.Empty(t, out[0].ConvertedData)
}

// Edge case: a pipeline without stages is a configuration error, consistent
// with stage.New which requires at least one processor.
func TestPipeline_new_withoutStages_returnsError(t *testing.T) {
	p, err := New[int, int]("no-stages-audit", "no stages", false)
	assert.Nil(t, p)
	assert.Error(t, err, "a pipeline without stages must not be created")
}

// Bug: re-running the same pipeline must not accumulate progress beyond 100%.
func TestPipeline_reuse_progressDoesNotExceed100Percent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := New("reuse-audit", "progress reset on re-run", false,
		newIdentityStage(t, "stage-reuse-audit"),
	)
	require.NoError(t, err)

	_, err = p.Run(ctx, []int{1})
	require.NoError(t, err)

	_, err = p.Run(ctx, []int{2})
	require.NoError(t, err)

	assert.Equal(t, int64(1), p.GetProgress().Value(),
		"progress must be relative to the current run")
	assert.Equal(t, "100%", p.GetProgressPercent().Value())
}

// Concurrency: two pipelines running at the same time must not interfere with
// each other's results. Run with -race.
func TestPipeline_twoPipelines_concurrentRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	p1, err := New("concurrent-a-audit", "pipeline a", false,
		newIdentityStage(t, "stage-concurrent-a"),
	)
	require.NoError(t, err)

	p2, err := New("concurrent-b-audit", "pipeline b", true,
		newIdentityStage(t, "stage-concurrent-b"),
	)
	require.NoError(t, err)

	errCh := make(chan error, 2)

	go func() {
		_, err := p1.Run(ctx, []int{1, 2, 3})
		errCh <- err
	}()

	go func() {
		_, err := p2.Run(ctx, []int{4, 5, 6})
		errCh <- err
	}()

	require.NoError(t, <-errCh)
	require.NoError(t, <-errCh)

	// Neither pipeline may end up paused by a normal run.
	assert.Equal(t, status.Runnning, p1.GetPaused())
	assert.Equal(t, status.Runnning, p2.GetPaused())
}

// v3: pausing one pipeline must NOT pause any other — pause is per pipeline,
// no longer a process-wide global.
func TestPipeline_pauseIsolation_betweenPipelines(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	paused, err := New("pause-isolated-a", "stays paused", false,
		newIdentityStage(t, "stage-pause-isolated-a"),
	)
	require.NoError(t, err)

	free, err := New("pause-isolated-b", "keeps running", false,
		newIdentityStage(t, "stage-pause-isolated-b"),
	)
	require.NoError(t, err)

	paused.SetPause(true)
	defer paused.SetPause(false)

	require.Equal(t, status.Paused, paused.GetPaused())
	require.Equal(t, status.Runnning, free.GetPaused(),
		"pausing pipeline A must not pause pipeline B")

	// The unpaused pipeline must run to completion while the other one is
	// paused.
	out, err := free.Run(ctx, []int{1, 2, 3})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, []int{1, 2, 3}, out[0].ConvertedData)
}

// v3: OnFinished receives the per-stage results — one task per stage, in
// stage order, final task last.
func TestPipeline_onFinished_receivesPerStageTasks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	double, err := processor.New(
		"double-onfinished-pl-audit",
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

	identityConv := converter.MustDefault(
		func(ctx context.Context, in int) (int, error) { return in, nil },
	)

	stg1, err := stage.New("stage-onfinished-pl-1", "double once", identityConv, double)
	require.NoError(t, err)

	stg2, err := stage.New("stage-onfinished-pl-2", "double twice", identityConv, double)
	require.NoError(t, err)

	p, err := New("onfinished-pl-audit", "per-stage results", false, stg1, stg2)
	require.NoError(t, err)

	var gotOriginal task.Task[int, int]

	var gotTasks []task.Task[int, int]

	p.SetOnFinished(func(ctx context.Context, pl IPipeline[int, int], original task.Task[int, int], tasksOut []task.Task[int, int]) {
		gotOriginal = original
		gotTasks = tasksOut
	})

	out, err := p.Run(ctx, []int{1})
	require.NoError(t, err)
	require.Len(t, out, 2)

	assert.Equal(t, []int{1}, gotOriginal.ProcessingData)
	require.Len(t, gotTasks, 2, "OnFinished must receive one task per stage")
	assert.Equal(t, []int{2}, gotTasks[0].ProcessingData)
	assert.Equal(t, []int{4}, gotTasks[1].ProcessingData)
	assert.Equal(t, []int{4}, gotTasks[len(gotTasks)-1].ConvertedData)
}
