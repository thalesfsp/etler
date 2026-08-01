package stage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/etler/v4/task"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
)

func TestRun_logsRunningWhenTraceIsAdmitted(t *testing.T) {
	recorder, recorderOutput := output.Recorder(level.Trace)

	identity, err := processor.New(
		"stage-running-log-processor",
		"returns the input unchanged",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	stg, err := New(
		"stage-running-log",
		"records the running log",
		identityConverter(),
		identity,
	)
	require.NoError(t, err)

	stg.(*Stage[int, int]).Logger = sypl.New("stage-running-log", recorderOutput)

	_, err = stg.Run(context.Background(), task.MustNew[int, int]([]int{1}))
	require.NoError(t, err)

	var found bool
	for _, record := range recorder.Messages() {
		if record.Level == level.Trace && record.OriginalContent == status.Runnning.String()+"\n" {
			found = true
		}
	}

	assert.True(t, found, "expected stage Run to emit the trace running log")
}
