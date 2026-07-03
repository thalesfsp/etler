package loader

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/status"
)

// Edge case: a loader without a load function is a configuration error — it
// would panic at Run time otherwise.
func TestLoader_new_nilFunc_returnsError(t *testing.T) {
	l, err := New[string, []int]("nil-func-audit", "no function", nil)
	assert.Nil(t, l)
	assert.Error(t, err, "a loader without a load function must not be created")
}

// Bad path: a failing load must propagate the cause and update metrics.
func TestLoader_error_updatesMetricsAndPropagatesCause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	boom := errors.New("boom-loader-audit")

	failing, err := New(
		"failing-loader-audit",
		"always fails",
		func(ctx context.Context, in string) ([]int, error) {
			return nil, boom
		},
	)
	require.NoError(t, err)

	out, err := failing.Run(ctx, "anything")
	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-loader-audit")

	assert.Equal(t, status.Failed.String(), failing.GetStatus().Value())
	assert.Equal(t, int64(1), failing.GetCounterFailed().Value())
	assert.Equal(t, int64(0), failing.GetCounterDone().Value())
}
