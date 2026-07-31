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

type processorMissingImplementationGuard struct {
	IProcessor[int] `validate:"required"`
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
			var (
				p   IProcessor[int]
				err error
			)

			assert.NotPanics(t, func() {
				p, err = New(
					"processor-guard-"+tt.name,
					"processor guard",
					func(_ context.Context, data []int) ([]int, error) {
						return data, nil
					},
					func(IProcessor[int]) IProcessor[int] {
						return tt.value
					},
				)
			})

			assert.Nil(t, p)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestIsNilOptionResult_processorValues(t *testing.T) {
	base := newGuardProcessor(t)
	var plainNil IProcessor[int]
	var typedNil *processorValueGuard

	tests := []struct {
		name    string
		value   IProcessor[int]
		wantErr string
	}{
		{
			name:    "plain nil",
			value:   plainNil,
			wantErr: "processor option result required",
		},
		{
			name:    "typed nil pointer",
			value:   typedNil,
			wantErr: "processor option result required",
		},
		{
			name:  "non-nil value",
			value: processorValueGuard{IProcessor: base},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(
				"processor-option-result-"+tt.name,
				"processor option result",
				func(_ context.Context, data []int) ([]int, error) {
					return data, nil
				},
				func(IProcessor[int]) IProcessor[int] {
					return tt.value
				},
			)

			if tt.wantErr != "" {
				assert.Nil(t, p)
				assert.EqualError(t, err, tt.wantErr)

				return
			}

			assert.NotNil(t, p)
			assert.NoError(t, err)
		})
	}
}

func TestNew_validatesOptionResultBeforeMetricGetters(t *testing.T) {
	var constructorErr error

	assert.NotPanics(t, func() {
		p, err := New(
			"processor-structural-validation-first",
			"processor structural validation first",
			func(_ context.Context, data []int) ([]int, error) {
				return data, nil
			},
			func(IProcessor[int]) IProcessor[int] {
				return &processorMissingImplementationGuard{}
			},
		)
		constructorErr = err

		assert.Nil(t, p)
	})
	require.Error(t, constructorErr)
	assert.Contains(t, constructorErr.Error(), "invalid struct")
}
