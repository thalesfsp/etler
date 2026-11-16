package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"sync"

	"github.com/thalesfsp/customerror/v2"
)

// File is a Store kept in one JSON file. Every Save rewrites the file through
// a temporary file and a rename, so a process crash leaves either the old or
// the new contents, never a partial file. It does not fsync, so it is not
// durable across power loss, and it does not lock across processes: use one
// writer per file, or a database-backed Store.
type File struct {
	mu   sync.Mutex
	path string
}

// NewFile returns a store backed by path. The file is created on the first
// Save; a missing file reads as empty.
func NewFile(path string) (*File, error) {
	if path == "" {
		return nil, customerror.NewRequiredError("path")
	}

	return &File{path: path}, nil
}

// Load returns runID's records from the file.
func (f *File) Load(_ context.Context, runID string) (map[string]Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	runs, err := f.read()
	if err != nil {
		return nil, err
	}

	records := runs[runID]
	if records == nil {
		records = map[string]Record{}
	}

	return records, nil
}

// Save inserts or replaces record under runID and rewrites the file.
func (f *File) Save(_ context.Context, runID string, record Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	runs, err := f.read()
	if err != nil {
		return err
	}

	if runs[runID] == nil {
		runs[runID] = map[string]Record{}
	}

	runs[runID][record.Key] = record

	// Marshalling maps of string-keyed records with plain fields cannot fail.
	data, _ := json.MarshalIndent(runs, "", "  ")

	tmp := f.path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return customerror.NewFailedToError("write checkpoint file", customerror.WithError(err))
	}

	if err := os.Rename(tmp, f.path); err != nil {
		return customerror.NewFailedToError("replace checkpoint file", customerror.WithError(err))
	}

	return nil
}

// read decodes the whole file; a missing file is an empty store.
func (f *File) read() (map[string]map[string]Record, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]map[string]Record{}, nil
	}

	if err != nil {
		return nil, customerror.NewFailedToError("read checkpoint file", customerror.WithError(err))
	}

	var runs map[string]map[string]Record
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, customerror.NewFailedToError("decode checkpoint file", customerror.WithError(err))
	}

	// A file holding JSON null decodes to a nil map, which Save writes into.
	if runs == nil {
		runs = map[string]map[string]Record{}
	}

	return runs, nil
}
