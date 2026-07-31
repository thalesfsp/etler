package storage

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

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

// Name identifies the storage processor.
const Name = "storage"

// defaultTarget is the storage target used when WithTarget is not specified.
const defaultTarget = "etl"

// Storage creates each input item in a storage backend and, on success, returns
// the input slice unchanged.
type Storage[In any] struct {
	// IProcessor provides the processor lifecycle and execution API.
	processor.IProcessor[In] `json:"processor" validate:"required"`
}

// IDFunc derives a storage ID from an item. Returning an error or a blank ID
// aborts that write.
type IDFunc[In any] func(ctx context.Context, in In) (string, error)

type config[In any] struct {
	idFunc           IDFunc[In]
	idFuncSet        bool
	idFieldName      string
	target           string
	processorOptions []processor.Func[In]
}

// Func configures a storage processor.
type Func[In any] func(c *config[In])

//////
// Options.
//////

// WithIDFunc sets the first and exclusive source of storage IDs. The function
// is called concurrently and must be safe for concurrent use.
func WithIDFunc[In any](idFunc IDFunc[In]) Func[In] {
	return func(c *config[In]) {
		c.idFunc = idFunc
		c.idFuncSet = true
	}
}

// WithIDFieldName sets the exported struct field read for an item's storage ID.
// String, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, and
// uint64 fields are supported after pointer and interface dereferencing;
// fmt.Stringer is not used implicitly.
func WithIDFieldName[In any](idFieldName string) Func[In] {
	return func(c *config[In]) {
		c.idFieldName = idFieldName
	}
}

// WithTarget sets the storage target used by Create. The default is "etl";
// New rejects a blank target.
func WithTarget[In any](target string) Func[In] {
	return func(c *config[In]) {
		c.target = target
	}
}

// WithProcessorOptions applies options to the underlying processor.
func WithProcessorOptions[In any](opts ...processor.Func[In]) Func[In] {
	return func(c *config[In]) {
		c.processorOptions = append(c.processorOptions, opts...)
	}
}

func resolveID[In any](ctx context.Context, cfg config[In], in In) (string, error) {
	if cfg.idFuncSet {
		id, err := cfg.idFunc(ctx, in)
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(id) == "" {
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

// New creates a storage processor. For each item it resolves the ID from
// WithIDFunc, then the configured or default ID/Id field, then a SHA-256 key
// derived from the item's JSON. It prepends idPrefix before calling Create.
//
// The field path is stable for an unchanged field. The content path is stable
// only when JSON serialization is deterministic; a custom MarshalJSON method
// can vary it. A custom IDFunc determines its own stability. Whether repeated
// writes are idempotent depends on the storage backend's Create implementation.
//
// NOTE: Concurrency values below 1 are treated as 1.
// NOTE: An idPrefix can namespace keys, for example "news-".
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

	cfg := config[In]{
		target: defaultTarget,
	}

	// Apply options before building the transform so it captures the resolved
	// storage configuration.
	for _, opt := range opts {
		if opt == nil {
			return nil, customerror.NewRequiredError("storage processor option")
		}

		opt(&cfg)
	}

	if cfg.idFuncSet && cfg.idFunc == nil {
		return nil, customerror.NewRequiredError(
			"custom id function",
			customerror.WithMessage("custom id function is nil"),
		)
	}

	if strings.TrimSpace(cfg.target) == "" {
		return nil, customerror.NewRequiredError(
			"storage target",
			customerror.WithMessage("storage target must not be empty"),
		)
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
					id, err := resolveID(ctx, cfg, in)
					if err != nil {
						return *new(In), err
					}

					// ID is lost because `in` is Generic and the Run func
					// signature forces to return the same type as the input.
					if _, err := s.Create(
						ctx,
						idPrefix+id,
						cfg.target,
						in,
						&create.Create{},
					); err != nil {
						return *new(In), err
					}

					return *new(In), nil
				},
				concurrentloopOpts...,
			); len(errs) > 0 {
				return nil, customerror.NewFailedToError(
					"map storage items",
					customerror.WithError(errors.Join(errs...)),
				)
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

// Must creates a storage processor or panics if validation fails.
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
