package metrics

import (
	"expvar"
	"fmt"
	"os"
	"sync"

	"github.com/thalesfsp/status"
)

const (
	// DefaultMetricCounterLabel is the default label for a counter metric.
	DefaultMetricCounterLabel = "counter"

	// Name is the prefix used for etler metric names.
	Name = "etler"

	// PublishEnvVar names the environment variable that, when set to "true",
	// publishes metrics to the global expvar registry under their pattern name
	// (visible on /debug/vars).
	//
	// Publishing is opt-in because the expvar registry can never be
	// unregistered from: applications that create many short-lived
	// pipelines would otherwise grow it without bound. When published,
	// entities sharing the same type and name SHARE the same metric.
	PublishEnvVar = "ETLER_METRICS_PUBLISH"
)

// registryMu guards the check-then-publish sequence against concurrent
// constructors — expvar.Publish panics on duplicate names.
var registryMu sync.Mutex

// shouldPublish returns whether metrics must be published to the global
// expvar registry. Read at call time so tests can toggle it.
func shouldPublish() bool {
	return os.Getenv(PublishEnvVar) == "true"
}

// NewString returns a string metric initialized to Created. Unless publishing
// is enabled through PublishEnvVar, it is not registered globally. A reused
// published metric is returned without resetting its value.
func NewString(name string) *expvar.String {
	if !shouldPublish() {
		v := new(expvar.String)

		v.Set(status.Created.String())

		return v
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	// Reuse: do NOT clobber the shared metric's current value.
	if v, ok := expvar.Get(name).(*expvar.String); ok {
		return v
	}

	v := expvar.NewString(name)

	v.Set(status.Created.String())

	return v
}

// NewStringWithPattern returns a string metric named
// "etler.<t>.<subject>.<status>".
func NewStringWithPattern(t, subject string, status status.Status) *expvar.String {
	return NewString(
		fmt.Sprintf(
			"%s.%s.%s.%s",
			Name,
			t,
			subject,
			status,
		),
	)
}

// NewInt returns a zero-valued integer metric. Unless publishing is enabled
// through PublishEnvVar, it is not registered globally. A reused published
// metric is returned without resetting its value.
func NewInt(name string) *expvar.Int {
	if !shouldPublish() {
		return new(expvar.Int)
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	// Reuse: do NOT clobber the shared metric's current value.
	if v, ok := expvar.Get(name).(*expvar.Int); ok {
		return v
	}

	return expvar.NewInt(name)
}

// NewIntWithPattern returns an integer metric named
// "etler.<t>.<subject>.<status>.counter".
func NewIntWithPattern(t, subject string, status status.Status) *expvar.Int {
	return NewInt(
		fmt.Sprintf(
			"%s.%s.%s.%s.%s",
			Name,
			t,
			subject,
			status,
			DefaultMetricCounterLabel,
		),
	)
}
