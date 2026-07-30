package csv

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/etler/v3/loader"
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
		assert.EqualError(t, err, "missing CSV header row")
	})
}

// Edge case: a header-only CSV yields a nil slice and no error.
func TestLoad_headerOnly_returnsEmpty(t *testing.T) {
	out, err := Load[[]auditRow](strings.NewReader("name,age\n"))
	require.NoError(t, err)
	assert.Nil(t, out)
}

// Bad path: malformed CSV (unbalanced quote) must return an error.
func TestLoad_malformedCSV_returnsError(t *testing.T) {
	_, err := Load[[]auditRow](strings.NewReader("name,age\n\"broken,30\n"))
	assert.Error(t, err)
}

// Bad path: a value that cannot be unmarshalled into the target type must
// return an error.
func TestLoad_typeMismatch_returnsError(t *testing.T) {
	tests := []struct {
		name string
		load func() error
		want string
	}{
		{
			name: "field type cannot accept CSV string",
			load: func() error {
				_, err := Load[[]auditRow](strings.NewReader("name,age\nalice,not-a-number\n"))
				return err
			},
			want: "invalid use of ,string struct tag, trying to unmarshal \"not-a-number\" into int",
		},
		{
			name: "output struct cannot accept row slice",
			load: func() error {
				_, err := Load[auditRow](strings.NewReader("name,age\nalice,30\n"))
				return err
			},
			want: "cannot unmarshal array into Go value of type csv.auditRow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				err := tt.load()
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.want)
			})
		})
	}
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

type auditTextRow struct {
	Name string `json:"name"`
	Note string `json:"note"`
}

func TestLoad_quotedFields_preserveCommasAndEmbeddedNewlines(t *testing.T) {
	in := "name,note\nalice,\"comma, then\nnew line\"\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{
		{Name: "alice", Note: "comma, then\nnew line"},
	}, out)
}

func TestLoad_CRLFLineEndings_leaveNoCarriageReturns(t *testing.T) {
	in := "name,note\r\nalice,\"first\r\nsecond\"\r\nbob,plain\r\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{
		{Name: "alice", Note: "first\nsecond"},
		{Name: "bob", Note: "plain"},
	}, out)
}

func TestLoad_duplicateHeaders_laterColumnWins(t *testing.T) {
	in := "name,name,age\nfirst,last,30\n"

	out, err := Load[[]auditRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditRow{{Name: "last", Age: 30}}, out)
}

func TestLoad_raggedRow_returnsCSVFieldCountError(t *testing.T) {
	out, err := Load[[]auditRow](strings.NewReader("name,age\nalice\n"))
	assert.Nil(t, out)
	require.EqualError(t, err, "record on line 2: wrong number of fields")

	var parseErr *csv.ParseError
	require.ErrorAs(t, err, &parseErr)
	assert.ErrorIs(t, parseErr.Err, csv.ErrFieldCount)
}

func TestLoad_readerFailure_propagatesError(t *testing.T) {
	errRead := errors.New("reader failed mid-stream")
	in := io.MultiReader(
		strings.NewReader("name,age\n"),
		errorReader{err: errRead},
	)

	out, err := Load[[]auditRow](in)
	assert.Nil(t, out)
	assert.ErrorIs(t, err, errRead)
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestNew_runPropagatesCSVError(t *testing.T) {
	csvLoader, err := New[auditRow]()
	require.NoError(t, err)

	out, err := csvLoader.Run(
		context.Background(),
		strings.NewReader("name,age\nalice\n"),
	)
	assert.Nil(t, out)
	assert.EqualError(
		t,
		err,
		"failed to process. Original Error: record on line 2: wrong number of fields. Fields: loader=csv",
	)
}

func TestNew_invalidLoaderOption_returnsError(t *testing.T) {
	invalidateName := func(
		l loader.ILoader[io.Reader, []auditRow],
	) loader.ILoader[io.Reader, []auditRow] {
		l.(*loader.Loader[io.Reader, []auditRow]).Name = ""

		return l
	}

	csvLoader, err := New[auditRow](invalidateName)
	assert.Nil(t, csvLoader)
	require.Error(t, err)
	assert.ErrorContains(t, err, "Name")
}
