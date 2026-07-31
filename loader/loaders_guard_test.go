package loader

import (
	"context"
	"expvar"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loaderNilStatusGuard struct {
	ILoader[string, []int]
}

func (loaderNilStatusGuard) GetStatus() *expvar.String {
	return nil
}

type loaderNilCreatedGuard struct {
	ILoader[string, []int]
}

func (loaderNilCreatedGuard) GetCounterCreated() *expvar.Int {
	return nil
}

type loaderNilRunningGuard struct {
	ILoader[string, []int]
}

func (loaderNilRunningGuard) GetCounterRunning() *expvar.Int {
	return nil
}

type loaderNilFailedGuard struct {
	ILoader[string, []int]
}

func (loaderNilFailedGuard) GetCounterFailed() *expvar.Int {
	return nil
}

type loaderNilDoneGuard struct {
	ILoader[string, []int]
}

func (loaderNilDoneGuard) GetCounterDone() *expvar.Int {
	return nil
}

type loaderNilDurationGuard struct {
	ILoader[string, []int]
}

func (loaderNilDurationGuard) GetDuration() *expvar.Int {
	return nil
}

type loaderValueGuard struct {
	ILoader[string, []int]
}

type loaderMissingImplementationGuard struct {
	ILoader[string, []int] `validate:"required"`
}

func newGuardLoader(t *testing.T) ILoader[string, []int] {
	t.Helper()

	l, err := New(
		"loader-guard-coverage",
		"loader guard coverage",
		func(_ context.Context, in string) ([]int, error) {
			return []int{len(in)}, nil
		},
	)
	require.NoError(t, err)

	return l
}

func TestValidateOptionMetrics_checksEveryLoaderMetric(t *testing.T) {
	base := newGuardLoader(t)

	tests := []struct {
		name    string
		value   ILoader[string, []int]
		wantErr string
	}{
		{
			name:    "status",
			value:   loaderNilStatusGuard{ILoader: base},
			wantErr: "loader status metric required",
		},
		{
			name:    "created counter",
			value:   loaderNilCreatedGuard{ILoader: base},
			wantErr: "loader created counter required",
		},
		{
			name:    "running counter",
			value:   loaderNilRunningGuard{ILoader: base},
			wantErr: "loader running counter required",
		},
		{
			name:    "failed counter",
			value:   loaderNilFailedGuard{ILoader: base},
			wantErr: "loader failed counter required",
		},
		{
			name:    "done counter",
			value:   loaderNilDoneGuard{ILoader: base},
			wantErr: "loader done counter required",
		},
		{
			name:    "duration",
			value:   loaderNilDurationGuard{ILoader: base},
			wantErr: "loader duration metric required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				l   ILoader[string, []int]
				err error
			)

			assert.NotPanics(t, func() {
				l, err = New(
					"loader-guard-"+tt.name,
					"loader guard",
					func(_ context.Context, in string) ([]int, error) {
						return []int{len(in)}, nil
					},
					func(ILoader[string, []int]) ILoader[string, []int] {
						return tt.value
					},
				)
			})

			assert.Nil(t, l)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestIsNilOptionResult_loaderValues(t *testing.T) {
	base := newGuardLoader(t)
	var plainNil ILoader[string, []int]
	var typedNil *loaderValueGuard

	tests := []struct {
		name    string
		value   ILoader[string, []int]
		wantErr string
	}{
		{
			name:    "plain nil",
			value:   plainNil,
			wantErr: "loader option result required",
		},
		{
			name:    "typed nil pointer",
			value:   typedNil,
			wantErr: "loader option result required",
		},
		{
			name:  "non-nil value",
			value: loaderValueGuard{ILoader: base},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := New(
				"loader-option-result-"+tt.name,
				"loader option result",
				func(_ context.Context, in string) ([]int, error) {
					return []int{len(in)}, nil
				},
				func(ILoader[string, []int]) ILoader[string, []int] {
					return tt.value
				},
			)

			if tt.wantErr != "" {
				assert.Nil(t, l)
				assert.EqualError(t, err, tt.wantErr)

				return
			}

			assert.NotNil(t, l)
			assert.NoError(t, err)
		})
	}
}

func TestNew_validatesOptionResultBeforeMetricGetters(t *testing.T) {
	var constructorErr error

	assert.NotPanics(t, func() {
		l, err := New(
			"loader-structural-validation-first",
			"loader structural validation first",
			func(_ context.Context, in string) ([]int, error) {
				return []int{len(in)}, nil
			},
			func(ILoader[string, []int]) ILoader[string, []int] {
				return &loaderMissingImplementationGuard{}
			},
		)
		constructorErr = err

		assert.Nil(t, l)
	})
	require.Error(t, constructorErr)
	assert.Contains(t, constructorErr.Error(), "invalid struct")
}
