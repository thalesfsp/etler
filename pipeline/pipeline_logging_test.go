package pipeline

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
)

type pipelineStatusRecordingLogger struct {
	sypl.ISypl

	mu       sync.Mutex
	pipeline *Pipeline[int, int]
	statuses []string
}

func (l *pipelineStatusRecordingLogger) PrintlnWithOptions(
	logLevel level.Level,
	message string,
	options ...sypl.OptionFunc,
) sypl.ISypl {
	if logLevel == level.Trace && message == status.Runnning.String() {
		l.mu.Lock()
		l.statuses = append(l.statuses, l.pipeline.GetStatus().Value())
		l.mu.Unlock()
	}

	l.ISypl.PrintlnWithOptions(logLevel, message, options...)

	return l
}

func (l *pipelineStatusRecordingLogger) recordedStatuses() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.statuses...)
}

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

func TestRun_nilInputPublishesRunningBeforeFailed(t *testing.T) {
	_, recorderOutput := output.Recorder(level.Trace)

	pipelineValue, err := New(
		"pipeline-nil-input-running-transition",
		"publishes running before validating input",
		false,
		newIdentityStage(t, "pipeline-nil-input-running-transition-stage"),
	)
	require.NoError(t, err)

	p := pipelineValue.(*Pipeline[int, int])
	logger := &pipelineStatusRecordingLogger{
		ISypl:    sypl.New("pipeline-nil-input-running-transition", recorderOutput),
		pipeline: p,
	}
	p.Logger = logger

	out, err := p.Run(context.Background(), nil)

	assert.Nil(t, out)
	require.Error(t, err)
	assert.Equal(
		t,
		[]string{status.Runnning.String()},
		logger.recordedStatuses(),
		"nil-input Run must publish running before it publishes failed",
	)
	assert.Equal(t, status.Failed.String(), p.GetStatus().Value())
}
