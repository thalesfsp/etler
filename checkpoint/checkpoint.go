package checkpoint

import (
	"context"
	"time"
)

//////
// Consts, vars and types.
//////

// Status is a chunk outcome.
type Status string

const (
	// StatusDone means the chunk ran, committed, and is skipped on resume.
	StatusDone Status = "done"

	// StatusFailed means the chunk's last attempt failed; resume retries it.
	StatusFailed Status = "failed"

	// StatusSkipped means a run found the chunk already done. It is reported
	// by pipeline.RunChunks and never stored.
	StatusSkipped Status = "skipped"
)

// Record is the stored outcome of one chunk within one run.
type Record struct {
	// Key identifies the chunk within its run.
	Key string `json:"key"`

	// Status is StatusDone or StatusFailed.
	Status Status `json:"status"`

	// Error is the last failure's message; empty when Status is StatusDone.
	Error string `json:"error,omitempty"`

	// Attempts counts how many times the chunk has run, including the
	// attempt this record describes.
	Attempts int `json:"attempts"`

	// UpdatedAt is when the record was written.
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store persists chunk records per run. Implementations must be safe for
// concurrent use.
type Store interface {
	// Load returns every record stored for runID, keyed by Record.Key. An
	// unknown run returns an empty map and no error.
	Load(ctx context.Context, runID string) (map[string]Record, error)

	// Save inserts or replaces the record for (runID, record.Key). A run
	// treats a chunk as checkpointed only after Save returns nil.
	Save(ctx context.Context, runID string, record Record) error
}
