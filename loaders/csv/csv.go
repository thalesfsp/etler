package csv

import (
	"context"
	"fmt"
	"io"

	"github.com/thalesfsp/etler/v3/loader"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Name of the loader.
const Name = "csv"

// CSV definition.
type CSV[Out any] struct {
	loader.ILoader[io.Reader, Out] `json:"loader" validate:"required"`
}

//////
// Factory.
//////

// New creates a new loader.
func New[Out any](
	opts ...loader.Func[io.Reader, []Out],
) (*CSV[[]Out], error) {
	// Enforces interface implementation.
	var _ loader.ILoader[io.Reader, []Out] = (*CSV[[]Out])(nil)

	conv, err := loader.New(
		Name,
		fmt.Sprintf("%s %s", Name, loader.Type),
		func(ctx context.Context, in io.Reader) ([]Out, error) {
			out, err := Load[[]Out](in)
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

	// NOTE: `opts` were already applied by `loader.New` — applying them
	// again here would run each option twice.

	// Validation.
	if err := validation.Validate(csv); err != nil {
		return nil, err
	}

	return csv, nil
}
