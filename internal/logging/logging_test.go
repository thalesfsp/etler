package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/fields"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
	"github.com/thalesfsp/sypl/v2/shared"
	"go.elastic.co/apm"
)

// Admits follows sypl's output max-level rule.
func TestAdmits(t *testing.T) {
	tests := []struct {
		name     string
		maxLevel level.Level
		logLevel level.Level
		want     bool
	}{
		{
			name:     "error rejects debug",
			maxLevel: level.Error,
			logLevel: level.Debug,
			want:     false,
		},
		{
			name:     "error admits error",
			maxLevel: level.Error,
			logLevel: level.Error,
			want:     true,
		},
		{
			name:     "debug admits debug",
			maxLevel: level.Debug,
			logLevel: level.Debug,
			want:     true,
		},
		{
			name:     "debug rejects trace",
			maxLevel: level.Debug,
			logLevel: level.Trace,
			want:     false,
		},
		{
			name:     "trace admits debug",
			maxLevel: level.Trace,
			logLevel: level.Debug,
			want:     true,
		},
		{
			name:     "trace admits trace",
			maxLevel: level.Trace,
			logLevel: level.Trace,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, o := output.SafeBuffer(level.Error)
			l := sypl.New("admission-test", o)
			l.SetMaxLevel(tt.maxLevel)

			assert.Equal(t, tt.want, Admits(l, tt.logLevel))
		})
	}
}

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

// SYPL_LEVEL and SYPL_FILTER raise the effective level at runtime, and sypl
// honours them only on its slow path. Gating on the outputs' configured max
// level alone would silently defeat them, so Admits must defer.
func TestAdmits_deferToSyplWhenEnvRaisesLevel(t *testing.T) {
	l := sypl.NewDefault("admits-env", level.Error)

	// Baseline: with no env var set, a debug call cannot reach an
	// error-level output.
	assert.False(t, Admits(l, level.Debug))

	for _, envVar := range []string{shared.LevelEnvVar, shared.FilterEnvVar} {
		t.Run(envVar, func(t *testing.T) {
			t.Setenv(envVar, "trace")

			assert.True(t, Admits(l, level.Debug),
				"%s can raise the level at runtime, so the payload must still be built", envVar)
		})
	}
}

// Fatal always runs the full pipeline because it exits the process, so it must
// never be gated out regardless of the outputs' levels.
func TestAdmits_neverGatesFatal(t *testing.T) {
	assert.True(t, Admits(sypl.NewDefault("admits-fatal", level.None), level.Fatal))
}
