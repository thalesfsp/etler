package loader

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

type replacementLoader struct {
	ILoader[string, []int]
}

type loaderWithNilStatus struct {
	ILoader[string, []int]
}

func (l *loaderWithNilStatus) GetStatus() *expvar.String {
	return nil
}

// Edge case: a loader without a load function is a configuration error — it
// would panic at Run time otherwise.
func TestLoader_new_nilFunc_returnsError(t *testing.T) {
	l, err := New[string, []int]("nil-func-audit", "no function", nil)
	assert.Nil(t, l)
	assert.Error(t, err, "a loader without a load function must not be created")
}

// A return-valued option may replace or wrap the loader.
func TestLoader_new_usesOptionReturnValue(t *testing.T) {
	var replacement *replacementLoader

	got, err := New(
		"replacement-option-loader",
		"uses the option return value",
		func(ctx context.Context, in string) ([]int, error) {
			return []int{len(in)}, nil
		},
		func(l ILoader[string, []int]) ILoader[string, []int] {
			replacement = &replacementLoader{ILoader: l}

			return replacement
		},
	)
	require.NoError(t, err)
	assert.Same(t, replacement, got)
}

// Returning nil from an option is an invalid constructor result.
func TestLoader_new_rejectsNilOptionReturn(t *testing.T) {
	got, err := New(
		"nil-option-loader",
		"rejects a nil option return",
		func(ctx context.Context, in string) ([]int, error) {
			return []int{len(in)}, nil
		},
		func(ILoader[string, []int]) ILoader[string, []int] {
			return nil
		},
	)

	assert.Nil(t, got)
	assert.Error(t, err)
}

// A nil option is invalid input and must be rejected before invocation.
func TestLoader_new_rejectsNilOptionWithoutPanic(t *testing.T) {
	var opt Func[string, []int]

	var (
		got ILoader[string, []int]
		err error
	)

	assert.NotPanics(t, func() {
		got, err = New(
			"nil-option-function-loader",
			"rejects a nil option function",
			func(ctx context.Context, in string) ([]int, error) {
				return []int{len(in)}, nil
			},
			opt,
		)
	})

	assert.Nil(t, got)
	assert.Error(t, err)
}

// An option replacement must retain non-nil metric getters.
func TestLoader_new_rejectsNilMetricGetterWithoutPanic(t *testing.T) {
	var (
		got ILoader[string, []int]
		err error
	)

	assert.NotPanics(t, func() {
		got, err = New(
			"nil-metric-getter-loader",
			"rejects a nil metric getter",
			func(ctx context.Context, in string) ([]int, error) {
				return []int{len(in)}, nil
			},
			func(l ILoader[string, []int]) ILoader[string, []int] {
				return &loaderWithNilStatus{ILoader: l}
			},
		)
	})

	assert.Nil(t, got)
	assert.Error(t, err)
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
