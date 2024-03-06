package metrics

import (
	"expvar"
	"fmt"

	"github.com/thalesfsp/etler/v2/internal/shared"
	"github.com/thalesfsp/status"
)

const (
	// DefaultMetricCounterLabel is the default label for a counter metric.
	DefaultMetricCounterLabel = "counter"

	// Name of the package/application.
	Name = "etler"
)

// NewString creates and initializes a new stribg metric.
func NewString(name string) *expvar.String {
	counter := expvar.NewString(name)

	counter.Set(status.Created.String())

	return counter
}

// NewStringWithPattern creates and initializes a new expvar.String with a
// consistent naming pattern where `t` is the type of the entity, `subject` is
// the subject of the metric, `status` is the status of the metric - usually
// from the `status` package.
func NewStringWithPattern(t, subject string, status status.Status) *expvar.String {
	return NewString(
		fmt.Sprintf(
			"%s.%s.%s.%s.%s",
			Name,
			t,
			subject,
			status,
			shared.GenerateUUID(),
		),
	)
}

// NewInt creates and initializes a new int metric.
func NewInt(name string) *expvar.Int {
	counter := expvar.NewInt(name)

	counter.Set(0)

	return counter
}

// NewIntWithPattern creates and initializes a new expvar.Int with a consistent
// naming pattern where `t` is the type of the entity, `subject` is the subject
// of the metric, `status` is the status of the metric - usually from the
// `status` package.
func NewIntWithPattern(t, subject string, status status.Status) *expvar.Int {
	return NewInt(
		fmt.Sprintf(
			"%s.%s.%s.%s.%s.%s",
			Name,
			t,
			subject,
			status,
			DefaultMetricCounterLabel,
			shared.GenerateUUID(),
		),
	)
}
