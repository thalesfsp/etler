package shared

import (
	"expvar"
	"time"

	"github.com/thalesfsp/sypl/v2"
)

// IMeta exposes an entity's identifying metadata and logger.
type IMeta interface {
	// GetName returns the entity's name.
	GetName() string

	// GetLogger returns the entity's logger.
	GetLogger() sypl.ISypl

	// GetDescription returns the entity's description.
	GetDescription() string

	// GetType returns the entity type.
	GetType() string
}

// IMetrics exposes an entity's lifecycle metrics.
type IMetrics interface {
	// GetCounterCreated returns the created counter.
	GetCounterCreated() *expvar.Int

	// GetCounterRunning returns the running counter.
	GetCounterRunning() *expvar.Int

	// GetCounterFailed returns the failed counter.
	GetCounterFailed() *expvar.Int

	// GetCounterDone returns the done counter.
	GetCounterDone() *expvar.Int

	// GetStatus returns the lifecycle status.
	GetStatus() *expvar.String

	// GetCreatedAt returns when the entity was constructed.
	GetCreatedAt() time.Time

	// GetDuration returns the most recent operation duration.
	GetDuration() *expvar.Int

	// GetMetrics returns selected metrics as strings.
	GetMetrics() map[string]string
}
