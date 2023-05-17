package customapm

import (
	"fmt"

	"github.com/thalesfsp/etler/internal/logging"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Logger satisfies `apm.Logger` interface.
type Logger struct {
	s sypl.ISypl `json:"-" validate:"required"`
}

//////
// Methods.
//////

// Errorf implements required `Errorf` interface.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.s.PrintlnWithOptions(
		level.Error,
		fmt.Sprintf(format, args...),
		sypl.WithTags("elastic", "apm"),
	)
}

// Debugf implements required  `Debugf` interface.
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
