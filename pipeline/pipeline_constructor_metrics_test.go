package pipeline

import (
	"expvar"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/metrics"
	"github.com/thalesfsp/etler/v4/stage"
)

var pipelineConstructorMetricTestID atomic.Uint64

func uniquePipelineMetricName(path string) string {
	return fmt.Sprintf("pipeline-rejected-constructor-%s-%d", path, pipelineConstructorMetricTestID.Add(1))
}

func pipelineMetricExpectations(name string) map[string]string {
	return map[string]string{
		fmt.Sprintf("etler.pipeline.%s.created.counter", name):  "1",
		fmt.Sprintf("etler.pipeline.%s.done.counter", name):     "0",
		fmt.Sprintf("etler.pipeline.%s.failed.counter", name):   "0",
		fmt.Sprintf("etler.pipeline.%s.running.counter", name):  "0",
		fmt.Sprintf("etler.pipeline.%s.duration.counter", name): "0",
		fmt.Sprintf("etler.pipeline.%s.progress.counter", name): "0",
		fmt.Sprintf("etler.pipeline.%s.progressPercent", name):  `"created"`,
		fmt.Sprintf("etler.pipeline.%s.status", name):           `"created"`,
	}
}

func assertRejectedPipelineDidNotPublish(t *testing.T, name string) {
	t.Helper()

	var published []string
	for metricName := range pipelineMetricExpectations(name) {
		if expvar.Get(metricName) != nil {
			published = append(published, metricName)
		}
	}

	assert.Empty(t, published, "rejected pipeline published metrics: %v", published)
}

func assertValidPipelinePublishedMetrics(t *testing.T, name string) {
	t.Helper()

	for metricName, expected := range pipelineMetricExpectations(name) {
		metric := expvar.Get(metricName)
		require.NotNil(t, metric, "valid pipeline did not publish %q", metricName)
		assert.Equal(t, expected, metric.String(), "unexpected value for %q", metricName)
	}
}

func TestPipelineNew_rejectedInputsDoNotPublishMetrics(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		reject  func(string) (IPipeline[int, int], error)
	}{
		{
			name:    "no-stages",
			wantErr: "invalid struct. Original Error: Key: 'Pipeline[int,int].Stages' Error:Field validation for 'Stages' failed on the 'required' tag",
			reject: func(name string) (IPipeline[int, int], error) {
				return New[int, int](name, "no stages", false)
			},
		},
		{
			name:    "empty-stages",
			wantErr: "invalid struct. Original Error: Key: 'Pipeline[int,int].Stages' Error:Field validation for 'Stages' failed on the 'gt' tag",
			reject: func(name string) (IPipeline[int, int], error) {
				empty := []stage.IStage[int, int]{}

				return New(name, "empty stages", false, empty...)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := uniquePipelineMetricName(tt.name)
			validStage := newIdentityStage(t, name+"-stage")

			t.Setenv(metrics.PublishEnvVar, "true")

			p, err := tt.reject(name)

			assert.Nil(t, p)
			assert.EqualError(t, err, tt.wantErr)
			assertRejectedPipelineDidNotPublish(t, name)

			p, err = New(name, "valid negative control", false, validStage)
			require.NoError(t, err)
			require.NotNil(t, p)
			assertValidPipelinePublishedMetrics(t, name)
		})
	}
}
