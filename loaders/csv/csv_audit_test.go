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
	"github.com/thalesfsp/etler/v4/loader"
)

type auditRow struct {
	Name   string
	Age    int
	Score  float64
	Active bool
}

func TestLoad_happyPath_decodesTypedFields(t *testing.T) {
	in := "Name,Age,Score,Active\nalice,30,98.5,true\nbob,0,-1.25,false\n"

	out, err := Load[[]auditRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditRow{
		{Name: "alice", Age: 30, Score: 98.5, Active: true},
		{Name: "bob", Age: 0, Score: -1.25, Active: false},
	}, out)
}

type auditTaggedRow struct {
	DisplayName string `csv:"display_name"`
	Count       int
}

func TestLoad_mapsCSVTagsAndUntaggedFieldNames(t *testing.T) {
	in := "display_name,Count\nAlice Example,3\n"

	out, err := Load[[]auditTaggedRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTaggedRow{{DisplayName: "Alice Example", Count: 3}}, out)
}

func TestLoad_emptyInput_returnsMissingHeaderError(t *testing.T) {
	assert.NotPanics(t, func() {
		out, err := Load[[]auditRow](strings.NewReader(""))
		assert.Nil(t, out)
		assert.EqualError(t, err, "missing CSV header row")
	})
}

func TestLoad_headerOnly_returnsNilSlice(t *testing.T) {
	out, err := Load[[]auditRow](strings.NewReader("Name,Age,Score,Active\n"))
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestLoad_malformedCSV_returnsError(t *testing.T) {
	out, err := Load[[]auditRow](strings.NewReader("Name,Age\n\"broken,30\n"))
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestLoad_invalidInteger_returnsGocsvConversionError(t *testing.T) {
	out, err := Load[[]auditRow](strings.NewReader("Name,Age\nalice,abc\n"))
	assert.Nil(t, out)
	require.Error(t, err)
	assert.ErrorContains(t, err, `strconv.ParseInt: parsing "abc": invalid syntax`)
}

func TestLoad_nonSliceOutput_returnsErrorWithoutPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		out, err := Load[auditRow](strings.NewReader("Name,Age\nalice,30\n"))
		assert.Zero(t, out)
		require.Error(t, err)
		assert.ErrorContains(t, err, "output type csv.auditRow, it must be a slice")
	})
}

type auditTextRow struct {
	Name string
	Note string
}

func TestLoad_UTF8BOM_mapsFirstColumn(t *testing.T) {
	in := "\ufeffName,Note\nalice,hello\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{{Name: "alice", Note: "hello"}}, out)
}

func TestLoad_UTF8BOMOnly_returnsMissingHeaderError(t *testing.T) {
	out, err := Load[[]auditTextRow](strings.NewReader("\ufeff"))
	assert.Nil(t, out)
	assert.EqualError(t, err, "missing CSV header row")
}

func TestLoad_quotedFields_preserveCommasAndEmbeddedNewlines(t *testing.T) {
	in := "Name,Note\nalice,\"comma, then\nnew line\"\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{
		{Name: "alice", Note: "comma, then\nnew line"},
	}, out)
}

func TestLoad_CRLFLineEndings_leaveNoCarriageReturns(t *testing.T) {
	in := "Name,Note\r\nalice,\"first\r\nsecond\"\r\nbob,plain\r\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{
		{Name: "alice", Note: "first\nsecond"},
		{Name: "bob", Note: "plain"},
	}, out)
}

func TestLoad_literalTab_isPreserved(t *testing.T) {
	in := "Name,Note\nalice,\"a\tb\"\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{{Name: "alice", Note: "a\tb"}}, out)
}

func TestLoad_raggedRow_returnsCSVFieldCountError(t *testing.T) {
	out, err := Load[[]auditTextRow](strings.NewReader("Name,Note\nalice\n"))
	assert.Nil(t, out)
	require.EqualError(t, err, "record on line 2: wrong number of fields")

	var parseErr *csv.ParseError
	require.ErrorAs(t, err, &parseErr)
	assert.ErrorIs(t, parseErr.Err, csv.ErrFieldCount)
}

func TestLoad_duplicateHeaders_laterColumnWins(t *testing.T) {
	in := "Name,Name\nfirst,last\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{{Name: "last"}}, out)
}

func TestLoad_unmatchedHeader_isIgnored(t *testing.T) {
	in := "Name,Ignored\nalice,value\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{{Name: "alice"}}, out)
}

func TestLoad_missingHeader_leavesStructFieldZero(t *testing.T) {
	in := "Name\nalice\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{{Name: "alice"}}, out)
}

func TestLoad_defaultFieldMapping_isCaseInsensitive(t *testing.T) {
	in := "nAmE,nOtE\nalice,hello\n"

	out, err := Load[[]auditTextRow](strings.NewReader(in))
	require.NoError(t, err)
	require.Equal(t, []auditTextRow{{Name: "alice", Note: "hello"}}, out)
}

func TestLoad_readerFailure_propagatesError(t *testing.T) {
	errRead := errors.New("reader failed mid-stream")
	in := io.MultiReader(
		strings.NewReader("Name,Note\n"),
		errorReader{err: errRead},
	)

	out, err := Load[[]auditTextRow](in)
	assert.Nil(t, out)
	assert.ErrorIs(t, err, errRead)
}

func TestLoad_readerFailureBeforeHeader_propagatesError(t *testing.T) {
	errRead := errors.New("reader failed before header")

	out, err := Load[[]auditTextRow](errorReader{err: errRead})
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
	csvLoader, err := New[auditTextRow]()
	require.NoError(t, err)

	out, err := csvLoader.Run(
		context.Background(),
		strings.NewReader("Name,Note\nalice\n"),
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
		l loader.ILoader[io.Reader, []auditTextRow],
	) loader.ILoader[io.Reader, []auditTextRow] {
		l.(*loader.Loader[io.Reader, []auditTextRow]).Name = ""

		return l
	}

	csvLoader, err := New[auditTextRow](invalidateName)
	assert.Nil(t, csvLoader)
	require.Error(t, err)
	assert.ErrorContains(t, err, "Name")
}
