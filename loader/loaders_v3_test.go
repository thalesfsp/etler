package loader

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Metadata getters + OnFinished option.
func TestLoader_metadataAndOnFinished(t *testing.T) {
	var gotIn string

	var gotOut []int

	l, err := New(
		"meta-loader",
		"metadata test",
		func(ctx context.Context, in string) ([]int, error) {
			return []int{len(in)}, nil
		},
		WithOnFinished(func(ctx context.Context, l ILoader[string, []int], in string, out []int) {
			gotIn = in
			gotOut = out
		}),
	)
	require.NoError(t, err)

	// The callback field is intentionally unexported; accessors are the
	// supported mutation path and therefore cannot bypass its mutex.
	assert.NotNil(t, l.GetOnFinished())

	l.SetOnFinished(nil)
	assert.Nil(t, l.GetOnFinished())

	l.SetOnFinished(func(ctx context.Context, l ILoader[string, []int], in string, out []int) {
		gotIn = in
		gotOut = out
	})
	assert.NotNil(t, l.GetOnFinished())

	assert.Equal(t, "meta-loader", l.GetName())
	assert.Equal(t, "metadata test", l.GetDescription())
	assert.Equal(t, Type, l.GetType())
	assert.NotZero(t, l.GetCreatedAt())

	m := l.GetMetrics()
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "counterDone")

	out, err := l.Run(context.Background(), "abcd")
	require.NoError(t, err)
	assert.Equal(t, []int{4}, out)

	assert.Equal(t, "abcd", gotIn, "OnFinished must receive the input")
	assert.Equal(t, []int{4}, gotOut, "OnFinished must receive the output")
}
