package customapm

import (
	"context"
	"expvar"

	"github.com/thalesfsp/etler/v4/internal/logging"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/fields"
	"github.com/thalesfsp/sypl/v2/level"
	"go.elastic.co/apm"
)

//////
// Vars, consts, and types.
//////

// Ender owns a traced span and, when Trace created it, its transaction.
type Ender struct {
	span        *apm.Span
	transaction *apm.Transaction
}

//////
// Methods.
//////

// End ends the span before ending the transaction owned by this ender.
func (e Ender) End() {
	e.span.End()

	if e.transaction != nil {
		e.transaction.End()
	}
}

// TXFromCtx returns the transaction in ctx or starts one with txName and
// txType when none is present. The caller must end a transaction started by
// this function.
func TXFromCtx(ctx context.Context, txName string, txType string) *apm.Transaction {
	tx, _ := txFromCtx(ctx, txName, txType)

	return tx
}

// txFromCtx returns a transaction and whether it was created by this call.
func txFromCtx(ctx context.Context, txName string, txType string) (*apm.Transaction, bool) {
	tx := apm.TransactionFromContext(ctx)
	if tx == nil {
		return apm.DefaultTracer.StartTransaction(txName, txType), true
	}

	return tx, false
}

// logAndMetrics logs and metrics.
func logAndMetrics(
	ctx context.Context,
	what, nameOf string,
	operation status.Status,
	l sypl.ISypl,
	metric *expvar.Int,
) {
	//////
	// Metrics.
	//////

	if metric != nil {
		metric.Add(1)
	}

	//////
	// Logging
	//////

	// Correlates the transaction, span and log, and logs it.
	if logging.Admits(l, level.Debug) {
		message := what + "." + nameOf + "." + operation.String()

		l.PrintlnWithOptions(
			level.Debug,
			message,
			sypl.WithFields(logging.ToAPM(ctx, make(fields.Fields))),
		)
	}
}

// Trace starts a span in the transaction from ctx, creating a transaction when
// necessary. It returns the tracing context and an Ender that the caller must
// end. metric is incremented when non-nil.
func Trace(
	ctx context.Context,
	what, nameOf string,
	operation status.Status,
	l sypl.ISypl,
	metric *expvar.Int,
) (context.Context, Ender) {
	tx, created := txFromCtx(ctx, nameOf, what)

	ctx = apm.ContextWithTransaction(ctx, tx)

	sType := what + "." + nameOf + "." + operation.String()

	span, ctx := apm.StartSpan(
		ctx,
		nameOf+"."+operation.String(),
		sType,
	)

	logAndMetrics(ctx, what, nameOf, operation, l, metric)

	if created {
		return ctx, Ender{span: span, transaction: tx}
	}

	return ctx, Ender{span: span}
}
