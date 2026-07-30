package csv

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type v3Row struct {
	Name string `csv:"name"`
	Age  int    `csv:"age"`
}

// Must returns a working converter (happy path).
func TestMust_happyPath(t *testing.T) {
	var c *CSV[[]v3Row]

	require.NotPanics(t, func() {
		c = Must[v3Row]()
	})

	out, err := c.Run(context.Background(), []v3Row{{Name: "alice", Age: 30}, {Name: "bob", Age: 0}})
	require.NoError(t, err)

	assert.Contains(t, out, "name,age")
	assert.Contains(t, out, "alice,30")
	assert.Contains(t, out, "bob,0")
}

// Edge case: an empty slice yields just the header row, no error.
func TestCSV_run_emptySlice(t *testing.T) {
	c, err := New[v3Row]()
	require.NoError(t, err)

	out, err := c.Run(context.Background(), []v3Row{})
	require.NoError(t, err)
	assert.Contains(t, out, "name,age")
}

// A context that is already done prevents conversion and does not count the
// run as completed.
func TestCSV_run_contextAlreadyDone(t *testing.T) {
	tests := []struct {
		name       string
		newContext func() (context.Context, context.CancelFunc)
		target     error
	}{
		{
			name: "cancelled",
			newContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx, cancel
			},
			target: context.Canceled,
		},
		{
			name: "deadline exceeded",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			target: context.DeadlineExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := test.newContext()
			defer cancel()

			c, err := New[v3Row]()
			require.NoError(t, err)

			_, err = c.Run(ctx, []v3Row{{Name: "alice", Age: 30}})
			assert.Error(t, err)
			assert.True(t, errors.Is(err, test.target))
			assert.Equal(t, int64(0), c.GetCounterDone().Value())
		})
	}
}
