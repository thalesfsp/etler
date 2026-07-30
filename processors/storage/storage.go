package storage

import (
	"context"
	"fmt"
	"reflect"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/dal/v2/storage"
	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/params/v2/create"
	"github.com/thalesfsp/validation"
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

// IDFunc derives a storage ID from an item.
type IDFunc[In any] func(in In) string

type config[In any] struct {
	idFunc           IDFunc[In]
	idFuncSet        bool
	idFieldName      string
	processorOptions []processor.Func[In]
}

// Func allows storage processor options to be specified.
type Func[In any] func(c *config[In])

//////
// Options.
//////

// WithIDFunc sets the function used to derive a storage ID from an item.
func WithIDFunc[In any](idFunc IDFunc[In]) Func[In] {
	return func(c *config[In]) {
		c.idFunc = idFunc
		c.idFuncSet = true
	}
}

// WithIDFieldName sets the struct field read for an item's storage ID.
func WithIDFieldName[In any](idFieldName string) Func[In] {
	return func(c *config[In]) {
		c.idFieldName = idFieldName
	}
}

// WithProcessorOptions applies options to the underlying processor.
func WithProcessorOptions[In any](opts ...processor.Func[In]) Func[In] {
	return func(c *config[In]) {
		c.processorOptions = append(c.processorOptions, opts...)
	}
}

func resolveID[In any](cfg config[In], in In) (string, error) {
	if cfg.idFuncSet {
		if cfg.idFunc == nil {
			return "", customerror.NewRequiredError(
				"custom id function",
				customerror.WithMessage("custom id function is nil"),
			)
		}

		id := cfg.idFunc(in)
		if id == "" {
			return "", customerror.NewRequiredError(
				"custom id",
				customerror.WithMessage("custom id function returned an empty id"),
			)
		}

		return id, nil
	}

	if id := shared.ExtractID(in, cfg.idFieldName); id != "" {
		return id, nil
	}

	content, err := shared.Marshal(in)
	if err != nil {
		return "", customerror.NewFailedToError(
			"derive storage id from content",
			customerror.WithError(err),
		)
	}

	return shared.GenerateIDBasedOnContent(string(content)), nil
}

//////
// Factory.
//////

// New creates a new Storage processor.
//
// IDs are derived deterministically from each item, making writes idempotent
// when callers retry the same item.
//
// NOTE: Mininum concurrency is 1.
// NOTE: idPrefix example: "news-"
func New[In any](
	s storage.IStorage,
	concurrency int,
	idPrefix string,
	opts ...Func[In],
) (*Storage[In], error) {
	// Enforces interface implementation.
	var _ processor.IProcessor[In] = (*Storage[In])(nil)

	if s == nil {
		return nil, customerror.NewRequiredError("storage")
	}

	switch reflect.ValueOf(s).Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflect.ValueOf(s).IsNil() {
			return nil, customerror.NewRequiredError("storage")
		}
	}

	cfg := config[In]{}

	// Apply options before building the transform so it captures the resolved
	// storage configuration.
	for _, opt := range opts {
		if opt == nil {
			return nil, customerror.NewRequiredError("storage processor option")
		}

		opt(&cfg)
	}

	for _, opt := range cfg.processorOptions {
		if opt == nil {
			return nil, customerror.NewRequiredError("processor option")
		}
	}

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
					id, err := resolveID(cfg, in)
					if err != nil {
						return *new(In), err
					}

					// ID is lost because `in` is Generic and the Run func
					// signature forces to return the same type as the input.
					if _, err := s.Create(
						tracedContext,
						idPrefix+id,
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
		cfg.processorOptions...,
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
	opts ...Func[In],
) *Storage[In] {
	c, err := New(s, concurrency, idPrefix, opts...)
	if err != nil {
		panic(err)
	}

	return c
}
