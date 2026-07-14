package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/sypl/v2/fields"
	"go.elastic.co/apm"
)

// Get returns a singleton.
func TestGet_singleton(t *testing.T) {
	l1 := Get()
	l2 := Get()

	require.NotNil(t, l1)
	assert.Same(t, l1, l2)
}

// ToAPM without a transaction: fields pass through; nil fields become empty.
func TestToAPM_noTransaction(t *testing.T) {
	f := ToAPM(context.Background(), nil)
	require.NotNil(t, f)
	assert.Empty(t, f)

	f2 := ToAPM(context.Background(), fields.Fields{"k": "v"})
	assert.Equal(t, "v", f2["k"])
	assert.NotContains(t, f2, "trace.id")
}

// ToAPM with a transaction and span: correlation fields are set.
func TestToAPM_withTransactionAndSpan(t *testing.T) {
	tx := apm.DefaultTracer.StartTransaction("tx", "test")
	defer tx.End()

	ctx := apm.ContextWithTransaction(context.Background(), tx)

	span, ctx := apm.StartSpan(ctx, "span", "test")
	defer span.End()

	f := ToAPM(ctx, fields.Fields{})

	assert.Contains(t, f, "trace.id")
	assert.Contains(t, f, "transaction.id")
	assert.Contains(t, f, "span.id")
}
