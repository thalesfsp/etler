package processor

import (
	"context"
	"expvar"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/metrics"
)

var processorConstructorMetricTestID atomic.Uint64

func uniqueProcessorMetricName(path string) string {
	return fmt.Sprintf("processor-rejected-constructor-%s-%d", path, processorConstructorMetricTestID.Add(1))
}

func processorMetricExpectations(name string) map[string]string {
	return map[string]string{
		fmt.Sprintf("etler.processor.%s.created.counter", name):     "1",
		fmt.Sprintf("etler.processor.%s.done.counter", name):        "0",
		fmt.Sprintf("etler.processor.%s.failed.counter", name):      "0",
		fmt.Sprintf("etler.processor.%s.running.counter", name):     "0",
		fmt.Sprintf("etler.processor.%s.interrupted.counter", name): "0",
		fmt.Sprintf("etler.processor.%s.duration.counter", name):    "0",
		fmt.Sprintf("etler.processor.%s.status", name):              `"created"`,
	}
}

func assertRejectedProcessorDidNotPublish(t *testing.T, name string) {
	t.Helper()

	var published []string
	for metricName := range processorMetricExpectations(name) {
		if expvar.Get(metricName) != nil {
			published = append(published, metricName)
		}

	}

	assert.Empty(t, published, "rejected processor published metrics: %v", published)
}

func assertValidProcessorPublishedMetrics(t *testing.T, name string) {
	t.Helper()

	for metricName, expected := range processorMetricExpectations(name) {
		metric := expvar.Get(metricName)
		require.NotNil(t, metric, "valid processor did not publish %q", metricName)
		assert.Equal(t, expected, metric.String(), "unexpected value for %q", metricName)
	}
}

func TestProcessorNew_rejectedInputsDoNotPublishMetrics(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		reject  func(string) (IProcessor[int], error)
	}{
		{
			name:    "nil-function",
			wantErr: "invalid struct. Original Error: Key: 'Processor[int].Func' Error:Field validation for 'Func' failed on the 'required' tag",
			reject: func(name string) (IProcessor[int], error) {
				return New[int](name, "nil function", nil)
			},
		},
		{
			name:    "nil-option",
			wantErr: "processor option required",
			reject: func(name string) (IProcessor[int], error) {
				return New(
					name,
					"nil option",
					func(_ context.Context, in []int) ([]int, error) { return in, nil },
					nil,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := uniqueProcessorMetricName(tt.name)
			t.Setenv(metrics.PublishEnvVar, "true")

			p, err := tt.reject(name)

			assert.Nil(t, p)
			assert.EqualError(t, err, tt.wantErr)
			assertRejectedProcessorDidNotPublish(t, name)

			p, err = New(
				name,
				"valid negative control",
				func(_ context.Context, in []int) ([]int, error) { return in, nil },
			)
			require.NoError(t, err)
			require.NotNil(t, p)
			assertValidProcessorPublishedMetrics(t, name)
		})
	}
}

func TestProcessorNew_rejectsStructurallyInvalidOptionResult(t *testing.T) {
	p, err := New(
		uniqueProcessorMetricName("invalid-option-result"),
		"invalid option result",
		func(_ context.Context, in []int) ([]int, error) { return in, nil },
		func(IProcessor[int]) IProcessor[int] {
			return &Processor[int]{}
		},
	)

	assert.Nil(t, p)
	assert.EqualError(
		t,
		err,
		"invalid struct. Original Error: Key: 'Processor[int].Func' Error:Field validation for 'Func' failed on the 'required' tag\nKey: 'Processor[int].Logger' Error:Field validation for 'Logger' failed on the 'required' tag\nKey: 'Processor[int].Name' Error:Field validation for 'Name' failed on the 'required' tag",
	)
}
