package pipeline

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
	"github.com/thalesfsp/etler/v4/stage"
	"github.com/thalesfsp/etler/v4/task"
	"github.com/thalesfsp/status"
)

func TestPipeline_concurrentRunPauseAndStatusAccess(t *testing.T) {
	const (
		iterations = 25
		workers    = 4
	)

	pipelineValue, err := New(
		"concurrent-run-pause-status-audit",
		"concurrent run pause and status access",
		false,
		newIdentityStage(t, "concurrent-run-pause-status-audit-stage"),
	)
	require.NoError(t, err)

	p := pipelineValue.(*Pipeline[int, int])
	start := make(chan struct{})

	legalLifecycleStatus := func(value string) bool {
		switch status.Status(value) {
		case status.Created, status.Runnning, status.Done, status.Failed:
			return true
		default:
			return false
		}
	}

	var wg sync.WaitGroup

	startWorker := func(work func()) {
		t.Helper()

		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("concurrent pipeline access panicked: %v", recovered)
				}
			}()

			<-start
			work()
		}()
	}

	for worker := range workers {
		startWorker(func() {
			for iteration := range iterations {
				_, runErr := p.Run(
					context.Background(),
					[]int{worker*iterations + iteration},
				)
				if runErr != nil {
					t.Errorf("Run failed during concurrent access: %v", runErr)
				}
			}
		})

		startWorker(func() {
			defer p.SetPause(false)

			for iteration := range iterations {
				p.SetPause((worker+iteration)%2 == 0)
			}
		})

		startWorker(func() {
			for range iterations {
				paused := p.GetPaused()
				if paused != status.Paused && paused != status.Runnning {
					t.Errorf("GetPaused returned illegal value %q", paused)
				}

				lifecycle := p.GetStatus().Value()
				if !legalLifecycleStatus(lifecycle) {
					t.Errorf("GetStatus returned illegal lifecycle value %q", lifecycle)
				}
				if lifecycle == status.Paused.String() {
					t.Errorf("GetStatus returned pause state %q", lifecycle)
				}
			}
		})
	}

	close(start)
	wg.Wait()

	p.SetPause(false)
	assert.Equal(t, status.Runnning, p.GetPaused())
	assert.True(t, legalLifecycleStatus(p.GetStatus().Value()))
	assert.NotEqual(t, status.Paused.String(), p.GetStatus().Value())
}

func TestPipeline_onFinishedCanRunPipelineAgain(t *testing.T) {
	pipelineValue, err := New(
		"on-finished-reentrant-run-audit",
		"on-finished can run the pipeline again",
		false,
		newIdentityStage(t, "on-finished-reentrant-run-audit-stage"),
	)
	require.NoError(t, err)

	p := pipelineValue.(*Pipeline[int, int])
	nestedRun := make(chan error, 1)

	p.SetOnFinished(func(
		ctx context.Context,
		pl IPipeline[int, int],
		_ task.Task[int, int],
		_ []task.Task[int, int],
	) {
		pl.SetOnFinished(nil)

		_, nestedErr := pl.Run(ctx, []int{2})
		nestedRun <- nestedErr
	})

	outerRun := make(chan error, 1)

	go func() {
		_, runErr := p.Run(context.Background(), []int{1})
		outerRun <- runErr
	}()

	select {
	case runErr := <-outerRun:
		require.NoError(t, runErr)
	case <-time.After(2 * time.Second):
		t.Fatal("OnFinished deadlocked while starting a nested Run")
	}

	require.NoError(t, <-nestedRun)
}

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

type runSignalingStage struct {
	stage.IStage[int, int]
	entered chan<- struct{}
}

func (s runSignalingStage) Run(
	ctx context.Context,
	tsk task.Task[int, int],
) (task.Task[int, int], error) {
	s.entered <- struct{}{}

	return s.IStage.Run(ctx, tsk)
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

// Pause state belongs to the controller and must not replace the run's
// lifecycle status. Repeating either transition is a no-op.
func TestPipeline_SetPause_changesOnlyControllerStateAndIsIdempotent(t *testing.T) {
	p, err := New("pause-audit", "pause toggling", false,
		newIdentityStage(t, "stage-pause-audit"),
	)
	require.NoError(t, err)
	require.Equal(t, status.Created.String(), p.GetStatus().Value())

	p.SetPause(true)
	require.Equal(t, status.Paused, p.GetPaused(), "SetPause(true) must pause")
	require.Equal(t, status.Created.String(), p.GetStatus().Value())
	require.NotEqual(t, status.Paused.String(), p.GetStatus().Value())

	p.SetPause(true)
	require.Equal(t, status.Paused, p.GetPaused(), "repeated pause must be a no-op")
	require.Equal(t, status.Created.String(), p.GetStatus().Value())

	p.SetPause(false)
	require.Equal(t, status.Runnning, p.GetPaused(), "SetPause(false) must resume")
	require.Equal(t, status.Created.String(), p.GetStatus().Value())

	p.SetPause(false)
	assert.Equal(t, status.Runnning, p.GetPaused(), "repeated resume must be a no-op")
	assert.Equal(t, status.Created.String(), p.GetStatus().Value())
}

// Pausing and resuming a terminal pipeline changes only its controller state.
func TestPipeline_SetPause_doesNotChangeTerminalStatus(t *testing.T) {
	t.Run("done", func(t *testing.T) {
		p, err := New("pause-done-audit", "done status survives pause", false,
			newIdentityStage(t, "stage-pause-done-audit"),
		)
		require.NoError(t, err)

		_, err = p.Run(context.Background(), []int{1})
		require.NoError(t, err)
		require.Equal(t, status.Done.String(), p.GetStatus().Value())

		p.SetPause(true)
		require.Equal(t, status.Paused, p.GetPaused())
		require.Equal(t, status.Done.String(), p.GetStatus().Value())

		p.SetPause(false)
		assert.Equal(t, status.Runnning, p.GetPaused())
		assert.Equal(t, status.Done.String(), p.GetStatus().Value())
	})

	t.Run("failed", func(t *testing.T) {
		boom := errors.New("pause-failed-audit")

		p, err := New("pause-failed-audit", "failed status survives pause", false,
			newFailingStage(t, "stage-pause-failed-audit", boom),
		)
		require.NoError(t, err)

		_, err = p.Run(context.Background(), []int{1})
		require.ErrorIs(t, err, boom)
		require.Equal(t, status.Failed.String(), p.GetStatus().Value())

		p.SetPause(true)
		require.Equal(t, status.Paused, p.GetPaused())
		require.Equal(t, status.Failed.String(), p.GetStatus().Value())

		p.SetPause(false)
		assert.Equal(t, status.Runnning, p.GetPaused())
		assert.Equal(t, status.Failed.String(), p.GetStatus().Value())
	})
}

// A run that finishes while its controller is paused publishes its terminal
// lifecycle status immediately; completing the run does not resume the
// controller.
func TestPipeline_runPublishesTerminalStatusWhileControllerPaused(t *testing.T) {
	tests := []struct {
		name       string
		runErr     error
		wantStatus status.Status
	}{
		{
			name:       "done",
			wantStatus: status.Done,
		},
		{
			name:       "failed",
			runErr:     errors.New("repeated-pause-failed"),
			wantStatus: status.Failed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})

			proc, err := processor.New(
				"repeated-pause-"+tt.name+"-processor",
				"finishes after the pipeline is paused",
				func(ctx context.Context, processingData []int) ([]int, error) {
					close(entered)
					<-release

					return processingData, tt.runErr
				},
			)
			require.NoError(t, err)

			conv, err := converter.New(
				"repeated-pause-"+tt.name+"-converter",
				"identity converter",
				func(ctx context.Context, in int) (int, error) {
					return in, nil
				},
			)
			require.NoError(t, err)

			stg, err := stage.New(
				"repeated-pause-"+tt.name+"-stage",
				"stage for repeated pause",
				conv,
				proc,
			)
			require.NoError(t, err)

			p, err := New(
				"repeated-pause-"+tt.name+"-pipeline",
				"pipeline for repeated pause",
				false,
				stg,
			)
			require.NoError(t, err)

			runDone := make(chan error, 1)

			go func() {
				_, err := p.Run(context.Background(), []int{1})
				runDone <- err
			}()

			<-entered
			require.Equal(t, status.Runnning.String(), p.GetStatus().Value())

			p.SetPause(true)
			require.Equal(t, status.Paused, p.GetPaused())

			close(release)

			if tt.runErr == nil {
				require.NoError(t, <-runDone)
			} else {
				require.ErrorIs(t, <-runDone, tt.runErr)
			}

			require.Equal(t, status.Paused, p.GetPaused(),
				"a completed run must not unpause its controller")
			assert.Equal(t, tt.wantStatus.String(), p.GetStatus().Value(),
				"the terminal lifecycle status must be visible without resuming")
		})
	}
}

// Starting a new run while paused must publish running immediately. Resuming
// must not resurrect the first run's terminal status over the second run.
func TestPipeline_secondRunReplacesTerminalStatusWhileControllerPaused(t *testing.T) {
	stageEntered := make(chan struct{})
	firstProcessorEntered := make(chan struct{})
	firstRelease := make(chan struct{})
	secondProcessorEntered := make(chan struct{})
	secondRelease := make(chan struct{})
	gates := make(chan struct {
		entered chan<- struct{}
		release <-chan struct{}
	}, 2)
	gates <- struct {
		entered chan<- struct{}
		release <-chan struct{}
	}{entered: firstProcessorEntered, release: firstRelease}
	gates <- struct {
		entered chan<- struct{}
		release <-chan struct{}
	}{entered: secondProcessorEntered, release: secondRelease}

	blocking, err := processor.New(
		"pause-second-run-blocking",
		"coordinates two runs",
		func(ctx context.Context, processingData []int) ([]int, error) {
			gate := <-gates
			gate.entered <- struct{}{}
			<-gate.release

			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := stage.New(
		"stage-pause-second-run",
		"second run pause stage",
		converter.MustDefault(
			func(ctx context.Context, in int) (int, error) {
				return in, nil
			},
		),
		blocking,
	)
	require.NoError(t, err)

	p, err := New(
		"pause-second-run-audit",
		"second run replaces first status",
		false,
		runSignalingStage{IStage: stg, entered: stageEntered},
	)
	require.NoError(t, err)

	firstRunDone := make(chan error, 1)

	go func() {
		_, err := p.Run(context.Background(), []int{1})
		firstRunDone <- err
	}()

	<-stageEntered
	<-firstProcessorEntered

	p.SetPause(true)
	close(firstRelease)
	require.NoError(t, <-firstRunDone)
	require.Equal(t, status.Paused, p.GetPaused())

	secondRunDone := make(chan error, 1)

	go func() {
		_, err := p.Run(context.Background(), []int{2})
		secondRunDone <- err
	}()

	<-stageEntered
	require.Equal(t, status.Paused, p.GetPaused())
	require.Equal(t, status.Runnning.String(), p.GetStatus().Value(),
		"the second run must replace the first run's terminal lifecycle status")

	p.SetPause(false)
	<-secondProcessorEntered
	require.Equal(t, status.Runnning.String(), p.GetStatus().Value(),
		"resume must not resurrect the first run's terminal status")

	close(secondRelease)
	require.NoError(t, <-secondRunDone)
	assert.Equal(t, status.Done.String(), p.GetStatus().Value())
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
	stageEntered := make(chan struct{})

	p, err := New(
		"pause-e2e-audit",
		"pause e2e",
		false,
		runSignalingStage{IStage: stg, entered: stageEntered},
	)
	require.NoError(t, err)

	defer p.SetPause(false)

	p.SetPause(true)

	done := make(chan error, 1)

	go func() {
		_, err := p.Run(ctx, []int{1})
		done <- err
	}()

	<-stageEntered

	// While paused, the processor must NOT run.
	select {
	case <-ran:
		t.Fatal("processor ran while the pipeline was paused")
	case <-time.After(1500 * time.Millisecond):
		// Still paused, as expected.
	}

	// The run lifecycle is independent from the paused controller.
	assert.Equal(t, status.Runnning.String(), p.GetStatus().Value(),
		"Run must publish running even while the controller is paused")

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
	assert.Equal(t, status.Failed.String(), p.GetStatus().Value(),
		"a nil-input failure must update pipeline status")

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

// Regression: every run replaces the duration metric, including failed runs.
func TestPipeline_failedRun_replacesPreviousDuration(t *testing.T) {
	boom := errors.New("boom-pipeline-duration-audit")

	runs := 0

	slowThenFail, err := processor.New(
		"slow-then-fail-pipeline-duration-audit",
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

	stg, err := stage.New(
		"stage-pipeline-duration-audit",
		"duration stage",
		converter.MustDefault(
			func(ctx context.Context, in int) (int, error) {
				return in, nil
			},
		),
		slowThenFail,
	)
	require.NoError(t, err)

	p, err := New(
		"pipeline-duration-audit",
		"failed run replaces duration",
		false,
		stg,
	)
	require.NoError(t, err)

	_, err = p.Run(context.Background(), []int{1})
	require.NoError(t, err)

	successfulDuration := p.GetDuration().Value()
	require.Positive(t, successfulDuration)

	_, err = p.Run(context.Background(), []int{1})
	require.ErrorIs(t, err, boom)

	assert.Less(t, p.GetDuration().Value(), successfulDuration,
		"the failed pipeline run must replace the previous successful duration")
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
