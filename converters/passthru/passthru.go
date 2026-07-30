package passthru

import (
	"context"
	"fmt"

	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/validation"
)

//////
// Consts, vars and types.
//////

// Name identifies the passthrough converter.
const Name = "passthru"

// Passthru returns each input value unchanged.
type Passthru[In any] struct {
	// IConverter provides the converter lifecycle and execution API.
	converter.IConverter[In, In] `json:"converter" validate:"required"`
}

//////
// Factory.
//////

// New creates a passthrough converter. Options are applied and validated by
// converter.New.
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

	// NOTE: `opts` were already applied by `converter.New` — applying them
	// again here would run each option twice.

	// Validation.
	if err := validation.Validate(passthru); err != nil {
		return nil, err
	}

	return passthru, nil
}

// Must creates a passthrough converter or panics if validation fails.
func Must[In any](
	opts ...converter.Func[In, In],
) *Passthru[In] {
	passthru, err := New(opts...)
	if err != nil {
		panic(err)
	}

	return passthru
}
