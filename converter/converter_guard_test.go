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
			assert.EqualError(t, validateOptionMetrics(tt.value), tt.wantErr)
		})
	}

	assert.NoError(t, validateOptionMetrics(base))
}

func TestIsNilOptionResult_converterValues(t *testing.T) {
	base := newGuardConverter(t)
	var plainNil IConverter[int, int]
	var typedNil *converterValueGuard

	tests := []struct {
		name  string
		value IConverter[int, int]
		want  bool
	}{
		{name: "plain nil", value: plainNil, want: true},
		{name: "typed nil pointer", value: typedNil, want: true},
		{
			name:  "non-nil value",
			value: converterValueGuard{IConverter: base},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNilOptionResult(tt.value))
		})
	}
}
