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
	"github.com/thalesfsp/etler/v4/processor"
	"github.com/thalesfsp/params/v2/create"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
)

type v3Item struct {
	Name string `json:"name"`
}

// stubStorage implements only what the processor uses (Create). A second DAL
// memory storage per test binary would panic on expvar re-registration, so a
// stub is used instead.
type stubStorage struct {
	storage.IStorage

	mu       sync.Mutex
	ids      []string
	targets  []string
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
	s.targets = append(s.targets, target)
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

func (s *stubStorage) createdTargets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.targets...)
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
				s, err := New[v3Item](test.storage, 1, "v3-test-")
				constructorErr = err

				// Exercise the pre-fix construction result to prove that an
				// accepted nil storage would panic on first use.
				if err == nil {
					_, _ = s.Run(context.Background(), []v3Item{{Name: "a"}})
				}
			})
			require.Error(t, constructorErr)
		})
	}
}

// Must + concurrency < 1 (clamped to 1): the processor persists all items and
// passes the data through unchanged.
func TestMust_clampedConcurrency_happyPath(t *testing.T) {
	stub := &stubStorage{}

	var s *Storage[v3Item]

	require.NotPanics(t, func() {
		s = Must[v3Item](stub, 0, "v3-test-")
	})

	in := []v3Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	out, err := s.Run(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, in, out, "the storage processor must pass data through unchanged")
	assert.Equal(t, int64(len(in)), stub.created.Load(), "every item must be persisted")
}

// Bad path: a failing storage backend fails the processor with the cause
// preserved.
func TestStorage_run_createFails(t *testing.T) {
	stub := &stubStorage{failWith: errors.New("boom-processor-create")}

	s, err := New[v3Item](stub, 2, "v3-fail-")
	require.NoError(t, err)

	out, err := s.Run(context.Background(), []v3Item{{Name: "late"}})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-processor-create")
}

// Edge case: no input means no storage calls and an unchanged empty slice.
func TestStorage_run_emptyInput(t *testing.T) {
	stub := &stubStorage{}

	s, err := New[v3Item](stub, 1, "v3-empty-")
	require.NoError(t, err)

	in := []v3Item{}

	out, err := s.Run(context.Background(), in)
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Empty(t, out)
	assert.Equal(t, int64(0), stub.created.Load())
}

// Bad path: cancellation is observed by the concurrent storage operation.
func TestStorage_run_cancelledContext(t *testing.T) {
	stub := &stubStorage{}

	s, err := New[v3Item](stub, 1, "v3-cancelled-")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out, err := s.Run(ctx, []v3Item{{Name: "never-created"}})
	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), context.Canceled.Error())
	assert.Equal(t, int64(0), stub.created.Load())
}

// Construction errors are returned by New rather than escaping as panics.
func TestNew_validationFailures_returnErrorNotPanic(t *testing.T) {
	tests := []struct {
		name string
		opt  processor.Func[v3Item]
	}{
		{
			name: "missing required logger",
			opt: func(p processor.IProcessor[v3Item]) processor.IProcessor[v3Item] {
				p.(*processor.Processor[v3Item]).Logger = nil

				return p
			},
		},
		{
			name: "nil function",
			opt: func(p processor.IProcessor[v3Item]) processor.IProcessor[v3Item] {
				p.(*processor.Processor[v3Item]).Func = nil

				return p
			},
		},
		{
			name: "empty name",
			opt: func(p processor.IProcessor[v3Item]) processor.IProcessor[v3Item] {
				p.(*processor.Processor[v3Item]).Name = ""

				return p
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				got *Storage[v3Item]
				err error
			)

			require.NotPanics(t, func() {
				got, err = New[v3Item](
					&stubStorage{},
					1,
					"v3-invalid-",
					WithProcessorOptions(tt.opt),
				)
			})

			assert.Nil(t, got)
			assert.Error(t, err)
		})
	}
}

// Must is intentionally the panicking constructor variant.
func TestMust_validationFailure_panics(t *testing.T) {
	nilFunc := func(p processor.IProcessor[v3Item]) processor.IProcessor[v3Item] {
		p.(*processor.Processor[v3Item]).Func = nil

		return p
	}

	assert.Panics(t, func() {
		Must[v3Item](
			&stubStorage{},
			1,
			"v3-invalid-",
			WithProcessorOptions(nilFunc),
		)
	})
}

// The wrapper's validation catches state invalidated after inner validation.
func TestNew_outerValidationFailure_returnsError(t *testing.T) {
	invalidateAfterInnerValidation := func(p processor.IProcessor[v3Item]) processor.IProcessor[v3Item] {
		concrete := p.(*processor.Processor[v3Item])
		concrete.Logger = &v3InvalidatingLogger{
			ISypl: concrete.Logger,
			invalidate: func() {
				concrete.Func = nil
			},
		}

		return p
	}

	got, err := New[v3Item](
		&stubStorage{},
		1,
		"v3-invalid-",
		WithProcessorOptions(invalidateAfterInnerValidation),
	)
	assert.Nil(t, got)
	assert.Error(t, err)
}
