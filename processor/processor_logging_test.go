package processor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v3/internal/shared"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
)

func TestRun_logsRunningWhenTraceIsAdmitted(t *testing.T) {
	recorder, recorderOutput := output.Recorder(level.Trace)

	proc, err := New(
		"processor-running-log",
		"records the running log",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	proc.(*Processor[int]).Logger = sypl.New("processor-running-log", recorderOutput)

	_, err = proc.Run(context.Background(), []int{1})
	require.NoError(t, err)

	var found bool
	for _, record := range recorder.Messages() {
		if record.Level == level.Trace && record.OriginalContent == status.Runnning.String()+"\n" {
			found = true
		}
	}

	assert.True(t, found, "expected processor Run to emit the trace running log")
}

func TestRun_logsPauseNoticeWhenTraceIsAdmitted(t *testing.T) {
	recorder, recorderOutput := output.Recorder(level.Trace)

	proc, err := New(
		"processor-pause-log",
		"records the pause log",
		func(ctx context.Context, processingData []int) ([]int, error) {
			return processingData, nil
		},
	)
	require.NoError(t, err)

	proc.(*Processor[int]).Logger = sypl.New("processor-pause-log", recorderOutput)

	pause := shared.NewPauseController()
	pause.Pause()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = proc.Run(shared.ContextWithPause(ctx, pause), []int{1})
	require.Error(t, err)

	var found bool
	for _, record := range recorder.Messages() {
		if record.Level == level.Trace &&
			record.OriginalContent == "Processor processor-pause-log is paused. Waiting to be resumed...\n" {
			found = true
		}
	}

	assert.True(t, found, "expected processor Run to emit the trace pause notice")
}
