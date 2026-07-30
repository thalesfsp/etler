package processor

import (
	"context"
	"errors"
	"expvar"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/status"
)

type replacementProcessor struct {
	IProcessor[int]
}

type processorWithNilStatus struct {
	IProcessor[int]
}

func (p *processorWithNilStatus) GetStatus() *expvar.String {
	return nil
}

// Bug: OnFinished must receive the processor's OUTPUT, not the input echoed
// back.
func TestProcessor_onFinished_receivesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var gotIn, gotOut []int

	double, err := New(
		"double-onfinished-audit",
		"doubles the input",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))

			for i, v := range processingData {
				out[i] = v * 2
			}

			return out, nil
		},
		WithOnFinished(func(ctx context.Context, p IProcessor[int], originalIn []int, processedOut []int) {
			gotIn = originalIn
			gotOut = processedOut
		}),
	)
	require.NoError(t, err)

	out, err := double.Run(ctx, []int{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, []int{2, 4, 6}, out)

	assert.Equal(t, []int{1, 2, 3}, gotIn, "OnFinished must receive the original input")
	assert.Equal(t, []int{2, 4, 6}, gotOut, "OnFinished must receive the processed output")
}

// Edge case: a processor without a transform function is a configuration
// error — it would panic at Run time otherwise.
func TestProcessor_new_nilFunc_returnsError(t *testing.T) {
	p, err := New[int]("nil-func-audit", "no function", nil)
	assert.Nil(t, p)
	assert.Error(t, err, "a processor without a transform function must not be created")
}

// A return-valued option may replace or wrap the processor.
func TestProcessor_new_usesOptionReturnValue(t *testing.T) {
	var replacement *replacementProcessor

	got, err := New(
		"replacement-option-processor",
		"uses the option return value",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
		func(p IProcessor[int]) IProcessor[int] {
			replacement = &replacementProcessor{IProcessor: p}

			return replacement
		},
	)
	require.NoError(t, err)
	assert.Same(t, replacement, got)
}

// Returning nil from an option is an invalid constructor result.
func TestProcessor_new_rejectsNilOptionReturn(t *testing.T) {
	got, err := New(
		"nil-option-processor",
		"rejects a nil option return",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
		func(IProcessor[int]) IProcessor[int] {
			return nil
		},
	)

	assert.Nil(t, got)
	assert.Error(t, err)
}

// A nil option is invalid input and must be rejected before invocation.
func TestProcessor_new_rejectsNilOptionWithoutPanic(t *testing.T) {
	var opt Func[int]

	var (
		got IProcessor[int]
		err error
	)

	assert.NotPanics(t, func() {
		got, err = New(
			"nil-option-function-processor",
			"rejects a nil option function",
			func(ctx context.Context, processingData []int) ([]int, error) {
				return processingData, nil
			},
			opt,
		)
	})

	assert.Nil(t, got)
	assert.Error(t, err)
}

// An option replacement must retain non-nil metric getters.
func TestProcessor_new_rejectsNilMetricGetterWithoutPanic(t *testing.T) {
	var (
		got IProcessor[int]
		err error
	)

	assert.NotPanics(t, func() {
		got, err = New(
			"nil-metric-getter-processor",
			"rejects a nil metric getter",
			func(ctx context.Context, processingData []int) ([]int, error) {
				return processingData, nil
			},
			func(p IProcessor[int]) IProcessor[int] {
				return &processorWithNilStatus{IProcessor: p}
			},
		)
	})

	assert.Nil(t, got)
	assert.Error(t, err)
}

// Bad path: a failing transform must propagate the cause and update metrics.
func TestProcessor_error_updatesMetricsAndPropagatesCause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	boom := errors.New("boom-processor-audit")

	failing, err := New(
		"failing-audit",
		"always fails",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return nil, boom
		},
	)
	require.NoError(t, err)

	out, err := failing.Run(ctx, []int{1})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-processor-audit")

	assert.Equal(t, status.Failed.String(), failing.GetStatus().Value())
	assert.Equal(t, int64(1), failing.GetCounterFailed().Value())
	assert.Equal(t, int64(0), failing.GetCounterDone().Value())
}

// E2E: while the global pause flag is set the processor must hold, then
// resume when unpaused.
func TestProcessor_pause_holdsAndResumes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ran := make(chan struct{})

	witness, err := New(
		"pause-witness-audit",
		"signals when it runs",
		func(ctx context.Context, processingData []int) ([]int, error) {
			close(ran)

			return processingData, nil
		},
	)
	require.NoError(t, err)

	// The pause controller reaches the processor via the context — exactly
	// how a pipeline injects it.
	pc := shared.NewPauseController()
	pc.Pause()

	pausedCtx := shared.ContextWithPause(ctx, pc)

	done := make(chan error, 1)

	go func() {
		_, err := witness.Run(pausedCtx, []int{1})
		done <- err
	}()

	select {
	case <-ran:
		t.Fatal("processor ran while paused")
	case <-time.After(500 * time.Millisecond):
		// Still paused, as expected.
	}

	assert.Equal(t, status.Paused.String(), witness.GetStatus().Value())

	pc.Resume()

	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("processor did not resume after unpausing")
	}

	require.NoError(t, <-done)
	assert.Equal(t, status.Done.String(), witness.GetStatus().Value())
}

// Edge case: without a pause controller in the context (standalone use), the
// processor never pauses.
func TestProcessor_noPauseController_runsImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	identity, err := New(
		"no-pause-audit",
		"identity",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	out, err := identity.Run(ctx, []int{1})
	require.NoError(t, err)
	assert.Equal(t, []int{1}, out)
}

// Bad path: cancelling the context while paused must unblock Run with an
// error instead of hanging.
func TestProcessor_pause_ctxCancel_unblocks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	blocked, err := New(
		"pause-cancel-audit",
		"never actually runs",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	pc := shared.NewPauseController()
	pc.Pause()

	pausedCtx := shared.ContextWithPause(ctx, pc)

	done := make(chan error, 1)

	go func() {
		_, err := blocked.Run(pausedCtx, []int{1})
		done <- err
	}()

	// Give Run a moment to enter the pause loop, then cancel.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.Error(t, err, "a cancelled context while paused must surface an error")
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not unblock after context cancellation while paused")
	}
}
