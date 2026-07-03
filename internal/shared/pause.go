package shared

import (
	"context"
	"sync"
)

//////
// Consts, vars and types.
//////

// pauseCtxKey is the context key under which a `PauseController` travels.
type pauseCtxKey struct{}

// PauseController coordinates pausing and resuming the processors running
// under a single pipeline. Each pipeline owns its own controller, so pausing
// one pipeline does not affect any other.
type PauseController struct {
	mu       sync.Mutex
	paused   bool
	resumeCh chan struct{}
}

//////
// Methods.
//////

// Pause pauses. Safe for concurrent use. Idempotent.
func (pc *PauseController) Pause() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.paused {
		pc.paused = true
		pc.resumeCh = make(chan struct{})
	}
}

// Resume resumes, waking up all `Wait`ers. Safe for concurrent use.
// Idempotent.
func (pc *PauseController) Resume() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.paused {
		pc.paused = false

		close(pc.resumeCh)
	}
}

// Paused returns whether the controller is currently paused.
func (pc *PauseController) Paused() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	return pc.paused
}

// Wait blocks while paused. It returns nil as soon as the controller is
// resumed, or the context error if the context is done first.
func (pc *PauseController) Wait(ctx context.Context) error {
	for {
		pc.mu.Lock()

		if !pc.paused {
			pc.mu.Unlock()

			return nil
		}

		ch := pc.resumeCh

		pc.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

//////
// Factory and context plumbing.
//////

// NewPauseController returns a new, running (not paused) controller.
func NewPauseController() *PauseController {
	return &PauseController{}
}

// ContextWithPause returns a copy of `ctx` carrying `pc`. The pipeline uses
// this to make its controller visible to the processors it runs.
func ContextWithPause(ctx context.Context, pc *PauseController) context.Context {
	return context.WithValue(ctx, pauseCtxKey{}, pc)
}

// PauseFromCtx extracts the `PauseController` from `ctx`, or nil if none —
// e.g., a processor running standalone, outside a pipeline.
func PauseFromCtx(ctx context.Context) *PauseController {
	pc, _ := ctx.Value(pauseCtxKey{}).(*PauseController)

	return pc
}
