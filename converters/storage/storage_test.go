package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/dal/memory"
	"github.com/thalesfsp/params/retrieve"

	"github.com/thalesfsp/etler/v2/converter"
)

// Test struct.
type Test struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestNew(t *testing.T) {
	tests := []Test{
		{
			ID:   "1",
			Name: "John",
		},
		{
			ID:   "2",
			Name: "Peter",
		},
	}

	buf := new(strings.Builder)

	memoryStorage, err := memory.New(context.Background())
	assert.NoError(t, err)

	csvConverter, err := New(
		memoryStorage,
		"test",
		converter.WithOnFinished(func(ctx context.Context, c converter.IConverter[[]Test, string], originalIn []Test, convertedOut string) {
			buf.WriteString(c.GetName() + " finished")
		}),
	)
	assert.NoError(t, err)

	id, err := csvConverter.Run(context.Background(), tests)
	assert.NoError(t, err)

	loadedData := []Test{}

	assert.NoError(t, memoryStorage.Retrieve(context.Background(), id, "test", &loadedData, &retrieve.Retrieve{}))

	assert.Equal(t, tests, loadedData)
	assert.Equal(t, "storage finished", buf.String())
}
