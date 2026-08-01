package processor

import (
	"context"
	"expvar"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/metrics"
)

// Metadata getters + async flag round trip via option and setters.
func TestProcessor_metadataAndAsyncFlag(t *testing.T) {
	p, err := New(
		"meta-proc",
		"metadata test",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
		WithAsync[int](true),
	)
	require.NoError(t, err)

	assert.Equal(t, "meta-proc", p.GetName())
	assert.Equal(t, "metadata test", p.GetDescription())
	assert.Equal(t, Type, p.GetType())
	assert.NotZero(t, p.GetCreatedAt())
	assert.NotNil(t, p.GetCounterInterrupted())

	m := p.GetMetrics()
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "counterFailed")
	assert.Contains(t, m, "counterInterrupted",
		"GetMetrics must expose the interrupted counter")

	assert.True(t, p.GetAsync(), "WithAsync(true) must set the async flag")

	// The fields are intentionally unexported; accessors are the supported
	// mutation path and therefore cannot bypass their mutex.
	p.SetAsync(false)
	assert.False(t, p.GetAsync())

	p.SetOnFinished(func(ctx context.Context, p IProcessor[int], originalIn, processedOut []int) {})
	assert.NotNil(t, p.GetOnFinished())

	p.SetOnFinished(nil)
	assert.Nil(t, p.GetOnFinished())
}

// A rejected name must not leak any permanently registered expvars.
func TestProcessor_New_emptyNameDoesNotPublishMetrics(t *testing.T) {
	t.Setenv(metrics.PublishEnvVar, "true")

	emptyNameMetrics := []string{
		"etler.processor..created.counter",
		"etler.processor..done.counter",
		"etler.processor..failed.counter",
		"etler.processor..running.counter",
		"etler.processor..interrupted.counter",
		"etler.processor..duration.counter",
		"etler.processor..status",
	}

	for _, name := range emptyNameMetrics {
		require.Nil(t, expvar.Get(name), "test precondition: %q is not published", name)
	}

	_, err := New(
		"",
		"invalid empty name",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.Error(t, err)

	for _, name := range emptyNameMetrics {
		assert.Nil(t, expvar.Get(name), "invalid processor published %q", name)
	}
}
