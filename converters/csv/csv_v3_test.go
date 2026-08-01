package csv

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
)

type v3Row struct {
	Name string `csv:"name"`
	Age  int    `csv:"age"`
}

type v3UnmarshallableValue struct{}

func (v3UnmarshallableValue) MarshalCSV() (string, error) {
	return "", errors.New("boom-csv-marshal")
}

type v3UnmarshallableRow struct {
	Value v3UnmarshallableValue `csv:"value"`
}

type v3InvalidatingLogger struct {
	sypl.ISypl
	invalidate func()
}

func (l *v3InvalidatingLogger) PrintlnWithOptions(level.Level, string, ...sypl.OptionFunc) sypl.ISypl {
	l.invalidate()

	return l
}

// Must returns a working converter (happy path).
func TestMust_happyPath(t *testing.T) {
	var c *CSV[[]v3Row]

	require.NotPanics(t, func() {
		c = Must[v3Row]()
	})

	out, err := c.Run(context.Background(), []v3Row{{Name: "alice", Age: 30}, {Name: "bob", Age: 0}})
	require.NoError(t, err)

	assert.Contains(t, out, "name,age")
	assert.Contains(t, out, "alice,30")
	assert.Contains(t, out, "bob,0")
}

// Edge case: an empty slice yields just the header row, no error.
func TestCSV_run_emptySlice(t *testing.T) {
	c, err := New[v3Row]()
	require.NoError(t, err)

	out, err := c.Run(context.Background(), []v3Row{})
	require.NoError(t, err)
	assert.Equal(t, "name,age\n", out)
}

// Bad path: a field marshalling error is surfaced by the converter.
func TestCSV_run_unmarshallableValue(t *testing.T) {
	c, err := New[v3UnmarshallableRow]()
	require.NoError(t, err)

	out, err := c.Run(context.Background(), []v3UnmarshallableRow{{}})
	assert.Empty(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-csv-marshal")
}

// Construction errors are returned by New rather than escaping as panics.
func TestNew_validationFailures_returnErrorNotPanic(t *testing.T) {
	tests := []struct {
		name string
		opt  converter.Func[[]v3Row, string]
	}{
		{
			name: "missing required logger",
			opt: func(c converter.IConverter[[]v3Row, string]) converter.IConverter[[]v3Row, string] {
				c.(*converter.Converter[[]v3Row, string]).Logger = nil

				return c
			},
		},
		{
			name: "nil function",
			opt: func(c converter.IConverter[[]v3Row, string]) converter.IConverter[[]v3Row, string] {
				c.(*converter.Converter[[]v3Row, string]).Func = nil

				return c
			},
		},
		{
			name: "empty name",
			opt: func(c converter.IConverter[[]v3Row, string]) converter.IConverter[[]v3Row, string] {
				c.(*converter.Converter[[]v3Row, string]).Name = ""

				return c
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				got *CSV[[]v3Row]
				err error
			)

			require.NotPanics(t, func() {
				got, err = New[v3Row](tt.opt)
			})

			assert.Nil(t, got)
			assert.Error(t, err)
		})
	}
}

// Must is intentionally the panicking constructor variant.
func TestMust_validationFailure_panics(t *testing.T) {
	nilFunc := func(c converter.IConverter[[]v3Row, string]) converter.IConverter[[]v3Row, string] {
		c.(*converter.Converter[[]v3Row, string]).Func = nil

		return c
	}

	assert.Panics(t, func() {
		Must[v3Row](nilFunc)
	})
}

// The wrapper's validation catches state invalidated after inner validation.
func TestNew_outerValidationFailure_returnsError(t *testing.T) {
	invalidateAfterInnerValidation := func(c converter.IConverter[[]v3Row, string]) converter.IConverter[[]v3Row, string] {
		concrete := c.(*converter.Converter[[]v3Row, string])
		concrete.Logger = &v3InvalidatingLogger{
			ISypl: concrete.Logger,
			invalidate: func() {
				concrete.Func = nil
			},
		}

		return c
	}

	got, err := New[v3Row](invalidateAfterInnerValidation)
	assert.Nil(t, got)
	assert.Error(t, err)
}

// A context that is already done prevents conversion and does not count the
// run as completed.
func TestCSV_run_contextAlreadyDone(t *testing.T) {
	tests := []struct {
		name       string
		newContext func() (context.Context, context.CancelFunc)
		target     error
	}{
		{
			name: "cancelled",
			newContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx, cancel
			},
			target: context.Canceled,
		},
		{
			name: "deadline exceeded",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			target: context.DeadlineExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := test.newContext()
			defer cancel()

			c, err := New[v3Row]()
			require.NoError(t, err)

			_, err = c.Run(ctx, []v3Row{{Name: "alice", Age: 30}})
			assert.Error(t, err)
			assert.True(t, errors.Is(err, test.target))
			assert.Equal(t, int64(0), c.GetCounterDone().Value())
		})
	}
}
