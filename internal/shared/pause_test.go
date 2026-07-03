package shared

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Happy path: pause blocks waiters, resume wakes them all.
func TestPauseController_pauseAndResume(t *testing.T) {
	pc := NewPauseController()

	assert.False(t, pc.Paused())

	// Not paused: Wait returns immediately.
	require.NoError(t, pc.Wait(context.Background()))

	pc.Pause()
	assert.True(t, pc.Paused())

	const waiters = 5

	var wg sync.WaitGroup

	released := make(chan struct{}, waiters)

	for i := 0; i < waiters; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := pc.Wait(context.Background()); err == nil {
				released <- struct{}{}
			}
		}()
	}

	// While paused, no waiter may be released.
	select {
	case <-released:
		t.Fatal("a waiter was released while paused")
	case <-time.After(300 * time.Millisecond):
	}

	pc.Resume()
	wg.Wait()

	assert.False(t, pc.Paused())
	assert.Len(t, released, waiters, "all waiters must be released on resume")
}

// Bad path: a done context unblocks Wait with the context error.
func TestPauseController_waitHonorsContext(t *testing.T) {
	pc := NewPauseController()
	pc.Pause()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- pc.Wait(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not unblock on context cancellation")
	}
}

// Edge case: Pause and Resume are idempotent and safe under concurrency.
func TestPauseController_idempotentAndConcurrent(t *testing.T) {
	pc := NewPauseController()

	var wg sync.WaitGroup

	for i := 0; i < 500; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if i%2 == 0 {
				pc.Pause()
			} else {
				pc.Resume()
			}

			_ = pc.Paused()
		}(i)
	}

	wg.Wait()

	// Leave it resumed; a waiter must not hang.
	pc.Resume()
	require.NoError(t, pc.Wait(context.Background()))
}

// Context plumbing: controller round-trips through the context; absent
// controller yields nil.
func TestPauseController_contextPlumbing(t *testing.T) {
	assert.Nil(t, PauseFromCtx(context.Background()))

	pc := NewPauseController()
	ctx := ContextWithPause(context.Background(), pc)

	assert.Same(t, pc, PauseFromCtx(ctx))
}
