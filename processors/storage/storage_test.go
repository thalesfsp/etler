package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thalesfsp/dal/memory"
	"github.com/thalesfsp/params/list"

	"github.com/thalesfsp/etler/v2/processor"
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

	memoryProcessor, err := New(
		memoryStorage,
		1,
		"test-",
		processor.WithOnFinished(
			func(ctx context.Context, c processor.IProcessor[Test], originalIn []Test, processedOut []Test) {
				buf.WriteString(c.GetName() + " finished")
			}),
	)
	assert.NoError(t, err)

	_, err = memoryProcessor.Run(context.Background(), tests)
	assert.NoError(t, err)

	// Get a list from the memory storage.
	//
	// NOTE: The usage of memory.ResponseList[Test] wrapper.
	var fromMemory memory.ResponseList[Test]
	assert.NoError(t, memoryStorage.List(context.Background(), "etl", &fromMemory, &list.List{}))

	peterName := false
	johnName := false

	for _, test := range fromMemory.Items {
		if test.Name == "Peter" {
			peterName = true
		}

		if test.Name == "John" {
			johnName = true
		}
	}

	assert.True(t, peterName)
	assert.True(t, johnName)
}
