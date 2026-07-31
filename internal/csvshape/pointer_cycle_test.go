package csvshape_test

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	convertercsv "github.com/thalesfsp/etler/v4/converters/csv"
	"github.com/thalesfsp/etler/v4/internal/csvshape"
	loadercsv "github.com/thalesfsp/etler/v4/loaders/csv"
)

type selfPointer *selfPointer

type mutualPointerA *mutualPointerB

type mutualPointerB *mutualPointerA

type selfPointerRow struct {
	Name string      `csv:"name"`
	Ptr  selfPointer `csv:"ptr"`
}

type mutualPointerRow struct {
	Ptr mutualPointerA `csv:"ptr" csv[]:"2"`
}

type taggedSelfPointerRow struct {
	Ptr selfPointer `csv:"ptr" csv[]:"2"`
}

type doubleScalarPointerRow struct {
	Value **int `csv:"value" csv[]:"2"`
}

type slicePointerRow struct {
	Values *[]int `csv:"values" csv[]:"2"`
}

type doubleSlicePointerRow struct {
	Values **[]int `csv:"values" csv[]:"2"`
}

func TestPointerExpansionWalkTerminates(t *testing.T) {
	const helperEnvironment = "ETLER_CSVSHAPE_POINTER_CYCLE_HELPER"
	if helperCase := os.Getenv(helperEnvironment); helperCase != "" {
		runPointerExpansionHelper(t, helperCase)

		return
	}

	for _, helperCase := range []string{
		"self-pointer field",
		"mutual pointer field",
		"self-pointer expansion tag",
		"acyclic pointer controls",
		"converter construction",
	} {
		t.Run(helperCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			arguments := []string{"-test.run=^TestPointerExpansionWalkTerminates$"}
			if coverDirectory := flag.Lookup("test.gocoverdir").Value.String(); coverDirectory != "" {
				arguments = append(arguments, "-test.gocoverdir="+coverDirectory)
			}
			command := exec.CommandContext(ctx, os.Args[0], arguments...)
			command.Env = append(os.Environ(), helperEnvironment+"="+helperCase, "GOTRACEBACK=none")
			output, err := command.CombinedOutput()

			require.NoError(t, ctx.Err(), "helper %q exceeded its timeout:\n%s", helperCase, output)
			require.NoError(t, err, "helper %q failed:\n%s", helperCase, output)
		})
	}
}

func runPointerExpansionHelper(t *testing.T, helperCase string) {
	switch helperCase {
	case "self-pointer field":
		require.NoError(t, csvshape.ValidateOutput(reflect.TypeFor[selfPointerRow]()))

		rows, err := loadercsv.Load[[]selfPointerRow](strings.NewReader("name\nAda\n"))
		require.NoError(t, err)
		require.Equal(t, []selfPointerRow{{Name: "Ada"}}, rows)
	case "mutual pointer field":
		require.NoError(t, csvshape.ValidateOutput(reflect.TypeFor[mutualPointerRow]()))
	case "self-pointer expansion tag":
		require.NoError(t, csvshape.ValidateOutput(reflect.TypeFor[taggedSelfPointerRow]()))
	case "acyclic pointer controls":
		require.NoError(t, csvshape.ValidateOutput(reflect.TypeFor[doubleScalarPointerRow]()))
		require.ErrorContains(
			t,
			csvshape.ValidateOutput(reflect.TypeFor[slicePointerRow]()),
			"pointer-wrapped slice or array fields",
		)
		require.ErrorContains(
			t,
			csvshape.ValidateOutput(reflect.TypeFor[doubleSlicePointerRow]()),
			"pointer-wrapped slice or array fields",
		)
	case "converter construction":
		converter, err := convertercsv.New[selfPointerRow]()
		require.NoError(t, err)
		require.NotNil(t, converter)
	default:
		t.Fatalf("unknown pointer expansion helper case %q", helperCase)
	}
}
