package stage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/internal/metrics"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/task"
)

// Metadata getters + WithOnFinished option.
func TestStage_metadataAndOnFinishedOption(t *testing.T) {
	identity, err := processor.New(
		"identity-meta",
		"identity",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"meta-stage",
		"metadata test",
		identityConverter(),
		identity,
	)
	require.NoError(t, err)

	assert.Equal(t, "meta-stage", stg.GetName())
	assert.Equal(t, "metadata test", stg.GetDescription())
	assert.Equal(t, Type, stg.GetType())
	assert.NotZero(t, stg.GetCreatedAt())

	m := stg.GetMetrics()
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "progressPercent")

	// WithOnFinished applies the callback.
	called := false

	WithOnFinished[int, int](func(ctx context.Context, s IStage[int, int], in task.Task[int, int], out task.Task[int, int]) {
		called = true
	})(stg)

	// The callback field is intentionally unexported; accessors are the
	// supported mutation path and therefore cannot bypass its mutex.
	assert.NotNil(t, stg.GetOnFinished())

	_, err = stg.Run(context.Background(), task.MustNew[int, int]([]int{1}))
	require.NoError(t, err)
	assert.True(t, called, "the OnFinished set via option must run")

	stg.SetOnFinished(nil)
	assert.Nil(t, stg.GetOnFinished())
}

// Edge case: the progress-percent guard for a bare stage without processors
// (unreachable via New, which validates gt=0).
func TestStage_setProgressPercent_zeroProcessorsGuard(t *testing.T) {
	s := &Stage[int, int]{
		Progress:        metrics.NewInt("bare-stage-progress"),
		ProgressPercent: metrics.NewString("bare-stage-percent"),
	}

	assert.NotPanics(t, func() {
		s.SetProgressPercent()
	})

	assert.Equal(t, "0%", s.GetProgressPercent().Value())
}
