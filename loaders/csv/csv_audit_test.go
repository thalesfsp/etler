package csv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auditRow struct {
	Name string `json:"name"`
	Age  int    `json:"age,string"`
}

// Bug: empty input must return an error, not panic on a missing header row.
func TestLoad_emptyInput_returnsError(t *testing.T) {
	assert.NotPanics(t, func() {
		out, err := Load[[]auditRow](strings.NewReader(""))
		assert.Nil(t, out)
		assert.Error(t, err, "empty CSV input must be an error, not a panic")
	})
}

// Edge case: a header-only CSV yields no rows and no error.
func TestLoad_headerOnly_returnsEmpty(t *testing.T) {
	out, err := Load[[]auditRow](strings.NewReader("name,age\n"))
	require.NoError(t, err)
	assert.Empty(t, out)
}

// Bad path: malformed CSV (unbalanced quote) must return an error.
func TestLoad_malformedCSV_returnsError(t *testing.T) {
	_, err := Load[[]auditRow](strings.NewReader("name,age\n\"broken,30\n"))
	assert.Error(t, err)
}

// Bad path: a value that cannot be unmarshalled into the target type must
// return an error.
func TestLoad_typeMismatch_returnsError(t *testing.T) {
	_, err := Load[[]auditRow](strings.NewReader("name,age\nalice,not-a-number\n"))
	assert.Error(t, err)
}

// Happy path: rows map to the target struct; tabs are stripped from values.
func TestLoad_happyPath_mapsRowsAndStripsTabs(t *testing.T) {
	out, err := Load[[]auditRow](strings.NewReader("name,age\na\tlice,30\nbob,0\n"))
	require.NoError(t, err)
	require.Len(t, out, 2)

	assert.Equal(t, "alice", out[0].Name, "tabs must be stripped from values")
	assert.Equal(t, 30, out[0].Age)
	assert.Equal(t, "bob", out[1].Name)
	assert.Equal(t, 0, out[1].Age)
}
