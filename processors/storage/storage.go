package storage

import (
	"context"
	"fmt"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/dal/storage"
	"github.com/thalesfsp/params/create"
	"github.com/thalesfsp/validation"

	"github.com/thalesfsp/etler/v2/internal/shared"
	"github.com/thalesfsp/etler/v2/processor"
)

//////
// Consts, vars and types.
//////

// Name of the processor.
const Name = "storage"

// Storage definition.
type Storage[In any] struct {
	processor.IProcessor[In] `json:"processor" validate:"required"`
}

//////
// Factory.
//////

// New creates a new Storage processor.
//
// NOTE: Mininum concurrency is 1.
// NOTE: idPrefix example: "news-"
func New[In any](
	s storage.IStorage,
	concurrency int,
	idPrefix string,
	opts ...processor.Func[In],
) (*Storage[In], error) {
	// Enforces interface implementation.
	var _ processor.IProcessor[In] = (*Storage[In])(nil)

	//////
	// Allows to control the concurrency.
	//////

	concurrentloopOpts := []concurrentloop.Func{}

	if concurrency < 1 {
		concurrentloopOpts = append(concurrentloopOpts, concurrentloop.WithBatchSize(1))
	} else {
		concurrentloopOpts = append(concurrentloopOpts, concurrentloop.WithBatchSize(concurrency))
	}

	// NOTE: Because this processor doesn't change the input data, it's
	// safe to use the input as the output.
	proc, err := processor.New(
		Name,
		fmt.Sprintf("%s %s", Name, processor.Type),
		func(tracedContext context.Context, processingData []In) ([]In, error) {
			// Concurrently creates the data.
			if _, errs := concurrentloop.Map(
				tracedContext,
				processingData,
				func(ctx context.Context, in In) (In, error) {
					// ID is lost because `in` is Generic and the Run func
					// signature forces to return the same type as the input.
					if _, err := s.Create(
						tracedContext,
						idPrefix+shared.GenerateUUID(),
						"etl",
						in,
						&create.Create{},
					); err != nil {
						return *new(In), err
					}

					return *new(In), nil
				},
				concurrentloopOpts...,
			); len(errs) > 0 {
				return nil, errs
			}

			return processingData, nil
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	str := &Storage[In]{
		proc,
	}

	// Validation.
	if err := validation.Validate(str); err != nil {
		return nil, err
	}

	return str, nil
}

// Must returns a new processor or panics if an error occurs.
func Must[In any](
	s storage.IStorage,
	concurrency int,
	idPrefix string,
	opts ...processor.Func[In],
) *Storage[In] {
	c, err := New(s, concurrency, idPrefix, opts...)
	if err != nil {
		panic(err)
	}

	return c
}
