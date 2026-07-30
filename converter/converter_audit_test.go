package converter

import (
	"context"
	"errors"
	"expvar"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/status"
)

type replacementConverter struct {
	IConverter[int, int]
}

type converterWithNilStatus struct {
	IConverter[int, int]
}

func (c *converterWithNilStatus) GetStatus() *expvar.String {
	return nil
}

// Edge case: a converter without a conversion function is a configuration
// error — it would panic at Run time otherwise.
func TestConverter_new_nilFunc_returnsError(t *testing.T) {
	c, err := New[int, int]("nil-func-audit", "no function", nil)
	assert.Nil(t, c)
	assert.Error(t, err, "a converter without a conversion function must not be created")
}

// Bug: Default has an error return — it must return the error, not panic.
func TestConverter_default_nilFunc_returnsErrorNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		c, err := Default[int, int](nil)
		assert.Nil(t, c)
		assert.Error(t, err)
	})
}

// MustDefault is the panicking variant.
func TestConverter_mustDefault_nilFunc_panics(t *testing.T) {
	assert.Panics(t, func() {
		MustDefault[int, int](nil)
	})
}

// A return-valued option may replace or wrap the converter.
func TestConverter_new_usesOptionReturnValue(t *testing.T) {
	var replacement *replacementConverter

	got, err := New(
		"replacement-option-converter",
		"uses the option return value",
		func(ctx context.Context, in int) (int, error) {
			return in, nil
		},
		func(c IConverter[int, int]) IConverter[int, int] {
			replacement = &replacementConverter{IConverter: c}

			return replacement
		},
	)
	require.NoError(t, err)
	assert.Same(t, replacement, got)
}

// Returning nil from an option is an invalid constructor result.
func TestConverter_new_rejectsNilOptionReturn(t *testing.T) {
	got, err := New(
		"nil-option-converter",
		"rejects a nil option return",
		func(ctx context.Context, in int) (int, error) {
			return in, nil
		},
		func(IConverter[int, int]) IConverter[int, int] {
			return nil
		},
	)

	assert.Nil(t, got)
	assert.Error(t, err)
}

// A nil option is invalid input and must be rejected before invocation.
func TestConverter_new_rejectsNilOptionWithoutPanic(t *testing.T) {
	var opt Func[int, int]

	var (
		got IConverter[int, int]
		err error
	)

	assert.NotPanics(t, func() {
		got, err = New(
			"nil-option-function-converter",
			"rejects a nil option function",
			func(ctx context.Context, in int) (int, error) {
				return in, nil
			},
			opt,
		)
	})

	assert.Nil(t, got)
	assert.Error(t, err)
}

// An option replacement must retain non-nil metric getters.
func TestConverter_new_rejectsNilMetricGetterWithoutPanic(t *testing.T) {
	var (
		got IConverter[int, int]
		err error
	)

	assert.NotPanics(t, func() {
		got, err = New(
			"nil-metric-getter-converter",
			"rejects a nil metric getter",
			func(ctx context.Context, in int) (int, error) {
				return in, nil
			},
			func(c IConverter[int, int]) IConverter[int, int] {
				return &converterWithNilStatus{IConverter: c}
			},
		)
	})

	assert.Nil(t, got)
	assert.Error(t, err)
}

// Bad path: a failing conversion must propagate the cause and update metrics.
func TestConverter_error_updatesMetricsAndPropagatesCause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	boom := errors.New("boom-converter-audit")

	failing, err := New(
		"failing-conv-audit",
		"always fails",
		func(ctx context.Context, in int) (int, error) {
			return 0, boom
		},
	)
	require.NoError(t, err)

	out, err := failing.Run(ctx, 1)
	assert.Zero(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-converter-audit")

	assert.Equal(t, status.Failed.String(), failing.GetStatus().Value())
	assert.Equal(t, int64(1), failing.GetCounterFailed().Value())
	assert.Equal(t, int64(0), failing.GetCounterDone().Value())
}

// Happy path: OnFinished receives the input and the converted output.
func TestConverter_onFinished_receivesInAndOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var gotIn, gotOut int

	double, err := New(
		"double-conv-audit",
		"doubles the input",
		func(ctx context.Context, in int) (int, error) {
			return in * 2, nil
		},
		WithOnFinished(func(ctx context.Context, c IConverter[int, int], in int, out int) {
			gotIn = in
			gotOut = out
		}),
	)
	require.NoError(t, err)

	out, err := double.Run(ctx, 21)
	require.NoError(t, err)
	assert.Equal(t, 42, out)

	assert.Equal(t, 21, gotIn)
	assert.Equal(t, 42, gotOut)
}
