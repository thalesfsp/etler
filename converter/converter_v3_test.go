package converter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Metadata getters.
func TestConverter_metadata(t *testing.T) {
	c, err := New(
		"meta-conv",
		"metadata test",
		func(ctx context.Context, in int) (int, error) { return in, nil },
	)
	require.NoError(t, err)

	assert.Equal(t, "meta-conv", c.GetName())
	assert.Equal(t, "metadata test", c.GetDescription())
	assert.Equal(t, Type, c.GetType())
	assert.NotZero(t, c.GetCreatedAt())

	m := c.GetMetrics()
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "counterCreated")
}

// Default happy path.
func TestConverter_default_happyPath(t *testing.T) {
	c, err := Default(
		func(ctx context.Context, in int) (int, error) { return in + 1, nil },
	)
	require.NoError(t, err)

	out, err := c.Run(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 2, out)
}
