package processor

import (
	"context"
	"expvar"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type processorNilStatusGuard struct {
	IProcessor[int]
}

func (processorNilStatusGuard) GetStatus() *expvar.String {
	return nil
}

type processorNilCreatedGuard struct {
	IProcessor[int]
}

func (processorNilCreatedGuard) GetCounterCreated() *expvar.Int {
	return nil
}

type processorNilRunningGuard struct {
	IProcessor[int]
}

func (processorNilRunningGuard) GetCounterRunning() *expvar.Int {
	return nil
}

type processorNilFailedGuard struct {
	IProcessor[int]
}

func (processorNilFailedGuard) GetCounterFailed() *expvar.Int {
	return nil
}

type processorNilDoneGuard struct {
	IProcessor[int]
}

func (processorNilDoneGuard) GetCounterDone() *expvar.Int {
	return nil
}

type processorNilInterruptedGuard struct {
	IProcessor[int]
}

func (processorNilInterruptedGuard) GetCounterInterrupted() *expvar.Int {
	return nil
}

type processorNilDurationGuard struct {
	IProcessor[int]
}

func (processorNilDurationGuard) GetDuration() *expvar.Int {
	return nil
}

type processorValueGuard struct {
	IProcessor[int]
}

func newGuardProcessor(t *testing.T) IProcessor[int] {
	t.Helper()

	p, err := New(
		"processor-guard-coverage",
		"processor guard coverage",
		func(_ context.Context, data []int) ([]int, error) {
			return data, nil
		},
	)
	require.NoError(t, err)

	return p
}

func TestValidateOptionMetrics_checksEveryProcessorMetric(t *testing.T) {
	base := newGuardProcessor(t)

	tests := []struct {
		name    string
		value   IProcessor[int]
		wantErr string
	}{
		{
			name:    "status",
			value:   processorNilStatusGuard{IProcessor: base},
			wantErr: "processor status metric required",
		},
		{
			name:    "created counter",
			value:   processorNilCreatedGuard{IProcessor: base},
			wantErr: "processor created counter required",
		},
		{
			name:    "running counter",
			value:   processorNilRunningGuard{IProcessor: base},
			wantErr: "processor running counter required",
		},
		{
			name:    "failed counter",
			value:   processorNilFailedGuard{IProcessor: base},
			wantErr: "processor failed counter required",
		},
		{
			name:    "done counter",
			value:   processorNilDoneGuard{IProcessor: base},
			wantErr: "processor done counter required",
		},
		{
			name:    "interrupted counter",
			value:   processorNilInterruptedGuard{IProcessor: base},
			wantErr: "processor interrupted counter required",
		},
		{
			name:    "duration",
			value:   processorNilDurationGuard{IProcessor: base},
			wantErr: "processor duration metric required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualError(t, validateOptionMetrics(tt.value), tt.wantErr)
		})
	}

	assert.NoError(t, validateOptionMetrics(base))
}

func TestIsNilOptionResult_processorValues(t *testing.T) {
	base := newGuardProcessor(t)
	var plainNil IProcessor[int]
	var typedNil *processorValueGuard

	tests := []struct {
		name  string
		value IProcessor[int]
		want  bool
	}{
		{name: "plain nil", value: plainNil, want: true},
		{name: "typed nil pointer", value: typedNil, want: true},
		{
			name:  "non-nil value",
			value: processorValueGuard{IProcessor: base},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNilOptionResult(tt.value))
		})
	}
}
