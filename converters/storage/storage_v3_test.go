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

type v3Doc struct {
	Name string `json:"name"`
}

// stubStorage implements only what the converter uses (Create). A second DAL
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
