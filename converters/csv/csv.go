package csv

import (
	"context"
	"fmt"
	"io"

	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/etler/v2/converter"
	"github.com/thalesfsp/etler/v2/internal/customapm"
	"github.com/thalesfsp/status"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Name of the converter.
const Name = "csv"

// CSV definition.
type CSV[T any] struct {
	*converter.Converter[T] `json:"converter" validate:"required"`
}

//////
// Methods.
//////

// Run the conversion.
func (c *CSV[T]) Run(ctx context.Context, r io.Reader) (T, error) {
	//////
	// Observability: logging, metrics, and tracing.
	//////

	tracedContext, span := customapm.Trace(
		ctx,
		c.GetType(),
		c.GetName(),
		status.Runnning,
		c.Logger,
		c.CounterRunning,
	)
	defer span.End()

	// Update the status.
	c.GetStatus().Set(status.Runnning.String())

	converted, err := Convert[T](r)
	if err != nil {
		// Observability: logging, metrics.
		c.GetStatus().Set(status.Failed.String())

		return *new(T), customapm.TraceError(
			tracedContext,
			customerror.NewFailedToError(
				"convert",
				customerror.WithError(err),
				customerror.WithField(c.GetType(), c.GetName()),
			),
			c.GetLogger(),
			c.GetCounterFailed(),
		)
	}

	// Observability: logging, metrics.
	c.GetStatus().Set(status.Done.String())

	c.GetCounterDone().Add(1)

	// Run onEvent callback.
	if c.GetOnFinished() != nil {
		c.GetOnFinished()(ctx, c, r, nil)
	}

	return converted, nil
}

//////
// Factory.
//////

// New creates a new converter.
func New[T any](opts ...converter.Func[T]) (*CSV[T], error) {
	// Enforces IStorage interface implementation.
	var _ converter.IConverter[T] = (*CSV[T])(nil)

	conv, err := converter.New[T](Name, fmt.Sprintf("%s %s", Name, converter.Type))
	if err != nil {
		return nil, err
	}

	csv := &CSV[T]{
		Converter: conv,
	}

	// Apply options.
	for _, opt := range opts {
		opt(csv)
	}

	// Validation.
	if err := validation.Validate(csv); err != nil {
		return nil, err
	}

	return csv, nil
}
