package converter

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

func TestRun_logsRunningWhenDebugIsAdmitted(t *testing.T) {
	recorder, recorderOutput := output.Recorder(level.Trace)

	conv, err := New(
		"converter-running-log",
		"records the running log",
		func(ctx context.Context, in int) (int, error) {
			return in, nil
		},
	)
	require.NoError(t, err)

	conv.(*Converter[int, int]).Logger = sypl.New("converter-running-log", recorderOutput)

	_, err = conv.Run(context.Background(), 1)
	require.NoError(t, err)

	var found bool
	for _, record := range recorder.Messages() {
		if record.Level == level.Debug && record.OriginalContent == status.Runnning.String()+"\n" {
			found = true
		}
	}

	assert.True(t, found, "expected converter Run to emit the debug running log")
}

func TestRun_errorLevelStillUpdatesMetricsAndStatus(t *testing.T) {
	// Hermetic: either env var makes the level gate defer to sypl, which would
	// let the debug logs through and defeat what this test pins.
	t.Setenv("SYPL_LEVEL", "")
	t.Setenv("SYPL_FILTER", "")

	recorder, recorderOutput := output.Recorder(level.Error)

	conv, err := New(
		"converter-error-metrics",
		"updates metrics with debug logging disabled",
		func(ctx context.Context, in int) (int, error) {
			return in, nil
		},
	)
	require.NoError(t, err)

	conv.(*Converter[int, int]).Logger = sypl.New("converter-error-metrics", recorderOutput)

	_, err = conv.Run(context.Background(), 1)
	require.NoError(t, err)

	assert.Equal(t, int64(1), conv.GetCounterRunning().Value())
	assert.Equal(t, int64(1), conv.GetCounterDone().Value())
	assert.Equal(t, status.Done.String(), conv.GetStatus().Value())
	assert.Empty(t, recorder.Messages())
}
