package loader

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/fields"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
)

type loaderAdmissionTrackingLogger struct {
	sypl.ISypl

	getOutputsCalls atomic.Int64
}

func (l *loaderAdmissionTrackingLogger) GetOutputs() []output.IOutput {
	l.getOutputsCalls.Add(1)

	return l.ISypl.GetOutputs()
}

func newLoggingTestLoader(t *testing.T, name string, logger sypl.ISypl) ILoader[int, int] {
	t.Helper()

	l, err := New(
		name,
		"exercises loader lifecycle logging",
		func(_ context.Context, in int) (int, error) {
			return in, nil
		},
		func(l ILoader[int, int]) ILoader[int, int] {
			l.(*Loader[int, int]).Logger = logger

			return l
		},
	)
	require.NoError(t, err)

	return l
}

func findLoaderDoneRecord(records []output.Record) *output.Record {
	for i := range records {
		if records[i].Level == level.Debug && records[i].OriginalContent == status.Done.String() {
			return &records[i]
		}
	}

	return nil
}

func TestLoader_debugAdmissionPreservesDoneMessageAndFields(t *testing.T) {
	t.Setenv("SYPL_LEVEL", "")
	t.Setenv("SYPL_FILTER", "")

	recorder, recorderOutput := output.Recorder(level.Debug)
	l := newLoggingTestLoader(
		t,
		"loader-debug-admission",
		sypl.New("loader-debug-admission", recorderOutput),
	)
	recorder.Reset()

	_, err := l.Run(context.Background(), 7)
	require.NoError(t, err)

	record := findLoaderDoneRecord(recorder.Messages())
	require.NotNil(t, record, "loader completion debug message was not captured")
	assert.Equal(t, fields.Fields{
		"createdAt":      l.GetCreatedAt().String(),
		"counterCreated": l.GetCounterCreated().String(),
		"counterDone":    l.GetCounterDone().String(),
		"counterFailed":  l.GetCounterFailed().String(),
		"counterRunning": l.GetCounterRunning().String(),
		"duration":       l.GetDuration().String(),
		"status":         l.GetStatus().String(),
	}, record.Fields)
}

func TestLoader_defaultLevelRejectsDebugAfterConsultingAdmissionGate(t *testing.T) {
	t.Setenv("SYPL_LEVEL", "")
	t.Setenv("SYPL_FILTER", "")

	recorder, recorderOutput := output.Recorder(level.Error)
	logger := &loaderAdmissionTrackingLogger{
		ISypl: sypl.New("loader-default-admission", recorderOutput),
	}
	l := newLoggingTestLoader(t, "loader-default-admission", logger)

	_, err := l.Run(context.Background(), 7)
	require.NoError(t, err)

	assert.Empty(t, recorder.Messages(), "default error level must discard loader debug messages")
	assert.Equal(
		t,
		int64(4),
		logger.getOutputsCalls.Load(),
		"expected admission checks for created, APM running, loader running, and loader done payloads",
	)
}

func TestLoader_traceEnvironmentOverrideStillEmitsDoneMessage(t *testing.T) {
	t.Setenv("SYPL_LEVEL", "trace")
	t.Setenv("SYPL_FILTER", "")

	recorder, recorderOutput := output.Recorder(level.Error)
	l := newLoggingTestLoader(
		t,
		"loader-trace-environment",
		sypl.New("loader-trace-environment", recorderOutput),
	)
	recorder.Reset()

	_, err := l.Run(context.Background(), 7)
	require.NoError(t, err)

	require.NotNil(
		t,
		findLoaderDoneRecord(recorder.Messages()),
		"SYPL_LEVEL=trace did not admit loader completion debug message",
	)
}
