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

// Pause changes the controller to paused. It is concurrency-safe and
// idempotent.
func (pc *PauseController) Pause() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.paused {
		pc.paused = true
		pc.resumeCh = make(chan struct{})
	}
}

// Resume changes the controller to running and wakes all Wait callers. It is
// concurrency-safe and idempotent.
func (pc *PauseController) Resume() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.paused {
		pc.paused = false

		close(pc.resumeCh)
	}
}

// Paused reports whether the controller is currently paused.
func (pc *PauseController) Paused() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	return pc.paused
}

// Wait blocks while the controller is paused. It returns nil after a resume or
// the context error if ctx ends first.
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

// NewPauseController returns a controller in the running state.
func NewPauseController() *PauseController {
	return &PauseController{}
}

// ContextWithPause returns a copy of ctx carrying pc.
func ContextWithPause(ctx context.Context, pc *PauseController) context.Context {
	return context.WithValue(ctx, pauseCtxKey{}, pc)
}

// PauseFromCtx returns the PauseController carried by ctx, or nil when absent.
func PauseFromCtx(ctx context.Context) *PauseController {
	pc, _ := ctx.Value(pauseCtxKey{}).(*PauseController)

	return pc
}
