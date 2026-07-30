package customapm

import (
	"context"
	"errors"
	"expvar"

	"github.com/thalesfsp/etler/v4/internal/logging"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/fields"
	"github.com/thalesfsp/sypl/v2/level"
	"go.elastic.co/apm"
)

// Outcome represents an APM span outcome.
type Outcome string

const (
	// Failure is the outcome when the span failed.
	Failure Outcome = "failure"

	// Success is the outcome when the span succeeded.
	Success Outcome = "success"
)

// String returns the outcome value.
func (o Outcome) String() string {
	return string(o)
}

// TraceError increments metric when non-nil, marks the current span failed,
// unwraps a single-error chain before capture and optional logging, and returns
// the original err.
func TraceError(
	ctx context.Context,
	err error,
	l sypl.ISypl,
	metric *expvar.Int,
) error {
	//////
	// Metric.
	//////

	if metric != nil {
		metric.Add(1)
	}

	//////
	// APM.
	//////

	// Get the current span from the context, if any, set the outcome.
	span := apm.SpanFromContext(ctx)
	if span != nil {
		span.Outcome = string(Failure)
	}

	originalError := err

	// Unwrap any nested errorcatalog. By default, apm.CaptureError() does not
	// automatically unwrap nested errors or extract any additional context or
	// metadata from the error.
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			break
		}

		err = unwrapped
	}

	// Tells APM that it that was an error.
	apm.CaptureError(ctx, err).Send()

	//////
	// Logging
	//////

	// Correlates the transaction, span and log, and logs it.
	if l != nil {
		l.PrintlnWithOptions(
			level.Error,
			err.Error(),
			sypl.WithFields(logging.ToAPM(ctx, make(fields.Fields))),
		)
	}

	return originalError
}
