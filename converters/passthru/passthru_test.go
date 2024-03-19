package passthru

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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

	csvConverter, err := New(
		converter.WithOnFinished(func(ctx context.Context, c converter.IConverter[[]Test, []Test], originalIn []Test, convertedOut []Test) {
			buf.WriteString(c.GetName() + " finished")
		}),
	)
	assert.NoError(t, err)

	convertedData, err := csvConverter.Run(context.Background(), tests)
	assert.NoError(t, err)

	assert.Equal(t, tests, convertedData)
	assert.Equal(t, "passthru finished", buf.String())
}
