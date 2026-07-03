package customapm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/etler/v3/internal/logging"
	"github.com/thalesfsp/etler/v3/internal/metrics"
	"github.com/thalesfsp/status"
	"go.elastic.co/apm"
)

// Happy path: Trace creates a transaction when none is in the context,
// increments the metric, and returns a usable span.
func TestTrace_createsTransactionAndCountsMetric(t *testing.T) {
	l := logging.Get().New("apm-test")
	metric := metrics.NewInt("etler.test.apm.running")

	ctx, span := Trace(context.Background(), "test", "unit", status.Runnning, l, metric)
	require.NotNil(t, span)
	require.NotNil(t, ctx)

	span.End()

	assert.Equal(t, int64(1), metric.Value())
}

// Edge case: an existing transaction in the context is reused; nil logger and
// nil metric are tolerated.
func TestTrace_reusesTransaction_nilLoggerAndMetric(t *testing.T) {
	tx := apm.DefaultTracer.StartTransaction("existing", "test")
	defer tx.End()

	ctx := apm.ContextWithTransaction(context.Background(), tx)

	tracedCtx, span := Trace(ctx, "test", "unit-reuse", status.Runnning, nil, nil)
	require.NotNil(t, span)

	span.End()

	assert.Same(t, tx, apm.TransactionFromContext(tracedCtx),
		"an existing transaction must be reused")
}

// TXFromCtx: both branches.
func TestTXFromCtx(t *testing.T) {
	created := TXFromCtx(context.Background(), "fresh", "test")
	require.NotNil(t, created)
	created.End()

	tx := apm.DefaultTracer.StartTransaction("existing-2", "test")
	defer tx.End()

	ctx := apm.ContextWithTransaction(context.Background(), tx)
	assert.Same(t, tx, TXFromCtx(ctx, "ignored", "test"))
}

// Bad path: TraceError returns the ORIGINAL (wrapped) error, increments the
// failure metric, and marks the span failed.
func TestTraceError_returnsOriginalAndCounts(t *testing.T) {
	l := logging.Get().New("apm-err-test")
	failed := metrics.NewInt("etler.test.apm.failed")

	inner := errors.New("inner-cause")
	wrapped := customerror.NewFailedToError("outer operation", customerror.WithError(inner))

	ctx, span := Trace(context.Background(), "test", "unit-err", status.Runnning, l, nil)

	got := TraceError(ctx, wrapped, l, failed)

	// NOTE: read the outcome BEFORE End() — the span data is released after.
	assert.Equal(t, string(Failure), span.Outcome)

	span.End()

	assert.Equal(t, wrapped, got, "the original error must be returned, not the unwrapped one")
	assert.Equal(t, int64(1), failed.Value())
}

// Edge case: nil metric and nil logger are tolerated.
func TestTraceError_nilMetricAndLogger(t *testing.T) {
	err := TraceError(context.Background(), errors.New("plain"), nil, nil)
	assert.EqualError(t, err, "plain")
}

// Outcome constants stringify correctly.
func TestOutcome_String(t *testing.T) {
	assert.Equal(t, "failure", Failure.String())
	assert.Equal(t, "success", Success.String())
}

// Logger satisfies apm.Logger and does not panic.
func TestNewLogger(t *testing.T) {
	l, err := NewLogger()
	require.NoError(t, err)
	require.NotNil(t, l)

	assert.NotPanics(t, func() {
		l.Errorf("error %d", 1)
		l.Debugf("debug %s", "x")
	})
}
