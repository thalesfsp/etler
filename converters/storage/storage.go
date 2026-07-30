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

// Name identifies the storage converter.
const Name = "storage"

// Storage converts an item into the key returned by a storage Create call.
type Storage[In any] struct {
	// IConverter provides the converter lifecycle and execution API.
	converter.IConverter[In, string] `json:"converter" validate:"required"`
}

// IDFunc derives a storage ID from an item. Returning an error or a blank ID
// aborts the write.
type IDFunc[In any] func(ctx context.Context, in In) (string, error)

type config[In any] struct {
	idFunc           IDFunc[In]
	idFuncSet        bool
	idFieldName      string
	converterOptions []converter.Func[In, string]
}

// Func configures a storage converter.
type Func[In any] func(c *config[In])

//////
// Options.
//////

// WithIDFunc sets the first and exclusive source of storage IDs. The function
// may be called concurrently and must be safe for concurrent use.
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

// New creates a storage converter. For each item it resolves the ID from
// WithIDFunc, then the configured or default ID/Id field, then a SHA-256 key
// derived from the item's JSON. The built-in field and content paths produce a
// stable key for stable input; a custom IDFunc determines its own stability.
// Whether repeated writes are idempotent depends on the storage backend's
// Create implementation.
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

// Must creates a storage converter or panics if validation fails.
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
