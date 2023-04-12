package etler

import (
	"context"
	"encoding/json"
	"io"

	"github.com/thalesfsp/etler/option"
)

// JSONAdapter is an adapter for reading and writing data in JSON format.
type JSONAdapter[C any] struct {
	Reader io.Reader
	Writer io.Writer
}

// Read reads data from a JSON document.
func (a *JSONAdapter[C]) Read(ctx context.Context, o ...option.Func) ([]C, error) {
	opt := option.New()

	// Apply the options.
	for _, f := range o {
		opt = f(opt)
	}

	var data []C
	err := json.NewDecoder(a.Reader).Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Upsert writes data to a JSON document.
func (a *JSONAdapter[C]) Upsert(ctx context.Context, v []C, o ...option.Func) error {
	opt := option.New()

	// Apply the options.
	for _, f := range o {
		opt = f(opt)
	}

	err := json.NewEncoder(a.Writer).Encode(v)
	if err != nil {
		return err
	}

	return nil
}
