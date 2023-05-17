package logging

import (
	"context"
	"sync"

	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/fields"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/sypl/processor"
	"go.elastic.co/apm"
)

//////
// Vars, consts, and types.
//////

// Singleton.
var (
	once            sync.Once
	singletonLogger *Logger
)

// Logger is the application logger.
type Logger struct {
	*sypl.Sypl
}

//////
// Exported functionalities.
//////

// Get returns a setup logger, or set it up. Default level is `ERROR`.
//
// All messages will be directed to StdOut unless in case of ERROR level, which
// will be directed to StdErr.
//
// NOTE: Use `SYPL_LEVEL` env var to overwrite the max level.
func Get() *Logger {
	once.Do(func() {
		// Setup logger with default sane values. Default outputs: stdout, and
		// stderr.
		l := sypl.NewDefault("pubsub", level.Error)

		//////
		// Default outputs' processors.
		//////

		// Add the lower case processor to all outputs.
		for _, o := range l.GetOutputs() {
			o.AddProcessors(processor.ChangeFirstCharCase(processor.Lowercase))
		}

		//////
		// Set singleton.
		//////

		singletonLogger = &Logger{l}

		//////
		// Notify end of logger setup.
		//////

		l.Debuglnf("global logger setup, outputs: %s", l.GetOutputs())
	})

	return singletonLogger
}

// ToAPM adds the required APM fields enabling log correlation.
//
// NOTE: It expects the `apm.Transaction` to be in the context.
func ToAPM(ctx context.Context, f fields.Fields) fields.Fields {
	if f == nil {
		f = fields.Fields{}
	}

	tx := apm.TransactionFromContext(ctx)
	if tx != nil {
		traceContext := tx.TraceContext()

		f["trace.id"] = traceContext.Trace.String()
		f["transaction.id"] = traceContext.Span.String()

		if span := apm.SpanFromContext(ctx); span != nil {
			f["span.id"] = span.TraceContext().Span.String()
		}
	}

	return f
}
