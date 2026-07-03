package passthru

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Must returns a working pass-through converter (happy path), including for
// zero values.
func TestMust_happyPath(t *testing.T) {
	var p *Passthru[int]

	require.NotPanics(t, func() {
		p = Must[int]()
	})

	out, err := p.Run(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, 42, out)

	zero, err := p.Run(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, 0, zero)
}
