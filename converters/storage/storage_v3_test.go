package storage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/dal/v2/storage"
	"github.com/thalesfsp/etler/v4/converter"
	"github.com/thalesfsp/params/v2/create"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
)

type v3Doc struct {
	Name string `json:"name"`
}

// stubStorage implements only what the converter uses (Create). A second DAL
// memory storage per test binary would panic on expvar re-registration, so a
// stub is used instead.
type stubStorage struct {
	storage.IStorage

	mu       sync.Mutex
	ids      []string
	created  atomic.Int64
	failWith error
}

type v3InvalidatingLogger struct {
	sypl.ISypl
	invalidate func()
}

func (l *v3InvalidatingLogger) PrintlnWithOptions(level.Level, string, ...sypl.OptionFunc) sypl.ISypl {
	l.invalidate()

	return l
}

func (s *stubStorage) Create(ctx context.Context, id, target string, v any, prm *create.Create, options ...storage.Func[*create.Create]) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	s.mu.Lock()
	s.ids = append(s.ids, id)
	s.mu.Unlock()

	if s.failWith != nil {
		return "", s.failWith
	}

	s.created.Add(1)

	return id, nil
}

func (s *stubStorage) createdIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.ids...)
}

// Missing storage is rejected during construction, including an interface
// containing a typed nil pointer.
func TestNew_nilStorage(t *testing.T) {
	tests := []struct {
		name    string
		storage storage.IStorage
	}{
		{name: "nil interface"},
		{name: "typed nil", storage: (*stubStorage)(nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var constructorErr error

			require.NotPanics(t, func() {
				s, err := New[v3Doc](test.storage, "etl")
				constructorErr = err

				// Exercise the pre-fix construction result to prove that an
				// accepted nil storage would panic on first use.
				if err == nil {
					_, _ = s.Run(context.Background(), v3Doc{Name: "alice"})
				}
			})
			require.Error(t, constructorErr)
		})
	}
}

// Must returns a working storage converter (happy path): converting a value
// persists it and yields its ID.
func TestMust_happyPath(t *testing.T) {
	stub := &stubStorage{}

	var s *Storage[v3Doc]

	require.NotPanics(t, func() {
		s = Must[v3Doc](stub, "etl")
	})

	id, err := s.Run(context.Background(), v3Doc{Name: "alice"})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Equal(t, int64(1), stub.created.Load())
}

// Bad path: a failing storage backend surfaces the cause.
func TestStorage_run_createFails(t *testing.T) {
	stub := &stubStorage{failWith: errors.New("boom-storage-create")}

	s, err := New[v3Doc](stub, "etl")
	require.NoError(t, err)

	_, err = s.Run(context.Background(), v3Doc{Name: "too-late"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-storage-create")
}

// Edge case: the zero value is still persisted and receives an ID.
func TestStorage_run_zeroValue(t *testing.T) {
	stub := &stubStorage{}

	s, err := New[v3Doc](stub, "etl")
	require.NoError(t, err)

	id, err := s.Run(context.Background(), v3Doc{})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Equal(t, int64(1), stub.created.Load())
}

// Bad path: cancellation is observed by the underlying storage operation.
func TestStorage_run_cancelledContext(t *testing.T) {
	stub := &stubStorage{}

	s, err := New[v3Doc](stub, "etl")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	id, err := s.Run(ctx, v3Doc{Name: "never-created"})
	assert.Empty(t, id)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int64(0), stub.created.Load())
}

// Construction errors are returned by New rather than escaping as panics.
func TestNew_validationFailures_returnErrorNotPanic(t *testing.T) {
	tests := []struct {
		name string
		opt  converter.Func[v3Doc, string]
	}{
		{
			name: "missing required logger",
			opt: func(c converter.IConverter[v3Doc, string]) converter.IConverter[v3Doc, string] {
				c.(*converter.Converter[v3Doc, string]).Logger = nil

				return c
			},
		},
		{
			name: "nil function",
			opt: func(c converter.IConverter[v3Doc, string]) converter.IConverter[v3Doc, string] {
				c.(*converter.Converter[v3Doc, string]).Func = nil

				return c
			},
		},
		{
			name: "empty name",
			opt: func(c converter.IConverter[v3Doc, string]) converter.IConverter[v3Doc, string] {
				c.(*converter.Converter[v3Doc, string]).Name = ""

				return c
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				got *Storage[v3Doc]
				err error
			)

			require.NotPanics(t, func() {
				got, err = New[v3Doc](
					&stubStorage{},
					"etl",
					WithConverterOptions(tt.opt),
				)
			})

			assert.Nil(t, got)
			assert.Error(t, err)
		})
	}
}

// Must is intentionally the panicking constructor variant.
func TestMust_validationFailure_panics(t *testing.T) {
	nilFunc := func(c converter.IConverter[v3Doc, string]) converter.IConverter[v3Doc, string] {
		c.(*converter.Converter[v3Doc, string]).Func = nil

		return c
	}

	assert.Panics(t, func() {
		Must[v3Doc](
			&stubStorage{},
			"etl",
			WithConverterOptions(nilFunc),
		)
	})
}

// The wrapper's validation catches state invalidated after inner validation.
func TestNew_outerValidationFailure_returnsError(t *testing.T) {
	invalidateAfterInnerValidation := func(c converter.IConverter[v3Doc, string]) converter.IConverter[v3Doc, string] {
		concrete := c.(*converter.Converter[v3Doc, string])
		concrete.Logger = &v3InvalidatingLogger{
			ISypl: concrete.Logger,
			invalidate: func() {
				concrete.Func = nil
			},
		}

		return c
	}

	got, err := New[v3Doc](
		&stubStorage{},
		"etl",
		WithConverterOptions(invalidateAfterInnerValidation),
	)
	assert.Nil(t, got)
	assert.Error(t, err)
}
