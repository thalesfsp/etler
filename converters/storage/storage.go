package storage

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/thalesfsp/customerror/v2"
	"github.com/thalesfsp/dal/v2/storage"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/etler/v4/internal/shared"
	"github.com/thalesfsp/params/v2/create"
	"github.com/thalesfsp/validation"
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

// IDFunc derives a storage ID from an item.
type IDFunc[In any] func(ctx context.Context, in In) (string, error)

type config[In any] struct {
	idFunc           IDFunc[In]
	idFuncSet        bool
	idFieldName      string
	converterOptions []converter.Func[In, string]
}

// Func allows storage converter options to be specified.
type Func[In any] func(c *config[In])

//////
// Options.
//////

// WithIDFunc sets the function used to derive a storage ID from an item. The
// function may be called concurrently and must be safe for concurrent use.
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

// WithConverterOptions applies options to the underlying converter.
func WithConverterOptions[In any](opts ...converter.Func[In, string]) Func[In] {
	return func(c *config[In]) {
		c.converterOptions = append(c.converterOptions, opts...)
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

// New creates a new Storage converter. IDs are derived deterministically from
// each item, making writes idempotent when callers retry the same item.
func New[In any](
	s storage.IStorage,
	target string,
	opts ...Func[In],
) (*Storage[In], error) {
	// Enforces interface implementation.
	var _ converter.IConverter[In, string] = (*Storage[In])(nil)

	if s == nil {
		return nil, customerror.NewRequiredError("storage")
	}

	switch reflect.ValueOf(s).Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflect.ValueOf(s).IsNil() {
			return nil, customerror.NewRequiredError("storage")
		}
	}

	if strings.TrimSpace(target) == "" {
		return nil, customerror.NewRequiredError(
			"storage target",
			customerror.WithMessage("storage target must not be empty"),
		)
	}

	cfg := config[In]{}

	// Apply options before building the transform so it captures the resolved
	// storage configuration.
	for _, opt := range opts {
		if opt == nil {
			return nil, customerror.NewRequiredError("storage converter option")
		}

		opt(&cfg)
	}

	if cfg.idFuncSet && cfg.idFunc == nil {
		return nil, customerror.NewRequiredError(
			"custom id function",
			customerror.WithMessage("custom id function is nil"),
		)
	}

	for _, opt := range cfg.converterOptions {
		if opt == nil {
			return nil, customerror.NewRequiredError("converter option")
		}
	}

	conv, err := converter.New(
		Name,
		fmt.Sprintf("%s %s", Name, converter.Type),
		func(tracedContext context.Context, in In) (string, error) {
			id, err := resolveID(tracedContext, cfg, in)
			if err != nil {
				return "", err
			}

			return s.Create(tracedContext, id, target, in, &create.Create{})
		},
		cfg.converterOptions...,
	)
	if err != nil {
		return nil, err
	}

	csv := &Storage[In]{
		conv,
	}

	// Validation.
	if err := validation.Validate(csv); err != nil {
		return nil, err
	}

	return csv, nil
}

// Must returns a new converter or panics if an error occurs.
func Must[In any](
	s storage.IStorage,
	target string,
	opts ...Func[In],
) *Storage[In] {
	c, err := New(s, target, opts...)
	if err != nil {
		panic(err)
	}

	return c
}
