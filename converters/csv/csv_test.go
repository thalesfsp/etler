package csv

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thalesfsp/etler/v2/converter"
)

// Test struct.
type Test []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestNew(t *testing.T) {
	csvContent := `id,name
	1,John
	2,Peter`

	buf := new(strings.Builder)

	csvConverter, err := New(converter.WithOnFinished(func(ctx context.Context, c converter.IConverter[Test], r io.Reader, processed []Test) {
		buf.WriteString(c.GetName() + " finished")
	}))
	assert.NoError(t, err)

	convertedData, err := csvConverter.Run(context.Background(), strings.NewReader(csvContent))
	assert.NoError(t, err)

	assert.Equal(t, "1", convertedData[0].ID)
	assert.Equal(t, "John", convertedData[0].Name)
	assert.Equal(t, "2", convertedData[1].ID)
	assert.Equal(t, "Peter", convertedData[1].Name)
	assert.Equal(t, "csv finished", buf.String())
}
