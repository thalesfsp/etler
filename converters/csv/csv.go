package csv

import (
	"context"
	"fmt"
	"io"

	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/converter"
)

//////
// Consts, vars and types.
//////

// Name of the converter.
const Name = "csv"

// CSV definition.
type CSV[Out any] struct {
	converter.IConverter[io.Reader, Out] `json:"converter" validate:"required"`
}

//////
// Factory.
//////

// New creates a new converter.
func New[Out any](
	opts ...converter.Func[io.Reader, []Out],
) (*CSV[[]Out], error) {
	// Enforces IStorage interface implementation.
	var _ converter.IConverter[io.Reader, []Out] = (*CSV[[]Out])(nil)

	conv, err := converter.New(
		Name,
		fmt.Sprintf("%s %s", Name, converter.Type),
		func(ctx context.Context, in io.Reader) ([]Out, error) {
			out, err := Convert[[]Out](in)
			if err != nil {
				return nil, err
			}

			return out, nil
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	csv := &CSV[[]Out]{
		conv,
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
