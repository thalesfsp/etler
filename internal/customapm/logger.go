package customapm

import (
	"fmt"

	"github.com/thalesfsp/etler/v4/internal/logging"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Logger adapts the application logger to the Elastic APM logger interface.
type Logger struct {
	s sypl.ISypl `json:"-" validate:"required"`
}

//////
// Methods.
//////

// Errorf logs a formatted APM error.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.s.PrintlnWithOptions(
		level.Error,
		fmt.Sprintf(format, args...),
		sypl.WithTags("elastic", "apm"),
	)
}

// Debugf logs a formatted APM debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.s.PrintlnWithOptions(
		level.Debug,
		fmt.Sprintf(format, args...),
		sypl.WithTags("elastic", "apm"),
	)
}

//////
// Factory.
//////

// NewLogger returns a new APM logger.
func NewLogger() (*Logger, error) {
	l := &Logger{
		s: logging.Get().New("apm").SetTags("elastic", "apm"),
	}

	if err := validation.Validate(l); err != nil {
		return nil, err
	}

	return l, nil
}
