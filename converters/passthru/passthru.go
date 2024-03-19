package passthru

import (
	"context"
	"fmt"

	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/converter"
)

//////
// Consts, vars and types.
//////

// Name of the converter.
const Name = "passthru"

// Passthru definition.
type Passthru[In any] struct {
	converter.IConverter[In, In] `json:"converter" validate:"required"`
}

//////
// Factory.
//////

// New creates a new converter.
func New[In any](
	opts ...converter.Func[In, In],
) (*Passthru[In], error) {
	// Enforces interface implementation.
	var _ converter.IConverter[In, In] = (*Passthru[In])(nil)

	conv, err := converter.New(
		Name,
		fmt.Sprintf("%s %s", Name, converter.Type),
		func(ctx context.Context, tu In) (In, error) {
			return tu, nil
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	passthru := &Passthru[In]{
		conv,
	}

	// Apply options.
	for _, opt := range opts {
		opt(passthru)
	}

	// Validation.
	if err := validation.Validate(passthru); err != nil {
		return nil, err
	}

	return passthru, nil
}

// Must returns a new converter or panics if an error occurs.
func Must[In any](
	opts ...converter.Func[In, In],
) *Passthru[In] {
	passthru, err := New(opts...)
	if err != nil {
		panic(err)
	}

	return passthru
}
