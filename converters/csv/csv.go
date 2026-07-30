package csv

import (
	"context"
	"fmt"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Name identifies the CSV converter.
const Name = "csv"

// CSV converts its input into CSV text.
type CSV[In any] struct {
	// IConverter provides the converter lifecycle and execution API.
	converter.IConverter[In, string] `json:"converter" validate:"required"`
}

//////
// Factory.
//////

// New creates a converter from []In to CSV text. Options are applied and
// validated by converter.New.
func New[In any](
	opts ...converter.Func[[]In, string],
) (*CSV[[]In], error) {
	// Enforces interface implementation.
	var _ converter.IConverter[[]In, string] = (*CSV[[]In])(nil)

	conv, err := converter.New(
		Name,
		fmt.Sprintf("%s %s", Name, converter.Type),
		func(tracedContext context.Context, in []In) (string, error) {
			if err := tracedContext.Err(); err != nil {
				return "", err
			}

			return gocsv.MarshalString(in)
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	csv := &CSV[[]In]{
		conv,
	}

	// NOTE: `opts` were already applied by `converter.New` — applying them
	// again here would run each option twice.

	// Validation.
	if err := validation.Validate(csv); err != nil {
		return nil, err
	}

	return csv, nil
}

// Must creates a CSV converter or panics if validation fails.
func Must[In any](
	opts ...converter.Func[[]In, string],
) *CSV[[]In] {
	csv, err := New(opts...)
	if err != nil {
		panic(err)
	}

	return csv
}
