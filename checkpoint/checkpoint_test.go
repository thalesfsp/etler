package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stores returns one fresh instance of every Store in this package.
func stores(t *testing.T) map[string]Store {
	t.Helper()

	file, err := NewFile(filepath.Join(t.TempDir(), "checkpoints.json"))
	require.NoError(t, err)

	return map[string]Store{"memory": NewMemory(), "file": file}
}

func TestStore_contract(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			empty, err := store.Load(ctx, "unknown")
			require.NoError(t, err)
			require.NotNil(t, empty)
			assert.Empty(t, empty)

			at := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
			failed := Record{Key: "7", Status: StatusFailed, Error: "timeout", Attempts: 1, UpdatedAt: at}

			require.NoError(t, store.Save(ctx, "run-1", Record{Key: "6", Status: StatusDone, Attempts: 1, UpdatedAt: at}))
			require.NoError(t, store.Save(ctx, "run-1", failed))
			require.NoError(t, store.Save(ctx, "run-2", Record{Key: "6", Status: StatusFailed, Attempts: 3, UpdatedAt: at}))

			records, err := store.Load(ctx, "run-1")
			require.NoError(t, err)
			assert.Equal(t, map[string]Record{
				"6": {Key: "6", Status: StatusDone, Attempts: 1, UpdatedAt: at},
				"7": failed,
			}, records)

			// Save replaces the record for the same key.
			done := Record{Key: "7", Status: StatusDone, Attempts: 2, UpdatedAt: at}
			require.NoError(t, store.Save(ctx, "run-1", done))

			records, err = store.Load(ctx, "run-1")
			require.NoError(t, err)
			assert.Equal(t, done, records["7"])
			assert.Len(t, records, 2)

			// Runs are isolated.
			other, err := store.Load(ctx, "run-2")
			require.NoError(t, err)
			assert.Equal(t, 3, other["6"].Attempts)

			// The returned map is a copy.
			records["8"] = Record{Key: "8"}
			delete(records, "6")

			again, err := store.Load(ctx, "run-1")
			require.NoError(t, err)
			assert.Len(t, again, 2)
			assert.Contains(t, again, "6")
		})
	}
}

func TestStore_concurrentSaves(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			var wg sync.WaitGroup

			for i := range 20 {
				wg.Go(func() {
					key := string(rune('a' + i))
					assert.NoError(t, store.Save(ctx, "run", Record{Key: key, Status: StatusDone}))
				})
			}

			wg.Wait()

			records, err := store.Load(ctx, "run")
			require.NoError(t, err)
			assert.Len(t, records, 20, "no save may be lost")
		})
	}
}

func TestFile_persistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoints.json")
	ctx := context.Background()

	first, err := NewFile(path)
	require.NoError(t, err)
	require.NoError(t, first.Save(ctx, "run", Record{Key: "0", Status: StatusDone, Attempts: 1}))

	second, err := NewFile(path)
	require.NoError(t, err)

	records, err := second.Load(ctx, "run")
	require.NoError(t, err)
	assert.Equal(t, StatusDone, records["0"].Status)

	_, err = os.Stat(path + ".tmp")
	assert.ErrorIs(t, err, os.ErrNotExist, "the temporary file is renamed away")
}

func TestNewFile_requiresPath(t *testing.T) {
	store, err := NewFile("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
	assert.Nil(t, store)
}

func TestFile_errors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	t.Run("corrupt file", func(t *testing.T) {
		path := filepath.Join(dir, "corrupt.json")
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

		store, err := NewFile(path)
		require.NoError(t, err)

		_, err = store.Load(ctx, "run")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode checkpoint file")

		err = store.Save(ctx, "run", Record{Key: "0"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode checkpoint file")

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "{not json", string(data), "a failed save leaves the file untouched")
	})

	t.Run("unreadable path", func(t *testing.T) {
		store, err := NewFile(dir) // a directory
		require.NoError(t, err)

		_, err = store.Load(ctx, "run")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read checkpoint file")
	})

	t.Run("unwritable directory", func(t *testing.T) {
		store, err := NewFile(filepath.Join(dir, "missing", "checkpoints.json"))
		require.NoError(t, err)

		err = store.Save(ctx, "run", Record{Key: "0"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write checkpoint file")
	})

	t.Run("failed temp write keeps the previous file", func(t *testing.T) {
		path := filepath.Join(dir, "atomic.json")

		store, err := NewFile(path)
		require.NoError(t, err)
		require.NoError(t, store.Save(ctx, "run", Record{Key: "0", Status: StatusDone}))

		before, err := os.ReadFile(path)
		require.NoError(t, err)

		// The temporary file cannot be created, so a direct write to path is
		// the only way the save could succeed.
		require.NoError(t, os.Mkdir(path+".tmp", 0o700))

		err = store.Save(ctx, "run", Record{Key: "1", Status: StatusDone})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write checkpoint file")

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("unencodable record keeps the previous file", func(t *testing.T) {
		path := filepath.Join(dir, "encode.json")

		store, err := NewFile(path)
		require.NoError(t, err)
		require.NoError(t, store.Save(ctx, "run", Record{Key: "0", Status: StatusDone}))

		before, err := os.ReadFile(path)
		require.NoError(t, err)

		err = store.Save(ctx, "run", Record{Key: "1", UpdatedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "encode checkpoint file")

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("invalid UTF-8", func(t *testing.T) {
		path := filepath.Join(dir, "utf8.json")

		store, err := NewFile(path)
		require.NoError(t, err)

		for _, tc := range []struct{ run, key string }{{"run-\xff", "0"}, {"run", "\xff"}} {
			err := store.Save(ctx, tc.run, Record{Key: tc.key, Status: StatusDone})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "UTF-8")
		}

		_, err = os.Stat(path)
		assert.ErrorIs(t, err, os.ErrNotExist, "nothing is written")
	})

	t.Run("null file", func(t *testing.T) {
		path := filepath.Join(dir, "null.json")
		require.NoError(t, os.WriteFile(path, []byte("null"), 0o600))

		store, err := NewFile(path)
		require.NoError(t, err)

		require.NoError(t, store.Save(ctx, "run", Record{Key: "0", Status: StatusDone}))

		records, err := store.Load(ctx, "run")
		require.NoError(t, err)
		assert.Len(t, records, 1)
	})
}
