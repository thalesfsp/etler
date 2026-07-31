package pipeline

import (
	"context"
	"expvar"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/metrics"
	"github.com/thalesfsp/etler/v4/stage"
)

func assertPipelineMetricsNotPublished(t *testing.T, name string) {
	t.Helper()

	metricNames := []string{
		fmt.Sprintf("etler.pipeline.%s.created.counter", name),
		fmt.Sprintf("etler.pipeline.%s.done.counter", name),
		fmt.Sprintf("etler.pipeline.%s.failed.counter", name),
		fmt.Sprintf("etler.pipeline.%s.running.counter", name),
		fmt.Sprintf("etler.pipeline.%s.duration.counter", name),
		fmt.Sprintf("etler.pipeline.%s.progress.counter", name),
		fmt.Sprintf("etler.pipeline.%s.progressPercent", name),
		fmt.Sprintf("etler.pipeline.%s.status", name),
	}

	for _, metricName := range metricNames {
		assert.Nil(t, expvar.Get(metricName), "rejected pipeline published %q", metricName)
	}
}

func TestPipelineNew_rejectsPlainNilStage(t *testing.T) {
	for _, tt := range []struct {
		name       string
		concurrent bool
	}{
		{name: "sequential", concurrent: false},
		{name: "concurrent", concurrent: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(metrics.PublishEnvVar, "true")

			name := "pipeline-plain-nil-stage-" + tt.name

			assertPipelineMetricsNotPublished(t, name)

			p, err := New[int, int](name, "plain nil stage", tt.concurrent, nil)

			assert.Nil(t, p)
			assert.EqualError(t, err, "stage required. Fields: position=0")
			assertPipelineMetricsNotPublished(t, name)
		})
	}
}

func TestPipelineNew_rejectsTypedNilStage(t *testing.T) {
	t.Setenv(metrics.PublishEnvVar, "true")

	const name = "pipeline-typed-nil-stage"

	var nilStage *stage.Stage[int, int]

	assertPipelineMetricsNotPublished(t, name)

	p, err := New(name, "typed nil stage", false, nilStage)

	assert.Nil(t, p)
	assert.EqualError(t, err, "stage required. Fields: position=0")
	assertPipelineMetricsNotPublished(t, name)
}

func TestPipelineNew_validStageRunsEndToEnd(t *testing.T) {
	p, err := New(
		"pipeline-valid-construction",
		"valid pipeline",
		false,
		newIdentityStage(t, "pipeline-valid-construction-stage"),
	)
	require.NoError(t, err)

	out, err := p.Run(context.Background(), []int{1, 2, 3})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, []int{1, 2, 3}, out[0].ProcessingData)
	assert.Equal(t, []int{1, 2, 3}, out[0].ConvertedData)
}
