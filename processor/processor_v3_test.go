package processor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	assert.True(t, p.GetAsync(), "WithAsync(true) must set the async flag")

	p.SetAsync(false)
	assert.False(t, p.GetAsync())
}
