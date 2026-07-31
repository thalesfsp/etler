package csv

import (
	"context"
	"encoding"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/require"
)

type textUnmarshalerCycle struct {
	Name string
	Next *textUnmarshalerCycle
}

func (*textUnmarshalerCycle) UnmarshalText([]byte) error {
	return nil
}

func TestLoad_rejectsCycleHiddenByTextUnmarshaller(t *testing.T) {
	const helperEnvironment = "ETLER_CSV_TEXT_UNMARSHALER_CYCLE_HELPER"
	if os.Getenv(helperEnvironment) == "1" {
		out, err := Load[[]textUnmarshalerCycle](strings.NewReader("Name\na\n"))
		fmt.Printf("rows=%d err=%v\n", len(out), err)
		if err == nil {
			os.Exit(2)
		}

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLoad_rejectsCycleHiddenByTextUnmarshaller$")
	command.Env = append(os.Environ(), helperEnvironment+"=1", "GOTRACEBACK=none")
	output, err := command.CombinedOutput()

	require.NoError(t, err, "cycle helper did not exit successfully (context error %v):\n%s", ctx.Err(), output)
	require.NoError(t, ctx.Err(), "cycle helper exceeded its timeout:\n%s", output)
	require.Contains(t, string(output), "err=invalid cyclic CSV output type csv.textUnmarshalerCycle")
}

type textMarshalerCycle struct {
	Name string
	Next *textMarshalerCycle
}

func (textMarshalerCycle) MarshalText() ([]byte, error) {
	return nil, nil
}

func TestLoad_acceptsCycleLeafWithTextMarshaler(t *testing.T) {
	out, err := Load[[]textMarshalerCycle](strings.NewReader("Name\na\n"))

	require.NoError(t, err)
	require.Equal(t, []textMarshalerCycle{{Name: "a"}}, out)
}

type fieldsUnmarshallerCycle struct {
	Name string
	Next *fieldsUnmarshallerCycle
}

func (*fieldsUnmarshallerCycle) UnmarshalCSVWithFields(_, _ string) error {
	return nil
}

func TestLoad_acceptsCycleLeafWithFieldsUnmarshaller(t *testing.T) {
	out, err := Load[[]fieldsUnmarshallerCycle](strings.NewReader("Name\na\n"))

	require.NoError(t, err)
	require.Equal(t, []fieldsUnmarshallerCycle{{Name: "a"}}, out)
}

func TestLoad_acceptsCycleBehindTwoPointers(t *testing.T) {
	type DD struct {
		Name string
		Next **DD
	}

	out, err := Load[[]DD](strings.NewReader("Name\na\n"))

	require.NoError(t, err)
	require.Equal(t, []DD{{Name: "a"}}, out)
}

func TestLoad_acceptsSkippedNonAnonymousCycle(t *testing.T) {
	type Skip struct {
		Name string
		Next *Skip `csv:"-"`
	}

	out, err := Load[[]Skip](strings.NewReader("Name\na\n"))

	require.NoError(t, err)
	require.Equal(t, []Skip{{Name: "a"}}, out)
}

func TestLoad_acceptsPointerWrappedSliceCycle(t *testing.T) {
	type SA struct {
		Name  string
		Items *[]SA
	}

	out, err := Load[[]SA](strings.NewReader("Name\na\n"))

	require.NoError(t, err)
	require.Equal(t, []SA{{Name: "a"}}, out)
}

type expansionInsideTextUnmarshaller struct {
	Items []struct {
		ID int
	} `csv:"Items" csv[]:"x"`
	Tail string
}

func (*expansionInsideTextUnmarshaller) UnmarshalText([]byte) error {
	return nil
}

func TestLoad_rejectsCSVSliceExpansionInsideTextUnmarshaller(t *testing.T) {
	type OuterRow struct {
		Name string
		In   expansionInsideTextUnmarshaller
	}

	const data = "Name,In.Items[0].ID,In.Items[1].ID,In.Tail\nalice,7,8,z\n"
	out, err := Load[[]OuterRow](strings.NewReader(data))

	require.Nil(t, out)
	require.ErrorContains(t, err, "invalid CSV output type csv.expansionInsideTextUnmarshaller uses unsupported csv[] slice expansion on field Items")
}

func TestLoad_acceptsCSVSliceExpansionOnSkippedField(t *testing.T) {
	type Row struct {
		Name  string
		Items []int `csv:"-" csv[]:"1"`
	}

	out, err := Load[[]Row](strings.NewReader("Name\na\n"))

	require.NoError(t, err)
	require.Equal(t, []Row{{Name: "a"}}, out)
}

func TestLoad_rejectsDirectSliceCycle(t *testing.T) {
	type DA struct {
		Name  string
		Items []DA
	}

	out, err := Load[[]DA](strings.NewReader("Name\na\n"))

	require.Nil(t, out)
	require.ErrorContains(t, err, "invalid cyclic CSV output type csv.DA")
}

type valueCSVMarshaler struct{}

func (valueCSVMarshaler) MarshalCSV() (string, error) {
	return "", nil
}

type pointerCSVMarshaler struct{}

func (*pointerCSVMarshaler) MarshalCSV() (string, error) {
	return "", nil
}

type valueTextMarshaler struct{}

func (valueTextMarshaler) MarshalText() ([]byte, error) {
	return nil, nil
}

type pointerTextMarshaler struct{}

func (*pointerTextMarshaler) MarshalText() ([]byte, error) {
	return nil, nil
}

type valueCSVUnmarshaller struct{}

func (valueCSVUnmarshaller) UnmarshalCSV(string) error {
	return nil
}

type pointerCSVUnmarshaller struct{}

func (*pointerCSVUnmarshaller) UnmarshalCSV(string) error {
	return nil
}

type valueFieldsUnmarshaller struct{}

func (valueFieldsUnmarshaller) UnmarshalCSVWithFields(_, _ string) error {
	return nil
}

type pointerFieldsUnmarshaller struct{}

func (*pointerFieldsUnmarshaller) UnmarshalCSVWithFields(_, _ string) error {
	return nil
}

type textUnmarshallerOnly struct{}

func (*textUnmarshallerOnly) UnmarshalText([]byte) error {
	return nil
}

func TestGocsvCanMarshal_matchesGocsvInterfaces(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{name: "value gocsv TypeMarshaller", typ: reflect.TypeFor[valueCSVMarshaler](), want: true},
		{name: "pointer gocsv TypeMarshaller", typ: reflect.TypeFor[pointerCSVMarshaler](), want: true},
		{name: "value encoding TextMarshaler", typ: reflect.TypeFor[valueTextMarshaler](), want: true},
		{name: "pointer encoding TextMarshaler", typ: reflect.TypeFor[pointerTextMarshaler](), want: true},
		{name: "value gocsv TypeUnmarshaller", typ: reflect.TypeFor[valueCSVUnmarshaller](), want: true},
		{name: "pointer gocsv TypeUnmarshaller", typ: reflect.TypeFor[pointerCSVUnmarshaller](), want: true},
		{name: "value gocsv TypeUnmarshalCSVWithFields", typ: reflect.TypeFor[valueFieldsUnmarshaller](), want: true},
		{name: "pointer gocsv TypeUnmarshalCSVWithFields", typ: reflect.TypeFor[pointerFieldsUnmarshaller](), want: true},
		{name: "encoding TextUnmarshaller only", typ: reflect.TypeFor[textUnmarshallerOnly](), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, gocsvCanMarshal(tt.typ))
		})
	}
}

var (
	_ gocsv.TypeMarshaller             = valueCSVMarshaler{}
	_ gocsv.TypeMarshaller             = (*pointerCSVMarshaler)(nil)
	_ encoding.TextMarshaler           = valueTextMarshaler{}
	_ encoding.TextMarshaler           = (*pointerTextMarshaler)(nil)
	_ gocsv.TypeUnmarshaller           = valueCSVUnmarshaller{}
	_ gocsv.TypeUnmarshaller           = (*pointerCSVUnmarshaller)(nil)
	_ gocsv.TypeUnmarshalCSVWithFields = valueFieldsUnmarshaller{}
	_ gocsv.TypeUnmarshalCSVWithFields = (*pointerFieldsUnmarshaller)(nil)
	_ encoding.TextUnmarshaler         = (*textUnmarshallerOnly)(nil)
)
