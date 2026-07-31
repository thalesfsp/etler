package csv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cyclicConverterNode struct {
	Name string
	Next *cyclicConverterNode
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
