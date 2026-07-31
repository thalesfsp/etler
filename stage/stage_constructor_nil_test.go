package stage

import (
	"context"
	"expvar"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/metrics"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/task"
)

func assertStageMetricsNotPublished(t *testing.T, name string) {
	t.Helper()

	metricNames := []string{
		fmt.Sprintf("etler.stage.%s.created.counter", name),
		fmt.Sprintf("etler.stage.%s.done.counter", name),
		fmt.Sprintf("etler.stage.%s.failed.counter", name),
		fmt.Sprintf("etler.stage.%s.running.counter", name),
		fmt.Sprintf("etler.stage.%s.duration.counter", name),
		fmt.Sprintf("etler.stage.%s.progress.counter", name),
		fmt.Sprintf("etler.stage.%s.progressPercent", name),
		fmt.Sprintf("etler.stage.%s.status", name),
	}

	for _, metricName := range metricNames {
		assert.Nil(t, expvar.Get(metricName), "rejected stage published %q", metricName)
	}
}

func TestStageNew_rejectsPlainNilProcessor(t *testing.T) {
	t.Setenv(metrics.PublishEnvVar, "true")

	const name = "stage-plain-nil-processor"

	assertStageMetricsNotPublished(t, name)

	stg, err := New[int, int](name, "plain nil processor", identityConverter(), nil)

	assert.Nil(t, stg)
	assert.EqualError(t, err, "processor required. Fields: position=0")
	assertStageMetricsNotPublished(t, name)
}

func TestStageNew_rejectsTypedNilProcessor(t *testing.T) {
	t.Setenv(metrics.PublishEnvVar, "true")

	const name = "stage-typed-nil-processor"

	var nilProcessor *processor.Processor[int]

	assertStageMetricsNotPublished(t, name)

	stg, err := New(name, "typed nil processor", identityConverter(), nilProcessor)

	assert.Nil(t, stg)
	assert.EqualError(t, err, "processor required. Fields: position=0")
	assertStageMetricsNotPublished(t, name)
}

func TestStageNew_reportsNilProcessorPosition(t *testing.T) {
	t.Setenv(metrics.PublishEnvVar, "true")

	const name = "stage-nil-processor-position"

	validProcessor, err := processor.New(
		name+"-valid",
		"valid processor before nil",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	assertStageMetricsNotPublished(t, name)

	stg, err := New(name, "nil processor position", identityConverter(), validProcessor, nil)

	assert.Nil(t, stg)
	assert.EqualError(t, err, "processor required. Fields: position=1")
	assertStageMetricsNotPublished(t, name)
}

func TestStageNew_validProcessorRunsEndToEnd(t *testing.T) {
	double, err := processor.New(
		"stage-valid-construction-processor",
		"double values",
		func(ctx context.Context, processingData []int) ([]int, error) {
			out := make([]int, len(processingData))
			for position, value := range processingData {
				out[position] = value * 2
			}

			return out, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-valid-construction",
		"valid stage",
		identityConverter(),
		double,
	)
	require.NoError(t, err)

	out, err := stg.Run(context.Background(), task.MustNew[int, int]([]int{1, 2, 3}))
	require.NoError(t, err)
	assert.Equal(t, []int{2, 4, 6}, out.ProcessingData)
	assert.Equal(t, []int{2, 4, 6}, out.ConvertedData)
}
