package csv

import (
	"context"
	"fmt"

	"github.com/jszwec/csvutil"
	"github.com/thalesfsp/etler/adapter"
	"github.com/thalesfsp/etler/option"
)

// CSV adapter.
type CSV[C any] struct {
	*adapter.Adapter

	Content []byte `json:"data"`
}

// Read data from data source.
func (a *CSV[C]) Read(ctx context.Context, o ...option.Func) ([]C, error) {
	opt := option.New()

	for _, f := range o {
		opt = f(opt)
	}

	var (
		results []C
	)
	if err := csvutil.Unmarshal(a.Content, &results); err != nil {
		return nil, fmt.Errorf("failed to unmarshal csv: %+v", err)
	}

	return results, nil
}

// Upsert data to data source.
func (a *CSV[C]) Upsert(ctx context.Context, v []C, o ...option.Func) error {
	opt := option.New()

	for _, f := range o {
		opt = f(opt)
	}

	b, err := csvutil.Marshal(v)
	if err != nil {
		return err
	}

	// Update the `Content` field with the JSON string
	a.Content = b

	return nil
}

// New returns a new JSON adapter.
func New[C any](content []byte) (adapter.IAdapter[C], error) {
	return &CSV[C]{
		Adapter: adapter.New("csv", "csv adapter"),

		Content: content,
	}, nil
}
