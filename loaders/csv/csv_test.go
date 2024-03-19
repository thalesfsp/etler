package csv

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thalesfsp/etler/v2/loader"
)

// Test struct.
type Test struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestNew(t *testing.T) {
	csvContent := `id,name
	1,John
	2,Peter`

	buf := new(strings.Builder)

	csvLoader, err := New(
		loader.WithOnFinished(func(ctx context.Context, c loader.ILoader[io.Reader, []Test], originalIn io.Reader, convertedOut []Test) {
			buf.WriteString(c.GetName() + " finished")
		}),
	)
	assert.NoError(t, err)

	loadedData, err := csvLoader.Run(context.Background(), strings.NewReader(csvContent))
	assert.NoError(t, err)

	assert.Equal(t, "1", loadedData[0].ID)
	assert.Equal(t, "John", loadedData[0].Name)
	assert.Equal(t, "2", loadedData[1].ID)
	assert.Equal(t, "Peter", loadedData[1].Name)
	assert.Equal(t, "csv finished", buf.String())
}
