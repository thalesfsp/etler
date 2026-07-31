package csv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	loadercsv "github.com/thalesfsp/etler/v4/loaders/csv"
)

type cyclicConverterNode struct {
	Name string
	Next *cyclicConverterNode
}

type ArrOverCount struct {
	ID   string
	Vals [3]string `csv:"vals" csv[]:"4"`
}

type PtrArrCount struct {
	ID   string
	Vals *[3]string `csv:"vals" csv[]:"2"`
}

type nonNumericCount struct {
	ID   string
	Vals []string `csv:"vals" csv[]:"many"`
}

type zeroCount struct {
	ID   string
	Vals []string `csv:"vals" csv[]:"0"`
}

type validDirectCount struct {
	ID   string
	Vals [3]string `csv:"vals" csv[]:"2"`
}

func converterValidationError[Row any]() error {
	_, err := New[Row]()

	return err
}

func loaderValidationError[Row any](input string) error {
	_, err := loadercsv.Load[[]Row](strings.NewReader(input))

	return err
}

func TestNew_rejectsArrayExpansionCountBeyondLength(t *testing.T) {
	csvConverter, err := New[ArrOverCount]()

	require.Nil(t, csvConverter)
	require.ErrorContains(t, err, "count 4 exceeds array length 3")
}

func TestNew_rejectsPointerWrappedArrayExpansion(t *testing.T) {
	csvConverter, err := New[PtrArrCount]()

	require.Nil(t, csvConverter)
	require.ErrorContains(t, err, "pointer-wrapped slice or array fields are ignored by gocsv")
}

func TestNew_rejectsInvalidExpansionCounts(t *testing.T) {
	t.Run("non-numeric", func(t *testing.T) {
		csvConverter, err := New[nonNumericCount]()

		require.Nil(t, csvConverter)
		require.ErrorContains(t, err, `count "many" is non-numeric or non-positive`)
	})

	t.Run("zero", func(t *testing.T) {
		csvConverter, err := New[zeroCount]()

		require.Nil(t, csvConverter)
		require.ErrorContains(t, err, `count "0" is non-numeric or non-positive`)
	})
}

func TestCSV_run_keepsValidDirectArrayExpansion(t *testing.T) {
	csvConverter, err := New[validDirectCount]()
	require.NoError(t, err)

	out, err := csvConverter.Run(context.Background(), []validDirectCount{{ID: "r1", Vals: [3]string{"a", "b", "c"}}})
	require.NoError(t, err)
	require.Equal(t, "ID,vals[0],vals[1]\nr1,a,b\n", out)
}

func TestConverterAndLoader_agreeOnCSVExpansionValidation(t *testing.T) {
	tests := []struct {
		name           string
		converter      func() error
		loader         func() error
		errorSubstring string
	}{
		{
			name:      "array count exceeds length",
			converter: converterValidationError[ArrOverCount],
			loader: func() error {
				return loaderValidationError[ArrOverCount]("ID,vals[0],vals[1],vals[2],vals[3]\nr1,a,b,c,\n")
			},
			errorSubstring: "count 4 exceeds array length 3",
		},
		{
			name:      "pointer-wrapped array",
			converter: converterValidationError[PtrArrCount],
			loader: func() error {
				return loaderValidationError[PtrArrCount]("ID,vals[0],vals[1]\nr1,a,b\n")
			},
			errorSubstring: "pointer-wrapped slice or array fields are ignored by gocsv",
		},
		{
			name:      "non-numeric count",
			converter: converterValidationError[nonNumericCount],
			loader: func() error {
				return loaderValidationError[nonNumericCount]("ID\nr1\n")
			},
			errorSubstring: `count "many" is non-numeric or non-positive`,
		},
		{
			name:      "zero count",
			converter: converterValidationError[zeroCount],
			loader: func() error {
				return loaderValidationError[zeroCount]("ID\nr1\n")
			},
			errorSubstring: `count "0" is non-numeric or non-positive`,
		},
		{
			name:      "valid direct array",
			converter: converterValidationError[validDirectCount],
			loader: func() error {
				return loaderValidationError[validDirectCount]("ID,vals[0],vals[1]\nr1,a,b\n")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converterErr := tt.converter()
			loaderErr := tt.loader()
			if tt.errorSubstring == "" {
				require.NoError(t, converterErr)
				require.NoError(t, loaderErr)

				return
			}

			require.ErrorContains(t, converterErr, tt.errorSubstring)
			require.ErrorContains(t, loaderErr, tt.errorSubstring)
		})
	}
}

func TestNew_rejectsCyclicRowBeforeMarshal(t *testing.T) {
	const helperEnvironment = "ETLER_CSV_CONVERTER_CYCLE_HELPER"
	if inputKind := os.Getenv(helperEnvironment); inputKind != "" {
		input := []cyclicConverterNode{{Name: "root"}}
		if inputKind == "empty" {
			input = []cyclicConverterNode{}
		}

		csvConverter, err := New[cyclicConverterNode]()
		if err != nil {
			fmt.Printf("input=%s len=%d err=%v\n", inputKind, len(input), err)

			return
		}

		_, _ = csvConverter.Run(context.Background(), input)

		return
	}

	for _, inputKind := range []string{"non-empty", "empty"} {
		t.Run(inputKind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNew_rejectsCyclicRowBeforeMarshal$")
			command.Env = append(os.Environ(), helperEnvironment+"="+inputKind, "GOTRACEBACK=none")
			output, err := command.CombinedOutput()

			require.NoError(t, err, "cycle helper did not exit successfully (context error %v):\n%s", ctx.Err(), output)
			require.NoError(t, ctx.Err(), "cycle helper exceeded its timeout:\n%s", output)
			require.Contains(t, string(output), "input="+inputKind+" len=")
			require.Contains(t, string(output), "invalid cyclic CSV output type csv.cyclicConverterNode")
		})
	}
}

func TestNew_rejectsPointerCyclicRow(t *testing.T) {
	csvConverter, err := New[*cyclicConverterNode]()

	require.Nil(t, csvConverter)
	require.ErrorContains(t, err, "invalid cyclic CSV output type csv.cyclicConverterNode")
}

func TestCSV_run_marshalsNonCyclicRows(t *testing.T) {
	type row struct {
		Name string `csv:"name"`
		Age  int    `csv:"age"`
	}

	csvConverter, err := New[row]()
	require.NoError(t, err)

	out, err := csvConverter.Run(context.Background(), []row{{Name: "Ada", Age: 36}})
	require.NoError(t, err)
	require.Equal(t, "name,age\nAda,36\n", out)
}

func TestCSV_run_keepsCSVSliceExpansion(t *testing.T) {
	type row struct {
		Name   string `csv:"name"`
		Values []int  `csv:"value" csv[]:"2"`
	}

	csvConverter, err := New[row]()
	require.NoError(t, err)

	out, err := csvConverter.Run(context.Background(), []row{{Name: "Ada", Values: []int{7, 8}}})
	require.NoError(t, err)
	require.Equal(t, "name,value[0],value[1]\nAda,7,8\n", out)
}

type panicCSVValue struct{}

func (panicCSVValue) MarshalCSV() (string, error) {
	panic("caller MarshalCSV panic")
}

func TestCSV_run_convertsMarshalCSVPanicToError(t *testing.T) {
	type row struct {
		Value panicCSVValue `csv:"value"`
	}

	csvConverter, err := New[row]()
	require.NoError(t, err)

	var (
		out    string
		runErr error
	)
	require.NotPanics(t, func() {
		out, runErr = csvConverter.Run(context.Background(), []row{{}})
	})
	require.Empty(t, out)
	require.ErrorContains(t, runErr, "gocsv panic for CSV input type []csv.row")
	require.ErrorContains(t, runErr, "caller MarshalCSV panic")
}
