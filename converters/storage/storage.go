package storage

import (
	"context"
	"fmt"

	"github.com/thalesfsp/dal/storage"
	"github.com/thalesfsp/params/create"
	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/converter"
	"github.com/thalesfsp/etler/v2/internal/shared"
)

//////
// Consts, vars and types.
//////

// Name of the converter.
const Name = "storage"

// Storage definition.
type Storage[In any] struct {
	converter.IConverter[In, string] `json:"converter" validate:"required"`
}

//////
// Factory.
//////

// New creates a new Storage converter.
func New[In any](
	s storage.IStorage,
	target string,
	opts ...converter.Func[In, string],
) (*Storage[In], error) {
	// Enforces interface implementation.
	var _ converter.IConverter[In, string] = (*Storage[In])(nil)

	conv, err := converter.New(
		Name,
		fmt.Sprintf("%s %s", Name, converter.Type),
		func(tracedContext context.Context, in In) (string, error) {
			return s.Create(tracedContext, shared.GenerateUUID(), target, in, &create.Create{})
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	csv := &Storage[In]{
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
