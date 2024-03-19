package csv

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
	csvContent := `ID,Name
1,John
2,Peter
`

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

	csvConverter, err := New[Test](
		converter.WithOnFinished(func(ctx context.Context, c converter.IConverter[[]Test, string], originalIn []Test, convertedOut string) {
			buf.WriteString(c.GetName() + " finished")
		}),
	)
	assert.NoError(t, err)

	convertedData, err := csvConverter.Run(context.Background(), tests)
	assert.NoError(t, err)

	assert.Equal(t, csvContent, convertedData)
	assert.Equal(t, "csv finished", buf.String())
}
