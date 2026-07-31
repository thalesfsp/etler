package stage

import (
	"context"
	"expvar"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/etler/v4/internal/metrics"
	"github.com/thalesfsp/etler/v4/processor"
)

var stageConstructorMetricTestID atomic.Uint64

func uniqueStageMetricName(path string) string {
	return fmt.Sprintf("stage-rejected-constructor-%s-%d", path, stageConstructorMetricTestID.Add(1))
}

func stageMetricExpectations(name string) map[string]string {
	return map[string]string{
		fmt.Sprintf("etler.stage.%s.created.counter", name):  "1",
		fmt.Sprintf("etler.stage.%s.done.counter", name):     "0",
		fmt.Sprintf("etler.stage.%s.failed.counter", name):   "0",
		fmt.Sprintf("etler.stage.%s.running.counter", name):  "0",
		fmt.Sprintf("etler.stage.%s.duration.counter", name): "0",
		fmt.Sprintf("etler.stage.%s.progress.counter", name): "0",
		fmt.Sprintf("etler.stage.%s.progressPercent", name):  `"created"`,
		fmt.Sprintf("etler.stage.%s.status", name):           `"created"`,
	}
}

func assertRejectedStageDidNotPublish(t *testing.T, name string) {
	t.Helper()

	var published []string
	for metricName := range stageMetricExpectations(name) {
		if expvar.Get(metricName) != nil {
			published = append(published, metricName)
		}
	}

	assert.Empty(t, published, "rejected stage published metrics: %v", published)
}

func assertValidStagePublishedMetrics(t *testing.T, name string) {
	t.Helper()

	for metricName, expected := range stageMetricExpectations(name) {
		metric := expvar.Get(metricName)
		require.NotNil(t, metric, "valid stage did not publish %q", metricName)
		assert.Equal(t, expected, metric.String(), "unexpected value for %q", metricName)
	}
}

func newStageMetricControlProcessor(t *testing.T, name string) processor.IProcessor[int] {
	t.Helper()

	proc, err := processor.New(
		name+"-processor",
		"valid processor",
		func(_ context.Context, in []int) ([]int, error) {
			return in, nil
		},
	)
	require.NoError(t, err)

	return proc
}

func TestStageNew_rejectedInputsDoNotPublishMetrics(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		reject  func(string, converter.IConverter[int, int], processor.IProcessor[int]) (IStage[int, int], error)
	}{
		{
			name:    "no-processors",
			wantErr: "invalid struct. Original Error: Key: 'Stage[int,int].Processors' Error:Field validation for 'Processors' failed on the 'required' tag",
			reject: func(name string, conversor converter.IConverter[int, int], _ processor.IProcessor[int]) (IStage[int, int], error) {
				return New[int, int](name, "no processors", conversor)
			},
		},
		{
			name:    "empty-processors",
			wantErr: "invalid struct. Original Error: Key: 'Stage[int,int].Processors' Error:Field validation for 'Processors' failed on the 'gt' tag",
			reject: func(name string, conversor converter.IConverter[int, int], _ processor.IProcessor[int]) (IStage[int, int], error) {
				empty := []processor.IProcessor[int]{}

				return New(name, "empty processors", conversor, empty...)
			},
		},
		{
			name:    "nil-converter",
			wantErr: "invalid struct. Original Error: Key: 'Stage[int,int].Conversor' Error:Field validation for 'Conversor' failed on the 'required' tag",
			reject: func(name string, _ converter.IConverter[int, int], proc processor.IProcessor[int]) (IStage[int, int], error) {
				return New[int, int](name, "nil converter", nil, proc)
			},
		},
		{
			name:    "typed-nil-converter",
			wantErr: "invalid struct. Original Error: Key: 'Stage[int,int].Conversor' Error:Field validation for 'Conversor' failed on the 'required' tag",
			reject: func(name string, _ converter.IConverter[int, int], proc processor.IProcessor[int]) (IStage[int, int], error) {
				var nilConverter *converter.Converter[int, int]

				return New(name, "typed nil converter", nilConverter, proc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := uniqueStageMetricName(tt.name)
			conversor := identityConverter()
			proc := newStageMetricControlProcessor(t, name)

			t.Setenv(metrics.PublishEnvVar, "true")

			stg, err := tt.reject(name, conversor, proc)

			assert.Nil(t, stg)
			assert.EqualError(t, err, tt.wantErr)
			assertRejectedStageDidNotPublish(t, name)

			stg, err = New(name, "valid negative control", conversor, proc)
			require.NoError(t, err)
			require.NotNil(t, stg)
			assertValidStagePublishedMetrics(t, name)
		})
	}
}
