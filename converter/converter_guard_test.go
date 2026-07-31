package converter

import (
	"context"
	"expvar"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type converterNilStatusGuard struct {
	IConverter[int, int]
}

func (converterNilStatusGuard) GetStatus() *expvar.String {
	return nil
}

type converterNilCreatedGuard struct {
	IConverter[int, int]
}

func (converterNilCreatedGuard) GetCounterCreated() *expvar.Int {
	return nil
}

type converterNilRunningGuard struct {
	IConverter[int, int]
}

func (converterNilRunningGuard) GetCounterRunning() *expvar.Int {
	return nil
}

type converterNilFailedGuard struct {
	IConverter[int, int]
}

func (converterNilFailedGuard) GetCounterFailed() *expvar.Int {
	return nil
}

type converterNilDoneGuard struct {
	IConverter[int, int]
}

func (converterNilDoneGuard) GetCounterDone() *expvar.Int {
	return nil
}

type converterNilDurationGuard struct {
	IConverter[int, int]
}

func (converterNilDurationGuard) GetDuration() *expvar.Int {
	return nil
}

type converterValueGuard struct {
	IConverter[int, int]
}

type converterMissingImplementationGuard struct {
	IConverter[int, int]
}

type converterTaggedMissingImplementationGuard struct {
	IConverter[int, int] `validate:"required"`
}

func newGuardConverter(t *testing.T) IConverter[int, int] {
	t.Helper()

	c, err := New(
		"converter-guard-coverage",
		"converter guard coverage",
		func(_ context.Context, in int) (int, error) {
			return in, nil
		},
	)
	require.NoError(t, err)

	return c
}

func TestValidateOptionMetrics_checksEveryConverterMetric(t *testing.T) {
	base := newGuardConverter(t)

	tests := []struct {
		name    string
		value   IConverter[int, int]
		wantErr string
	}{
		{
			name:    "status",
			value:   converterNilStatusGuard{IConverter: base},
			wantErr: "converter status metric required",
		},
		{
			name:    "created counter",
			value:   converterNilCreatedGuard{IConverter: base},
			wantErr: "converter created counter required",
		},
		{
			name:    "running counter",
			value:   converterNilRunningGuard{IConverter: base},
			wantErr: "converter running counter required",
		},
		{
			name:    "failed counter",
			value:   converterNilFailedGuard{IConverter: base},
			wantErr: "converter failed counter required",
		},
		{
			name:    "done counter",
			value:   converterNilDoneGuard{IConverter: base},
			wantErr: "converter done counter required",
		},
		{
			name:    "duration",
			value:   converterNilDurationGuard{IConverter: base},
			wantErr: "converter duration metric required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				c   IConverter[int, int]
				err error
			)

			assert.NotPanics(t, func() {
				c, err = New(
					"converter-guard-"+tt.name,
					"converter guard",
					func(_ context.Context, in int) (int, error) {
						return in, nil
					},
					func(IConverter[int, int]) IConverter[int, int] {
						return tt.value
					},
				)
			})

			assert.Nil(t, c)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestIsNilOptionResult_converterValues(t *testing.T) {
	base := newGuardConverter(t)
	var plainNil IConverter[int, int]
	var typedNil *converterValueGuard

	tests := []struct {
		name    string
		value   IConverter[int, int]
		wantErr string
	}{
		{
			name:    "plain nil",
			value:   plainNil,
			wantErr: "converter option result required",
		},
		{
			name:    "typed nil pointer",
			value:   typedNil,
			wantErr: "converter option result required",
		},
		{
			name:  "non-nil value",
			value: converterValueGuard{IConverter: base},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(
				"converter-option-result-"+tt.name,
				"converter option result",
				func(_ context.Context, in int) (int, error) {
					return in, nil
				},
				func(IConverter[int, int]) IConverter[int, int] {
					return tt.value
				},
			)

			if tt.wantErr != "" {
				assert.Nil(t, c)
				assert.EqualError(t, err, tt.wantErr)

				return
			}

			assert.NotNil(t, c)
			assert.NoError(t, err)
		})
	}
}

func TestNew_rejectsMissingEmbeddedImplementationBeforeMetricGetters(t *testing.T) {
	tests := []struct {
		name  string
		value IConverter[int, int]
	}{
		{
			name:  "untagged",
			value: &converterMissingImplementationGuard{},
		},
		{
			name:  "tagged",
			value: &converterTaggedMissingImplementationGuard{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var constructorErr error

			assert.NotPanics(t, func() {
				c, err := New(
					"converter-missing-implementation-"+tt.name,
					"converter missing implementation",
					func(_ context.Context, in int) (int, error) {
						return in, nil
					},
					func(IConverter[int, int]) IConverter[int, int] {
						return tt.value
					},
				)
				constructorErr = err

				assert.Nil(t, c)
			})
			require.EqualError(t, constructorErr, "converter option result required")
		})
	}
}
