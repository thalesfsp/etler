package console

import (
	"bufio"
	"context"
	"encoding/json"
	"os"

	"github.com/thalesfsp/etler/adapter"
	"github.com/thalesfsp/etler/option"
	"github.com/thalesfsp/validation"
)

// Console definition.
type Console[C any] struct {
	*adapter.Adapter
}

// Read from data source.
func (c *Console[C]) Read(ctx context.Context, o ...option.Func) ([]C, error) {
	// Parse and marshal the input into the provided interface
	var results []C

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		var item C

		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, err
		}

		results = append(results, item)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// Upsert write to the data source.
func (c *Console[C]) Upsert(ctx context.Context, v []C, o ...option.Func) error {
	// Marshal the slice of values into a slice of JSON objects
	items, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Write the JSON objects to the stdout
	_, err = os.Stdout.Write(items)
	return err
}

// New creates a new Console adapter.
func New[C any]() (adapter.IAdapter[C], error) {
	a, err := adapter.New(
		"Console",
		"Reads and writes data to and from the console.",
	)
	if err != nil {
		return nil, err
	}

	c := &Console[C]{a}

	if err := validation.Validate(c); err != nil {
		return nil, err
	}

	return c, nil
}
