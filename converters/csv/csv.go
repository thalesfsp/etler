package csv

import (
	"context"
	"fmt"
	"reflect"

	"github.com/gocarina/gocsv"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/etler/v4/internal/csvshape"
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
	rowType := reflect.TypeFor[In]()
	if rowType.Kind() == reflect.Pointer {
		rowType = rowType.Elem()
	}
	if err := csvshape.ValidateOutput(rowType); err != nil {
		return nil, err
	}

	conv, err := converter.New(
		Name,
		fmt.Sprintf("%s %s", Name, converter.Type),
		func(tracedContext context.Context, in []In) (string, error) {
			if err := tracedContext.Err(); err != nil {
				return "", err
			}

			return callGocsv(reflect.TypeFor[[]In](), func() (string, error) {
				return gocsv.MarshalString(in)
			})
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

//////
// Helpers.
//////

func callGocsv[Result any](inType reflect.Type, call func() (Result, error)) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("gocsv panic for CSV input type %s: %v", inType, recovered)
		}
	}()

	return call()
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
