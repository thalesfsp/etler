package csvshape

import (
	"encoding"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/require"
)

type pointerCycle struct {
	Next *pointerCycle
}

type sliceCycle struct {
	Children []sliceCycle
}

type overlongArrayExpansion struct {
	Values [1]int `csv:"values" csv[]:"2"`
}

type cacheP struct {
	Q *cacheQ
}

type cacheQ struct {
	P *cacheP
}

type soloQHolder struct {
	Q cacheQ
}

func TestValidateOutput_cyclicErrorNamesRequestedTypeInBothValidationOrders(t *testing.T) {
	const helperEnvironment = "ETLER_CSVSHAPE_CYCLE_CACHE_ORDER"
	if order := os.Getenv(helperEnvironment); order != "" {
		var firstType reflect.Type
		switch order {
		case "forward":
			firstType = reflect.TypeFor[cacheP]()
		case "reverse":
			firstType = reflect.TypeFor[cacheQ]()
		default:
			t.Fatalf("unknown validation order %q", order)
		}

		require.Error(t, ValidateOutput(firstType))
		fmt.Printf("order=%s err=%v\n", order, ValidateOutput(reflect.TypeFor[soloQHolder]()))

		return
	}

	for _, order := range []string{"forward", "reverse"} {
		t.Run(order, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestValidateOutput_cyclicErrorNamesRequestedTypeInBothValidationOrders$")
			command.Env = append(os.Environ(), helperEnvironment+"="+order)
			output, err := command.CombinedOutput()

			require.NoError(t, err, "cycle-cache helper failed:\n%s", output)
			require.Contains(
				t,
				string(output),
				"order="+order+" err=invalid cyclic CSV output type csvshape.soloQHolder",
			)
		})
	}
}

func TestValidateOutput_rejectsCycles(t *testing.T) {
	require.ErrorContains(t, ValidateOutput(reflect.TypeFor[pointerCycle]()), "invalid cyclic CSV output type csvshape.pointerCycle")
	require.ErrorContains(t, ValidateOutput(reflect.TypeFor[sliceCycle]()), "invalid cyclic CSV output type csvshape.sliceCycle")
}

func TestValidateOutput_cachesCyclicSentinel(t *testing.T) {
	rowType := reflect.TypeFor[pointerCycle]()
	key := csvOutputTypeValidationCacheKey{rowType: rowType, tagName: gocsv.TagName}
	csvOutputTypeValidationCache.Delete(key)
	defer csvOutputTypeValidationCache.Delete(key)

	require.ErrorContains(t, ValidateOutput(rowType), "invalid cyclic CSV output type csvshape.pointerCycle")
	cached, exists := csvOutputTypeValidationCache.Load(key)
	require.True(t, exists)
	require.ErrorIs(t, cached.(csvOutputTypeValidationCacheValue).err, errCyclicCSVOutputType)
	require.EqualError(t, ValidateOutput(rowType), "invalid cyclic CSV output type csvshape.pointerCycle")
}

func TestValidateOutput_handlesExpansionAndTagNameForms(t *testing.T) {
	t.Run("direct positive slice expansion", func(t *testing.T) {
		type row struct {
			Values []int `csv:"values" csv[]:"2"`
		}

		require.NoError(t, ValidateOutput(reflect.TypeFor[row]()))
	})

	t.Run("pointer-wrapped expansion", func(t *testing.T) {
		type row struct {
			Values *[]int `csv:"values" csv[]:"2"`
		}

		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[row]()), "pointer-wrapped slice or array fields")
	})

	t.Run("non-numeric expansion", func(t *testing.T) {
		type row struct {
			Values []int `csv:"values" csv[]:"many"`
		}

		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[row]()), "positive integer count")
	})

	t.Run("array count exceeds length", func(t *testing.T) {
		require.EqualError(
			t,
			ValidateOutput(reflect.TypeFor[overlongArrayExpansion]()),
			"invalid CSV output type csvshape.overlongArrayExpansion uses unsupported csv[] slice expansion on field Values: count 2 exceeds array length 1; use a count no greater than the array length",
		)
	})

	t.Run("tag name has its own cache entry", func(t *testing.T) {
		type row struct {
			Values []int `csv:"values" csv[]:"0" alternate:"values" alternate[]:"2"`
		}

		originalTagName := gocsv.TagName
		defer func() { gocsv.TagName = originalTagName }()

		gocsv.TagName = "csv"
		require.ErrorContains(t, ValidateOutput(reflect.TypeFor[row]()), "positive integer count")

		gocsv.TagName = "alternate"
		require.NoError(t, ValidateOutput(reflect.TypeFor[row]()))
	})
}

type valueCSVMarshaler struct {
	Next *valueCSVMarshaler
}

func (valueCSVMarshaler) MarshalCSV() (string, error) {
	return "", nil
}

type pointerTextMarshaler struct {
	Next *pointerTextMarshaler
}

func (*pointerTextMarshaler) MarshalText() ([]byte, error) {
	return nil, nil
}

type valueCSVUnmarshaller struct {
	Next *valueCSVUnmarshaller
}

func (valueCSVUnmarshaller) UnmarshalCSV(string) error {
	return nil
}

type pointerFieldsUnmarshaller struct {
	Next *pointerFieldsUnmarshaller
}

func (*pointerFieldsUnmarshaller) UnmarshalCSVWithFields(_, _ string) error {
	return nil
}

func TestValidateOutput_treatsGocsvScalarInterfacesAsLeaves(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeFor[valueCSVMarshaler](),
		reflect.TypeFor[pointerTextMarshaler](),
		reflect.TypeFor[valueCSVUnmarshaller](),
		reflect.TypeFor[pointerFieldsUnmarshaller](),
	}

	for _, typ := range types {
		require.NoError(t, ValidateOutput(typ))
	}
}

func TestValidateOutput_handlesNonStructAndSupportedNesting(t *testing.T) {
	type child struct {
		Name string
	}
	type rows []child
	type row struct {
		Child  child
		Rows   rows
		hidden *pointerCycle
	}

	require.NoError(t, ValidateOutput(reflect.TypeFor[int]()))
	require.NoError(t, ValidateOutput(reflect.TypeFor[row]()))
	require.NotNil(t, row{hidden: &pointerCycle{}}.hidden)
}

var (
	_ gocsv.TypeMarshaller             = valueCSVMarshaler{}
	_ encoding.TextMarshaler           = (*pointerTextMarshaler)(nil)
	_ gocsv.TypeUnmarshaller           = valueCSVUnmarshaller{}
	_ gocsv.TypeUnmarshalCSVWithFields = (*pointerFieldsUnmarshaller)(nil)
)
