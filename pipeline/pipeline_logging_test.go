package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
)

func TestRun_logsRunningWhenTraceIsAdmitted(t *testing.T) {
	recorder, recorderOutput := output.Recorder(level.Trace)

	p, err := New(
		"pipeline-running-log",
		"records the running log",
		false,
		newIdentityStage(t, "pipeline-running-log-stage"),
	)
	require.NoError(t, err)

	p.(*Pipeline[int, int]).Logger = sypl.New("pipeline-running-log", recorderOutput)

	_, err = p.Run(context.Background(), []int{1})
	require.NoError(t, err)

	var found bool
	for _, record := range recorder.Messages() {
		if record.Level == level.Trace && record.OriginalContent == status.Runnning.String()+"\n" {
			found = true
		}
	}

	assert.True(t, found, "expected pipeline Run to emit the trace running log")
}
