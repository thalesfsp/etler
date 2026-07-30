package logging

import (
	"context"
	"os"
	"sync"

	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/fields"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/processor"
	"github.com/thalesfsp/sypl/v2/shared"
	"github.com/thalesfsp/sypl/v2/status"
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

// Admits reports whether a log call at `lvl` can actually reach an output, so
// callers can skip building a payload that would be discarded.
//
// It mirrors sypl's own fast-gate rule (`(*Sypl).fastGated`) so gating here can
// never suppress a message sypl would have emitted:
//
//   - `SYPL_LEVEL` and `SYPL_FILTER` raise levels and filter by component at
//     runtime, so when either is set this defers to sypl and admits everything.
//   - `Fatal` always runs the full pipeline — it exits the process.
//   - Otherwise a call is admitted when any ENABLED output's max level covers
//     it. Levels are read live, so reconfiguration is observed immediately.
func Admits(l sypl.ISypl, lvl level.Level) bool {
	if l == nil || lvl == level.None {
		return false
	}

	// Both env vars can raise the effective level at runtime. sypl honours them
	// on its slow path only, so gating here would silently defeat them.
	if os.Getenv(shared.LevelEnvVar) != "" || os.Getenv(shared.FilterEnvVar) != "" {
		return true
	}

	if lvl == level.Fatal {
		return true
	}

	for _, o := range l.GetOutputs() {
		if o.GetStatus() == status.Enabled && lvl <= o.GetMaxLevel() {
			return true
		}
	}

	return false
}

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
		l := sypl.NewDefault("etler", level.Error)

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
