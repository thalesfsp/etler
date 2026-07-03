package csv

import (
	"context"
	"fmt"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/etler/v3/converter"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Name of the converter.
const Name = "csv"

// CSV definition.
type CSV[In any] struct {
	converter.IConverter[In, string] `json:"converter" validate:"required"`
}

//////
// Factory.
//////

// New creates a new converter.
func New[In any](
	opts ...converter.Func[[]In, string],
) (*CSV[[]In], error) {
	// Enforces interface implementation.
	var _ converter.IConverter[[]In, string] = (*CSV[[]In])(nil)

	conv, err := converter.New(
		Name,
		fmt.Sprintf("%s %s", Name, converter.Type),
		func(tracedContext context.Context, in []In) (string, error) {
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

// Must returns a new converter or panics if an error occurs.
func Must[In any](
	opts ...converter.Func[[]In, string],
) *CSV[[]In] {
	csv, err := New(opts...)
	if err != nil {
		panic(err)
	}

	return csv
}
