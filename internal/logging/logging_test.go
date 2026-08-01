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
	"github.com/thalesfsp/sypl/v2/status"
	"go.elastic.co/apm"
)

type loggerValueGuard struct {
	sypl.ISypl
}

func TestIsNilLogger_values(t *testing.T) {
	var plainNil sypl.ISypl
	var typedNil *sypl.Sypl
	nonNil := loggerValueGuard{
		ISypl: sypl.NewDefault("non-nil-logger-guard", level.Error),
	}

	tests := []struct {
		name  string
		value sypl.ISypl
		want  bool
	}{
		{name: "plain nil", value: plainNil, want: true},
		{name: "typed nil pointer", value: typedNil, want: true},
		{name: "non-nil value", value: nonNil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNilLogger(tt.value))
		})
	}
}

// Admits follows sypl's output max-level rule.
func TestAdmits(t *testing.T) {
	t.Setenv(shared.LevelEnvVar, "")
	t.Setenv(shared.FilterEnvVar, "")

	tests := []struct {
		name            string
		outputLevels    []level.Level
		disabledOutputs map[int]bool
		nilLogger       bool
		logLevel        level.Level
		want            bool
	}{
		{
			name:         "error rejects debug",
			outputLevels: []level.Level{level.Error},
			logLevel:     level.Debug,
			want:         false,
		},
		{
			name:         "error admits error",
			outputLevels: []level.Level{level.Error},
			logLevel:     level.Error,
			want:         true,
		},
		{
			name:         "debug admits debug",
			outputLevels: []level.Level{level.Debug},
			logLevel:     level.Debug,
			want:         true,
		},
		{
			name:         "debug rejects trace",
			outputLevels: []level.Level{level.Debug},
			logLevel:     level.Trace,
			want:         false,
		},
		{
			name:         "trace admits debug",
			outputLevels: []level.Level{level.Trace},
			logLevel:     level.Debug,
			want:         true,
		},
		{
			name:         "trace admits trace",
			outputLevels: []level.Level{level.Trace},
			logLevel:     level.Trace,
			want:         true,
		},
		{
			name:         "none is rejected",
			outputLevels: []level.Level{level.Trace},
			logLevel:     level.None,
			want:         false,
		},
		{
			name:      "nil logger is rejected",
			nilLogger: true,
			logLevel:  level.Error,
			want:      false,
		},
		{
			name:            "disabled output is ignored",
			outputLevels:    []level.Level{level.Trace},
			disabledOutputs: map[int]bool{0: true},
			logLevel:        level.Debug,
			want:            false,
		},
		{
			name:         "highest enabled output wins",
			outputLevels: []level.Level{level.Error, level.Trace, level.Debug},
			logLevel:     level.Debug,
			want:         true,
		},
		{
			name:            "disabled higher output does not admit",
			outputLevels:    []level.Level{level.Error, level.Trace},
			disabledOutputs: map[int]bool{1: true},
			logLevel:        level.Debug,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l sypl.ISypl

			if !tt.nilLogger {
				outputs := make([]output.IOutput, 0, len(tt.outputLevels))

				for i, maxLevel := range tt.outputLevels {
					_, o := output.SafeBuffer(maxLevel)
					if tt.disabledOutputs[i] {
						o.SetStatus(status.Disabled)
					}

					outputs = append(outputs, o)
				}

				l = sypl.New("admission-test", outputs...)
			}

			assert.Equal(t, tt.want, Admits(l, tt.logLevel))
		})
	}
}

// A non-nil interface can hold a nil logger pointer. Admits must reject it
// before dispatching GetOutputs through the interface.
func TestAdmits_typedNilLogger(t *testing.T) {
	t.Setenv(shared.LevelEnvVar, "")
	t.Setenv(shared.FilterEnvVar, "")

	var concrete *sypl.Sypl
	var logger sypl.ISypl = concrete
	var got bool

	assert.NotPanics(t, func() {
		got = Admits(logger, level.Error)
	})
	assert.False(t, got)
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
	// Start from a known-clean environment so the baseline below holds even for
	// a developer who exports either var.
	t.Setenv(shared.LevelEnvVar, "")
	t.Setenv(shared.FilterEnvVar, "")

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
