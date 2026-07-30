package passthru

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v3/converter"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
)

type v3InvalidatingLogger struct {
	sypl.ISypl
	invalidate func()
}

func (l *v3InvalidatingLogger) PrintlnWithOptions(level.Level, string, ...sypl.OptionFunc) sypl.ISypl {
	l.invalidate()

	return l
}

// Must returns a working pass-through converter (happy path), including for
// zero values.
func TestMust_happyPath(t *testing.T) {
	var p *Passthru[int]

	require.NotPanics(t, func() {
		p = Must[int]()
	})

	out, err := p.Run(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, 42, out)

	zero, err := p.Run(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, 0, zero)
}

// Edge case: an empty slice is passed through without becoming nil.
func TestPassthru_run_emptySlice(t *testing.T) {
	p, err := New[[]int]()
	require.NoError(t, err)

	in := []int{}

	out, err := p.Run(context.Background(), in)
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Empty(t, out)
}

// Construction errors are returned by New rather than escaping as panics.
func TestNew_validationFailures_returnErrorNotPanic(t *testing.T) {
	tests := []struct {
		name string
		opt  converter.Func[int, int]
	}{
		{
			name: "missing required logger",
			opt: func(c converter.IConverter[int, int]) converter.IConverter[int, int] {
				c.(*converter.Converter[int, int]).Logger = nil

				return c
			},
		},
		{
			name: "nil function",
			opt: func(c converter.IConverter[int, int]) converter.IConverter[int, int] {
				c.(*converter.Converter[int, int]).Func = nil

				return c
			},
		},
		{
			name: "empty name",
			opt: func(c converter.IConverter[int, int]) converter.IConverter[int, int] {
				c.(*converter.Converter[int, int]).Name = ""

				return c
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				got *Passthru[int]
				err error
			)

			require.NotPanics(t, func() {
				got, err = New[int](tt.opt)
			})

			assert.Nil(t, got)
			assert.Error(t, err)
		})
	}
}

// Must is intentionally the panicking constructor variant.
func TestMust_validationFailure_panics(t *testing.T) {
	nilFunc := func(c converter.IConverter[int, int]) converter.IConverter[int, int] {
		c.(*converter.Converter[int, int]).Func = nil

		return c
	}

	assert.Panics(t, func() {
		Must[int](nilFunc)
	})
}

// The wrapper's validation catches state invalidated after inner validation.
func TestNew_outerValidationFailure_returnsError(t *testing.T) {
	invalidateAfterInnerValidation := func(c converter.IConverter[int, int]) converter.IConverter[int, int] {
		concrete := c.(*converter.Converter[int, int])
		concrete.Logger = &v3InvalidatingLogger{
			ISypl: concrete.Logger,
			invalidate: func() {
				concrete.Func = nil
			},
		}

		return c
	}

	got, err := New[int](invalidateAfterInnerValidation)
	assert.Nil(t, got)
	assert.Error(t, err)
}
