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
			assert.EqualError(t, validateOptionMetrics(tt.value), tt.wantErr)
		})
	}

	assert.NoError(t, validateOptionMetrics(base))
}

func TestIsNilOptionResult_loaderValues(t *testing.T) {
	base := newGuardLoader(t)
	var plainNil ILoader[string, []int]
	var typedNil *loaderValueGuard

	tests := []struct {
		name  string
		value ILoader[string, []int]
		want  bool
	}{
		{name: "plain nil", value: plainNil, want: true},
		{name: "typed nil pointer", value: typedNil, want: true},
		{
			name:  "non-nil value",
			value: loaderValueGuard{ILoader: base},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNilOptionResult(tt.value))
		})
	}
}
