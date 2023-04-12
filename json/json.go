package json

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thalesfsp/etler/adapter"
	"github.com/thalesfsp/etler/option"
)

// JSON adapter.
type JSON[C any] struct {
	*adapter.Adapter

	Content []byte `json:"data"`
}

// Read data from data source.
func (j *JSON[C]) Read(ctx context.Context, o ...option.Func) ([]C, error) {
	opt := option.New()

	for _, f := range o {
		opt = f(opt)
	}

	var results []C

	if err := json.Unmarshal(j.Content, &results); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return results, nil
}

// Upsert data to data source.
func (j *JSON[C]) Upsert(ctx context.Context, v []C, o ...option.Func) error {
	opt := option.New()

	for _, f := range o {
		opt = f(opt)
	}

	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Update the `Content` field with the JSON string
	j.Content = b

	return nil
}

// New returns a new JSON adapter.
func New[C any](content []byte) (adapter.IAdapter[C], error) {
	return &JSON[C]{
		Adapter: adapter.New("json", "json adapter"),

		Content: content,
	}, nil
}
