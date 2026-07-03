package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v3/internal/metrics"
	"github.com/thalesfsp/etler/v3/task"
)

// Metadata getters + WithOnFinished option.
func TestPipeline_metadataAndOnFinishedOption(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := New("meta-pipeline", "metadata test", false,
		newIdentityStage(t, "stage-meta-pipeline"),
	)
	require.NoError(t, err)

	assert.Equal(t, "meta-pipeline", p.GetName())
	assert.Equal(t, "metadata test", p.GetDescription())
	assert.Equal(t, Type, p.GetType())
	assert.NotZero(t, p.GetCreatedAt())

	m := p.GetMetrics()
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "counterRunning")

	// WithOnFinished applies the callback.
	called := false

	WithOnFinished[int, int](func(ctx context.Context, pl IPipeline[int, int], original task.Task[int, int], tasksOut []task.Task[int, int]) {
		called = true
	})(p)

	_, err = p.Run(ctx, []int{1})
	require.NoError(t, err)
	assert.True(t, called, "the OnFinished set via option must run")
}

// Regression: after a CONCURRENT run, the final progress percentage must be
// authoritative (the per-stage goroutines' updates can interleave mid-run).
func TestPipeline_concurrent_finalProgressPercentIs100(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	p, err := New("concurrent-percent-audit", "final percent check", true,
		newIdentityStage(t, "stage-percent-a"),
		newIdentityStage(t, "stage-percent-b"),
		newIdentityStage(t, "stage-percent-c"),
	)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		_, err := p.Run(ctx, []int{1, 2})
		require.NoError(t, err)

		require.Equal(t, "100%", p.GetProgressPercent().Value(),
			"the final progress percentage must always land on 100%%")
	}
}

// Edge case: the progress-percent guard for a bare pipeline without stages
// (unreachable via New, which validates gt=0).
func TestPipeline_setProgressPercent_zeroStagesGuard(t *testing.T) {
	p := &Pipeline[int, int]{
		Progress:        metrics.NewInt("bare-pipeline-progress"),
		ProgressPercent: metrics.NewString("bare-pipeline-percent"),
	}

	assert.NotPanics(t, func() {
		p.SetProgressPercent()
	})

	assert.Equal(t, "0%", p.GetProgressPercent().Value())
}
