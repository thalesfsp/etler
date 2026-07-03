package storage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/dal/storage"
	"github.com/thalesfsp/params/create"
)

type v3Item struct {
	Name string `json:"name"`
}

// stubStorage implements only what the processor uses (Create). A second DAL
// memory storage per test binary would panic on expvar re-registration, so a
// stub is used instead.
type stubStorage struct {
	storage.IStorage

	created  atomic.Int64
	failWith error
}

func (s *stubStorage) Create(ctx context.Context, id, target string, v any, prm *create.Create, options ...storage.Func[*create.Create]) (string, error) {
	if s.failWith != nil {
		return "", s.failWith
	}

	s.created.Add(1)

	return id, nil
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
