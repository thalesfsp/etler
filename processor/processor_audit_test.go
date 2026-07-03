package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v2/internal/shared"
	"github.com/thalesfsp/status"
)

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

	shared.SetPaused(1)
	defer shared.SetPaused(0)

	done := make(chan error, 1)

	go func() {
		_, err := witness.Run(ctx, []int{1})
		done <- err
	}()

	select {
	case <-ran:
		t.Fatal("processor ran while paused")
	case <-time.After(1300 * time.Millisecond):
		// Still paused, as expected.
	}

	assert.Equal(t, status.Paused.String(), witness.GetStatus().Value())

	shared.SetPaused(0)

	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("processor did not resume after unpausing")
	}

	require.NoError(t, <-done)
	assert.Equal(t, status.Done.String(), witness.GetStatus().Value())
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

	shared.SetPaused(1)
	defer shared.SetPaused(0)

	done := make(chan error, 1)

	go func() {
		_, err := blocked.Run(ctx, []int{1})
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
