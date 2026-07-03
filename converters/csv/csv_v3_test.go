package csv

import (
	"context"
	"testing"

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
