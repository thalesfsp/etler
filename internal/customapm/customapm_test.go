package customapm

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/etler/v3/internal/logging"
	"github.com/thalesfsp/etler/v3/internal/metrics"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
	"go.elastic.co/apm"
	"go.elastic.co/apm/apmtest"
	"go.elastic.co/apm/transport"
)

func setDefaultTracer(t *testing.T, active bool) {
	t.Helper()

	t.Setenv("ELASTIC_APM_ACTIVE", strconv.FormatBool(active))
	t.Setenv("ELASTIC_APM_CENTRAL_CONFIG", "false")
	t.Setenv("ELASTIC_APM_METRICS_INTERVAL", "0s")
	t.Setenv("ELASTIC_APM_RECORDING", "true")

	tracer, err := apm.NewTracerOptions(apm.TracerOptions{
		ServiceName: "customapm-test",
		Transport:   transport.Discard,
	})
	require.NoError(t, err)
	require.Equal(t, active, tracer.Active())

	original := apm.DefaultTracer
	apm.DefaultTracer = tracer

	t.Cleanup(func() {
		apm.DefaultTracer = original
		tracer.Close()
	})
}

func setRecordingDefaultTracer(t *testing.T) *apmtest.RecordingTracer {
	t.Helper()

	t.Setenv("ELASTIC_APM_ACTIVE", "true")
	t.Setenv("ELASTIC_APM_CENTRAL_CONFIG", "false")
	t.Setenv("ELASTIC_APM_METRICS_INTERVAL", "0s")
	t.Setenv("ELASTIC_APM_RECORDING", "true")

	recorder := apmtest.NewRecordingTracer()

	original := apm.DefaultTracer
	apm.DefaultTracer = recorder.Tracer

	t.Cleanup(func() {
		apm.DefaultTracer = original
		recorder.Close()
	})

	return recorder
}

// Trace ends a transaction it creates and reports it with the expected name
// and type.
func TestTrace_endsCreatedTransaction(t *testing.T) {
	recorder := setRecordingDefaultTracer(t)

	_, ender := Trace(
		context.Background(),
		"converter",
		"normalize",
		status.Runnning,
		nil,
		nil,
	)
	ender.End()
	recorder.Flush(nil)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	assert.Equal(t, "normalize", payloads.Transactions[0].Name)
	assert.Equal(t, "converter", payloads.Transactions[0].Type)
}

// Trace leaves an inherited transaction open for its caller to end.
func TestTrace_doesNotEndInheritedTransaction(t *testing.T) {
	recorder := setRecordingDefaultTracer(t)
	tx := recorder.StartTransaction("caller-owned", "request")
	ctx := apm.ContextWithTransaction(context.Background(), tx)

	_, ender := Trace(ctx, "converter", "normalize", status.Runnning, nil, nil)
	ender.End()

	require.NotNil(t, tx.TransactionData, "the caller-owned transaction must remain open")

	recorder.Flush(nil)
	assert.Empty(t, recorder.Payloads().Transactions)

	tx.End()
	recorder.Flush(nil)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	assert.Equal(t, "caller-owned", payloads.Transactions[0].Name)
	assert.Equal(t, "request", payloads.Transactions[0].Type)
}

// Trace reports its span as a child of the transaction in the traced context.
func TestTrace_reportsSpanAsTransactionChild(t *testing.T) {
	recorder := setRecordingDefaultTracer(t)

	_, ender := Trace(
		context.Background(),
		"converter",
		"normalize",
		status.Runnning,
		nil,
		nil,
	)
	ender.End()
	recorder.Flush(nil)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Len(t, payloads.Spans, 1)
	assert.Equal(t, payloads.Transactions[0].ID, payloads.Spans[0].TransactionID)
	assert.Equal(t, payloads.Transactions[0].ID, payloads.Spans[0].ParentID)
	assert.Equal(t, payloads.Transactions[0].TraceID, payloads.Spans[0].TraceID)
}

// Trace returns a context carrying the exact span name and type components.
func TestTrace_spanNameAndType(t *testing.T) {
	tests := []struct {
		name         string
		what         string
		nameOf       string
		operation    status.Status
		wantName     string
		wantSpanType string
	}{
		{
			name:         "running converter",
			what:         "converter",
			nameOf:       "normalize",
			operation:    status.Runnning,
			wantName:     "normalize.running",
			wantSpanType: "converter.normalize.running",
		},
		{
			name:         "done processor",
			what:         "processor",
			nameOf:       "sanitize",
			operation:    status.Done,
			wantName:     "sanitize.done",
			wantSpanType: "processor.sanitize.done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaultTracer(t, true)

			ctx, ender := Trace(
				context.Background(),
				tt.what,
				tt.nameOf,
				tt.operation,
				nil,
				nil,
			)
			span := ender.span
			require.NotNil(t, ctx)
			require.NotNil(t, span)
			require.NotNil(t, apm.TransactionFromContext(ctx))
			assert.Same(t, span, apm.SpanFromContext(ctx))
			assert.Equal(t, tt.wantName, span.Name)
			assert.Equal(
				t,
				tt.wantSpanType,
				span.Type+"."+span.Subtype+"."+span.Action,
			)

			ender.End()
		})
	}
}

// Trace logs when admitted, but only attaches APM correlation fields for an
// active tracer.
func TestTrace_logCorrelationTracksTracerActivity(t *testing.T) {
	tests := []struct {
		name          string
		tracerActive  bool
		wantAPMFields bool
	}{
		{
			name:          "inactive tracer logs without correlation",
			tracerActive:  false,
			wantAPMFields: false,
		},
		{
			name:          "active tracer logs with correlation",
			tracerActive:  true,
			wantAPMFields: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaultTracer(t, tt.tracerActive)

			recorder, recorderOutput := output.Recorder(level.Trace)
			l := sypl.New("customapm-test", recorderOutput)

			ctx, ender := Trace(
				context.Background(),
				"converter",
				"normalize",
				status.Runnning,
				l,
				nil,
			)
			span := ender.span
			require.NotNil(t, ctx)
			require.NotNil(t, span)

			records := recorder.Messages()
			require.Len(t, records, 1)
			assert.Equal(t, level.Debug, records[0].Level)
			assert.Equal(t, "converter.normalize.running\n", records[0].OriginalContent)

			if tt.wantAPMFields {
				assert.Len(t, records[0].Fields, 3)
				assert.NotEmpty(t, records[0].Fields["trace.id"])
				assert.NotEmpty(t, records[0].Fields["transaction.id"])
				assert.NotEmpty(t, records[0].Fields["span.id"])
			} else {
				assert.Empty(t, records[0].Fields)
			}

			ender.End()
		})
	}
}

// Happy path: Trace creates a transaction when none is in the context,
// increments the metric, and returns a usable span.
func TestTrace_createsTransactionAndCountsMetric(t *testing.T) {
	l := logging.Get().New("apm-test")
	metric := metrics.NewInt("etler.test.apm.running")

	ctx, ender := Trace(context.Background(), "test", "unit", status.Runnning, l, metric)
	require.NotNil(t, ender.span)
	require.NotNil(t, ctx)

	ender.End()

	assert.Equal(t, int64(1), metric.Value())
}

// Edge case: an existing transaction in the context is reused; nil logger and
// nil metric are tolerated.
func TestTrace_reusesTransaction_nilLoggerAndMetric(t *testing.T) {
	tx := apm.DefaultTracer.StartTransaction("existing", "test")
	defer tx.End()

	ctx := apm.ContextWithTransaction(context.Background(), tx)

	tracedCtx, ender := Trace(ctx, "test", "unit-reuse", status.Runnning, nil, nil)
	require.NotNil(t, ender.span)

	ender.End()

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

	ctx, ender := Trace(context.Background(), "test", "unit-err", status.Runnning, l, nil)

	got := TraceError(ctx, wrapped, l, failed)

	// NOTE: read the outcome BEFORE End() — the span data is released after.
	assert.Equal(t, string(Failure), ender.span.Outcome)

	ender.End()

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
